package atlassiansamplingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/priority"
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

var testTraceID3 pcommon.TraceID = [16]byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0xa0, 0xb9, 0xc0, 0xd1, 0xe2, 0xf3, 0xf1, 0xf2,
}

type mockDecider struct {
	NextDecision       evaluators.Decision
	NextDecisionPolicy *policy
}

var _ deciderI = (*mockDecider)(nil)

func (m *mockDecider) MakeDecision(ctx context.Context, id pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) (evaluators.Decision, *policy) {
	return m.NextDecision, m.NextDecisionPolicy
}

func (m *mockDecider) Start(ctx context.Context, host component.Host) error {
	return nil
}

func TestConsumeTraces_Basic(t *testing.T) {
	t.Parallel()
	f := NewFactory()
	cfg := f.CreateDefaultConfig()

	ctx := context.Background()
	set := processortest.NewNopSettings()
	host := componenttest.NewNopHost()

	tracesSink := new(consumertest.TracesSink)
	tracesProcessor, err := f.CreateTraces(ctx, set, cfg, tracesSink)
	assert.NoError(t, err)
	assert.NotNil(t, tracesProcessor)

	assert.NoError(t, tracesProcessor.Start(ctx, host))

	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	assert.NoError(t, tracesProcessor.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.NoError(t, tracesProcessor.Shutdown(ctx))
}

func TestConsumeTraces_CachedDataIsSent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)
	cfg.PrimaryCacheSize = 100
	cfg.SecondaryCacheSize = 10
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
	decider.NextDecisionPolicy = &policy{name: "test-policy", policyType: RootSpans}
	require.NoError(t, asp.ConsumeTraces(ctx, trace1))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces())) // blocks until previous consumption is completed

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
	t.Parallel()
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{NextDecision: evaluators.Sampled,
		NextDecisionPolicy: &policy{name: "test-policy", policyType: RootSpans}}
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
	t.Parallel()
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{NextDecision: evaluators.Pending,
		NextDecisionPolicy: &policy{name: "test-policy", policyType: RootSpans}}
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
	assert.Equal(t, int32(3), cachedMetadata.SpanCount)
	assert.Equal(t, pcommon.Timestamp(1), cachedMetadata.EarliestStartTime)
	assert.Equal(t, pcommon.Timestamp(8), cachedMetadata.LatestEndTime)
}

