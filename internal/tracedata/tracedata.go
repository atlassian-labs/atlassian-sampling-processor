package tracedata // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"

import (
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TraceData stores the sampling related trace data.
// The zero value for this type is invalid, use NewTraceData to create.
type TraceData struct {
	// Arrival time the first span for the trace was received.
	ArrivalTime time.Time
	// EarliestStartTime is the time of the earliest start of a span that we've seen for this trace.
	EarliestStartTime pcommon.Timestamp
	// LatestEndTime is the latest time of the latest end to a span that we've seen for this trace.
	LatestEndTime pcommon.Timestamp
	// SpanCount track the number of spans on the trace.
	SpanCount *atomic.Int64
	// ReceivedBatches stores all the batches received for the trace.
	ReceivedBatches ptrace.Traces
}

func NewTraceData(arrival time.Time, traces ptrace.Traces) *TraceData {
	spanCount := &atomic.Int64{}
	spanCount.Store(int64(traces.SpanCount()))

	traceData := &TraceData{
		ArrivalTime:     arrival,
		SpanCount:       spanCount,
		ReceivedBatches: traces,
	}

	// Calculate earliest start and latest end
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ss := ilss.At(j).Spans()
			for k := 0; k < ss.Len(); k++ {
				start := ss.At(k).StartTimestamp()
				end := ss.At(k).EndTimestamp()

				if start < traceData.EarliestStartTime || traceData.EarliestStartTime == 0 {
					traceData.EarliestStartTime = start
				}
				if end > traceData.LatestEndTime {
					traceData.LatestEndTime = end
				}
			}
		}
	}

	return traceData
}

// MergeWith merges the data from the other TraceData into this TraceData
func (td *TraceData) MergeWith(other *TraceData) {
	td.SpanCount.Add(other.SpanCount.Load())

	if other.EarliestStartTime < td.EarliestStartTime {
		td.EarliestStartTime = other.EarliestStartTime
	}
	if other.LatestEndTime > td.LatestEndTime {
		td.LatestEndTime = other.LatestEndTime
	}

	for i := 0; i < other.ReceivedBatches.ResourceSpans().Len(); i++ {
		rs := other.ReceivedBatches.ResourceSpans().At(i)
		dest := td.ReceivedBatches.ResourceSpans().AppendEmpty()
		rs.CopyTo(dest)
	}
}
