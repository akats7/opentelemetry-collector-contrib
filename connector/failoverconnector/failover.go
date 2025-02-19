// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"
	"go.opentelemetry.io/collector/pipeline"
)

type consumerProvider[C any] func(...pipeline.ID) (C, error)

type wrapConsumer[C any] func(consumer C) internal.SignalConsumer

type failoverRouter[C any] struct {
	consumerProvider consumerProvider[C]
	cfg              *Config
	pS               *state.PipelineSelector
	consumers        []internal.SignalConsumer

	errTryLock  *state.TryLock
	notifyRetry chan struct{}
	done        chan struct{}
}

var (
	errNoValidPipeline = errors.New("All provided pipelines return errors")
	errConsumer        = errors.New("Error registering consumer")
)

func newFailoverRouter[C any](provider consumerProvider[C], cfg *Config) *failoverRouter[C] {
	done := make(chan struct{})
	notifyRetry := make(chan struct{}, 1)
	pSConstants := state.PSConstants{
		RetryInterval: cfg.RetryInterval,
		RetryGap:      cfg.RetryGap,
		MaxRetries:    cfg.MaxRetries,
	}

	selector := state.NewPipelineSelector(notifyRetry, done, pSConstants)
	return &failoverRouter[C]{
		consumerProvider: provider,
		cfg:              cfg,
		pS:               selector,
		errTryLock:       state.NewTryLock(),
		done:             done,
		notifyRetry:      notifyRetry,
	}
}

func (f *failoverRouter[C]) getCurrentConsumer() (internal.SignalConsumer, int) {
	var nilConsumer internal.SignalConsumer
	pl := f.pS.CurrentPipeline()
	if pl >= len(f.cfg.PipelinePriority) {
		return nilConsumer, pl
	}
	return f.consumers[pl], pl
}

func (f *failoverRouter[C]) getConsumerAtIndex(idx int) internal.SignalConsumer {
	return f.consumers[idx]
}

func (f *failoverRouter[C]) sampleRetryConsumers(ctx context.Context, pd any) bool {
	stableIndex := f.pS.CurrentPipeline()
	for i := 0; i < stableIndex; i++ {
		consumer := f.getConsumerAtIndex(i)
		err := consumer.Consume(ctx, pd)
		if err == nil {
			f.pS.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

func (f *failoverRouter[C]) reportConsumerError(idx int) {
	f.errTryLock.TryExecute(f.pS.HandleError, idx)
}

func (f *failoverRouter[C]) Shutdown() {
	close(f.done)
}

//func (f *failoverRouter[C]) resetStablePipeline(idx int) {
//	f.pS.SetStableIndex(idx)
//}
//
//func (f *failoverRouter[C]) handlePipelineFailure() {
//	// maybe account for race condition here
//	f.ps.NextStablePipeline()
//	f.ps.TryEnableRetry()
//}

//func (f *failoverRouter[C]) registerConsumers() error {
//	consumers := make([]C, 0)
//	for _, pipelines := range f.cfg.PipelinePriority {
//		newConsumer, err := f.consumerProvider(pipelines...)
//		if err != nil {
//			return errConsumer
//		}
//		consumers = append(consumers, newConsumer)
//	}
//	f.consumers = consumers
//	return nil
//}

func (f *failoverRouter[C]) registerConsumers(wrap wrapConsumer[C]) error {
	consumers := make([]internal.SignalConsumer, 0)
	for _, pipelines := range f.cfg.PipelinePriority {
		baseConsumer, err := f.consumerProvider(pipelines...)
		if err != nil {
			return errConsumer
		}
		newConsumer := wrap(baseConsumer)

		consumers = append(consumers, newConsumer)
	}
	f.consumers = consumers
	return nil
}

// For Testing

func (f *failoverRouter[C]) ModifyConsumerAtIndex(idx int, consumerWrapper wrapConsumer[C], c C) {
	f.consumers[idx] = consumerWrapper(c)
}

func (f *failoverRouter[C]) TestGetCurrentConsumerIndex() int {
	return f.pS.CurrentPipeline()
}

func (f *failoverRouter[C]) TestSetStableConsumerIndex(idx int) {
	f.pS.TestSetCurrentPipeline(idx)
}

func (f *failoverRouter[C]) GetConsumerAtIndex(idx int) internal.SignalConsumer {
	return f.consumers[idx]
}

//func (f *failoverRouter[C]) Shutdown() {
//	f.pS.RS.InvokeCancel()
//
//	close(f.done)
//	f.wg.Wait()
//}
//
//func (f *failoverRouter[C]) currentIndex() int {
//	return f.pS.StableIndex()
//}

// For Testing
//func (f *failoverRouter[C]) GetConsumerAtIndex(idx int) C {
//	return f.consumers[idx]
//}
//
//func (f *failoverRouter[C]) ModifyConsumerAtIndex(idx int, c C) {
//	f.consumers[idx] = c
//}
