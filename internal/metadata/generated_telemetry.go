// Code generated by mdatagen. DO NOT EDIT.

package metadata

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Deprecated: [v0.108.0] use LeveledMeter instead.
func Meter(settings component.TelemetrySettings) metric.Meter {
	return settings.MeterProvider.Meter("bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor")
}

func LeveledMeter(settings component.TelemetrySettings, level configtelemetry.Level) metric.Meter {
	return settings.LeveledMeterProvider(level).Meter("bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor")
}

func Tracer(settings component.TelemetrySettings) trace.Tracer {
	return settings.TracerProvider.Tracer("bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor")
}

// TelemetryBuilder provides an interface for components to report telemetry
// as defined in metadata and user config.
type TelemetryBuilder struct {
	meter                                                        metric.Meter
	ProcessorAtlassianSamplingCacheReads                         metric.Int64Counter
	ProcessorAtlassianSamplingChanBlockingTime                   metric.Int64Histogram
	ProcessorAtlassianSamplingDecisionEvictionTime               metric.Float64Gauge
	ProcessorAtlassianSamplingOverlyEagerLonelyRootSpanDecisions metric.Int64Counter
	ProcessorAtlassianSamplingPolicyDecisions                    metric.Int64Counter
	ProcessorAtlassianSamplingPrimaryCacheSize                   metric.Int64Gauge
	ProcessorAtlassianSamplingTraceEvictionTime                  metric.Float64Gauge
	ProcessorAtlassianSamplingTracesNotSampled                   metric.Int64Counter
	ProcessorAtlassianSamplingTracesSampled                      metric.Int64Counter
	meters                                                       map[configtelemetry.Level]metric.Meter
}

// TelemetryBuilderOption applies changes to default builder.
type TelemetryBuilderOption interface {
	apply(*TelemetryBuilder)
}

type telemetryBuilderOptionFunc func(mb *TelemetryBuilder)

func (tbof telemetryBuilderOptionFunc) apply(mb *TelemetryBuilder) {
	tbof(mb)
}

// NewTelemetryBuilder provides a struct with methods to update all internal telemetry
// for a component
func NewTelemetryBuilder(settings component.TelemetrySettings, options ...TelemetryBuilderOption) (*TelemetryBuilder, error) {
	builder := TelemetryBuilder{meters: map[configtelemetry.Level]metric.Meter{}}
	for _, op := range options {
		op.apply(&builder)
	}
	builder.meters[configtelemetry.LevelBasic] = LeveledMeter(settings, configtelemetry.LevelBasic)
	var err, errs error
	builder.ProcessorAtlassianSamplingCacheReads, err = builder.meters[configtelemetry.LevelBasic].Int64Counter(
		"otelcol_processor_atlassian_sampling_cache_reads",
		metric.WithDescription("Amount of times a cache was read from"),
		metric.WithUnit("{accesses}"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingChanBlockingTime, err = builder.meters[configtelemetry.LevelBasic].Int64Histogram(
		"otelcol_processor_atlassian_sampling_chan_blocking_time",
		metric.WithDescription("Amount of time spent blocking on the chan send in ConsumeTraces()"),
		metric.WithUnit("ns"), metric.WithExplicitBucketBoundaries([]float64{50000, 100000, 500000, 1e+06, 5e+06, 1e+07, 5e+07, 1e+08, 1e+09, 1.5e+09}...),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingDecisionEvictionTime, err = builder.meters[configtelemetry.LevelBasic].Float64Gauge(
		"otelcol_processor_atlassian_sampling_decision_eviction_time",
		metric.WithDescription("Time that a trace ID spent in the decision cache before it was evicted"),
		metric.WithUnit("s"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingOverlyEagerLonelyRootSpanDecisions, err = builder.meters[configtelemetry.LevelBasic].Int64Counter(
		"otelcol_processor_atlassian_sampling_overly_eager_lonely_root_span_decisions",
		metric.WithDescription("Number of spans that have been aggressively sampled out by root span policy"),
		metric.WithUnit("{spans}"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingPolicyDecisions, err = builder.meters[configtelemetry.LevelBasic].Int64Counter(
		"otelcol_processor_atlassian_sampling_policy_decisions",
		metric.WithDescription("Sampling decisions made specifying policy and decision."),
		metric.WithUnit("{decisions}"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingPrimaryCacheSize, err = builder.meters[configtelemetry.LevelBasic].Int64Gauge(
		"otelcol_processor_atlassian_sampling_primary_cache_size",
		metric.WithDescription("Size on the primary cache"),
		metric.WithUnit("{traces}"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingTraceEvictionTime, err = builder.meters[configtelemetry.LevelBasic].Float64Gauge(
		"otelcol_processor_atlassian_sampling_trace_eviction_time",
		metric.WithDescription("Time that a non-sampled trace was kept in memory from arrival to being evicted"),
		metric.WithUnit("s"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingTracesNotSampled, err = builder.meters[configtelemetry.LevelBasic].Int64Counter(
		"otelcol_processor_atlassian_sampling_traces_not_sampled",
		metric.WithDescription("Number of traces dropped and not sampled"),
		metric.WithUnit("{traces}"),
	)
	errs = errors.Join(errs, err)
	builder.ProcessorAtlassianSamplingTracesSampled, err = builder.meters[configtelemetry.LevelBasic].Int64Counter(
		"otelcol_processor_atlassian_sampling_traces_sampled",
		metric.WithDescription("Number of traces sampled"),
		metric.WithUnit("{traces}"),
	)
	errs = errors.Join(errs, err)
	return &builder, errs
}
