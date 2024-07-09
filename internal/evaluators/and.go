// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package evaluators // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

type andEvaluator struct {
	// the subPolicy evaluators
	subpolicies []PolicyEvaluator
}

func NewAndEvaluator(
	subpolicies []PolicyEvaluator,
) PolicyEvaluator {
	return &andEvaluator{
		subpolicies: subpolicies,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (c *andEvaluator) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *tracedata.TraceData) (Decision, error) {
	// The policy iterates over all sub-policies and returns Sampled if all sub-policies returned a Sampled Decision.
	// If any subPolicy returns NotSampled or InvertNotSampled it returns that
	for _, sub := range c.subpolicies {
		decision, err := sub.Evaluate(ctx, traceID, trace)
		if err != nil {
			return Unspecified, err
		}
		if decision != Sampled {
			return decision, nil
		}
	}
	return Sampled, nil
}
