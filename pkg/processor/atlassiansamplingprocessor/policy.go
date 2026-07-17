package atlassiansamplingprocessor // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor"

import (
	"fmt"
	"regexp"

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
	// decisionGrouper optionally normalizes the recordDecisionFrom value before it is
	// emitted as the "decision_from" metric attribute, to bound metric cardinality.
	// It is nil when no record_decision_from mappings are configured.
	decisionGrouper *decisionGrouper
}

// decisionGrouper normalizes an extracted value into a low-cardinality group by
// applying an ordered list of compiled regex rules.
type decisionGrouper struct {
	rules []decisionRule
}

type decisionRule struct {
	re    *regexp.Regexp
	value string
}

// group returns the normalized value for the given extracted value.
func (g *decisionGrouper) group(value string) string {
	for _, r := range g.rules {
		if r.re.MatchString(value) {
			return r.value
		}
	}
	return value
}

// newDecisionGrouper compiles the given mappings into a decisionGrouper. It returns nil
// (with no error) when there are no mappings, meaning no normalization should be applied.
func newDecisionGrouper(mappings []DecisionMapping) (*decisionGrouper, error) {
	if len(mappings) == 0 {
		return nil, nil
	}
	rules := make([]decisionRule, 0, len(mappings))
	for i, m := range mappings {
		re, err := regexp.Compile(m.Pattern)
		if err != nil {
			return nil, fmt.Errorf("record_decision_from.mappings[%d].pattern %q: %w", i, m.Pattern, err)
		}
		rules = append(rules, decisionRule{re: re, value: m.Value})
	}
	return &decisionGrouper{rules: rules}, nil
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
		var recordDecisionFrom string
		var grouper *decisionGrouper
		if policyCfg.RecordDecisionFrom != nil {
			recordDecisionFrom = policyCfg.RecordDecisionFrom.ResAttrKey
			grouper, err = newDecisionGrouper(policyCfg.RecordDecisionFrom.Mappings)
			if err != nil {
				return nil, fmt.Errorf("policy %q: %w", policyCfg.Name, err)
			}
		}
		p := &policy{
			name:                        policyCfg.Name,
			policyType:                  policyCfg.Type,
			emitSingleSpanForNotSampled: policyCfg.EmitSingleSpanForNotSampled,
			evaluator:                   eval,
			recordDecisionFrom:          recordDecisionFrom,
			decisionGrouper:             grouper,
		}
		pols[i] = p
	}
	return pols, nil
}
