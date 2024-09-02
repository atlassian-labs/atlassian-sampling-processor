package tracedata

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/priority"
)

func TestCreation(t *testing.T) {
	t.Parallel()
	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	now := time.Now()
	td := NewTraceData(now, trace, priority.Low)
	m := td.Metadata
	assert.Equal(t, int64(1), m.SpanCount.Load())
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

	td1 := NewTraceData(time.Now(), trace1, priority.Low)
	td2 := NewTraceData(time.Now(), trace2, priority.Unspecified)
	td1.MergeWith(td2)
	m1 := td1.Metadata

	assert.Equal(t, int64(2), m1.SpanCount.Load())
	assert.Equal(t, 2, td1.GetTraces().ResourceSpans().Len())
	assert.Equal(t, pcommon.Timestamp(1), m1.EarliestStartTime)
	assert.Equal(t, pcommon.Timestamp(4), m1.LatestEndTime)
	assert.Equal(t, priority.Unspecified, td1.GetPriority())
	assert.Equal(t, priority.Unspecified, td2.GetPriority())
}

func TestMerge_OtherIsEarlier(t *testing.T) {
	trace1 := ptrace.NewTraces()
	span1 := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetStartTimestamp(1)
	span1.SetEndTimestamp(4)
	trace2 := ptrace.NewTraces()
	span2 := trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span2.SetStartTimestamp(2)
	span2.SetEndTimestamp(3)

	td1 := NewTraceData(time.UnixMilli(2), trace1, priority.Unspecified)
	td2 := NewTraceData(time.UnixMilli(1), trace2, priority.Unspecified)
	td1.MergeWith(td2)
	m1 := td1.Metadata

	assert.Equal(t, int64(2), m1.SpanCount.Load())
	assert.Equal(t, 2, td1.GetTraces().ResourceSpans().Len())
	assert.Equal(t, pcommon.Timestamp(1), m1.EarliestStartTime)
	assert.Equal(t, pcommon.Timestamp(4), m1.LatestEndTime)
	assert.Equal(t, priority.Unspecified, td1.GetPriority())
	assert.Equal(t, priority.Unspecified, td2.GetPriority())
}

func TestGetTraces(t *testing.T) {
	t.Parallel()
	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	td := NewTraceData(time.Now(), trace, priority.Unspecified)

	assert.Equal(t, trace, td.GetTraces())
}
