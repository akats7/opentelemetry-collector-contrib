// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"
import (
	"context"
	"errors"
	"go.opentelemetry.io/collector/pdata/plog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

var errLogsConsumer = errors.New("Error from ConsumeLogs")

func TestLogsRegisterConsumers(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.LogsSink
	logsFirst := component.NewIDWithName(component.DataTypeLogs, "logs/first")
	logsSecond := component.NewIDWithName(component.DataTypeLogs, "logs/second")
	logsThird := component.NewIDWithName(component.DataTypeLogs, "logs/third")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{logsFirst}, {logsSecond}, {logsThird}},
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetries:       5,
	}

	router := connector.NewLogsRouter(map[component.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateLogsToLogs(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Logs))

	failoverConnector := conn.(*logsFailover)
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	require.NoError(t, err)
	require.NotNil(t, conn)

	lc, _, ok := failoverConnector.failover.getCurrentConsumer()
	lc1 := failoverConnector.failover.GetConsumerAtIndex(1)
	lc2 := failoverConnector.failover.GetConsumerAtIndex(2)

	assert.True(t, ok)
	require.Implements(t, (*consumer.Logs)(nil), lc)
	require.Implements(t, (*consumer.Logs)(nil), lc1)
	require.Implements(t, (*consumer.Logs)(nil), lc2)
}

func TestLogsWithValidFailover(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.LogsSink
	logsFirst := component.NewIDWithName(component.DataTypeLogs, "logs/first")
	logsSecond := component.NewIDWithName(component.DataTypeLogs, "logs/second")
	logsThird := component.NewIDWithName(component.DataTypeLogs, "logs/third")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{logsFirst}, {logsSecond}, {logsThird}},
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetries:       5,
	}

	router := connector.NewLogsRouter(map[component.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateLogsToLogs(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Logs))

	require.NoError(t, err)

	failoverConnector := conn.(*logsFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	ld := sampleLog()

	require.NoError(t, conn.ConsumeLogs(context.Background(), ld))
	_, ch, ok := failoverConnector.failover.getCurrentConsumer()
	idx := failoverConnector.failover.pS.ChannelIndex(ch)
	assert.True(t, ok)
	require.Equal(t, idx, 1)
}

func TestLogsWithFailoverError(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird consumertest.LogsSink
	logsFirst := component.NewIDWithName(component.DataTypeLogs, "logs/first")
	logsSecond := component.NewIDWithName(component.DataTypeLogs, "logs/second")
	logsThird := component.NewIDWithName(component.DataTypeLogs, "logs/third")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{logsFirst}, {logsSecond}, {logsThird}},
		RetryInterval:    5 * time.Minute,
		RetryGap:         10 * time.Second,
		MaxRetries:       5,
	}

	router := connector.NewLogsRouter(map[component.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateLogsToLogs(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Logs))

	require.NoError(t, err)

	failoverConnector := conn.(*logsFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errLogsConsumer))
	failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errLogsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	ld := sampleLog()

	assert.EqualError(t, conn.ConsumeLogs(context.Background(), ld), "All provided pipelines return errors")
}

func TestLogsWithFailoverRecovery(t *testing.T) {
	t.Skip("Flaky Test - See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/31005")
	var sinkFirst, sinkSecond, sinkThird consumertest.LogsSink
	logsFirst := component.NewIDWithName(component.DataTypeLogs, "logs/first")
	logsSecond := component.NewIDWithName(component.DataTypeLogs, "logs/second")
	logsThird := component.NewIDWithName(component.DataTypeLogs, "logs/third")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{logsFirst}, {logsSecond}, {logsThird}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       1000,
	}

	router := connector.NewLogsRouter(map[component.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
	})

	conn, err := NewFactory().CreateLogsToLogs(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Logs))

	require.NoError(t, err)

	failoverConnector := conn.(*logsFailover)
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	ld := sampleLog()

	require.NoError(t, conn.ConsumeLogs(context.Background(), ld))
	_, ch, ok := failoverConnector.failover.getCurrentConsumer()
	idx := failoverConnector.failover.pS.ChannelIndex(ch)

	assert.True(t, ok)
	require.Equal(t, idx, 1)

	// Simulate recovery of exporter
	failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewNop())

	require.Eventually(t, func() bool {
		_, ch, ok = failoverConnector.failover.getCurrentConsumer()
		idx = failoverConnector.failover.pS.ChannelIndex(ch)
		return ok && idx == 0
	}, 3*time.Second, 10*time.Millisecond)
}

