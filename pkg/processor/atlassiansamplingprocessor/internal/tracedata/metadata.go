package tracedata // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/tracedata"

import (
	"slices"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/priority"
)

type Metadata struct {
	// Arrival time the first span for the trace was received.
	ArrivalTime time.Time
	// EarliestStartTime is the time of the earliest start of a span that we've seen for this trace.
	EarliestStartTime pcommon.Timestamp
	// LatestEndTime is the latest time of the latest end to a span that we've seen for this trace.
	LatestEndTime pcommon.Timestamp
	// SpanCount track the number of spans on the trace.
	SpanCount int32
	// Priority of the trace data. May be used to tier caching.
	Priority priority.Priority
	// LastLowPriorityDecisionName is the policy's name that was previously made decision to low priority.
	LastLowPriorityDecisionName *string
}

// MergeWith merges the data from the other Metadata into this Metadata
// It modifies the current Metadata instance(i.e. `m`) but does not modify the other Metadata provided(i.e. `other`)
func (m *Metadata) MergeWith(other *Metadata) {
	m.SpanCount += other.SpanCount

	if other.ArrivalTime.Before(m.ArrivalTime) {
		m.ArrivalTime = other.ArrivalTime
	}
	m.EarliestStartTime = slices.Min([]pcommon.Timestamp{m.EarliestStartTime, other.EarliestStartTime})
	m.LatestEndTime = slices.Max([]pcommon.Timestamp{m.LatestEndTime, other.LatestEndTime})
	m.Priority = slices.Max([]priority.Priority{m.Priority, other.Priority})
	m.LastLowPriorityDecisionName = other.LastLowPriorityDecisionName
}

// DeepCopy returns a deep copy of this Metadata
func (m *Metadata) DeepCopy() *Metadata {
	dup := *m
	return &dup
}
