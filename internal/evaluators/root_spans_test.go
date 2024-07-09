package evaluators

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/cache"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/metadata"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

var testSpanID pcommon.SpanID = [8]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
var testTraceID pcommon.TraceID = [16]byte{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
}

func TestRootSpanEvaluator(t *testing.T) {
	tests := []struct {
		name           string
		cach           cache.Cache[any]
		traceID        pcommon.TraceID
		trace          ptrace.Traces
		subPolicy      PolicyEvaluator
		expectDecision Decision
		expectErr      bool
	}{
		{
			name:    "Not root span",
			cach:    newTestCache(t),
			traceID: pcommon.NewTraceIDEmpty(),
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				span := trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetParentSpanID(testSpanID)
				return trace
			}(),
			subPolicy:      &errorPolicy{},
			expectDecision: Pending,
			expectErr:      false,
		},
		{
			name:    "More than one span in trace",
			cach:    newTestCache(t),
			traceID: pcommon.NewTraceIDEmpty(),
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				spans := trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
				spans.AppendEmpty()
				spans.AppendEmpty()
				return trace
			}(),
			subPolicy:      NewProbabilisticSampler("", 100.0),
			expectDecision: Pending,
			expectErr:      false,
		},
		{
			name: "Other spans from trace present in cache",
			cach: func() cache.Cache[any] {
				c := newTestCache(t)
				c.Put(testTraceID, struct{}{})
				return c
			}(),
			traceID: testTraceID,
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				return trace
			}(),
			subPolicy:      NewProbabilisticSampler("", 100.0),
			expectDecision: Pending,
			expectErr:      false,
		},
		{
			name:    "Dont sample when sub policy returns pending",
			cach:    newTestCache(t),
			traceID: testTraceID,
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				return trace
			}(),
			subPolicy:      NewProbabilisticSampler("", 0),
			expectDecision: NotSampled,
			expectErr:      false,
		},
		{
			name:    "Sub policy returns error",
			cach:    newTestCache(t),
			traceID: pcommon.NewTraceIDEmpty(),
			trace: func() ptrace.Traces {
				trace := ptrace.NewTraces()
				trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				return trace
			}(),
			subPolicy:      &errorPolicy{},
			expectDecision: Unspecified,
			expectErr:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evaluator := NewRootSpan(test.subPolicy, test.cach)
			decision, err := evaluator.Evaluate(
				context.Background(),
				test.traceID,
				tracedata.NewTraceData(time.Now(), test.trace))
			assert.Equal(t, test.expectDecision, decision)
			assert.Equal(t, test.expectErr, err != nil)
		})
	}
}

func TestRootSpanEvaluatorErrored(t *testing.T) {
	c := newTestCache(t)
	eval := NewRootSpan(&errorPolicy{}, c)

	// Create trace with one span, so it can be classified as lone root span
	trace := ptrace.NewTraces()
	trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	decision, err := eval.Evaluate(
		context.Background(),
		pcommon.NewTraceIDEmpty(),
		tracedata.NewTraceData(time.Now(), trace))
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

func newTestCache(t *testing.T) cache.Cache[any] {
	telemetry, err := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	c, err := cache.NewLRUCache[any](10, func(uint64, any) {}, telemetry)
	require.NoError(t, err)
	return c
}

type errorPolicy struct{}

func (ep *errorPolicy) Evaluate(_ context.Context, _ pcommon.TraceID, _ *tracedata.TraceData) (Decision, error) {
	return Unspecified, fmt.Errorf("test error")
}
