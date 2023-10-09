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
	pipelines        []component.ID
	retryInterval    time.Duration
	retryGap         time.Duration
	consumers        []C
	indexLock        sync.Mutex
	pipelineRetries  map[int]int
	done             chan bool // Need to init channel
	maxRetry         int
	index            int
	nextIndex        int
	stableIndex      int
	inRetry          bool
}

var errNoValidPipeline = errors.New("All provided pipelines return errors")

func newFailoverRouter[C any](provider consumerProvider[C], cfg *Config) *failoverRouter[C] {

	return &failoverRouter[C]{
		consumerProvider: provider,
		pipelines:        cfg.ExporterPriority,
		retryInterval:    cfg.RetryInterval,
		retryGap:         cfg.RetryGap,
		maxRetry:         cfg.MaxRetry,
		index:            0,
		nextIndex:        0,
	}
}

func (f *failoverRouter[C]) getCurrentConsumer() C {
	return f.consumers[f.index]
}

func (f *failoverRouter[C]) registerConsumers() {
	// TODO support fanout consumers
	consumers := make([]C, 0)
	for _, pipeline := range f.pipelines {
		newConsumer, err := f.consumerProvider(pipeline)
		if err == nil {
			consumers = append(consumers, newConsumer)
		} else {
			fmt.Println(err)
		}
	}
	f.consumers = consumers
}

func (f *failoverRouter[C]) handlePipelineError() {

	// May need to move check of index == stable to know whether to kill existing retry
	if f.inRetry && (f.index != f.stableIndex) {
		f.pipelineRetries[f.index]++
		f.updatePipelineIndex()
	} else {
		// kill existing retry goroutine when pipeline fails again
		if f.inRetry {
			f.done <- true
		}
		// Need to adjust to ensure it keeps retrying, possibly use ticker instead
		time.AfterFunc(f.retryInterval-f.retryGap, f.retryHighPriorityPipelines)
		f.nextPipeline()
	}

}

func (f *failoverRouter[C]) nextPipeline() {

	// WILL NEED TO ASSIGN INDEX TO NEXTINDEX AND FIGURE OUT VALUE OF NEXTINDEX and check if max retries used
	f.indexLock.Lock()

	for ok := true; ok; ok = f.pipelineRetries[f.index] < f.maxRetry {
		f.index++
	}
	f.stableIndex = f.index

	f.indexLock.Unlock()
}

func (f *failoverRouter[C]) pipelineIsValid() bool {
	return f.index < len(f.pipelines)
}

/*
If inRetry mode, until a stable pipeline is found should try next pipeline every interval

expected behavior,

interval triggers -> pipeline switches to next retry pipeline,

if retry pipeline returns error -> switch to stable
pipeline, set nextIndex values to next retry pipeline

else if retrypipeline is stable then, end retry ticker and set original pipeline to stable

*/

func (f *failoverRouter[C]) retryHighPriorityPipelines() {

	ticker := time.NewTicker(f.retryGap)
	f.inRetry = true

	defer func() {
		ticker.Stop()
		f.inRetry = false
	}()

	f.resetNextIndex()
	// Schedule swap every retry gap
	for i := 0; i < f.stableIndex; i++ {
		if f.maxRetriesUsed(i) {
			continue
		}
		select {
		case <-f.done:
			return
		case <-ticker.C:
			f.updatePipelineIndex()
		}
	}
}

func (f *failoverRouter[C]) updatePipelineIndex() {

	f.indexLock.Lock()

	tmpIndex := f.index
	f.index = f.nextIndex
	if tmpIndex == f.stableIndex {
		f.nextIndex = f.stableIndex
	} else {
		f.nextIndex = tmpIndex + 1
	}

	f.indexLock.Unlock()
}

func (f *failoverRouter[C]) resetNextIndex() {

	f.nextIndex = 0
}

// Potentially change mechanism to directly change elements in pipelines slice instead of tracking pipelines to skip
func (f *failoverRouter[C]) maxRetriesUsed(index int) bool {

	return f.pipelineRetries[index] < f.maxRetry
}

func (f *failoverRouter[C]) reportStable() {
	if f.index == f.stableIndex {
		return
	}
	f.pipelineRetries[f.index] = 0
	f.done <- true
}

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
