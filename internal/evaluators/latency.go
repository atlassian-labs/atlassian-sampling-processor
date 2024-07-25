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

type latency struct {
	thresholdMs int64
}

var _ PolicyEvaluator = (*latency)(nil)

// NewLatency creates a policy evaluator sampling traces with a duration equal to or higher than the configured threshold
func NewLatency(thresholdMs int64) PolicyEvaluator {
	return &latency{
		thresholdMs: thresholdMs,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (l *latency) Evaluate(_ context.Context, _ pcommon.TraceID, _ ptrace.Traces, mergedMetadata *tracedata.Metadata) (Decision, error) {
	start := mergedMetadata.EarliestStartTime.AsTime()
	end := mergedMetadata.LatestEndTime.AsTime()
	duration := end.Sub(start)

	if duration.Milliseconds() >= l.thresholdMs {
		return Sampled, nil
	}

	return Pending, nil
}
