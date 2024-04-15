// Code generated by mdatagen. DO NOT EDIT.
//go:build !darwin && !windows

package journaldreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestComponentFactoryType(t *testing.T) {
	require.Equal(t, "journald", NewFactory().Type().String())
}

func TestComponentConfigStruct(t *testing.T) {
	require.NoError(t, componenttest.CheckConfigStruct(NewFactory().CreateDefaultConfig()))
}

func TestComponentLifecycle(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name     string
		createFn func(ctx context.Context, set receiver.CreateSettings, cfg component.Config) (component.Component, error)
	}{

		{
			name: "logs",
			createFn: func(ctx context.Context, set receiver.CreateSettings, cfg component.Config) (component.Component, error) {
				return factory.CreateLogsReceiver(ctx, set, cfg, consumertest.NewNop())
			},
		},
	}

	cm, err := confmaptest.LoadConf("metadata.yaml")
	require.NoError(t, err)
	cfg := factory.CreateDefaultConfig()
	sub, err := cm.Sub("tests::config")
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	for _, test := range tests {
		t.Run(test.name+"-shutdown", func(t *testing.T) {
			c, err := test.createFn(context.Background(), receivertest.NewNopCreateSettings(), cfg)
			require.NoError(t, err)
			err = c.Shutdown(context.Background())
			require.NoError(t, err)
		})
		t.Run(test.name+"-lifecycle", func(t *testing.T) {
			firstRcvr, err := test.createFn(context.Background(), receivertest.NewNopCreateSettings(), cfg)
			require.NoError(t, err)
			host := componenttest.NewNopHost()
			require.NoError(t, err)
			require.NoError(t, firstRcvr.Start(context.Background(), host))
			require.NoError(t, firstRcvr.Shutdown(context.Background()))
			secondRcvr, err := test.createFn(context.Background(), receivertest.NewNopCreateSettings(), cfg)
			require.NoError(t, err)
			require.NoError(t, secondRcvr.Start(context.Background(), host))
			require.NoError(t, secondRcvr.Shutdown(context.Background()))
		})
	}
}
