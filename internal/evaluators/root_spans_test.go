package evaluators

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/priority"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

var (
	low                         = priority.Low
	unspecified                 = priority.Unspecified
	testSpanID  pcommon.SpanID  = [8]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	testTraceID pcommon.TraceID = [16]byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}
)

func TestRootSpanEvaluator(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		spanCount      int32
		traceID        pcommon.TraceID
		trace          ptrace.Traces
		subPolicy      PolicyEvaluator
		priority       *priority.Priority
		expectDecision Decision
		expectErr      bool
	}{
		{
			name:      "Lonely root span returns low priority",
			spanCount: 1,
			traceID:   pcommon.NewTraceIDEmpty(),
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				return trace
			}(),
			subPolicy:      &staticTestEvaluator{d: Pending},
			expectDecision: LowPriority,
			expectErr:      false,
		},
		{
			name:      "Non root span should return pending, with no pre-existing metadata",
			spanCount: 1,
			traceID:   pcommon.NewTraceIDEmpty(),
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				span := trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetParentSpanID(testSpanID)
				return trace
			}(),
			subPolicy:      &staticTestEvaluator{d: Pending},
			expectDecision: Pending,
			expectErr:      false,
		},
		{
			name:      "Non root span should return low priority, with existing low priority metadata and pending sub decision",
			spanCount: 1,
			traceID:   pcommon.NewTraceIDEmpty(),
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				span := trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetParentSpanID(testSpanID)
				return trace
			}(),
			subPolicy:      &staticTestEvaluator{d: Pending},
			priority:       &low,
			expectDecision: LowPriority,
			expectErr:      false,
		},
		{
			name:      "Non root span should return pending, with existing low priority metadata and sampled sub decision",
			spanCount: 1,
			traceID:   pcommon.NewTraceIDEmpty(),
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				span := trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetParentSpanID(testSpanID)
				return trace
			}(),
			subPolicy:      &staticTestEvaluator{d: Sampled},
			priority:       &low,
			expectDecision: Pending,
			expectErr:      false,
		},
		{
			name:      "More than one span in trace should return Pending, with no pre existing metadata",
			spanCount: 2,
			traceID:   pcommon.NewTraceIDEmpty(),
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				spans := trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
				spans.AppendEmpty()
				spans.AppendEmpty()
				return trace
			}(),
			subPolicy:      staticTestEvaluator{d: Sampled},
			expectDecision: Pending,
			expectErr:      false,
		},
		{
			name:      "Other spans from trace present in cache",
			spanCount: 10,
			traceID:   testTraceID,
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				return trace
			}(),
			subPolicy:      staticTestEvaluator{d: Pending},
			expectDecision: Pending,
			expectErr:      false,
		},
		{
			name:      "Sub policy returns error",
			spanCount: 1,
			traceID:   pcommon.NewTraceIDEmpty(),
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				return trace
			}(),
			subPolicy:      staticTestEvaluator{d: Unspecified, e: fmt.Errorf("test err")},
			expectDecision: Unspecified,
			expectErr:      true,
		},
		{
			name:           "Span count = 1, but trace doesnt actually have span",
			spanCount:      1,
			traceID:        pcommon.NewTraceIDEmpty(),
			trace:          ptrace.NewTraces(),
			subPolicy:      staticTestEvaluator{d: Pending},
			expectDecision: Unspecified,
			expectErr:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evaluator := NewRootSpan(test.subPolicy)

			p := test.priority
			if p == nil {
				p = &unspecified
			}
			metadata := &tracedata.Metadata{SpanCount: test.spanCount, Priority: *p}

			decision, err := evaluator.Evaluate(
				context.Background(),
				test.traceID,
				test.trace,
				metadata)
			assert.Equal(t, test.expectDecision, decision)
			assert.Equal(t, test.expectErr, err != nil)
		})
	}
}

func TestRootSpanEvaluatorErrored(t *testing.T) {
	eval := NewRootSpan(staticTestEvaluator{d: Unspecified, e: fmt.Errorf("test error")})

	// Create trace with one span, so it can be classified as lone root span
	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	metadata := &tracedata.Metadata{SpanCount: 1}

	decision, err := eval.Evaluate(
		context.Background(),
		pcommon.NewTraceIDEmpty(),
		trace,
		metadata)
	require.Error(t, err)
	assert.Equal(t, Unspecified, decision)
}

func TestFindOnlySpan(t *testing.T) {
	trace := ptrace.NewTraces()
	span := trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	assert.Equal(t, &span, findOnlySpan(trace))

	trace = ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty() // empty resource spans
	span = trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	assert.Equal(t, &span, findOnlySpan(trace))

	trace = ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty() // empty scope spans
	span = trace.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	assert.Equal(t, &span, findOnlySpan(trace))
}

type staticTestEvaluator struct {
	StartFunc
	d Decision
	e error
}

func (s staticTestEvaluator) Evaluate(_ context.Context, _ pcommon.TraceID, _ ptrace.Traces, _ *tracedata.Metadata) (Decision, error) {
	return s.d, s.e
}
