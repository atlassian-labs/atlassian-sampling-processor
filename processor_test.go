package atlassiansamplingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
)

var testTraceID pcommon.TraceID = [16]byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
}

func TestConsumeTraces_basic(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()

	ctx := context.Background()
	set := processortest.NewNopSettings()
	host := componenttest.NewNopHost()

	tracesSink := new(consumertest.TracesSink)
	tracesProcessor, err := f.CreateTracesProcessor(ctx, set, cfg, tracesSink)
	assert.NoError(t, err)
	assert.NotNil(t, tracesProcessor)

	assert.NoError(t, tracesProcessor.Start(ctx, host))

	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	assert.NoError(t, tracesProcessor.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.NoError(t, tracesProcessor.Shutdown(ctx))
}

func TestConsumeTraces_CachedDataIsSent(t *testing.T) {
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)
	cfg.MaxTraces = 100
	cfg.DecisionCacheCfg.SampledCacheSize = 100
	cfg.DecisionCacheCfg.NonSampledCacheSize = 100
	cfg.PolicyConfig = []PolicyConfig{
		{
			SharedPolicyConfig: SharedPolicyConfig{
				Name: "test",
				Type: "span_count",
				SpanCountConfig: SpanCountConfig{
					MinSpans: 2,
				}},
		},
	}

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	trace1 := ptrace.NewTraces()
	span := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(testTraceID)
	require.NoError(t, asp.ConsumeTraces(ctx, trace1))

	// Still pending because hasn't reached min span in trace of 2
	assert.Equal(t, 0, sink.SpanCount())

	trace2 := ptrace.NewTraces()
	span = trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(testTraceID)
	require.NoError(t, asp.ConsumeTraces(ctx, trace2))
	require.NoError(t, asp.Shutdown(ctx))

	// Both the first and second span should now have been released.
	// The first one being cached, being released once the trace satisfies the policy.
	assert.Equal(t, 2, sink.SpanCount())
}
