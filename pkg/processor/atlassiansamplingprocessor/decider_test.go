package atlassiansamplingprocessor

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/metadata"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/metadatatest"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

type testPols struct {
	Policies map[string]*policy
}

func setupTestPolicies() testPols {
	return testPols{Policies: map[string]*policy{
		"alwaysPending":              alwaysPending(),
		"alwaysSample":               alwaysSample(),
		"alwaysSampleWithRecordFrom": alwaysSampleWithRecordFrom(),
		"alwaysSampleWithGrouping":   alwaysSampleWithGrouping(),
		"hardNotSampled":             hardNotSampled(),
		"alwaysLowPriority":          alwaysLowPriority(),
		"alwaysError":                alwaysError(),
	}}
}

func newTraceWithService(svc string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", svc)
	rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	return td
}

func policyDecisionDP(policyName string, decision evaluators.Decision, decisionFrom ...string) metricdata.DataPoint[int64] {
	attrs := []attribute.KeyValue{
		attribute.String("policy", policyName),
		attribute.String("decision", decision.String()),
	}
	if len(decisionFrom) > 0 {
		attrs = append(attrs, attribute.String("decision_from", decisionFrom[0]))
	}
	return metricdata.DataPoint[int64]{Value: 1, Attributes: attribute.NewSet(attrs...)}
}