func TestConsumeTraces_TraceDataPrioritised(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)
	cfg.PrimaryCacheSize = 10
	cfg.SecondaryCacheSize = 1 // 1 secondary cache size -> 1 low priority cache size

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{NextDecision: evaluators.LowPriority, NextDecisionPolicy: &policy{name: "test-policy", policyType: RootSpans}}
	asp.decider = decider

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	// Consume first trace at low priority
	trace1 := ptrace.NewTraces()
	span1 := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetTraceID(testTraceID)
	span1.SetStartTimestamp(2)
	span1.SetEndTimestamp(5)
	require.NoError(t, asp.ConsumeTraces(ctx, trace1))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Equal(t, 0, sink.SpanCount())
	td, ok := asp.traceData.Get(testTraceID)
	assert.True(t, ok)
	assert.Equal(t, priority.Low, td.GetPriority())
	assert.Equal(t, int32(1), td.Metadata.SpanCount)

	// Consume trace, same ID still low priority decision. Should be combined as low priority data
	trace2 := ptrace.NewTraces()
	span2 := trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span2.SetTraceID(testTraceID)
	span2.SetStartTimestamp(1)
	span2.SetEndTimestamp(3)
	require.NoError(t, asp.ConsumeTraces(ctx, trace2))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Equal(t, 0, sink.SpanCount())
	td, ok = asp.traceData.Get(testTraceID)
	assert.True(t, ok)
	assert.Equal(t, priority.Low, td.GetPriority())
	assert.Equal(t, int32(2), td.Metadata.SpanCount)

	// Consume second trace with new ID, also low priority.
	// Size of low priority cache should be 1, so this should evict the existing low priority trace.
	trace3 := ptrace.NewTraces()
	span3 := trace3.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span3.SetTraceID(testTraceID2)
	span3.SetStartTimestamp(1)
	span3.SetEndTimestamp(3)
	require.NoError(t, asp.ConsumeTraces(ctx, trace3))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Equal(t, 0, sink.SpanCount())
	_, ok = asp.traceData.Get(testTraceID)
	assert.False(t, ok)
	td, ok = asp.traceData.Get(testTraceID2)
	assert.True(t, ok)
	assert.Equal(t, priority.Low, td.GetPriority())
	assert.Equal(t, int32(1), td.Metadata.SpanCount)
	// fully evicted
	_, ok = asp.nonSampledDecisionCache.Get(testTraceID)
	assert.True(t, ok)

	// promote testTraceID2 into regular priority
	decider.NextDecision = evaluators.Pending
	decider.NextDecisionPolicy = &policy{name: "test-policy2", policyType: Probabilistic}
	require.NoError(t, asp.ConsumeTraces(ctx, trace3))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))
	td, ok = asp.traceData.Get(testTraceID2)
	assert.True(t, ok)
	assert.Equal(t, priority.Unspecified, td.GetPriority())
	assert.Equal(t, int32(2), td.Metadata.SpanCount)
	// evicted from secondary, but NOT fully evicted
	_, ok = asp.nonSampledDecisionCache.Get(testTraceID2)
	assert.False(t, ok)

	// Other low priority decisions now shouldn't evict traceID2
	decider.NextDecision = evaluators.LowPriority
	decider.NextDecisionPolicy = &policy{name: "test-policy2", policyType: Probabilistic}
	trace4 := ptrace.NewTraces()
	span4 := trace4.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span4.SetTraceID(testTraceID3)
	span4.SetStartTimestamp(1)
	span4.SetEndTimestamp(3)
	require.NoError(t, asp.ConsumeTraces(ctx, trace4))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	td, ok = asp.traceData.Get(testTraceID2)
	assert.True(t, ok)
	assert.Equal(t, priority.Unspecified, td.GetPriority())
	assert.Equal(t, int32(2), td.Metadata.SpanCount)

	td, ok = asp.traceData.Get(testTraceID3)
	assert.True(t, ok)
	assert.Equal(t, priority.Low, td.GetPriority())
	assert.Equal(t, int32(1), td.Metadata.SpanCount)

	// now cause testTraceID2 to be sampled
	assert.Equal(t, 0, sink.SpanCount())
	decider.NextDecision = evaluators.Sampled
	require.NoError(t, asp.ConsumeTraces(ctx, trace3))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))
	assert.Equal(t, 3, sink.SpanCount())

	require.NoError(t, asp.Shutdown(ctx))
}

func TestConsumeTraces_PriorityNotDemoted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{NextDecision: evaluators.Pending,
		NextDecisionPolicy: &policy{name: "test-policy", policyType: RootSpans}}
	asp.decider = decider

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	// Consume first trace, should be put in regular priority cache
	trace1 := ptrace.NewTraces()
	span1 := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetTraceID(testTraceID)
	span1.SetStartTimestamp(2)
	span1.SetEndTimestamp(5)
	require.NoError(t, asp.ConsumeTraces(ctx, trace1))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	td, ok := asp.traceData.Get(testTraceID)
	assert.True(t, ok)
	assert.Equal(t, priority.Unspecified, td.GetPriority())
	assert.Equal(t, int32(1), td.Metadata.SpanCount)

	// Consume again, should remain unspecified priority
	decider.NextDecision = evaluators.LowPriority
	require.NoError(t, asp.ConsumeTraces(ctx, trace1))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	td, ok = asp.traceData.Get(testTraceID)
	assert.True(t, ok)
	assert.Equal(t, priority.Unspecified, td.GetPriority())
	assert.Equal(t, int32(2), td.Metadata.SpanCount)

	require.NoError(t, asp.Shutdown(ctx))
}

func TestShutdown_Flushes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)
	cfg.FlushOnShutdown = true

	sink := &consumertest.TracesSink{}
	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{NextDecision: evaluators.Pending,
		NextDecisionPolicy: &policy{name: "test-policy", policyType: RootSpans}}
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

