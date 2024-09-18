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

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/metadata"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
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
	var mergedMetadata *tracedata.Metadata
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dec := newDecider(test.policies, zap.NewNop(), telemetry)
			decision, policy := dec.MakeDecision(context.Background(), testTraceID, traceData, mergedMetadata)
			assert.Equal(t, test.expectDecision, decision)
			assert.Equal(t, test.expectPolicy, policy)
		})
	}
}

func alwaysPending() *policy {
	return &policy{
		name:      "returns Pending " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator: evaluators.NewProbabilisticSampler("", 0),
	}
}

func alwaysSample() *policy {
	return &policy{
		name:      "returns Sampled " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator: evaluators.NewProbabilisticSampler("", 100.0),
	}
}

func hardNotSampled() *policy {
	return &policy{
		name:      "returns NotSampled " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator: &staticTestEvaluator{d: evaluators.NotSampled},
	}
}

func alwaysLowPriority() *policy {
	return &policy{
		name:      "returns LowPriority " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator: &staticTestEvaluator{d: evaluators.LowPriority},
	}
}

func alwaysError() *policy {
	return &policy{
		name:      "always error " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator: &staticTestEvaluator{d: evaluators.Unspecified, e: fmt.Errorf("test error")},
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
