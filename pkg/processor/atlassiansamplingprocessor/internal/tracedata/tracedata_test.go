package tracedata

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/priority"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestCreation(t *testing.T) {
	t.Parallel()
	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	now := time.Now()
	td, err := NewTraceData(now, trace, priority.Low, false)
	require.NoError(t, err)
	m := td.Metadata
	assert.Equal(t, int32(1), m.SpanCount)
	assert.Equal(t, now, m.ArrivalTime)
}

func TestMerge(t *testing.T) {
	t.Parallel()
	trace1 := ptrace.NewTraces()
	span1 := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetStartTimestamp(2)
	span1.SetEndTimestamp(3)
	trace2 := ptrace.NewTraces()
	span2 := trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span2.SetStartTimestamp(1)
	span2.SetEndTimestamp(4)

	td1, err := NewTraceData(time.Now(), trace1, priority.Low, false)
	require.NoError(t, err)
	td2, err := NewTraceData(time.Now(), trace2, priority.Unspecified, false)
	require.NoError(t, err)
	err = td1.AbsorbTraceData(td2)
	require.NoError(t, err)
	m1 := td1.Metadata

	assert.Equal(t, int32(2), m1.SpanCount)
	td1Get, err := td1.GetTraces()
	require.NoError(t, err)
	assert.Equal(t, 2, td1Get.ResourceSpans().Len())
	assert.Equal(t, pcommon.Timestamp(1), m1.EarliestStartTime)
	assert.Equal(t, pcommon.Timestamp(4), m1.LatestEndTime)
	assert.Equal(t, priority.Unspecified, td1.GetPriority())
	assert.Equal(t, priority.Unspecified, td2.GetPriority())
}

func TestMerge_OtherIsEarlier(t *testing.T) {
	t.Parallel()
	trace1 := ptrace.NewTraces()
	span1 := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetStartTimestamp(1)
	span1.SetEndTimestamp(4)
	trace2 := ptrace.NewTraces()
	span2 := trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span2.SetStartTimestamp(2)
	span2.SetEndTimestamp(3)

	td1, err := NewTraceData(time.UnixMilli(2), trace1, priority.Unspecified, false)
	require.NoError(t, err)
	td2, err := NewTraceData(time.UnixMilli(1), trace2, priority.Unspecified, false)
	require.NoError(t, err)
	err = td1.AbsorbTraceData(td2)
	require.NoError(t, err)
	m1 := td1.Metadata

	assert.Equal(t, int32(2), m1.SpanCount)
	td1Get, err := td1.GetTraces()
	require.NoError(t, err)
	assert.Equal(t, 2, td1Get.ResourceSpans().Len())
	assert.Equal(t, pcommon.Timestamp(1), m1.EarliestStartTime)
	assert.Equal(t, pcommon.Timestamp(4), m1.LatestEndTime)
	assert.Equal(t, priority.Unspecified, td1.GetPriority())
	assert.Equal(t, priority.Unspecified, td2.GetPriority())
}

func TestGetTraces(t *testing.T) {
	t.Parallel()
	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	td, err := NewTraceData(time.Now(), trace, priority.Unspecified, false)
	require.NoError(t, err)

	tdGet, err := td.GetTraces()
	require.NoError(t, err)
	assert.Equal(t, trace, tdGet)
}

func TestCreateGetCompressedTrace(t *testing.T) {
	t.Parallel()
	trace := tracesWithAttributes1(ptrace.NewTraces())

	td, err := NewTraceData(time.Now(), trace, priority.Unspecified, true)
	require.NoError(t, err)

	tdGet, err := td.GetTraces()
	require.NoError(t, err)
	assert.Equal(t, trace, tdGet)
}

