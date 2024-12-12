package priority // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/priority"

// Getter is an interface for structs that wish to indicate their Priority.
// For example, implemented by tracedata.TraceData to indicate its caching priority.
type Getter interface {
	GetPriority() Priority
}

// Priority indicates the priority of a sampling decision
// We use small uint8 type because it will be stored in cache
// Higher priority should be represented by higher integer value.
type Priority uint8

const (
	_ Priority = iota
	// Low indicates that the trace has a low priority as it is unlikely to be sampled
	Low
	// Unspecified indicates that the priority of a sampling decision has not been specified.
	Unspecified
)

func (p Priority) String() string {
	switch p {
	case Unspecified:
		return "Unspecified"
	case Low:
		return "Low"
	}
	return ""
}
