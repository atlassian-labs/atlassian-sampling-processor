// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package evaluators

import (
	"context"
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

func TestProbabilisticSampling(t *testing.T) {
	tests := []struct {
		name                       string
		samplingPercentage         float64
		hashSalt                   string
		expectedSamplingPercentage float64
	}{
		{
			"100%",
			100,
			"",
			100,
		},
		{
			"0%",
			0,
			"",
			0,
		},
		{
			"25%",
			25,
			"",
			25,
		},
		{
			"33%",
			33,
			"",
			33,
		},
		{
			"33% - custom salt",
			33,
			"test-salt",
			33,
		},
		{
			"-%50",
			-50,
			"",
			0,
		},
		{
			"150%",
			150,
			"",
			100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceCount := 100_000

			probabilisticSampler := NewProbabilisticSampler(tt.hashSalt, tt.samplingPercentage)

			sampled := 0
			for _, traceID := range genRandomTraceIDs(traceCount) {
				trace := newTraceStringAttrs(nil, "example", "value")
				mergedMetadata := &tracedata.Metadata{}

				decision, err := probabilisticSampler.Evaluate(context.Background(), traceID, trace, mergedMetadata)
				assert.NoError(t, err)

				if decision == Sampled {
					sampled++
				}
			}

			effectiveSamplingPercentage := float32(sampled) / float32(traceCount) * 100
			assert.InDelta(t, tt.expectedSamplingPercentage, effectiveSamplingPercentage, 0.2,
				"Effective sampling percentage is %f, expected %f", effectiveSamplingPercentage, tt.expectedSamplingPercentage,
			)
		})
	}
}

func genRandomTraceIDs(num int) (ids []pcommon.TraceID) {
	r := rand.New(rand.NewSource(1))
	ids = make([]pcommon.TraceID, 0, num)
	for i := 0; i < num; i++ {
		traceID := [16]byte{}
		binary.BigEndian.PutUint64(traceID[:8], r.Uint64())
		binary.BigEndian.PutUint64(traceID[8:], r.Uint64())
		ids = append(ids, pcommon.TraceID(traceID))
	}
	return ids
}

func newTraceStringAttrs(nodeAttrs map[string]any, spanAttrKey string, spanAttrValue string) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	//nolint:errcheck
	rs.Resource().Attributes().FromRaw(nodeAttrs)
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.Attributes().PutStr(spanAttrKey, spanAttrValue)
	return traces
}
