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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

func TestLatencyEvaluator(t *testing.T) {
	evaluator := NewLatency(5000)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	now := time.Now()

	cases := []struct {
		Desc      string
		StartTime time.Time
		Duration  time.Duration
		Decision  Decision
	}{
		{
			"trace duration shorter than threshold",
			now,
			4500 * time.Millisecond,
			Pending,
		},
		{
			"trace duration is equal to threshold",
			now,
			5000 * time.Millisecond,
			Sampled,
		},
		{
			"trace duration is longer than threshold",
			now,
			8000 * time.Millisecond,
			Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			traces := ptrace.NewTraces()

			mergedMetadata := &tracedata.Metadata{
				EarliestStartTime: pcommon.NewTimestampFromTime(c.StartTime),
				LatestEndTime:     pcommon.NewTimestampFromTime(c.StartTime.Add(c.Duration)),
			}
			decision, err := evaluator.Evaluate(context.Background(), traceID, traces, mergedMetadata)

			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}
