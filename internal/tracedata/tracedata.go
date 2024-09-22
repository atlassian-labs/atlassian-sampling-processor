package tracedata // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/priority"
)

// TraceData stores the sampling related trace data.
// The zero value for this type is invalid, use NewTraceData to create.
type TraceData struct {
	Metadata *Metadata

	// traces stores all trace data received for the trace.
	traces ptrace.Traces
}

var _ priority.Getter = (*TraceData)(nil)

func NewTraceData(arrival time.Time, traces ptrace.Traces, p priority.Priority) *TraceData {
	metadata := &Metadata{
		ArrivalTime: arrival,
		SpanCount:   int32(traces.SpanCount()), //nolint G115: integer overflow conversion int
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
		Metadata: metadata,
		traces:   traces,
	}
}

// AbsorbTraceData merges the data from the other TraceData into this TraceData.
// It modifies both the current TraceData instance (td) and the one passed in (other).
// other MUST NOT be used after calling this function, as it's trace data is moved into td.
func (td *TraceData) AbsorbTraceData(other *TraceData) {
	td.Metadata.MergeWith(other.Metadata)

	for i := 0; i < other.traces.ResourceSpans().Len(); i++ {
		rs := other.traces.ResourceSpans().At(i)
		dest := td.traces.ResourceSpans().AppendEmpty()
		rs.MoveTo(dest)
	}
}

// GetPriority implements priority.Getter
func (td *TraceData) GetPriority() priority.Priority {
	return td.Metadata.Priority
}

// GetTraces returns all batches received for the trace
// The traces returned should not be modified by the caller, AbsorbTraceData should be used to add data to a TraceData instead.
func (td *TraceData) GetTraces() ptrace.Traces {
	return td.traces
}
