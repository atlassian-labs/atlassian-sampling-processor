package evaluators // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

type downgrader struct {
	downgradeTo Decision
	subPolicy   PolicyEvaluator
}

func NewDowngrader(downGradeTo Decision, subPolicy PolicyEvaluator) (PolicyEvaluator, error) {
	return &downgrader{
		downgradeTo: downGradeTo,
		subPolicy:   subPolicy,
	}, nil
}

func (r *downgrader) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (r *downgrader) Evaluate(ctx context.Context, traceID pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) (Decision, error) {
	subDecision, err := r.subPolicy.Evaluate(ctx, traceID, currentTrace, mergedMetadata)
	if err != nil {
		return Unspecified, err
	}
	if subDecision == Sampled {
		return r.downgradeTo, nil
	}
	return subDecision, nil
}
