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
	done             chan bool // Need to init channel
	index            int
	nextIndex        int
	stableIndex      int
	inRetry          bool
}

var errNoValidPipeline = errors.New("All provided pipelines return errors")

func newFailoverRouter[C any](provider consumerProvider[C], cfg *Config) *failoverRouter[C] {
	// TODO build consumers list from pipelines
	return &failoverRouter[C]{
		consumerProvider: provider,
		pipelines:        cfg.ExporterPriority,
		retryInterval:    cfg.RetryInterval,
		retryGap:         cfg.retryGap,
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

// GET LOOPING WORKING PROPERLY
func (f *failoverRouter[C]) handlePipelineError() {

	// May need to move check of index == stable to know whether to kill existing retry
	if f.inRetry && (f.index != f.stableIndex) {
		f.updatePipelineIndex()
	} else {
		if f.inRetry {
			f.done <- true
		}
		// Figure out way to kill existing retry goroutine when pipeline fails again
		time.AfterFunc(f.retryInterval-f.retryGap, f.retryHighPriorityPipelines)
		f.nextPipeline()
	}

}

func (f *failoverRouter[C]) nextPipeline() {

	// WILL NEED TO ASIGN INDEX TO NEXTINDEX AND FIGURE OUT VALUE OF NEXTINDEX
	f.indexLock.Lock()

	f.index++
	f.stableIndex = f.index

	f.indexLock.Unlock()
}

func (f *failoverRouter[C]) pipelineIsValid() bool {
	return f.index < len(f.pipelines)
}

func (f *failoverRouter[C]) retryHighPriorityPipelines() {

	ticker := time.NewTicker(f.retryGap)
	defer ticker.Stop()

	f.inRetry = true
	f.resetTmpIndex()
	// Schedule swap every retry gap
	for i := 0; i < (f.stableIndex*2 - 1); i++ {
		select {
		case <-f.done:
			return
		case <-ticker.C:
			f.updatePipelineIndex()
		}
	}
	f.inRetry = false
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
	/*
		   index = stableindex then index should be the nextIndex from the retryPipeline
		   nextIndex becomes stableIndex

		   index != stableIndex then set tmpIndex = index, set index = stableIndex, nextIndex = tmpIndex + 1

		timer := time.AfterFunc(f.retryInterval, f.retryHighPriorityPipelines)
		f.nextPipeline()
		return
	*/
}

func (f *failoverRouter[C]) resetTmpIndex() {

	f.nextIndex = 0
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
