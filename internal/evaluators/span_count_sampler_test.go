// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package evaluators

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

func TestEvaluate(t *testing.T) {
	minSpans := 3

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc             string
		NumberSpans      int
		NumberCachedSpan int
		Decision         Decision
	}{
		{
			"Spans less than the minSpans, in one single batch",
			1,
			0,
			Pending,
		},
		{
			"Same number of spans as the minSpans, in one single batch",
			3,
			0,
			Sampled,
		},
		{
			"Combined with cached data satisfies min spans",
			2,
			1,
			Sampled,
		},
	}

	evaluator := NewSpanCount(minSpans)

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			decision, err := evaluator.Evaluate(context.Background(), traceID, newTraceWithMultipleSpans(c.NumberSpans), newMergedMetadata(c.NumberSpans, c.NumberCachedSpan))
			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func newTraceWithMultipleSpans(numberSpans int) ptrace.Traces {
	// For each resource, going to create the number of spans defined in the array
	traces := ptrace.NewTraces()
	for i := 0; i < numberSpans; i++ {
		// Creates trace
		rs := traces.ResourceSpans().AppendEmpty()
		ils := rs.ScopeSpans().AppendEmpty()

		span := ils.Spans().AppendEmpty()
		span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	}

	return traces
}

func newMergedMetadata(numberSpans int, numberCachedSpans int) *tracedata.Metadata {
	sc := &atomic.Int64{}
	sc.Store(int64(numberSpans + numberCachedSpans))
	return &tracedata.Metadata{
		SpanCount: sc,
	}
}
