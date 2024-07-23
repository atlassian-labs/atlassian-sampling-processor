// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package evaluators // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

type spanCount struct {
	minSpans int
}

var _ PolicyEvaluator = (*spanCount)(nil)

// NewSpanCount creates a policy evaluator sampling traces with more than one span per trace
func NewSpanCount(minSpans int) PolicyEvaluator {
	return &spanCount{
		minSpans: minSpans,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (sc *spanCount) Evaluate(_ context.Context, id pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) (Decision, error) {
	if int(mergedMetadata.SpanCount.Load()) >= sc.minSpans {
		return Sampled, nil
	}
	return Pending, nil
}
