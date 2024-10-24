// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package evaluators // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/internal/ptraceutil"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

type statusCodeEvaluator struct {
	StartFunc
	statusCodes []ptrace.StatusCode
}

var _ PolicyEvaluator = (*statusCodeEvaluator)(nil)

// NewStatusCodeEvaluator creates a policy evaluator that samples all traces with
// a given status code.
func NewStatusCodeEvaluator(statusCodeString []string) (PolicyEvaluator, error) {
	if len(statusCodeString) == 0 {
		return nil, errors.New("expected at least one status code to filter on")
	}

	statusCodes := make([]ptrace.StatusCode, len(statusCodeString))

	for i := range statusCodeString {
		switch statusCodeString[i] {
		case "OK":
			statusCodes[i] = ptrace.StatusCodeOk
		case "ERROR":
			statusCodes[i] = ptrace.StatusCodeError
		case "UNSET":
			statusCodes[i] = ptrace.StatusCodeUnset
		default:
			return nil, fmt.Errorf("unknown status code %q, supported: OK, ERROR, UNSET", statusCodeString[i])
		}
	}

	return &statusCodeEvaluator{
		statusCodes: statusCodes,
	}, nil
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (sce *statusCodeEvaluator) Evaluate(_ context.Context, _ pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) (Decision, error) {
	for item := range ptraceutil.TraceIterator(currentTrace) {
		for _, statusCode := range sce.statusCodes {
			if item.Span.Status().Code() == statusCode {
				return Sampled, nil
			}
		}
	}

	return Pending, nil
}
