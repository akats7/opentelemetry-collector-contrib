// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "failoverconnector"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

const (
	Type             = "failover"
	TracesStability  = component.StabilityLevelDevelopment
	MetricsStability = component.StabilityLevelDevelopment
	LogsStability    = component.StabilityLevelDevelopment
)

func NewFactory() connector.Factory {
	return connector.NewFactory(
		Type,
		createDefaultConfig,
		connector.WithTracesToTraces(createTracesToTraces, TracesStability),
		connector.WithMetricsToMetrics(createMetricsToMetrics, MetricsStability),
		connector.WithLogsToLogs(createLogsToLogs, LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		RetryGap:      30 * time.Second,
		RetryInterval: 10 * time.Minute,
		MaxRetries:    10,
	}
}

func createTracesToTraces(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	traces consumer.Traces,
) (connector.Traces, error) {
	return newTracesToTraces(set, cfg, traces)
}

func createMetricsToMetrics(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	metrics consumer.Metrics,
) (connector.Metrics, error) {
	return newMetricsToMetrics(set, cfg, metrics)
}

func createLogsToLogs(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	logs consumer.Logs,
) (connector.Logs, error) {
	return newLogsToLogs(set, cfg, logs)
}
