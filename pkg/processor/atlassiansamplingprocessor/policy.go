package atlassiansamplingprocessor // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
)

// policy wraps a PolicyEvaluator including name provided by user and any measurements options
type policy struct {
	// name used to identify this policy instance.
	name string
	// type is used to identify this policy instance's policyType
	policyType PolicyType
	// emitSingleSpanForNotSampled is used to determine if a single span should be emitted for a trace that is not sampled.
	emitSingleSpanForNotSampled bool
	// evaluator that decides if a trace is sampled or not by this policy instance.
	evaluator evaluators.PolicyEvaluator
	// recordDecisionFrom is an optional resource attribute key whose value is included
	// as a metric attribute on the policy_decisions metric for decisive evaluations.
	recordDecisionFrom string
}

func newPolicies(cfg []PolicyConfig, set component.TelemetrySettings) ([]*policy, error) {
	policyNames := map[string]bool{}
	pols := make([]*policy, len(cfg))
	for i := range cfg {
		policyCfg := &cfg[i]

		if policyNames[policyCfg.Name] {
			return nil, fmt.Errorf("duplicate policy name %q", policyCfg.Name)
		}
		policyNames[policyCfg.Name] = true

		eval, err := getPolicyEvaluator(policyCfg, set)
		if err != nil {
			return nil, err
		}
		p := &policy{
			name:                        policyCfg.Name,
			policyType:                  policyCfg.Type,
			emitSingleSpanForNotSampled: policyCfg.EmitSingleSpanForNotSampled,
			evaluator:                   eval,
			recordDecisionFrom:          policyCfg.RecordDecisionFrom,
		}
		pols[i] = p
	}
	return pols, nil
}
