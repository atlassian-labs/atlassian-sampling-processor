package tracedata // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"

import (
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type Metadata struct {
	// Arrival time the first span for the trace was received.
	ArrivalTime time.Time
	// EarliestStartTime is the time of the earliest start of a span that we've seen for this trace.
	EarliestStartTime pcommon.Timestamp
	// LatestEndTime is the latest time of the latest end to a span that we've seen for this trace.
	LatestEndTime pcommon.Timestamp
	// SpanCount track the number of spans on the trace.
	SpanCount *atomic.Int64
}

// MergeWith merges the data from the other Metadata into this Metadata
// It modifies the current Metadata instance(i.e. `m`) but does not modify the other Metadata provided(i.e. `other`)
func (m *Metadata) MergeWith(other *Metadata) {
	m.SpanCount.Add(other.SpanCount.Load())

	if other.ArrivalTime.Before(m.ArrivalTime) {
		m.ArrivalTime = other.ArrivalTime
	}

	if other.EarliestStartTime < m.EarliestStartTime {
		m.EarliestStartTime = other.EarliestStartTime
	}
	if other.LatestEndTime > m.LatestEndTime {
		m.LatestEndTime = other.LatestEndTime
	}
}

// DeepCopy returns a deep copy of this Metadata
func (m *Metadata) DeepCopy() *Metadata {
	dup := *m

	spanCount := &atomic.Int64{}
	spanCount.Store(m.SpanCount.Load())
	dup.SpanCount = spanCount

	return &dup
}
