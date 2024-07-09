package evaluators // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"
	"fmt"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/cache"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

var emptySpanID = pcommon.NewSpanIDEmpty()

type rootSpansEvaluator[T any] struct {
	subPolicy  PolicyEvaluator
	cachedData cache.Cache[T]
}

var _ PolicyEvaluator = (*rootSpansEvaluator[any])(nil)

func NewRootSpan[T any](
	subPolicy PolicyEvaluator,
	cachedData cache.Cache[T],
) PolicyEvaluator {
	return &rootSpansEvaluator[T]{
		subPolicy:  subPolicy,
		cachedData: cachedData,
	}
}

// Evaluate evaluates if a span is a root span without children.
// If it has no children, it evaluates the sub policy turning Pending decisions into NotSampled.
func (r *rootSpansEvaluator[T]) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *tracedata.TraceData) (Decision, error) {
	if trace.SpanCount.Load() != 1 {
		return Pending, nil
	}
	if _, ok := r.cachedData.Get(traceID); ok {
		return Pending, nil
	}

	// we know there's only one span so check that span if it's a root span
	onlySpan := findOnlySpan(trace.ReceivedBatches)
	if onlySpan == nil {
		return Unspecified, fmt.Errorf("span count was 1 but no span in trace data, tracedata invalid/corrupt")
	}
	if !isRootSpan(*onlySpan) {
		return Pending, nil
	}

	subDecision, err := r.subPolicy.Evaluate(ctx, traceID, trace)
	if err != nil {
		return Unspecified, err
	}

	if subDecision == NotSampled || subDecision == Pending {
		return NotSampled, nil
	}
	return Pending, nil
}

// findOnlySpan returns the first span it sees in the traces.
// This will usually just be the zeroth element, but in the example case that there's
// a resource span with no scope spans, or a scope spans with no spans, it copes.
func findOnlySpan(trace ptrace.Traces) *ptrace.Span {
	for i := 0; i < trace.ResourceSpans().Len(); i++ {
		ss := trace.ResourceSpans().At(i).ScopeSpans()
		for j := 0; j < ss.Len(); j++ {
			spans := trace.ResourceSpans().At(i).ScopeSpans().At(j).Spans()
			if spans.Len() > 0 {
				span := spans.At(0)
				return &span
			}
		}
	}
	return nil
}

func isRootSpan(span ptrace.Span) bool {
	parentSpan := span.ParentSpanID()

	if slices.Equal(parentSpan[:], emptySpanID[:]) {
		return true
	}

	traceID := span.TraceID()
	rightHalfTraceID := traceID[8:16]
	return slices.Equal(parentSpan[:], rightHalfTraceID[:])
}