func TestConsumeTraces_PrimaryCacheSizeConfigApplied(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)
	cfg.PrimaryCacheSize = 2 // 2 primary cache size -> 2 non-low priority cache size
	cfg.SecondaryCacheSize = 1

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{NextDecision: evaluators.Pending,
		NextDecisionPolicy: &policy{name: "test-policy", policyType: RootSpans}}
	asp.decider = decider

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	// Consume first trace at pending priority
	trace1 := ptrace.NewTraces()
	span1 := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetTraceID(testTraceID)
	span1.SetStartTimestamp(2)
	span1.SetEndTimestamp(5)
	require.NoError(t, asp.ConsumeTraces(ctx, trace1))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Equal(t, 0, sink.SpanCount())
	td, ok := asp.traceData.Get(testTraceID)
	assert.True(t, ok)
	assert.Equal(t, priority.Unspecified, td.GetPriority())
	assert.Equal(t, int32(1), td.Metadata.SpanCount)

	// Consume second trace with new ID, also non-low priority.
	trace2 := ptrace.NewTraces()
	span2 := trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span2.SetTraceID(testTraceID2)
	span2.SetStartTimestamp(1)
	span2.SetEndTimestamp(3)
	require.NoError(t, asp.ConsumeTraces(ctx, trace2))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Equal(t, 0, sink.SpanCount())
	_, ok = asp.traceData.Get(testTraceID)
	assert.True(t, ok)
	td, ok = asp.traceData.Get(testTraceID2)
	assert.True(t, ok)
	assert.Equal(t, priority.Unspecified, td.GetPriority())

	// Third non-low priority decisions now should evict traceID1
	trace3 := ptrace.NewTraces()
	span3 := trace3.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span3.SetTraceID(testTraceID3)
	span3.SetStartTimestamp(1)
	span3.SetEndTimestamp(3)
	require.NoError(t, asp.ConsumeTraces(ctx, trace3))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	_, ok = asp.traceData.Get(testTraceID)
	assert.False(t, ok)

	td, ok = asp.traceData.Get(testTraceID3)
	assert.True(t, ok)
	assert.Equal(t, priority.Unspecified, td.GetPriority())
	assert.Equal(t, int32(1), td.Metadata.SpanCount)

	require.NoError(t, asp.Shutdown(ctx))
}

func TestConsumeTraces_SecondaryCacheSizeConfigApplied(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)
	cfg.PrimaryCacheSize = 10
	cfg.SecondaryCacheSize = 2 // 2 secondary cache size -> 2 low priority cache size

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{NextDecision: evaluators.LowPriority,
		NextDecisionPolicy: &policy{name: "test-policy", policyType: RootSpans}}
	asp.decider = decider

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	// Consume first trace at low priority
	trace1 := ptrace.NewTraces()
	span1 := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetTraceID(testTraceID)
	span1.SetStartTimestamp(2)
	span1.SetEndTimestamp(5)
	require.NoError(t, asp.ConsumeTraces(ctx, trace1))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Equal(t, 0, sink.SpanCount())
	td, ok := asp.traceData.Get(testTraceID)
	assert.True(t, ok)
	assert.Equal(t, priority.Low, td.GetPriority())
	assert.Equal(t, int32(1), td.Metadata.SpanCount)

	// Consume second trace with new ID, also low priority.
	// Size of low priority cache should be 2, so this should not evict the existing low priority trace.
	trace2 := ptrace.NewTraces()
	span2 := trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span2.SetTraceID(testTraceID2)
	span2.SetStartTimestamp(1)
	span2.SetEndTimestamp(3)
	require.NoError(t, asp.ConsumeTraces(ctx, trace2))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Equal(t, 0, sink.SpanCount())
	_, ok = asp.traceData.Get(testTraceID)
	assert.True(t, ok)
	td, ok = asp.traceData.Get(testTraceID2)
	assert.True(t, ok)
	assert.Equal(t, priority.Low, td.GetPriority())

	// Third low priority decisions now should evict traceID1
	trace3 := ptrace.NewTraces()
	span3 := trace3.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span3.SetTraceID(testTraceID3)
	span3.SetStartTimestamp(1)
	span3.SetEndTimestamp(3)
	require.NoError(t, asp.ConsumeTraces(ctx, trace3))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	_, ok = asp.traceData.Get(testTraceID)
	assert.False(t, ok)

	td, ok = asp.traceData.Get(testTraceID3)
	assert.True(t, ok)
	assert.Equal(t, priority.Low, td.GetPriority())
	assert.Equal(t, int32(1), td.Metadata.SpanCount)

	require.NoError(t, asp.Shutdown(ctx))
}