func TestLogsWithRecovery(t *testing.T) {
	var sinkFirst, sinkSecond, sinkThird, sinkFourth consumertest.LogsSink
	logsFirst := component.NewIDWithName(component.DataTypeLogs, "logs/first")
	logsSecond := component.NewIDWithName(component.DataTypeLogs, "logs/second")
	logsThird := component.NewIDWithName(component.DataTypeLogs, "logs/third")
	logsFourth := component.NewIDWithName(component.DataTypeLogs, "logs/fourth")

	cfg := &Config{
		PipelinePriority: [][]component.ID{{logsFirst}, {logsSecond}, {logsThird}, {logsFourth}},
		RetryInterval:    50 * time.Millisecond,
		RetryGap:         10 * time.Millisecond,
		MaxRetries:       10000,
	}

	router := connector.NewLogsRouter(map[component.ID]consumer.Logs{
		logsFirst:  &sinkFirst,
		logsSecond: &sinkSecond,
		logsThird:  &sinkThird,
		logsFourth: &sinkFourth,
	})

	conn, err := NewFactory().CreateLogsToLogs(context.Background(),
		connectortest.NewNopCreateSettings(), cfg, router.(consumer.Logs))

	require.NoError(t, err)

	failoverConnector := conn.(*logsFailover)

	lr := sampleLog()

	defer func() {
		assert.NoError(t, failoverConnector.Shutdown(context.Background()))
	}()

	t.Run("single failover recovery to primary consumer: level 2 -> 1", func(t *testing.T) {
		defer func() {
			resetLogsConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))

		require.NoError(t, conn.ConsumeLogs(context.Background(), lr))
		idx := failoverConnector.failover.pS.TestStableIndex()
		require.Equal(t, idx, 1)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 0, lr)
		}, 3*time.Second, 10*time.Millisecond)
	})

	t.Run("double failover recovery: level 3 -> 2 -> 1", func(t *testing.T) {
		defer func() {
			resetLogsConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errLogsConsumer))

		require.NoError(t, conn.ConsumeLogs(context.Background(), lr))
		idx := failoverConnector.failover.pS.TestStableIndex()
		require.Equal(t, idx, 2)

		// Simulate recovery of exporter
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 1, lr)
		}, 3*time.Second, 10*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkFirst)

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 0, lr)
		}, 3*time.Second, 10*time.Millisecond)
	})

	t.Run("multiple failover recovery: level 3 -> 2 -> 4 -> 3 -> 1", func(t *testing.T) {
		defer func() {
			resetLogsConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errLogsConsumer))

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 2, lr)
		}, 3*time.Second, 10*time.Millisecond)

		// Simulate recovery of exporter
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 1, lr)
		}, 3*time.Second, 10*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(2, consumertest.NewErr(errLogsConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errLogsConsumer))

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 3, lr)
		}, 3*time.Second, 10*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(2, &sinkThird)

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 2, lr)
		}, 3*time.Second, 10*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkThird)

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 0, lr)
		}, 3*time.Second, 10*time.Millisecond)
	})

	t.Run("failover with max retries exceeded: level 3 -> 1 -> 3 -> 1(Skipped due to max retries) -> 2", func(t *testing.T) {
		defer func() {
			resetLogsConsumers(failoverConnector, &sinkFirst, &sinkSecond, &sinkThird, &sinkFourth)
		}()
		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errLogsConsumer))

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 2, lr)
		}, 3*time.Second, 10*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkSecond)
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 0, lr)
		}, 3*time.Second, 10*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, consumertest.NewErr(errLogsConsumer))
		failoverConnector.failover.ModifyConsumerAtIndex(1, consumertest.NewErr(errLogsConsumer))
		failoverConnector.failover.pS.SetRetryCountToMax(0)

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 2, lr)
		}, 3*time.Second, 10*time.Millisecond)

		failoverConnector.failover.ModifyConsumerAtIndex(0, &sinkSecond)
		failoverConnector.failover.ModifyConsumerAtIndex(1, &sinkSecond)

		require.Eventually(t, func() bool {
			return ConsumeLogsAndCheckStable(failoverConnector, 1, lr)
		}, 3*time.Second, 10*time.Millisecond)

	})
}

func ConsumeLogsAndCheckStable(conn *logsFailover, idx int, lr plog.Logs) bool {
	conn.ConsumeLogs(context.Background(), lr)
	stableIndex := conn.failover.pS.TestStableIndex()
	return stableIndex == idx
}

func resetLogsConsumers(conn *logsFailover, consumers ...consumer.Logs) {
	for i, sink := range consumers {

		conn.failover.ModifyConsumerAtIndex(i, sink)
	}
	conn.failover.pS.TestSetStableIndex(0)
}

func sampleLog() plog.Logs {
	l := plog.NewLogs()
	rl := l.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("test", "logs-test")
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	return l
}
