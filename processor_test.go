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

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

var testTraceID pcommon.TraceID = [16]byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
}

var testTraceID2 pcommon.TraceID = [16]byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15,
}

type mockDecider struct {
	NextDecision       evaluators.Decision
	NextDecisionPolicy *policy
}

var _ deciderI = (*mockDecider)(nil)

func (m *mockDecider) MakeDecision(ctx context.Context, id pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) (evaluators.Decision, *policy) {
	return m.NextDecision, m.NextDecisionPolicy
}

func TestConsumeTraces_Basic(t *testing.T) {
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

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{}
	asp.decider = decider

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	trace1 := ptrace.NewTraces()
	span := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(testTraceID)

	decider.NextDecision = evaluators.Pending
	require.NoError(t, asp.ConsumeTraces(ctx, trace1))

	// No span sent as decision is still pending
	assert.Equal(t, 0, sink.SpanCount())

	trace2 := ptrace.NewTraces()
	span = trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(testTraceID)

	decider.NextDecision = evaluators.Sampled
	require.NoError(t, asp.ConsumeTraces(ctx, trace2))
	require.NoError(t, asp.Shutdown(ctx))

	// Both the first and second span should now have been released.
	// The first one being cached, being released once the trace satisfies the policy.
	assert.Equal(t, 2, sink.SpanCount())
}

func TestConsumeTraces_MultipleTracesInOneResourceSpan(t *testing.T) {
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{NextDecision: evaluators.Sampled}
	asp.decider = decider

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	// two different trace IDs, one resource span
	trace := ptrace.NewTraces()
	spans := trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	span1 := spans.AppendEmpty()
	span1.SetTraceID(testTraceID)
	span1.SetStartTimestamp(2)
	span1.SetEndTimestamp(5)
	span2 := spans.AppendEmpty()
	span2.SetTraceID(testTraceID2)
	span2.SetStartTimestamp(1)
	span2.SetEndTimestamp(3)

	require.NoError(t, asp.ConsumeTraces(ctx, trace))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))
	require.NoError(t, asp.Shutdown(ctx))

	// Sampled immediately, so not in cache
	_, ok := asp.traceData.Get(testTraceID)
	assert.False(t, ok)
	_, ok = asp.traceData.Get(testTraceID2)
	assert.False(t, ok)

	assert.Equal(t, 2, sink.SpanCount())
	require.Equal(t, 2, len(sink.AllTraces())) // initial trace is split into 2
}

func TestConsumeTraces_CacheMetadata(t *testing.T) {
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{NextDecision: evaluators.Pending}
	asp.decider = decider

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	// Consume first trace
	trace1 := ptrace.NewTraces()
	span1 := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetTraceID(testTraceID)
	span1.SetStartTimestamp(2)
	span1.SetEndTimestamp(5)

	require.NoError(t, asp.ConsumeTraces(ctx, trace1))

	// Consume second trace
	trace2 := ptrace.NewTraces()
	spans := trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()

	span2 := spans.AppendEmpty()
	span2.SetTraceID(testTraceID)
	span2.SetStartTimestamp(1)
	span2.SetEndTimestamp(3)

	span3 := spans.AppendEmpty()
	span3.SetTraceID(testTraceID)
	span3.SetStartTimestamp(4)
	span3.SetEndTimestamp(8)

	require.NoError(t, asp.ConsumeTraces(ctx, trace2))
	require.NoError(t, asp.Shutdown(ctx))

	// Cached metadata should represent all spans since they have the same trace id
	cachedData, ok := asp.traceData.Get(testTraceID)
	require.Equal(t, true, ok)

	cachedMetadata := cachedData.Metadata
	assert.Equal(t, int64(3), cachedMetadata.SpanCount.Load())
	assert.Equal(t, pcommon.Timestamp(1), cachedMetadata.EarliestStartTime)
	assert.Equal(t, pcommon.Timestamp(8), cachedMetadata.LatestEndTime)
}

func TestShutdown_Flushes(t *testing.T) {
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)
	cfg.FlushOnShutdown = true

	sink := &consumertest.TracesSink{}
	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{NextDecision: evaluators.Pending}
	asp.decider = decider

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	// One trace, two ResourceSpans, one with the flushes attr already set
	trace := ptrace.NewTraces()
	span1 := trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetTraceID(testTraceID)
	rs2 := trace.ResourceSpans().AppendEmpty()
	rs2.Resource().Attributes().PutInt(flushCountKey, 5)
	span2 := rs2.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span2.SetTraceID(testTraceID)

	// Nothing sends because policy never samples
	require.NoError(t, asp.ConsumeTraces(ctx, trace))
	assert.Equal(t, 0, sink.SpanCount())

	assert.NoError(t, asp.Shutdown(context.Background()))

	assert.Equal(t, 2, sink.SpanCount())
	sentData := sink.AllTraces()
	require.Equal(t, 1, len(sentData))
	require.Equal(t, 2, sentData[0].ResourceSpans().Len())

	// First resource (didn't initially have any flushes set)
	v, ok := sentData[0].ResourceSpans().At(0).Resource().Attributes().Get(flushCountKey)
	require.True(t, ok)
	flushes := v.Int()
	assert.Equal(t, int64(1), flushes)

	// Second resource (had incoming flush count of 5)
	v, ok = sentData[0].ResourceSpans().At(1).Resource().Attributes().Get(flushCountKey)
	require.True(t, ok)
	flushes = v.Int()
	assert.Equal(t, int64(6), flushes)
}

func TestShutdown_TimesOut(t *testing.T) {
	// This test verifies that shutdown times out when the context reaches its deadline.
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)
	cfg.FlushOnShutdown = true

	sink := &consumertest.TracesSink{}
	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel() // Immediately cancel, simulating timeout
	require.Error(t, asp.Shutdown(cancelledCtx))
}

func TestLonelyRootSpanPolicy_LateSpanIsHandled(t *testing.T) {
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)
	cfg.MaxTraces = 10
	cfg.DecisionCacheCfg.SampledCacheSize = 10
	cfg.DecisionCacheCfg.NonSampledCacheSize = 10

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	p := &policy{
		name:       "test_drop_lonely_root_span_policy",
		policyType: "root_spans",
	}
	decider := &mockDecider{NextDecision: evaluators.NotSampled, NextDecisionPolicy: p}
	asp.decider = decider

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	trace1 := ptrace.NewTraces()
	span := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(testTraceID)
	require.NoError(t, asp.ConsumeTraces(ctx, trace1))

	// We send a second span belonging to the same trace (late arriving span)
	trace2 := ptrace.NewTraces()
	span2 := trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span2.SetTraceID(testTraceID)
	require.NoError(t, asp.ConsumeTraces(ctx, trace2))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces())) // blocks until previous consumption is completed

	// The late arriving span should not be sampled should not change the decision
	assert.Equal(t, 0, sink.SpanCount())

	// NSD cache should have the trace ID and related metadata (policy, decision time)
	nsdValue, ok := asp.nonSampledDecisionCache.Get(testTraceID)
	assert.True(t, ok)

	assert.Equal(t, PolicyType("root_spans"), nsdValue.decisionPolicy.policyType)
	assert.Equal(t, "test_drop_lonely_root_span_policy", nsdValue.decisionPolicy.name)

	require.NoError(t, asp.Shutdown(ctx))
}
