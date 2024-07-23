package atlassiansamplingprocessor // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/metadata"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

type deciderI interface {
	MakeDecision(ctx context.Context, id pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) evaluators.Decision
}

type decider struct {
	policies  []*policy
	log       *zap.Logger
	telemetry *metadata.TelemetryBuilder
}

var _ deciderI = (*decider)(nil)

func newDecider(pols []*policy, log *zap.Logger, telemetry *metadata.TelemetryBuilder) *decider {
	return &decider{
		policies:  pols,
		log:       log,
		telemetry: telemetry,
	}
}

// MakeDecision evaluates all policies, returning the first decision that isn't Pending.
// If all decisions are non-decisive, it returns Pending.
func (d *decider) MakeDecision(ctx context.Context, id pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) evaluators.Decision {
	for _, p := range d.policies {
		decision, err := p.evaluator.Evaluate(ctx, id, currentTrace, mergedMetadata)
		if err != nil {
			d.log.Warn("policy evaluation errored", zap.Error(err), zap.String("policy.name", p.name))
		}
		d.telemetry.ProcessorAtlassianSamplingPolicyDecisions.Add(ctx, 1, metric.WithAttributes(
			attribute.String("policy", p.name),
			attribute.String("decision", decision.String()),
		))
		if decision == evaluators.Sampled || decision == evaluators.NotSampled {
			return decision
		}
	}
	return evaluators.Pending
}

func getPolicyEvaluator(cfg *PolicyConfig) (evaluators.PolicyEvaluator, error) {
	switch cfg.Type {
	case And:
		return getNewAndPolicy(&cfg.AndConfig)
	case RootSpans:
		return getNewRootSpansPolicy(&cfg.RootSpansConfig)
	default:
		return getSharedPolicyEvaluator(&cfg.SharedPolicyConfig)
	}
}

// Return instance of and sub-policy
func getAndSubPolicyEvaluator(cfg *AndSubPolicyConfig) (evaluators.PolicyEvaluator, error) {
	return getSharedPolicyEvaluator(&cfg.SharedPolicyConfig)
}

func getSharedPolicyEvaluator(cfg *SharedPolicyConfig) (evaluators.PolicyEvaluator, error) {
	switch cfg.Type {
	case Probabilistic:
		pCfg := cfg.ProbabilisticConfig
		return evaluators.NewProbabilisticSampler(pCfg.HashSalt, pCfg.SamplingPercentage), nil
	case SpanCount:
		spCfg := cfg.SpanCountConfig
		return evaluators.NewSpanCount(spCfg.MinSpans), nil

	default:
		return nil, fmt.Errorf("unknown sampling policy type %s", cfg.Type)
	}
}

func getNewAndPolicy(config *AndConfig) (evaluators.PolicyEvaluator, error) {
	subPolicyEvaluators := make([]evaluators.PolicyEvaluator, len(config.SubPolicyCfg))
	for i := range config.SubPolicyCfg {
		policyCfg := &config.SubPolicyCfg[i]
		policy, err := getAndSubPolicyEvaluator(policyCfg)
		if err != nil {
			return nil, err
		}
		subPolicyEvaluators[i] = policy
	}
	return evaluators.NewAndEvaluator(subPolicyEvaluators), nil
}

func getNewRootSpansPolicy(cfg *RootSpansConfig) (evaluators.PolicyEvaluator, error) {
	subPolicy, err := getSharedPolicyEvaluator(&cfg.SubPolicyCfg)
	if err != nil {
		return nil, err
	}
	return evaluators.NewRootSpan(subPolicy), nil
}