func TestMakeDecision(t *testing.T) {
	t.Parallel()
	testPols := setupTestPolicies()

	tests := []struct {
		name             string
		policies         []*policy
		traces           ptrace.Traces
		expectDecision   evaluators.Decision
		expectPolicy     *policy
		expectDataPoints []metricdata.DataPoint[int64]
	}{
		{
			name: "One pending, one sampled results in overall sampled",
			policies: []*policy{
				testPols.Policies["alwaysPending"],
				testPols.Policies["alwaysSample"],
			},
			traces:         newTraceWithService("atlassian-sampling-service"),
			expectDecision: evaluators.Sampled,
			expectPolicy:   testPols.Policies["alwaysSample"],
			expectDataPoints: []metricdata.DataPoint[int64]{
				policyDecisionDP(testPols.Policies["alwaysSample"].name, evaluators.Sampled, ""),
			},
		},
		{
			name: "All pending results in Pending",
			policies: []*policy{
				testPols.Policies["alwaysPending"],
			},
			traces:         newTraceWithService("my-service"),
			expectDecision: evaluators.Pending,
			expectPolicy:   nil,
		},
		{
			name: "Pending and NotSampled returns NotSampled",
			policies: []*policy{
				testPols.Policies["alwaysPending"],
				testPols.Policies["hardNotSampled"],
			},
			traces:         newTraceWithService("my-service"),
			expectDecision: evaluators.NotSampled,
			expectPolicy:   testPols.Policies["hardNotSampled"],
			expectDataPoints: []metricdata.DataPoint[int64]{
				policyDecisionDP(testPols.Policies["hardNotSampled"].name, evaluators.NotSampled, ""),
			},
		},
		{
			name: "Order matters, sampled first",
			policies: []*policy{
				testPols.Policies["alwaysSample"],
				testPols.Policies["alwaysLowPriority"],
			},
			traces:         newTraceWithService("my-service"),
			expectDecision: evaluators.Sampled,
			expectPolicy:   testPols.Policies["alwaysSample"],
			expectDataPoints: []metricdata.DataPoint[int64]{
				policyDecisionDP(testPols.Policies["alwaysSample"].name, evaluators.Sampled, ""),
			},
		},
		{
			name: "Order matters, NotSampled first",
			policies: []*policy{
				testPols.Policies["hardNotSampled"],
				testPols.Policies["alwaysSample"],
			},
			traces:         newTraceWithService("my-service"),
			expectDecision: evaluators.NotSampled,
			expectPolicy:   testPols.Policies["hardNotSampled"],
			expectDataPoints: []metricdata.DataPoint[int64]{
				policyDecisionDP(testPols.Policies["hardNotSampled"].name, evaluators.NotSampled, ""),
			},
		},
		{
			name: "Order matters, LowPriority first",
			policies: []*policy{
				testPols.Policies["alwaysLowPriority"],
				testPols.Policies["alwaysSample"],
			},
			traces:         newTraceWithService("my-service"),
			expectDecision: evaluators.LowPriority,
			expectPolicy:   testPols.Policies["alwaysLowPriority"],
			expectDataPoints: []metricdata.DataPoint[int64]{
				policyDecisionDP(testPols.Policies["alwaysLowPriority"].name, evaluators.LowPriority),
			},
		},
		{
			name: "On all errors, returns pending",
			policies: []*policy{
				testPols.Policies["alwaysError"],
			},
			traces:         newTraceWithService("my-service"),
			expectDecision: evaluators.Pending,
			expectPolicy:   nil,
			expectDataPoints: []metricdata.DataPoint[int64]{
				policyDecisionDP(testPols.Policies["alwaysError"].name, evaluators.Unspecified),
			},
		},
		{
			name: "On some errors, returns non-error result",
			policies: []*policy{
				testPols.Policies["alwaysError"],
				testPols.Policies["alwaysSample"],
			},
			traces:         newTraceWithService("my-service"),
			expectDecision: evaluators.Sampled,
			expectPolicy:   testPols.Policies["alwaysSample"],
			expectDataPoints: []metricdata.DataPoint[int64]{
				policyDecisionDP(testPols.Policies["alwaysError"].name, evaluators.Unspecified),
				policyDecisionDP(testPols.Policies["alwaysSample"].name, evaluators.Sampled, ""),
			},
		},
		{
			name: "Decider returns the right policy when decision is not sampled",
			policies: []*policy{
				testPols.Policies["hardNotSampled"],
				testPols.Policies["alwaysSample"],
			},
			traces:         newTraceWithService("my-service"),
			expectDecision: evaluators.NotSampled,
			expectPolicy:   testPols.Policies["hardNotSampled"],
			expectDataPoints: []metricdata.DataPoint[int64]{
				policyDecisionDP(testPols.Policies["hardNotSampled"].name, evaluators.NotSampled, ""),
			},
		},
		{
			name: "recordDecisionFrom populates decision_from with extracted service.name",
			policies: []*policy{
				testPols.Policies["alwaysPending"],
				testPols.Policies["alwaysSampleWithRecordFrom"],
			},
			traces:         newTraceWithService("my-service"),
			expectDecision: evaluators.Sampled,
			expectPolicy:   testPols.Policies["alwaysSampleWithRecordFrom"],
			expectDataPoints: []metricdata.DataPoint[int64]{
				policyDecisionDP(testPols.Policies["alwaysSampleWithRecordFrom"].name, evaluators.Sampled, "my-service"),
			},
		},
		{
			name: "decisionGrouper collapses high-cardinality decision_from",
			policies: []*policy{
				testPols.Policies["alwaysPending"],
				testPols.Policies["alwaysSampleWithGrouping"],
			},
			traces:         newTraceWithService("confluence-shard-0042"),
			expectDecision: evaluators.Sampled,
			expectPolicy:   testPols.Policies["alwaysSampleWithGrouping"],
			expectDataPoints: []metricdata.DataPoint[int64]{
				policyDecisionDP(testPols.Policies["alwaysSampleWithGrouping"].name, evaluators.Sampled, "confluence-monolith"),
			},
		},
	}

	mergedMetadata := &tracedata.Metadata{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testTel := componenttest.NewTelemetry()
			t.Cleanup(func() { require.NoError(t, testTel.Shutdown(context.Background())) })
			telemetry, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
			require.NoError(t, err)

			dec := newDecider(test.policies, zap.NewNop(), telemetry)
			decision, p := dec.MakeDecision(context.Background(), testTraceID, test.traces, mergedMetadata)
			assert.Equal(t, test.expectDecision, decision)
			assert.Equal(t, test.expectPolicy, p)

			if test.expectDataPoints != nil {
				metadatatest.AssertEqualProcessorAtlassianSamplingPolicyDecisions(t, testTel,
					test.expectDataPoints, metricdatatest.IgnoreTimestamp())
			} else {
				_, err := testTel.GetMetric("otelcol_processor_atlassian_sampling_policy_decisions")
				assert.Error(t, err, "expected no policy_decisions metric to be emitted")
			}
		})
	}
}

