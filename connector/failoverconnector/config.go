package failoverconnector

import (
	"go.opentelemetry.io/collector/component"
	"time"
)

type Config struct {
	ExporterPriority []component.ID `mapstructure:"priority"`
	retryGap         time.Duration  `mapstructure:"min_failover_interval"`
	RetryInterval    time.Duration  `mapstructure:"retry_pipeline_interval"`
}

// In validate need to make sure RetryInterval > num pipelines * retryGap
