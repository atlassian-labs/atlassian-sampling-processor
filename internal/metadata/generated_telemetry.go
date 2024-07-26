// Code generated by mdatagen. DO NOT EDIT.

package metadata

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

func Meter(settings component.TelemetrySettings) metric.Meter {
	return settings.MeterProvider.Meter("bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor")
}

func Tracer(settings component.TelemetrySettings) trace.Tracer {
	return settings.TracerProvider.Tracer("bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor")
}

// TelemetryBuilder provides an interface for components to report telemetry
// as defined in metadata and user config.
type TelemetryBuilder struct {
	meter                                          metric.Meter
	ProcessorAtlassianSamplingCacheReads           metric.Int64Counter
	ProcessorAtlassianSamplingChanBlockingTime     metric.Int64Histogram
	ProcessorAtlassianSamplingDecisionEvictionTime metric.Float64Gauge
	ProcessorAtlassianSamplingPolicyDecisions      metric.Int64Counter
	ProcessorAtlassianSamplingTraceEvictionTime    metric.Float64Gauge
	ProcessorAtlassianSamplingTracesNotSampled     metric.Int64Counter
	ProcessorAtlassianSamplingTracesSampled        metric.Int64Counter
	level                                          configtelemetry.Level
}

// telemetryBuilderOption applies changes to default builder.
type telemetryBuilderOption func(*TelemetryBuilder)

// WithLevel sets the current telemetry level for the component.
func WithLevel(lvl configtelemetry.Level) telemetryBuilderOption {
	return func(builder *TelemetryBuilder) {
		builder.level = lvl
	}
}

// NewTelemetryBuilder provides a struct with methods to update all internal telemetry
// for a component
func NewTelemetryBuilder(settings component.TelemetrySettings, options ...telemetryBuilderOption) (*TelemetryBuilder, error) {
	builder := TelemetryBuilder{level: configtelemetry.LevelBasic}
	for _, op := range options {
		op(&builder)
	}
	var err, errs error
	if builder.level >= configtelemetry.LevelBasic {
		builder.meter = Meter(settings)
	} else {
		builder.meter = noop.Meter{}
	}
	builder.ProcessorAtlassianSamplingCacheReads, err = builder.meter.Int64Counter(
		"processor_atlassian_sampling_cache_reads",
		metric.WithDescription("Amount of times a cache was read from"),
		metric.WithUnit("{accesses}"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingChanBlockingTime, err = builder.meter.Int64Histogram(
		"processor_atlassian_sampling_chan_blocking_time",
		metric.WithDescription("Amount of time spent blocking on the chan send in ConsumeTraces()"),
		metric.WithUnit("ns"), metric.WithExplicitBucketBoundaries([]float64{5000, 10000, 50000, 100000, 500000, 1e+06, 5e+06, 1e+07}...),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingDecisionEvictionTime, err = builder.meter.Float64Gauge(
		"processor_atlassian_sampling_decision_eviction_time",
		metric.WithDescription("Time that a trace ID spent in the decision cache before it was evicted"),
		metric.WithUnit("s"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingPolicyDecisions, err = builder.meter.Int64Counter(
		"processor_atlassian_sampling_policy_decisions",
		metric.WithDescription("Sampling decisions made specifying policy and decision."),
		metric.WithUnit("{decisions}"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingTraceEvictionTime, err = builder.meter.Float64Gauge(
		"processor_atlassian_sampling_trace_eviction_time",
		metric.WithDescription("Time that a non-sampled trace was kept in memory from arrival to being evicted"),
		metric.WithUnit("s"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingTracesNotSampled, err = builder.meter.Int64Counter(
		"processor_atlassian_sampling_traces_not_sampled",
		metric.WithDescription("Number of traces dropped and not sampled"),
		metric.WithUnit("{traces}"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingTracesSampled, err = builder.meter.Int64Counter(
		"processor_atlassian_sampling_traces_sampled",
		metric.WithDescription("Number of traces sampled"),
		metric.WithUnit("{traces}"),
	)
	errs = errors.Join(errs, err)
	return &builder, errs
}
