// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package atlassiansamplingprocessor // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor"

// PolicyConfig holds the common configuration to all policies.
type PolicyConfig struct {
	SharedPolicyConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// Configs for defining and policy
	AndConfig AndConfig `mapstructure:"and"`

	RootSpansConfig RootSpansConfig `mapstructure:"root_spans"`
}

// SharedPolicyConfig holds the common configuration to all policies that are used in derivative policy configurations
// such as the "and" policy.
type SharedPolicyConfig struct {
	// Name given to the instance of the policy to make easy to identify it in metrics and logs.
	Name string `mapstructure:"name"`
	// Type of the policy this will be used to match the proper configuration of the policy.
	Type PolicyType `mapstructure:"type"`
	// Configs for probabilistic sampling policy evaluator.
	ProbabilisticConfig `mapstructure:"probabilistic"`
	// Configs for span count filter sampling policy evaluator.
	SpanCountConfig `mapstructure:"span_count"`
}

// AndSubPolicyConfig holds the common configuration to all policies under and policy.
type AndSubPolicyConfig struct {
	SharedPolicyConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
}

// AndConfig holds the common configuration to all and policies.
type AndConfig struct {
	SubPolicyCfg []AndSubPolicyConfig `mapstructure:"and_sub_policy"`
}

// ProbabilisticConfig holds the configurable settings to create a probabilistic
// sampling policy evaluator.
type ProbabilisticConfig struct {
	// HashSalt allows one to configure the hashing salts. This is important in scenarios where multiple layers of collectors
	// have different sampling rates: if they use the same salt all passing one layer may pass the other even if they have
	// different sampling rates, configuring different salts avoids that.
	HashSalt string `mapstructure:"hash_salt"`
	// SamplingPercentage is the percentage rate at which traces are going to be sampled. Defaults to zero, i.e.: no sample.
	// Values greater or equal 100 are treated as "sample all traces".
	SamplingPercentage float64 `mapstructure:"sampling_percentage"`
}

// SpanCountConfig holds the configurable settings to create a Span Count filter sampling
// policy evaluator
type SpanCountConfig struct {
	// Minimum number of spans in a Trace
	MinSpans int `mapstructure:"min_spans"`
}

type RootSpansConfig struct {
	SubPolicyCfg SharedPolicyConfig `mapstructure:"sub_policy"`
}

// PolicyType indicates the type of sampling policy.
type PolicyType string

const (
	// Probabilistic samples a given percentage of traces.
	Probabilistic PolicyType = "probabilistic"
	// And allows defining a policy, combining the other policies in one
	And PolicyType = "and"
	// SpanCount sample traces that are have more spans per Trace than a given threshold.
	SpanCount PolicyType = "span_count"
	// RootSpans allows a sub-policy to be defined, and operates the sub-policy only on root spans with no children.
	RootSpans PolicyType = "root_spans"
)
