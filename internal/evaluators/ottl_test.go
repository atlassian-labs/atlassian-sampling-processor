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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestOTTLEvaluator(t *testing.T) {
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc                string
		SpanConditions      []string
		SpanEventConditions []string
		Spans               []spanWithAttributes
		WantErr             bool
		Decision            Decision
	}{
		{
			// policy
			"OTTL conditions not set",
			[]string{},
			[]string{},
			[]spanWithAttributes{{SpanAttributes: map[string]string{"attr_k_1": "attr_v_1"}}},
			true,
			Pending,
		},
		{
			"OTTL conditions match specific span attributes 1",
			[]string{"attributes[\"attr_k_1\"] == \"attr_v_1\""},
			[]string{},
			[]spanWithAttributes{{SpanAttributes: map[string]string{"attr_k_1": "attr_v_1"}}},
			false,
			Sampled,
		},
		{
			"OTTL conditions match specific span attributes 2",
			[]string{"attributes[\"attr_k_1\"] != \"attr_v_1\""},
			[]string{},
			[]spanWithAttributes{{SpanAttributes: map[string]string{"attr_k_1": "attr_v_1"}}},
			false,
			Pending,
		},
		{
			"OTTL conditions inverse match(!=) span attributes 2",
			[]string{"attributes[\"attr_k_1\"] != \"attr_v_1\""},
			[]string{},
			[]spanWithAttributes{{SpanAttributes: map[string]string{"attr_k_1": "attr_v_2"}}},
			false,
			Sampled,
		},
		{
			"OTTL conditions match specific span event attributes",
			[]string{},
			[]string{"attributes[\"event_attr_k_1\"] == \"event_attr_v_1\""},
			[]spanWithAttributes{{SpanEventAttributes: map[string]string{"event_attr_k_1": "event_attr_v_1"}}},
			false,
			Sampled,
		},
		{
			"OTTL conditions match specific span event name",
			[]string{},
			[]string{"name != \"incorrect event name\""},
			[]spanWithAttributes{{SpanEventAttributes: nil}},
			false,
			Sampled,
		},
		{
			"OTTL conditions not matched",
			[]string{"attributes[\"attr_k_1\"] == \"attr_v_1\""},
			[]string{"attributes[\"event_attr_k_1\"] == \"event_attr_v_1\""},
			[]spanWithAttributes{},
			false,
			Pending,
		},
		{
			"OTTL conditions invalid",
			[]string{"invalid_expr"},
			[]string{},
			[]spanWithAttributes{},
			true,
			Pending,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			filter, err := NewOTTLConditionEvaluator(c.SpanConditions, c.SpanEventConditions, ottl.IgnoreError)
			assert.Equal(t, err != nil, c.WantErr)

			if err == nil {
				decision, err := filter.Evaluate(context.Background(), traceID, newTraceWithSpansAttributes(c.Spans), &tracedata.Metadata{})
				assert.Equal(t, err != nil, c.WantErr)
				assert.Equal(t, decision, c.Decision)
			}
		})
	}
}

type spanWithAttributes struct {
	SpanAttributes      map[string]string
	SpanEventAttributes map[string]string
}

func newTraceWithSpansAttributes(spans []spanWithAttributes) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	for _, s := range spans {
		span := ils.Spans().AppendEmpty()
		span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
		for k, v := range s.SpanAttributes {
			span.Attributes().PutStr(k, v)
		}
		spanEvent := span.Events().AppendEmpty()
		spanEvent.SetName("test event")
		for k, v := range s.SpanEventAttributes {
			spanEvent.Attributes().PutStr(k, v)
		}
	}

	return traces
}
