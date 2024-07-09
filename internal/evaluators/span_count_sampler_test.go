// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package evaluators

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/cache"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/metadata"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

func TestEvaluate(t *testing.T) {
	minSpans := 3

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc        string
		NumberSpans int
		Evaluator   PolicyEvaluator
		Decision    Decision
	}{
		{
			"Spans less than the minSpans, in one single batch",
			1,
			NewSpanCount(minSpans, cache.NewNopDecisionCache[*tracedata.TraceData]()),
			Pending,
		},
		{
			"Same number of spans as the minSpans, in one single batch",
			3,
			NewSpanCount(minSpans, cache.NewNopDecisionCache[*tracedata.TraceData]()),
			Sampled,
		},
		{
			"Combined with cached data satisfies min spans",
			2,
			func() PolicyEvaluator {
				tel, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
				c, _ := cache.NewLRUCache[*tracedata.TraceData](10, func(_ uint64, _ *tracedata.TraceData) {}, tel)
				c.Put(traceID, newTraceWithMultipleSpans(2))
				return NewSpanCount(minSpans, c)
			}(),
			Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			decision, err := c.Evaluator.Evaluate(context.Background(), traceID, newTraceWithMultipleSpans(c.NumberSpans))
			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func newTraceWithMultipleSpans(numberSpans int) *tracedata.TraceData {
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

	return tracedata.NewTraceData(time.Now(), traces)
}
