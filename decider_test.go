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
	"go.uber.org/zap"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/metadata"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

func TestMakeDecision(t *testing.T) {

	tests := []struct {
		name           string
		policies       []*policy
		expectDecision evaluators.Decision
	}{
		{
			name: "One pending, one sampled results in overall sampled",
			policies: []*policy{
				alwaysPending(),
				alwaysSample(),
			},
			expectDecision: evaluators.Sampled,
		},
		{
			name: "All pending results in NotSampled",
			policies: []*policy{
				alwaysPending(),
				alwaysPending(),
			},
			expectDecision: evaluators.Pending,
		},
		{
			name: "Pending and NotSampled returns NotSampled",
			policies: []*policy{
				alwaysPending(),
				hardNotSampled(),
			},
			expectDecision: evaluators.NotSampled,
		},
		{
			name: "Order matters, sampled first",
			policies: []*policy{
				alwaysSample(),
				hardNotSampled(),
			},
			expectDecision: evaluators.Sampled,
		},
		{
			name: "Order matters, NotSampled first",
			policies: []*policy{
				hardNotSampled(),
				alwaysSample(),
			},
			expectDecision: evaluators.NotSampled,
		},
		{
			name: "On all errors, returns pending",
			policies: []*policy{
				alwaysError(),
			},
			expectDecision: evaluators.Pending,
		},
		{
			name: "On some errors, returns non-error result",
			policies: []*policy{
				alwaysError(),
				alwaysSample(),
			},
			expectDecision: evaluators.Sampled,
		},
	}

	telemetry, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	var traceData *tracedata.TraceData
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dec := newDecider(test.policies, zap.NewNop(), telemetry)
			decision := dec.MakeDecision(context.Background(), testTraceID, traceData)
			assert.Equal(t, test.expectDecision, decision)
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
		evaluator: &hardNotSampledPolicy{},
	}
}

func alwaysError() *policy {
	return &policy{
		name:      "always error " + strconv.FormatUint(uint64(rand.Uint32()), 16),
		evaluator: &errorPolicy{},
	}
}

type errorPolicy struct{}

func (ep *errorPolicy) Evaluate(_ context.Context, _ pcommon.TraceID, _ *tracedata.TraceData) (evaluators.Decision, error) {
	return evaluators.Unspecified, fmt.Errorf("test error")
}

type hardNotSampledPolicy struct{}

func (p *hardNotSampledPolicy) Evaluate(_ context.Context, _ pcommon.TraceID, _ *tracedata.TraceData) (evaluators.Decision, error) {
	return evaluators.NotSampled, nil
}