func TestLastLowDecision(t *testing.T) {
	t.Parallel()
	testPols := setupTestPolicies()
	tests := []struct {
		name                       string
		lastLowDecisionPolicyIndex int32
		policies                   []*policy
		expectDecision             evaluators.Decision
		expectedPolicy             *policy
	}{
		{
			/*
			   [Policy A, Policy B]
			   Span 1
			     - A -> Pending
			     - B -> Low
			     - C -> Skipped

			   LastLowPriorityDecisionName: B

			   Span 2
			     - A -> Sampled
			     - B -> Skipped
			     - C -> Skipped

			   Expected Result: Sampled
			*/
			name:                       "Previously low priority decision is promoted to sampled",
			lastLowDecisionPolicyIndex: 1,
			policies: []*policy{
				testPols.Policies["alwaysSample"],
				testPols.Policies["alwaysLowPriority"],
				testPols.Policies["alwaysPending"],
			},
			expectDecision: evaluators.Sampled,
			expectedPolicy: testPols.Policies["alwaysSample"],
		},
		{
			/*
			   			   [Policy A, Policy B]
			   			   Span 1
			   			     - A -> Low
			   			     - B -> Skipped
			            LastLowPriorityDecisionName: A
			   			   Span 2
			   			     - A -> Pending
			   			     - B -> Low

			   			   Expected Result: Pending
			*/
			name:                       "Higher tier policy is return LowPriority, but not the last low priority decision return Pending",
			lastLowDecisionPolicyIndex: 0,
			policies: []*policy{
				testPols.Policies["alwaysPending"],
				testPols.Policies["alwaysLowPriority"],
				testPols.Policies["alwaysPending"],
			},
			expectDecision: evaluators.Pending,
			expectedPolicy: nil,
		},
		{
			/*
			   [Policy A, Policy B]
			   Span 1
			     - A -> Pending
			     - B -> Low
			   LastLowPriorityDecisionName: B
			   Span 2
			     - A -> Low
			     - B -> Pending
			   Expected Result: Pending
			*/
			name:                       "Higher tier policy is return LowPriority, but not the last low priority decision return Pending",
			lastLowDecisionPolicyIndex: 1,
			policies: []*policy{
				testPols.Policies["alwaysLowPriority"],
				testPols.Policies["alwaysPending"],
			},
			expectDecision: evaluators.Pending,
			expectedPolicy: nil,
		},
	}

	telemetry, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	var traceData ptrace.Traces
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lastLowDecisionPolicy := test.policies[test.lastLowDecisionPolicyIndex]
			var mergedMetadata = &tracedata.Metadata{LastLowPriorityDecisionName: &lastLowDecisionPolicy.name}
			dec := newDecider(test.policies, zap.NewNop(), telemetry)
			decision, policy := dec.MakeDecision(context.Background(), testTraceID, traceData, mergedMetadata)
			assert.Equal(t, test.expectDecision, decision)
			assert.Equal(t, test.expectedPolicy, policy)
		})
	}
}

func alwaysPending() *policy {
	return &policy{
		name:       "returns Pending " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator:  evaluators.NewProbabilisticSampler("", 0),
		policyType: Probabilistic,
	}
}

func alwaysSample() *policy {
	return &policy{
		name:       "returns Sampled " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator:  evaluators.NewProbabilisticSampler("", 100.0),
		policyType: Probabilistic,
	}
}

func alwaysSampleWithRecordFrom() *policy {
	return &policy{
		name:               "returns Sampled with recordDecisionFrom " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator:          evaluators.NewProbabilisticSampler("", 100.0),
		policyType:         Probabilistic,
		recordDecisionFrom: "service.name",
	}
}

func alwaysSampleWithGrouping() *policy {
	grouper, err := newDecisionGrouper([]DecisionMapping{
		{Pattern: "^(conf|confluence)-.*", Value: "confluence-monolith"},
	})
	if err != nil {
		panic(err)
	}
	return &policy{
		name:               "returns Sampled with grouping " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator:          evaluators.NewProbabilisticSampler("", 100.0),
		policyType:         Probabilistic,
		recordDecisionFrom: "service.name",
		decisionGrouper:    grouper,
	}
}

func hardNotSampled() *policy {
	return &policy{
		name:       "returns NotSampled " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator:  &staticTestEvaluator{d: evaluators.NotSampled},
		policyType: Probabilistic,
	}
}

func alwaysLowPriority() *policy {
	return &policy{
		name:       "returns LowPriority " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator:  &staticTestEvaluator{d: evaluators.LowPriority},
		policyType: RootSpans,
	}
}

func alwaysError() *policy {
	return &policy{
		name:       "always error " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator:  &staticTestEvaluator{d: evaluators.Unspecified, e: fmt.Errorf("test error")},
		policyType: OTTLCondition,
	}
}

type staticTestEvaluator struct {
	evaluators.StartFunc
	d evaluators.Decision
	e error
}

func (e *staticTestEvaluator) Evaluate(_ context.Context, _ pcommon.TraceID, _ ptrace.Traces, _ *tracedata.Metadata) (evaluators.Decision, error) {
	return e.d, e.e
}
