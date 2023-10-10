package failoverconnector

import (
	"go.opentelemetry.io/collector/component"
	"time"
)

type Config struct {
	ExporterPriority [][]component.ID `mapstructure:"priority"`
	RetryInterval    time.Duration    `mapstructure:"retry_pipeline_interval"`
	RetryGap         time.Duration    `mapstructure:"retry_gap"`
	MaxRetry         int              `mapstructure:"max_retry"`
}

// In validate need to make sure RetryInterval > num pipelines * retryGap
