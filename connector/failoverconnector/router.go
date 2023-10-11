package failoverconnector

import (
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/component"
	"sync"
	"time"
)

type consumerProvider[C any] func(...component.ID) (C, error)

type failoverRouter[C any] struct {
	consumerProvider consumerProvider[C]
	cfg              *Config
	pS               *pipelineSelector
	consumers        []C
	done             chan bool
	inRetry          bool
}

var errNoValidPipeline = errors.New("All provided pipelines return errors")

func newFailoverRouter[C any](provider consumerProvider[C], cfg *Config) *failoverRouter[C] {
	return &failoverRouter[C]{
		consumerProvider: provider,
		cfg:              cfg,
		pS:               newPipelineSelector(cfg),
	}
}

func (f *failoverRouter[C]) getCurrentConsumer() C {
	return f.consumers[f.pS.currentIndex]
}

func (f *failoverRouter[C]) registerConsumers() {
	consumers := make([]C, 0)
	for _, pipeline := range f.cfg.ExporterPriority {
		newConsumer, err := f.consumerProvider(pipeline...)
		if err == nil {
			consumers = append(consumers, newConsumer)
		} else {
			fmt.Println(err)
		}
	}
	f.consumers = consumers
}

func (f *failoverRouter[C]) handlePipelineError() {
	if f.pS.currentIndexStable() {
		f.pS.nextPipeline()
		f.enableRetry()
	} else {
		f.pS.setStableIndex()
	}
}

func (f *failoverRouter[C]) enableRetry() {
	if f.inRetry {
		f.done <- true
	}
	f.inRetry = true
	ticker := time.NewTicker(f.cfg.RetryInterval)
	ch := make(chan bool)

	go func() {
		for {
			select {
			case <-ticker.C:
				f.retryHighPriorityPipelines(f.pS.stableIndex, ch)
			case <-f.done:
				ch <- true
				f.inRetry = false
				return
			}
		}
	}()
}

func (f *failoverRouter[C]) pipelineIsValid() bool {
	fmt.Printf("pipeline is valid: %v \n", f.pS.currentIndex < len(f.cfg.ExporterPriority))
	return f.pS.currentIndex < len(f.cfg.ExporterPriority)
}

func (f *failoverRouter[C]) retryHighPriorityPipelines(stableIndex int, ch chan bool) {
	ticker := time.NewTicker(f.cfg.RetryGap)
	f.inRetry = true

	defer func() {
		ticker.Stop()
		f.inRetry = false
	}()

	for i := 0; i < stableIndex; i++ {
		if f.pS.maxRetriesUsed(i) {
			continue
		}
		select {
		case <-ch:
			return
		case <-ticker.C:
			f.pS.setToRetryIndex(i)
		}
	}
}

func (f *failoverRouter[C]) reportStable() {
	if f.pS.currentIndexStable() {
		return
	}
	f.pS.setStable()
	f.done <- true
}

type pipelineSelector struct {
	currentIndex    int
	stableIndex     int
	lock            sync.Mutex
	pipelineRetries []int
	maxRetry        int
}

func (p *pipelineSelector) nextPipeline() {
	p.lock.Lock()
	for ok := true; ok; ok = p.pipelineRetries[p.currentIndex] >= p.maxRetry {
		p.currentIndex++
	}
	p.stableIndex = p.currentIndex
	p.lock.Unlock()
}

func (p *pipelineSelector) setStableIndex() {
	p.lock.Lock()
	p.pipelineRetries[p.currentIndex]++
	p.currentIndex = p.stableIndex
	p.lock.Unlock()
}

func (p *pipelineSelector) setToRetryIndex(index int) {
	p.lock.Lock()
	p.currentIndex = index
	p.lock.Unlock()
}

// Potentially change mechanism to directly change elements in pipelines slice instead of tracking pipelines to skip
func (p *pipelineSelector) maxRetriesUsed(index int) bool {
	return p.pipelineRetries[index] >= p.maxRetry
}

func (p *pipelineSelector) setStable() {
	p.pipelineRetries[p.currentIndex] = 0
	p.stableIndex = p.currentIndex
}

func (p *pipelineSelector) currentIndexStable() bool {
	return p.currentIndex == p.stableIndex
}

func newPipelineSelector(cfg *Config) *pipelineSelector {
	return &pipelineSelector{
		currentIndex:    0,
		stableIndex:     0,
		lock:            sync.Mutex{},
		pipelineRetries: make([]int, len(cfg.ExporterPriority)),
		maxRetry:        cfg.MaxRetry,
	}
}

//func (f *failoverRouter[C]) setToStableIndex() {
//	f.indexLock.Lock()
//
//	f.nextIndex = f.index + 1
//	f.index = f.stableIndex
//
//	f.indexLock.Unlock()
//
//}

//func (f *failoverRouter[C]) setToRetryIndex(index int) {
//	f.indexLock.Lock()
//
//	f.index = index
//	f.nextIndex = f.stableIndex
//
//	f.indexLock.Unlock()
//
//}

// Potentially change mechanism to directly change elements in pipelines slice instead of tracking pipelines to skip
//func (f *failoverRouter[C]) maxRetriesUsed(index int) bool {
//
//	return f.pipelineRetries[index] < f.cfg.MaxRetry
//}

//func getConsumeSignal(router *failoverRouter) {
//
//}

//func (f *failoverRouter[C]) Consume(ctx context.Context, td ptrace.Traces) error {
//	// First determine signal type
//
//	// should move to consume traces
//	for f.pipelineIsValid() {
//		tc := f.getCurrentConsumer()
//		//if tc, ok := interface{}(c).(consumer.Traces); ok {
//		err := tc.ConsumeTraces(ctx, td)
//		if err != nil {
//			ctx = context.Background()
//			f.handlePipelineError()
//			continue
//		}
//		return nil
//		//}
//	}
//	return fmt.Errorf("%v", errNoValidPipeline)
//}
//
//func (f *failoverRouter[C]) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
//	// LOOK into why needs to converted to interface
//	c := f.getCurrentConsumer()
//	if tc, ok := interface{}(c).(consumer.Traces); ok {
//		err := tc.ConsumeTraces(ctx, td)
//		if err != nil {
//			f.handlePipelineError()
//			return err
//		}
//	}
//	return nil
//}

//func (f *failoverRouter[C]) updatePipelineIndex() {
//
//	f.indexLock.Lock()
//
//	tmpIndex := f.index
//	f.index = f.nextIndex
//	if tmpIndex == f.stableIndex {
//		f.nextIndex = f.stableIndex
//	} else {
//		f.nextIndex = tmpIndex + 1
//	}
//
//	f.indexLock.Unlock()
//}

//func (f *failoverRouter[C]) handlePipelineError() {
//
//	// May need to move check of index == stable to know whether to kill existing retry
//	if f.inRetry && (f.index != f.stableIndex) {
//		f.pipelineRetries[f.index]++
//		f.updatePipelineIndex()
//	} else {
//		// kill existing retry goroutine when pipeline fails again
//		if f.inRetry {
//			f.done <- true
//		}
//		// Need to adjust to ensure it keeps retrying, possibly use ticker instead
//		time.AfterFunc(f.retryInterval-f.retryGap, f.retryHighPriorityPipelines)
//		f.nextPipeline()
//	}
//
//}

//func (f *failoverRouter[C]) nextPipeline() {
//
//	f.indexLock.Lock()
//
//	for ok := true; ok; ok = f.pipelineRetries[f.index] < f.cfg.MaxRetry {
//		f.index++
//	}
//	f.stableIndex = f.index
//
//	f.indexLock.Unlock()
//}
