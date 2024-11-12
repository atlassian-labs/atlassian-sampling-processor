// Code generated by mdatagen. DO NOT EDIT.

package atlassiansamplingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

type componentTestTelemetry struct {
	reader        *sdkmetric.ManualReader
	meterProvider *sdkmetric.MeterProvider
}

func (tt *componentTestTelemetry) NewSettings() processor.Settings {
	set := processortest.NewNopSettings()
	set.TelemetrySettings = tt.newTelemetrySettings()
	set.ID = component.NewID(component.MustNewType("atlassian_sampling"))
	return set
}

func (tt *componentTestTelemetry) newTelemetrySettings() component.TelemetrySettings {
	set := componenttest.NewNopTelemetrySettings()
	set.MeterProvider = tt.meterProvider
	set.LeveledMeterProvider = func(_ configtelemetry.Level) metric.MeterProvider {
		return tt.meterProvider
	}
	return set
}

func setupTestTelemetry() componentTestTelemetry {
	reader := sdkmetric.NewManualReader()
	return componentTestTelemetry{
		reader:        reader,
		meterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)),
	}
}

func (tt *componentTestTelemetry) assertMetrics(t *testing.T, expected []metricdata.Metrics) {
	var md metricdata.ResourceMetrics
	require.NoError(t, tt.reader.Collect(context.Background(), &md))
	// ensure all required metrics are present
	for _, want := range expected {
		got := tt.getMetric(want.Name, md)
		metricdatatest.AssertEqual(t, want, got, metricdatatest.IgnoreTimestamp())
	}

	// ensure no additional metrics are emitted
	require.Equal(t, len(expected), tt.len(md))
}

func (tt *componentTestTelemetry) getMetric(name string, got metricdata.ResourceMetrics) metricdata.Metrics {
	for _, sm := range got.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m
			}
		}
	}

	return metricdata.Metrics{}
}

func (tt *componentTestTelemetry) len(got metricdata.ResourceMetrics) int {
	metricsCount := 0
	for _, sm := range got.ScopeMetrics {
		metricsCount += len(sm.Metrics)
	}

	return metricsCount
}

func (tt *componentTestTelemetry) Shutdown(ctx context.Context) error {
	return tt.meterProvider.Shutdown(ctx)
}
