package failoverconnector

import (
	"context"
	"errors"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricsFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *failoverRouter[consumer.Metrics]
	logger   *zap.Logger
}

func (f *metricsFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeMetrics will try to export to the current set priority level and handle failover in the case of an error
func (f *metricsFailover) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return nil
}

func (f *metricsFailover) Shutdown(ctx context.Context) error {
	return nil
}

func newMetricsToMetrics(set connector.CreateSettings, cfg component.Config, metrics consumer.Metrics) (connector.Metrics, error) {
	config := cfg.(*Config)
	mr, ok := metrics.(connector.MetricsRouter)
	if !ok {
		return nil, errors.New("consumer is not of type MetricsRouter")
	}

	failover := newFailoverRouter(mr.Consumer, config)
	return &metricsFailover{
		config:   config,
		failover: failover,
		logger:   set.TelemetrySettings.Logger,
	}, nil
}
