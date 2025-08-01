package atlassiansamplingprocessor // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/metadata"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

type deciderI interface {
	Start(ctx context.Context, host component.Host) error
	MakeDecision(ctx context.Context, id pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) (evaluators.Decision, *policy)
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
func (d *decider) MakeDecision(ctx context.Context, id pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) (evaluators.Decision, *policy) {
	for _, p := range d.policies {
		decision, err := p.evaluator.Evaluate(ctx, id, currentTrace, mergedMetadata)
		if err != nil {
			d.log.Warn("policy evaluation errored", zap.Error(err), zap.String("policy.name", p.name))
		}
		d.telemetry.ProcessorAtlassianSamplingPolicyDecisions.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
			attribute.String("policy", p.name),
			attribute.String("decision", decision.String()))))

		// Assume we have policy list [X, Y, Z],
		// 1. Trace A/Span A is marked as low priority by a policy Z, we will set LastLowPriorityDecisionName to Z.
		// 2. If Trace A/Span B comes and policy X/Y marks it as sampled/not sampled, we will reset LastLowPriorityDecisionName to nil.
		// 3. If Trace A/Span C comes and policy X/Y marks it as Low, we will set priority to Pending, then reset LastLowPriorityDecisionName to nil and re-evaluate Z.
		if mergedMetadata.LastLowPriorityDecisionName != nil &&
			*mergedMetadata.LastLowPriorityDecisionName != p.name &&
			decision == evaluators.LowPriority {
			decision = evaluators.Pending
		}

		if decision == evaluators.Sampled || decision == evaluators.NotSampled || decision == evaluators.LowPriority {
			return decision, p
		}
	}
	return evaluators.Pending, nil
}

func (d *decider) Start(ctx context.Context, host component.Host) error {
	for _, p := range d.policies {
		err := p.evaluator.Start(ctx, host)
		if err != nil {
			return err
		}
	}
	return nil
}

func getPolicyEvaluator(cfg *PolicyConfig, set component.TelemetrySettings) (evaluators.PolicyEvaluator, error) {
	switch cfg.Type {
	case And:
		return getNewAndPolicy(&cfg.AndConfig, set)
	case RootSpans:
		return getNewRootSpansPolicy(&cfg.RootSpansConfig, set)
	case Downgrader:
		return getNewDowngraderPolicy(&cfg.DowngraderConfig, set)
	default:
		return getSharedPolicyEvaluator(&cfg.SharedPolicyConfig, set)
	}
}

// Return instance of and sub-policy
func getAndSubPolicyEvaluator(cfg *AndSubPolicyConfig, set component.TelemetrySettings) (evaluators.PolicyEvaluator, error) {
	return getSharedPolicyEvaluator(&cfg.SharedPolicyConfig, set)
}

func getSharedPolicyEvaluator(cfg *SharedPolicyConfig, set component.TelemetrySettings) (evaluators.PolicyEvaluator, error) {
	switch cfg.Type {
	case Probabilistic:
		pCfg := cfg.ProbabilisticConfig
		return evaluators.NewProbabilisticSampler(pCfg.HashSalt, pCfg.SamplingPercentage), nil
	case SpanCount:
		spCfg := cfg.SpanCountConfig
		var log *zap.Logger
		if spCfg.LogSampled {
			log = set.Logger
		}
		return evaluators.NewSpanCount(spCfg.MinSpans, log), nil
	case Latency:
		lCfg := cfg.LatencyConfig
		return evaluators.NewLatency(lCfg.ThresholdMs), nil
	case StatusCode:
		sceCfg := cfg.StatusCodeConfig
		return evaluators.NewStatusCodeEvaluator(sceCfg.StatusCodes)
	case OTTLCondition:
		ottlcCfg := cfg.OTTLConditionConfig
		return evaluators.NewOTTLConditionEvaluator(ottlcCfg.SpanConditions, ottlcCfg.SpanEventConditions, ottlcCfg.ErrorMode)
	case Threshold:
		return evaluators.NewThresholdEvaluator(), nil
	case RemoteProbabilistic:
		rpCfg := cfg.RemoteProbabilisticConfig
		return evaluators.NewRemoteProbabilisticSampler(rpCfg.HashSalt, rpCfg.DefaultRate, rpCfg.RateGetterExt), nil
	default:
		return nil, fmt.Errorf("unknown sampling policy type %s", cfg.Type)
	}
}

func getNewAndPolicy(config *AndConfig, set component.TelemetrySettings) (evaluators.PolicyEvaluator, error) {
	subPolicyEvaluators := make([]evaluators.PolicyEvaluator, len(config.SubPolicyCfg))
	for i := range config.SubPolicyCfg {
		policyCfg := &config.SubPolicyCfg[i]
		policy, err := getAndSubPolicyEvaluator(policyCfg, set)
		if err != nil {
			return nil, err
		}
		subPolicyEvaluators[i] = policy
	}
	return evaluators.NewAndEvaluator(subPolicyEvaluators), nil
}

func getNewRootSpansPolicy(cfg *RootSpansConfig, set component.TelemetrySettings) (evaluators.PolicyEvaluator, error) {
	subPolicy, err := getSharedPolicyEvaluator(&cfg.SubPolicyCfg, set)
	if err != nil {
		return nil, err
	}
	return evaluators.NewRootSpan(subPolicy), nil
}

func getNewDowngraderPolicy(cfg *DowngraderConfig, set component.TelemetrySettings) (evaluators.PolicyEvaluator, error) {
	subPolicy, err := getSharedPolicyEvaluator(&cfg.SubPolicyCfg, set)
	if err != nil {
		return nil, err
	}
	downgradeTo, err := evaluators.StringToDecision(cfg.DowngradeTo)
	if err != nil {
		return nil, err
	}
	return evaluators.NewDowngrader(downgradeTo, subPolicy)
}
