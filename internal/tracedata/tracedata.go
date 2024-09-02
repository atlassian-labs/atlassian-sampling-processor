package tracedata // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"

import (
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/priority"
)

// TraceData stores the sampling related trace data.
// The zero value for this type is invalid, use NewTraceData to create.
type TraceData struct {
	Metadata *Metadata

	// receivedBatches stores all the batches received for the trace.
	receivedBatches ptrace.Traces
}

var _ priority.Getter = (*TraceData)(nil)

func NewTraceData(arrival time.Time, traces ptrace.Traces, p priority.Priority) *TraceData {
	spanCount := &atomic.Int64{}
	spanCount.Store(int64(traces.SpanCount()))

	metadata := &Metadata{
		ArrivalTime: arrival,
		SpanCount:   spanCount,
		Priority:    p,
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

				if start < metadata.EarliestStartTime || metadata.EarliestStartTime == 0 {
					metadata.EarliestStartTime = start
				}
				if end > metadata.LatestEndTime {
					metadata.LatestEndTime = end
				}
			}
		}
	}

	return &TraceData{
		Metadata:        metadata,
		receivedBatches: traces,
	}
}

// MergeWith merges the data from the other TraceData into this TraceData
// It modifies the current TraceData instance(i.e. `td`) but does not modify the other TradeData provided(i.e. `other`)
func (td *TraceData) MergeWith(other *TraceData) {
	td.Metadata.MergeWith(other.Metadata)

	for i := 0; i < other.receivedBatches.ResourceSpans().Len(); i++ {
		rs := other.receivedBatches.ResourceSpans().At(i)
		dest := td.receivedBatches.ResourceSpans().AppendEmpty()
		rs.CopyTo(dest)
	}
}

// GetPriority implements priority.Getter
func (td *TraceData) GetPriority() priority.Priority {
	return td.Metadata.Priority
}

// GetTraces returns all batches received for the trace
// The traces returned should not be modified by the caller, MergeWith should be used to add data to a TraceData instead.
func (td *TraceData) GetTraces() ptrace.Traces {
	return td.receivedBatches
}
