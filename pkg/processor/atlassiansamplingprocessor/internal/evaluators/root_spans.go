package evaluators // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"
	"fmt"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/priority"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

var emptySpanID = pcommon.NewSpanIDEmpty()

type rootSpansEvaluator struct {
	StartFunc
	subPolicy PolicyEvaluator
}

var _ PolicyEvaluator = (*rootSpansEvaluator)(nil)

func NewRootSpan(subPolicy PolicyEvaluator) PolicyEvaluator {
	return &rootSpansEvaluator{
		subPolicy: subPolicy,
	}
}

// Evaluate evaluates a sub policy, and if it returns Sampled, converts the decision to Pending.
// Then, if the trace data only contains a single root span with no children, then the decision is LowPriority.
// Once the metadata of this mergedMetadata has a LowPriority decision, it must get a Sampled result
// from the sub policy to be promoted to a regular Pending.
func (r *rootSpansEvaluator) Evaluate(ctx context.Context, traceID pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) (Decision, error) {
	defaultDecision := Pending
	if mergedMetadata.Priority == priority.Low {
		defaultDecision = LowPriority
	}

	subDecision, err := r.subPolicy.Evaluate(ctx, traceID, currentTrace, mergedMetadata)
	if err != nil {
		return Unspecified, err
	}
	if subDecision == Sampled {
		return Pending, nil
	}

	if mergedMetadata.SpanCount != 1 {
		return defaultDecision, nil
	}
	// we know there's only one span so check that span if it's a root span
	onlySpan := findOnlySpan(currentTrace)
	if onlySpan == nil {
		return Unspecified, fmt.Errorf("span count was 1 but no span in trace data, tracedata invalid/corrupt")
	}
	if isRootSpan(*onlySpan) {
		return LowPriority, nil
	}
	return defaultDecision, nil
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
