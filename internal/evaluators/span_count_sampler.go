// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package evaluators // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/cache"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

type spanCount struct {
	minSpans   int
	cachedData cache.Cache[*tracedata.TraceData]
}

var _ PolicyEvaluator = (*spanCount)(nil)

// NewSpanCount creates a policy evaluator sampling traces with more than one span per trace
func NewSpanCount(minSpans int, cachedData cache.Cache[*tracedata.TraceData]) PolicyEvaluator {
	return &spanCount{
		minSpans:   minSpans,
		cachedData: cachedData,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (sc *spanCount) Evaluate(_ context.Context, id pcommon.TraceID, traceData *tracedata.TraceData) (Decision, error) {
	var cachedCount int64
	if cachedTraceData, ok := sc.cachedData.Get(id); ok {
		cachedCount = cachedTraceData.SpanCount.Load()
	}
	if int(traceData.SpanCount.Load()+cachedCount) >= sc.minSpans {
		return Sampled, nil
	}
	return Pending, nil
}
