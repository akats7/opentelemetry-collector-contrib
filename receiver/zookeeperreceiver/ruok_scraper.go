package zookeeperreceiver

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"net"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver/internal/metadata"
)

type ruokMetricsScraper struct {
	logger *zap.Logger
	config *Config
	cancel context.CancelFunc
	mb     *metadata.MetricsBuilder

	// For mocking.
	closeConnection       func(net.Conn) error
	setConnectionDeadline func(net.Conn, time.Time) error
	sendCmd               func(net.Conn, string) (*bufio.Scanner, error)
}

func (z *ruokMetricsScraper) Name() string {
	return typeStr
}

func newruokMetricsScraper(settings receiver.CreateSettings, config *Config) (*ruokMetricsScraper, error) {
	_, _, err := net.SplitHostPort(config.TCPAddr.Endpoint)
	if err != nil {
		return nil, err
	}

	if config.Timeout <= 0 {
		return nil, errors.New("timeout must be a positive duration")
	}

	z := &ruokMetricsScraper{
		logger:                settings.Logger,
		config:                config,
		mb:                    metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		closeConnection:       closeConnection,
		setConnectionDeadline: setConnectionDeadline,
		sendCmd:               sendCmd,
	}

	return z, nil
}

func (z *ruokMetricsScraper) shutdown(_ context.Context) error {
	if z.cancel != nil {
		z.cancel()
		z.cancel = nil
	}
	return nil
}

func (z *ruokMetricsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var ctxWithTimeout context.Context
	ctxWithTimeout, z.cancel = context.WithTimeout(ctx, z.config.Timeout)

	conn, err := z.config.Dial()
	if err != nil {
		z.logger.Error("failed to establish connection",
			zap.String("endpoint", z.config.Endpoint),
			zap.Error(err),
		)
		return pmetric.NewMetrics(), err
	}
	defer func() {
		if closeErr := z.closeConnection(conn); closeErr != nil {
			z.logger.Warn("failed to shutdown connection", zap.Error(closeErr))
		}
	}()

	deadline, ok := ctxWithTimeout.Deadline()
	if ok {
		if err := z.setConnectionDeadline(conn, deadline); err != nil {
			z.logger.Warn("failed to set deadline on connection", zap.Error(err))
		}
	}
	return z.getResourceMetrics(conn)

}

//func (z *ruokMetricsScraper) getRUOKMetric()

func (z *ruokMetricsScraper) getResourceMetrics(conn net.Conn) (pmetric.Metrics, error) {
	startTime := pcommon.NewTimestampFromTime(time.Now())
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	ilMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	otelMetrics := ilMetrics.Metrics()
	metric := otelMetrics.AppendEmpty()
	metric.SetName("zookeeper.ruok")
	summary := metric.SetEmptySummary()
	dataPoints := summary.DataPoints()
	dataPoints.EnsureCapacity(1)
	inclusiveDataPoint := dataPoints.AppendEmpty()
	scanner, err := z.sendCmd(conn, ruokCommand)
	if err != nil {
		z.logger.Error("failed to send command",
			zap.Error(err),
			zap.String("command", ruokCommand),
		)
		return metrics, err
	}

	for scanner.Scan() {
		line := scanner.Text()
		parts := regexp.MustCompile("^\\S*").FindStringSubmatch(line)
		fmt.Println(parts)
		fmt.Println(len(parts))
		if len(parts) != 1 {
			z.logger.Warn("unexpected line in response",
				zap.String("command", ruokCommand),
				zap.String("line", line),
			)
			continue
		}
		if parts[0] == "imok" {
			fmt.Println("Helloo")
			inclusiveDataPoint.Attributes().PutStr("Response", parts[0])
		} else {
			inclusiveDataPoint.Attributes().PutStr("Response", "")
		}
		endTime := pcommon.NewTimestampFromTime(time.Now())
		inclusiveDataPoint.SetStartTimestamp(startTime)
		inclusiveDataPoint.SetTimestamp(endTime)

	}

	// Generate computed metrics
	//creator.generateComputedMetrics(z.logger, now)
	//fmt.Println(scanner2.Scan())
	//return z.mb.Emit(resourceOpts...), nil
	return metrics, nil
}

//func closeConnection(conn net.Conn) error {
//	return conn.Close()
//}
//
//func setConnectionDeadline(conn net.Conn, deadline time.Time) error {
//	return conn.SetDeadline(deadline)
//}
//
//func sendCmd(conn net.Conn, cmd string) (*bufio.Scanner, error) {
//	_, err := fmt.Fprintf(conn, "%s\n", cmd)
//	if err != nil {
//		return nil, err
//	}
//	reader := bufio.NewReader(conn)
//	scanner := bufio.NewScanner(reader)
//	return scanner, nil
//}
