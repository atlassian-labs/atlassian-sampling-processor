package ptraceutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestEmptyTraceIterator(t *testing.T) {
	traces := ptrace.NewTraces()
	for range TraceIterator(traces) {
		t.Errorf("empty trace iterator, we should not get to this code path")
	}
}

func TestTraceIterator(t *testing.T) {
	trace := ptrace.NewTraces()
	rs1 := trace.ResourceSpans().AppendEmpty()
	rs1.Resource().Attributes().PutStr("name", "rs1")
	spans1 := rs1.ScopeSpans().AppendEmpty().Spans()
	spans1.AppendEmpty().SetName("a")
	spans1.AppendEmpty().SetName("b")

	rs2 := trace.ResourceSpans().AppendEmpty()
	rs2.Resource().Attributes().PutStr("name", "rs2")
	ss1 := rs2.ScopeSpans().AppendEmpty()
	ss1.Spans().AppendEmpty().SetName("c")
	ss2 := rs2.ScopeSpans().AppendEmpty()
	ss2.Spans().AppendEmpty().SetName("d")

	var items []*SpanItem
	for item := range TraceIterator(trace) {
		items = append(items, item)
	}

	assert.NoError(t, ptracetest.CompareResourceSpans(rs1, items[0].ResourceSpans))
	assert.NoError(t, ptracetest.CompareScopeSpans(rs1.ScopeSpans().At(0), items[0].ScopeSpans))
	assert.NoError(t, ptracetest.CompareSpan(spans1.At(0), items[0].Span))

	assert.NoError(t, ptracetest.CompareResourceSpans(rs1, items[1].ResourceSpans))
	assert.NoError(t, ptracetest.CompareScopeSpans(rs1.ScopeSpans().At(0), items[1].ScopeSpans))
	assert.NoError(t, ptracetest.CompareSpan(spans1.At(1), items[1].Span))

	assert.NoError(t, ptracetest.CompareResourceSpans(rs2, items[2].ResourceSpans))
	assert.NoError(t, ptracetest.CompareScopeSpans(ss1, items[2].ScopeSpans))
	assert.NoError(t, ptracetest.CompareSpan(ss1.Spans().At(0), items[2].Span))

	assert.NoError(t, ptracetest.CompareResourceSpans(rs2, items[3].ResourceSpans))
	assert.NoError(t, ptracetest.CompareScopeSpans(ss2, items[3].ScopeSpans))
	assert.NoError(t, ptracetest.CompareSpan(ss2.Spans().At(0), items[3].Span))
}
