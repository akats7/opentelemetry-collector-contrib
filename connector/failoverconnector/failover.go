// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"
)

type consumerProvider[C any] func(...component.ID) (C, error)

type failoverRouter[C any] struct {
	consumerProvider consumerProvider[C]
	cfg              *Config
	pS               *state.PipelineSelector
	consumers        []C
}

var (
	errNoValidPipeline = errors.New("All provided pipelines return errors")
	errConsumer        = errors.New("Error registering consumer")
)

func newFailoverRouter[C any](provider consumerProvider[C], cfg *Config, done chan struct{}) *failoverRouter[C] {
	pSConstants := state.PSConstants{
		RetryInterval: cfg.RetryInterval,
		RetryGap:      cfg.RetryGap,
		MaxRetries:    cfg.MaxRetries,
	}

	selector := state.NewPipelineSelector(len(cfg.PipelinePriority), pSConstants, done)
	selector.Start(done)
	return &failoverRouter[C]{
		consumerProvider: provider,
		cfg:              cfg,
		pS:               selector,
	}
}

func (f *failoverRouter[C]) getCurrentConsumer() (C, chan bool, bool) {
	var nilConsumer C
	pl, ch := f.pS.SelectedPipeline()
	if pl >= len(f.cfg.PipelinePriority) {
		return nilConsumer, nil, false
	}
	return f.consumers[pl], ch, true
}

func (f *failoverRouter[C]) registerConsumers() error {
	consumers := make([]C, 0)
	for _, pipelines := range f.cfg.PipelinePriority {
		newConsumer, err := f.consumerProvider(pipelines...)
		if err != nil {
			return errConsumer
		}
		consumers = append(consumers, newConsumer)
	}
	f.consumers = consumers
	return nil
}

func (f *failoverRouter[C]) Shutdown() {
	f.pS.RS.InvokeCancel()
}

// For Testing
func (f *failoverRouter[C]) GetConsumerAtIndex(idx int) C {
	return f.consumers[idx]
}

func (f *failoverRouter[C]) ModifyConsumerAtIndex(idx int, c C) {
	f.consumers[idx] = c
}