func TestConsumeTraces_FirstLowPriorityDecisionWasMade_MetaDataUpdated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)

	decider := &mockDecider{NextDecision: evaluators.LowPriority,
		NextDecisionPolicy: &policy{name: "test-policy",
			policyType: RootSpans}}
	asp.decider = decider

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	// Consume first trace at low priority
	trace1 := ptrace.NewTraces()
	span1 := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetTraceID(testTraceID)
	span1.SetStartTimestamp(2)
	span1.SetEndTimestamp(5)
	require.NoError(t, asp.ConsumeTraces(ctx, trace1))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	td, ok := asp.traceData.Get(testTraceID)
	assert.True(t, ok)
	// Should store the policy name and evaluator to metadata
	require.NotNil(t, td.Metadata.LastLowPriorityDecisionName)
	assert.Equal(t, "test-policy", *td.Metadata.LastLowPriorityDecisionName)
	assert.Equal(t, priority.Low, td.GetPriority())
	require.NoError(t, asp.Shutdown(ctx))
}

func TestConsumeTraces_PromotedFromLowPriority(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := NewFactory()
	cfg := (f.CreateDefaultConfig()).(*Config)

	sink := &consumertest.TracesSink{}

	asp, err := newAtlassianSamplingProcessor(cfg, componenttest.NewNopTelemetrySettings(), sink)
	require.NoError(t, err)
	require.NotNil(t, asp)
	// Set Metadata.LastLowPriorityDecisionName to test-policy
	decider := &mockDecider{NextDecision: evaluators.LowPriority,
		NextDecisionPolicy: &policy{name: "test-policy",
			policyType: RootSpans}}
	asp.decider = decider

	require.NoError(t, asp.Start(ctx, componenttest.NewNopHost()))

	// Consume first trace at low priority
	trace1 := ptrace.NewTraces()
	span1 := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetTraceID(testTraceID)
	span1.SetStartTimestamp(2)
	span1.SetEndTimestamp(5)
	require.NoError(t, asp.ConsumeTraces(ctx, trace1))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Equal(t, 0, sink.SpanCount())
	td, ok := asp.traceData.Get(testTraceID)
	assert.True(t, ok)
	assert.Equal(t, priority.Low, td.GetPriority())
	assert.Equal(t, int32(1), td.Metadata.SpanCount)

	decider.NextDecision = evaluators.Pending
	decider.NextDecisionPolicy = &policy{name: "test-policy2", policyType: Probabilistic}

	// Consume trace, same ID still low priority decision. Should be combined as low priority data
	trace2 := ptrace.NewTraces()
	span2 := trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span2.SetTraceID(testTraceID)
	span2.SetStartTimestamp(1)
	span2.SetEndTimestamp(3)
	require.NoError(t, asp.ConsumeTraces(ctx, trace2))
	require.NoError(t, asp.ConsumeTraces(ctx, ptrace.NewTraces()))

	assert.Equal(t, 0, sink.SpanCount())
	td, ok = asp.traceData.Get(testTraceID)
	assert.True(t, ok)
	assert.Equal(t, priority.Unspecified, td.GetPriority())
	assert.Nil(t, td.Metadata.LastLowPriorityDecisionName)
	assert.Equal(t, int32(2), td.Metadata.SpanCount)

	require.NoError(t, asp.Shutdown(ctx))
}
