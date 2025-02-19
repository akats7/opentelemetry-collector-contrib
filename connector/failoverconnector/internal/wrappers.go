// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"

import (
	"context"
	"errors"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var errConsumeType = errors.New("Error in consume type assertion")

var _ SignalConsumer = (*TracesWrapper)(nil)
var _ SignalConsumer = (*MetricsWrapper)(nil)
var _ SignalConsumer = (*LogsWrapper)(nil)

type SignalConsumer interface {
	Consume(context.Context, any) error
}

func NewTracesWrapper(C consumer.Traces) *TracesWrapper {
	return &TracesWrapper{C}
}

type TracesWrapper struct {
	consumer.Traces
}

func (t *TracesWrapper) Consume(ctx context.Context, pd any) error {
	td, ok := pd.(ptrace.Traces)
	if !ok {
		return errConsumeType
	}
	return t.ConsumeTraces(ctx, td)
}

func NewMetricsWrapper(C consumer.Metrics) *MetricsWrapper {
	return &MetricsWrapper{C}
}

type MetricsWrapper struct {
	consumer.Metrics
}

func (t *MetricsWrapper) Consume(ctx context.Context, pd any) error {
	td, ok := pd.(pmetric.Metrics)
	if !ok {
		return errConsumeType
	}
	return t.ConsumeMetrics(ctx, td)
}

func NewLogsWrapper(C consumer.Logs) *LogsWrapper {
	return &LogsWrapper{C}
}

type LogsWrapper struct {
	consumer.Logs
}

func (t *LogsWrapper) Consume(ctx context.Context, pd any) error {
	td, ok := pd.(plog.Logs)
	if !ok {
		return errConsumeType
	}
	return t.ConsumeLogs(ctx, td)
}
