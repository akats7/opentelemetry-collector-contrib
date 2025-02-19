// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal"
)

type tracesFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *failoverRouter[consumer.Traces]
	logger   *zap.Logger
}

func (f *tracesFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces will try to export to the current set priority level and handle failover in the case of an error
func (f *tracesFailover) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	select {
	case <-f.failover.notifyRetry:
		fmt.Println("in traces")
		ok := f.failover.sampleRetryConsumers(ctx, td)
		if !ok {
			return f.consumeByHealthyPipeline(ctx, td)
		} else {
			return nil
		}
	default:
		return f.consumeByHealthyPipeline(ctx, td)
	}
}

func (f *tracesFailover) consumeByHealthyPipeline(ctx context.Context, td ptrace.Traces) error {
	for {
		tc, idx := f.failover.getCurrentConsumer()
		if idx >= len(f.config.PipelinePriority) {
			return errNoValidPipeline
		}
		err := tc.Consume(ctx, td)
		if err != nil {
			f.failover.reportConsumerError(idx)
		} else {
			return nil
		}
	}
}

func (f *tracesFailover) Shutdown(_ context.Context) error {
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

	failover := newFailoverRouter[consumer.Traces](tr.Consumer, config)
	err := failover.registerConsumers(wrapTraces)
	if err != nil {
		return nil, err
	}

	return &tracesFailover{
		config:   config,
		failover: failover,
		logger:   set.TelemetrySettings.Logger,
	}, nil
}

func wrapTraces(c consumer.Traces) internal.SignalConsumer {
	return internal.NewTracesWrapper(c)
}
