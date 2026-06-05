// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package atlassiansamplingprocessor // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// spanAndScope a structure for holding information about span and its instrumentation scope.
// required for preserving the instrumentation library information while sampling.
// We use pointers there to fast find the span in the map.
type spanAndScope struct {
	span                 *ptrace.Span
	instrumentationScope *pcommon.InstrumentationScope
}

func groupSpansByTraceID(resourceSpans ptrace.ResourceSpans) spansGroupedByTraceID {
	idToSpans := make(map[pcommon.TraceID][]spanAndScope)
	ilss := resourceSpans.ScopeSpans()
	for j := 0; j < ilss.Len(); j++ {
		scope := ilss.At(j)
		spans := scope.Spans()
		is := scope.Scope()
		spansLen := spans.Len()
		for k := 0; k < spansLen; k++ {
			span := spans.At(k)
			key := span.TraceID()
			idToSpans[key] = append(idToSpans[key], spanAndScope{
				span:                 &span,
				instrumentationScope: &is,
			})
		}
	}
	return idToSpans
}

// appendAndMoveToTraces appends the given spans onto dest, grouped under a single
// ResourceSpans for the given resource.
//
// IMPORTANT - DO NOT USE spanAndScopes after calling this method.
// This function MOVES each *ptrace.Span out of spanAndScopes into dest.
//
// The Resource and InstrumentationScope are shared across different trace IDs
// from the same incoming ResourceSpans so they must be copied instead of moved.
func appendAndMoveToTraces(dest ptrace.Traces, resource pcommon.Resource, spanAndScopes []spanAndScope) {
	rs := dest.ResourceSpans().AppendEmpty()
	resource.CopyTo(rs.Resource())

	scopePointerToNewScope := make(map[*pcommon.InstrumentationScope]*ptrace.ScopeSpans)
	for i := range spanAndScopes {
		ss := &spanAndScopes[i]
		// If the scope of the spanAndScope is not in the map, add it to the map and the destination.
		if scope, ok := scopePointerToNewScope[ss.instrumentationScope]; !ok {
			is := rs.ScopeSpans().AppendEmpty()
			ss.instrumentationScope.CopyTo(is.Scope())
			scopePointerToNewScope[ss.instrumentationScope] = &is

			sp := is.Spans().AppendEmpty()
			ss.span.MoveTo(sp)
		} else {
			sp := scope.Spans().AppendEmpty()
			ss.span.MoveTo(sp)
		}
		// the span is moved - explicitly nil it out to loudly prevent access
		ss.span = nil
	}
}