// TestAbsorbCompressed tests that the trace contains the same data regardless of the
// compression flag, but the traces can be reordered within the root-level ResourceSpans array
// depending on the implementation
func TestAbsorbCompressed(t *testing.T) {
	t.Run("TestAbsorbUncompressedIntoUncompressed", testAbsorbCompressed(false, false))
	t.Run("TestAbsorbCompressedIntoUncompressed", testAbsorbCompressed(false, true))
	t.Run("TestAbsorbUncompressedIntoCompressed", testAbsorbCompressed(true, false))
	t.Run("TestAbsorbCompressedIntoCompressed", testAbsorbCompressed(true, true))
}

func testAbsorbCompressed(compress1, compress2 bool) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		trace1 := tracesWithAttributes1(ptrace.NewTraces())
		trace2 := tracesWithAttributes2(ptrace.NewTraces())
		merged := ptrace.NewTraces()
		merged = tracesWithAttributes1(merged)
		merged = tracesWithAttributes2(merged)

		td1, err := NewTraceData(time.Now(), trace1, priority.Unspecified, compress1)
		require.NoError(t, err)
		td2, err := NewTraceData(time.Now(), trace2, priority.Unspecified, compress2)
		require.NoError(t, err)

		err = td1.AbsorbTraceData(td2)

		if compress1 != compress2 {
			assert.ErrorContains(t, err, "incompatible trace data")
			return
		}

		require.NoError(t, err)

		td1Get, err := td1.GetTraces()
		require.NoError(t, err)
		err = ptracetest.CompareTraces(
			merged,
			td1Get,
			ptracetest.IgnoreResourceSpansOrder(),
			ptracetest.IgnoreScopeSpansOrder(),
			ptracetest.IgnoreSpansOrder(),
		)
		require.NoError(t, err)
	}
}

func tracesWithAttributes1(traceBag ptrace.Traces) ptrace.Traces {
	trace1 := traceBag.ResourceSpans().AppendEmpty()
	trace1Resource := trace1.Resource()
	trace1Resource.Attributes().PutStr("cpu", "42")
	trace1Resource.Attributes().PutStr("instance", "h-9000")
	trace1Span1 := trace1.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	trace1Span1.Attributes().PutStr("route", "/hello/world")
	trace1Span1.Attributes().PutStr("customer", "very-big")
	trace1Span2 := trace1.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	trace1Span2.Attributes().PutStr("route", "/user/create")
	trace1Span2.Attributes().PutStr("customer", "enterprise")

	return traceBag
}

func tracesWithAttributes2(traceBag ptrace.Traces) ptrace.Traces {
	trace1 := traceBag.ResourceSpans().AppendEmpty()
	trace1Resource := trace1.Resource()
	// NOTE: Slightly different resource here because IgnoreResourceSpansOrder
	// uses only these attributes to sort instead of fully matching duplicated resources
	trace1Resource.Attributes().PutStr("cpu", "43")
	trace1Resource.Attributes().PutStr("instance", "h-9001")
	trace1Span1 := trace1.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	trace1Span1.Attributes().PutStr("route", "/hello/earth")
	trace1Span1.Attributes().PutStr("customer", "even-bigger")
	trace2 := traceBag.ResourceSpans().AppendEmpty()
	trace2Resource := trace2.Resource()
	trace2Resource.Attributes().PutStr("cpu", "8")
	trace2Resource.Attributes().PutStr("instance", "laptop")
	trace2Span1 := trace2.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	trace2Span1.Attributes().PutStr("route", "/tracking")
	trace2Span1.Attributes().PutStr("customer", "free")

	return traceBag
}

// TestCompressedGetDataMultiple tests that calling TraceData.GetTraces does not
// decompress and duplicate data
func TestCompressedGetDataMultiple(t *testing.T) {
	trace1 := tracesWithAttributes1(ptrace.NewTraces())

	td1, err := NewTraceData(time.Now(), trace1, priority.Unspecified, true)
	require.NoError(t, err)

	td1Get, err := td1.GetTraces()
	require.NoError(t, err)
	assert.Equal(t, trace1, td1Get)

	// Calling a second time
	td1Get, err = td1.GetTraces()
	require.NoError(t, err)
	assert.Equal(t, trace1, td1Get)
}
