package tracedata

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestCreation(t *testing.T) {
	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	now := time.Now()
	td := NewTraceData(now, trace)
	assert.Equal(t, int64(1), td.SpanCount.Load())
	assert.Equal(t, now, td.ArrivalTime)
}

func TestMerge(t *testing.T) {
	trace1 := ptrace.NewTraces()
	span1 := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span1.SetStartTimestamp(2)
	span1.SetEndTimestamp(3)
	trace2 := ptrace.NewTraces()
	span2 := trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span2.SetStartTimestamp(1)
	span2.SetEndTimestamp(4)

	td1 := NewTraceData(time.Now(), trace1)
	td2 := NewTraceData(time.Now(), trace2)
	td1.MergeWith(td2)

	assert.Equal(t, int64(2), td1.SpanCount.Load())
	assert.Equal(t, 2, td1.ReceivedBatches.ResourceSpans().Len())
	assert.Equal(t, pcommon.Timestamp(1), td1.EarliestStartTime)
	assert.Equal(t, pcommon.Timestamp(4), td1.LatestEndTime)
}
