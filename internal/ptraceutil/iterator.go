package ptraceutil // import "github.com/atlassian-labs/atlassian-sampling-processor/internal/ptraceutil"

import (
	"iter"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

type SpanItem struct {
	ResourceSpans ptrace.ResourceSpans
	ScopeSpans    ptrace.ScopeSpans
	Span          ptrace.Span
}

func TraceIterator(td ptrace.Traces) iter.Seq[*SpanItem] {
	return func(yield func(*SpanItem) bool) {
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			rs := td.ResourceSpans().At(i)
			for ii := 0; ii < rs.ScopeSpans().Len(); ii++ {
				ss := rs.ScopeSpans().At(ii)
				for iii := 0; iii < ss.Spans().Len(); iii++ {
					span := ss.Spans().At(iii)
					item := &SpanItem{
						ResourceSpans: rs,
						ScopeSpans:    ss,
						Span:          span,
					}
					if !yield(item) {
						return
					}
				}
			}
		}
	}
}
