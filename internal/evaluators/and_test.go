// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package evaluators

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

var traceID = pcommon.TraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x52, 0x96, 0x9A, 0x89, 0x55, 0x57, 0x1A, 0x3F})

func TestAndEvaluatorNotSampled(t *testing.T) {
	n1 := NewProbabilisticSampler("name", 0)
	n2 := NewProbabilisticSampler("name2", 100)

	and := NewAndEvaluator([]PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &tracedata.TraceData{
		ReceivedBatches: traces,
	}
	decision, err := and.Evaluate(context.Background(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %w", err)
	assert.Equal(t, decision, Pending)

}

func TestAndEvaluatorSampled(t *testing.T) {
	n1 := NewProbabilisticSampler("name", 100)
	n2 := NewProbabilisticSampler("name2", 100)

	and := NewAndEvaluator([]PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value")
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &tracedata.TraceData{
		ReceivedBatches: traces,
	}
	decision, err := and.Evaluate(context.Background(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %w", err)
	assert.Equal(t, decision, Sampled)
}
