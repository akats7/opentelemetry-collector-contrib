// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type tracesRouter struct {
	*baseFailoverRouter[consumer.Traces]
}

func newTracesRouter(provider consumerProvider[consumer.Traces], cfg *Config) (*tracesRouter, error) {
	failover, err := newBaseFailoverRouter(provider, cfg)
	if err != nil {
		return nil, err
	}
	return &tracesRouter{baseFailoverRouter: failover}, nil
}

// Consume is the traces-specific consumption method
func (f *tracesRouter) Consume(ctx context.Context, td ptrace.Traces) error {
	// If we're currently in failover (pipeline index > 0), opportunistically
	// attempt recovery on every call. This removes timing dependence on retry signals.
	if f.pS.CurrentPipeline() > 0 {
		if f.sampleRetryConsumers(ctx, td) {
			return nil
		}
	} else {
		// Even when healthy (index 0), we may have a pending retry token; drain it.
		select {
		case <-f.notifyRetry:
		default:
		}
	}

	// Best-effort: if a retry notification is present now, try sampling again.
	select {
	case <-f.notifyRetry:
		if f.sampleRetryConsumers(ctx, td) {
			return nil
		}
	default:
	}

	return f.consumeByHealthyPipeline(ctx, td)
}

// consumeByHealthyPipeline will consume the traces by the current healthy level
func (f *tracesRouter) consumeByHealthyPipeline(ctx context.Context, td ptrace.Traces) error {
	for {
		tc, idx := f.getCurrentConsumer()
		if idx >= len(f.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		if err := tc.ConsumeTraces(ctx, td); err != nil {
			f.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

// sampleRetryConsumers iterates through all unhealthy consumers to re-establish a healthy connection
func (f *tracesRouter) sampleRetryConsumers(ctx context.Context, td ptrace.Traces) bool {
	stableIndex := f.pS.CurrentPipeline()
	for i := 0; i < stableIndex; i++ {
		consumer := f.getConsumerAtIndex(i)
		err := consumer.ConsumeTraces(ctx, td)
		if err == nil {
			f.pS.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

type tracesFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *tracesRouter
	logger   *zap.Logger
}

func (*tracesFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces will try to export to the current set priority level and handle failover in the case of an error
func (f *tracesFailover) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return f.failover.Consume(ctx, td)
}

func (f *tracesFailover) Shutdown(context.Context) error {
	if f.failover != nil {
		f.failover.Shutdown()
	}
	return nil
}

func newTracesToTraces(set connector.Settings, cfg component.Config, traces consumer.Traces) (connector.Traces, error) {
	config := cfg.(*Config)
	tr, ok := traces.(connector.TracesRouterAndConsumer)
	if !ok {
		return nil, errors.New("consumer is not of type TracesRouter")
	}

	failover, err := newTracesRouter(tr.Consumer, config)
	if err != nil {
		return nil, err
	}

	return &tracesFailover{
		config:   config,
		failover: failover,
		logger:   set.Logger,
	}, nil
}
