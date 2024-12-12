// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package evaluators // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
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
	// LowPriority indicates that this trace is unlikely to be eventually sampled.
	LowPriority
)

func (d Decision) String() string {
	switch d {
	case Unspecified:
		return "Unspecified"
	case Pending:
		return "Pending"
	case Sampled:
		return "Sampled"
	case NotSampled:
		return "NotSampled"
	case LowPriority:
		return "LowPriority"
	}
	return ""
}

func StringToDecision(s string) (Decision, error) {
	switch s {
	case "Unspecified":
		return Unspecified, nil
	case "Pending":
		return Pending, nil
	case "Sampled":
		return Sampled, nil
	case "NotSampled":
		return NotSampled, nil
	case "LowPriority":
		return LowPriority, nil
	}
	return Unspecified, fmt.Errorf("expected valid decision string, got: %s", s)
}

// PolicyEvaluator implements a tail-based sampling policy evaluator,
// which makes a sampling decision for a given trace when requested.
type PolicyEvaluator interface {
	// Start is called when the component is first started.
	// It allows the evaluator to perform any configuration that requires the host, e.g. accessing extensions.
	Start(ctx context.Context, host component.Host) error

	// Evaluate looks at the trace data and merged metadata, and returns a corresponding SamplingDecision.
	// The cached trace data is not passed on to avoid evaluators going through the old spans.
	// The metadata should contain all relevant information about the old spans needed to make a decision.
	Evaluate(ctx context.Context, traceID pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) (Decision, error)
}

// StartFunc specifies the function invoked when the evaluator is being started.
type StartFunc func(context.Context, component.Host) error

func (f StartFunc) Start(ctx context.Context, host component.Host) error {
	if f == nil {
		return nil
	}
	return f(ctx, host)
}
