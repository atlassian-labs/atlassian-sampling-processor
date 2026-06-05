// Copyright © 2024 Atlassian Pty Ltd.
// SPDX-License-Identifier: Apache-2.0

package atlassiansamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TestAppendAndMoveToTraces_MovesSpansAndPreservesContent verifies that span
// attributes/events/links are correctly reparented into the destination — i.e.
// that the move-based implementation produces output equivalent to the
// previous deep-copy implementation.
func TestAppendAndMoveToTraces_MovesSpansAndPreservesContent(t *testing.T) {
	src := makeTracesForMoveTest(t)
	rs := src.ResourceSpans().At(0)
	resource := rs.Resource()
	spans := flattenScopeSpans(rs.ScopeSpans())

	dest := ptrace.NewTraces()
	appendAndMoveToTraces(dest, resource, spans)

	require.Equal(t, 1, dest.ResourceSpans().Len())
	destRS := dest.ResourceSpans().At(0)

	// Resource is copied over.
	v, ok := destRS.Resource().Attributes().Get("service.name")
	require.True(t, ok)
	assert.Equal(t, "svc", v.Str())

	// All spans landed in the destination, grouped under their scope.
	require.Equal(t, 1, destRS.ScopeSpans().Len(), "expected a single scope group since all spans share one scope")
	destScope := destRS.ScopeSpans().At(0)
	assert.Equal(t, "scope-a", destScope.Scope().Name())
	require.Equal(t, 3, destScope.Spans().Len())

	// Verify span data made it across (attributes, events, links).
	for i := 0; i < destScope.Spans().Len(); i++ {
		sp := destScope.Spans().At(i)
		attr, ok := sp.Attributes().Get("attr.k")
		require.True(t, ok, "span %d missing expected attribute", i)
		assert.NotEmpty(t, attr.Str())
		assert.Equal(t, 1, sp.Events().Len(), "span %d missing event", i)
		assert.Equal(t, 1, sp.Links().Len(), "span %d missing link", i)
	}
}

// TestAppendAndMoveToTraces_MultipleTracesSharingScope verifies the realistic
// scenario where multiple trace IDs share the same instrumentation scope
// pointer (because they came from the same ScopeSpans batch). Each trace ID's
// slice is moved independently into its own destination, and the shared scope
// is correctly copied into each.
func TestAppendAndMoveToTraces_MultipleTracesSharingScope(t *testing.T) {
	// One ResourceSpans, one ScopeSpans, two trace IDs (2 spans each).
	src := ptrace.NewTraces()
	rs := src.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "svc")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("shared-scope")

	traceIDs := []pcommon.TraceID{
		{0x01}, {0x02},
	}
	for _, tid := range traceIDs {
		for j := 0; j < 2; j++ {
			sp := ss.Spans().AppendEmpty()
			sp.SetTraceID(tid)
			sp.Attributes().PutStr("tid", string(tid[:1]))
		}
	}

	// Group by trace ID — this is what ConsumeTraces does in production.
	grouped := groupSpansByTraceID(rs)
	require.Len(t, grouped, 2, "expected two trace ID buckets")

	// Sanity-check: the two buckets share the same *InstrumentationScope pointer.
	var firstScopePtr *pcommon.InstrumentationScope
	for _, spans := range grouped {
		require.NotEmpty(t, spans)
		if firstScopePtr == nil {
			firstScopePtr = spans[0].instrumentationScope
		} else {
			assert.Same(t, firstScopePtr, spans[0].instrumentationScope,
				"different trace IDs in the same ScopeSpans must share the scope pointer")
		}
	}

	// Move each bucket into its own destination, in arbitrary map-iteration
	// order. This mimics the per-shard processTraces path.
	for tid, spans := range grouped {
		dest := ptrace.NewTraces()
		appendAndMoveToTraces(dest, rs.Resource(), spans)

		require.Equal(t, 1, dest.ResourceSpans().Len())
		require.Equal(t, 1, dest.ResourceSpans().At(0).ScopeSpans().Len())
		destScope := dest.ResourceSpans().At(0).ScopeSpans().At(0)
		assert.Equal(t, "shared-scope", destScope.Scope().Name(),
			"scope must be copied into each destination even though source is shared")
		require.Equal(t, 2, destScope.Spans().Len(), "expected 2 spans for trace %v", tid)
		for i := 0; i < destScope.Spans().Len(); i++ {
			sp := destScope.Spans().At(i)
			assert.Equal(t, tid, sp.TraceID(),
				"every moved span must carry its source trace ID, not a hollow zero-valued span")
		}
	}
}

// makeTracesForMoveTest produces a single ResourceSpans with a single
// ScopeSpans containing 3 spans, each with one attribute, one event, and one
// link. Used as a realistic input for the move tests.
func makeTracesForMoveTest(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "svc")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("scope-a")

	var tid pcommon.TraceID
	tid[0] = 0xAA

	for i := 0; i < 3; i++ {
		sp := ss.Spans().AppendEmpty()
		sp.SetTraceID(tid)
		sp.SetName("span")
		sp.Attributes().PutStr("attr.k", "attr.v")
		ev := sp.Events().AppendEmpty()
		ev.SetName("event")
		ln := sp.Links().AppendEmpty()
		ln.SetTraceID(tid)
	}
	return td
}

// flattenScopeSpans mirrors what groupSpansByTraceID produces when all spans
// belong to one trace ID.
func flattenScopeSpans(ilss ptrace.ScopeSpansSlice) []spanAndScope {
	var out []spanAndScope
	for j := 0; j < ilss.Len(); j++ {
		scope := ilss.At(j)
		is := scope.Scope()
		spans := scope.Spans()
		for k := 0; k < spans.Len(); k++ {
			sp := spans.At(k)
			out = append(out, spanAndScope{
				span:                 &sp,
				instrumentationScope: &is,
			})
		}
	}
	return out
}
