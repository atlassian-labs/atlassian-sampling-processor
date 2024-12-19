package atlassiansamplingprocessor

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/metadata"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

type testPols struct {
	Policies map[string]*policy
}

func setupTestPolicies() testPols {
	return testPols{Policies: map[string]*policy{
		"alwaysPending":     alwaysPending(),
		"alwaysSample":      alwaysSample(),
		"hardNotSampled":    hardNotSampled(),
		"alwaysLowPriority": alwaysLowPriority(),
		"alwaysError":       alwaysError(),
	}}
}

func TestMakeDecision(t *testing.T) {
	t.Parallel()
	testPols := setupTestPolicies()
	tests := []struct {
		name           string
		policies       []*policy
		expectDecision evaluators.Decision
		expectPolicy   *policy
	}{
		{
			name: "One pending, one sampled results in overall sampled",
			policies: []*policy{
				testPols.Policies["alwaysPending"],
				testPols.Policies["alwaysSample"],
			},
			expectDecision: evaluators.Sampled,
			expectPolicy:   testPols.Policies["alwaysSample"],
		},
		{
			name: "All pending results in NotSampled",
			policies: []*policy{
				testPols.Policies["alwaysPending"],
				testPols.Policies["alwaysPending"],
			},
			expectDecision: evaluators.Pending,
			expectPolicy:   nil,
		},
		{
			name: "Pending and NotSampled returns NotSampled",
			policies: []*policy{
				testPols.Policies["alwaysPending"],
				testPols.Policies["hardNotSampled"],
			},
			expectDecision: evaluators.NotSampled,
			expectPolicy:   testPols.Policies["hardNotSampled"],
		},
		{
			name: "Order matters, sampled first",
			policies: []*policy{
				testPols.Policies["alwaysSample"],
				testPols.Policies["alwaysLowPriority"],
			},
			expectDecision: evaluators.Sampled,
			expectPolicy:   testPols.Policies["alwaysSample"],
		},
		{
			name: "Order matters, NotSampled first",
			policies: []*policy{
				testPols.Policies["hardNotSampled"],
				testPols.Policies["alwaysSample"],
			},
			expectDecision: evaluators.NotSampled,
			expectPolicy:   testPols.Policies["hardNotSampled"],
		},
		{
			name: "Order matters, LowPriority first",
			policies: []*policy{
				testPols.Policies["alwaysLowPriority"],
				testPols.Policies["alwaysSample"],
			},
			expectDecision: evaluators.LowPriority,
			expectPolicy:   testPols.Policies["alwaysLowPriority"],
		},
		{
			name: "On all errors, returns pending",
			policies: []*policy{
				testPols.Policies["alwaysError"],
			},
			expectDecision: evaluators.Pending,
			expectPolicy:   nil,
		},
		{
			name: "On some errors, returns non-error result",
			policies: []*policy{
				testPols.Policies["alwaysError"],
				testPols.Policies["alwaysSample"],
			},
			expectDecision: evaluators.Sampled,
			expectPolicy:   testPols.Policies["alwaysSample"],
		},
		{
			name: "Decider returns the right policy when decision is not sampled",
			policies: []*policy{
				testPols.Policies["hardNotSampled"],
				testPols.Policies["alwaysSample"],
			},
			expectDecision: evaluators.NotSampled,
			expectPolicy:   testPols.Policies["hardNotSampled"],
		},
	}

	telemetry, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	var traceData ptrace.Traces
	mergedMetadata := &tracedata.Metadata{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dec := newDecider(test.policies, zap.NewNop(), telemetry)
			decision, policy := dec.MakeDecision(context.Background(), testTraceID, traceData, mergedMetadata)
			assert.Equal(t, test.expectDecision, decision)
			assert.Equal(t, test.expectPolicy, policy)
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
