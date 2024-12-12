package atlassiansamplingprocessor // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
)

// policy wraps a PolicyEvaluator including name provided by user and any measurements options
type policy struct {
	// name used to identify this policy instance.
	name string
	// type is used to identify this policy instance's policyType
	policyType PolicyType
	// evaluator that decides if a trace is sampled or not by this policy instance.
	evaluator evaluators.PolicyEvaluator
	// attribute to use in the telemetry to denote the policy.
	attribute metric.MeasurementOption
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
			name:       policyCfg.Name,
			policyType: policyCfg.Type,
			evaluator:  eval,
			attribute:  metric.WithAttributes(attribute.String("policy", policyCfg.Name)),
		}
		pols[i] = p
	}
	return pols, nil
}