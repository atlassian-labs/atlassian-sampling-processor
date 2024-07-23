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

// Decision gives the status of sampling decision.
type Decision int32

const (
	// Unspecified indicates that the policy has not been evaluated yet.
	Unspecified Decision = iota
	// Pending indicates that the final decision is still pending.
	Pending
	// Sampled is used to indicate that a final decision was made and the trace should be sent.
	Sampled
	// NotSampled is used to indicate that the decision has been taken to not sample the data, and it should be dropped.
	NotSampled
)

func (d *Decision) String() string {
	switch *d {
	case Unspecified:
		return "Unspecified"
	case Pending:
		return "Pending"
	case Sampled:
		return "Sampled"
	case NotSampled:
		return "NotSampled"
	}
	return ""
}

// PolicyEvaluator implements a tail-based sampling policy evaluator,
// which makes a sampling decision for a given trace when requested.
type PolicyEvaluator interface {
	// Evaluate looks at the trace data and merged metadata, and returns a corresponding SamplingDecision.
	// The cached trace data is not passed on to avoid evaluators going through the old spans.
	// The metadata should contain all relevant information about the old spans needed to make a decision.
	Evaluate(ctx context.Context, traceID pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) (Decision, error)
}
