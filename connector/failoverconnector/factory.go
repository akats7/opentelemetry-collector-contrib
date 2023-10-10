package failoverconnector

import (
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

const (
	Type            = "failover"
	TracesStability = component.StabilityLevelDevelopment
	//MetricsStability = component.StabilityLevelDevelopment
	//LogsStability    = component.StabilityLevelDevelopment
)

type tracesFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *failoverRouter[consumer.Traces]
	logger   *zap.Logger
}

func NewFactory() connector.Factory {
	return connector.NewFactory(
		Type,
		createDefaultConfig,
		connector.WithTracesToTraces(createTracesToTraces, TracesStability),
		//connector.WithMetricsToMetrics(createMetricsToMetrics, MetricsStability),
		//connector.WithLogsToLogs(createLogsToLogs, LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesToTraces(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	traces consumer.Traces,
) (connector.Traces, error) {
	config := cfg.(*Config)

	tr, ok := traces.(connector.TracesRouter)
	if !ok {
		return nil, errors.New("expected consumer to be a connector router")
	}

	fmt.Printf("consumers  are %v \n")

	//fmt.Printf("Pipeline IDs %v \n", func() []string {
	//	strings := make([]string, 0)
	//	for _, id := range tr.PipelineIDs() {
	//		strings = append(strings, string(id.Name()))
	//	}
	//	return strings
	//}())

	failover := newFailoverRouter(tr.Consumer, config)
	failover.registerConsumers()
	return &tracesFailover{
		config:   config,
		failover: failover,
		logger:   set.TelemetrySettings.Logger,
	}, nil
}

func (f *tracesFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (f *tracesFailover) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	fmt.Printf("index: %v, stableIndex: %v, nextIndex: %v \n", f.failover.index, f.failover.stableIndex, f.failover.nextIndex)
	for f.failover.pipelineIsValid() {
		tc := f.failover.getCurrentConsumer()
		err := tc.ConsumeTraces(ctx, td)
		if err != nil {
			ctx = context.Background()
			f.failover.handlePipelineError()
			continue
		}
		f.failover.reportStable()
		return nil
	}
	return fmt.Errorf("%v", errNoValidPipeline)
}

//	fmt.Println("Calling failover ConsumeTraces")
//	failoverRouter := f.failover
//	//consumer := failoverRouter.getCurrentConsumer()
//	////fmt.Printf("type of consumr %v \n", reflect.TypeOf(consumer))
//	//err := consumer.ConsumeTraces(ctx, td)
//	err := failoverRouter.Consume(ctx, td)
//	fmt.Printf("Default Error returned: %v \n", err)
//	if err != nil {
//		fmt.Printf("Error returned: %v \n", err)
//		return err
//	}
//	return nil
//}

//func createMetricsToMetrics(
//	_ context.Context,
//	set connector.CreateSettings,
//	cfg component.Config,
//	metrics consumer.Metrics,
//) (connector.Metrics, error) {
//	return newMetricsConnector(set, cfg, metrics)
//}
//
//func createLogsToLogs(
//	_ context.Context,
//	set connector.CreateSettings,
//	cfg component.Config,
//	logs consumer.Logs,
//) (connector.Logs, error) {
//	return newLogsConnector(set, cfg, logs)
//}
