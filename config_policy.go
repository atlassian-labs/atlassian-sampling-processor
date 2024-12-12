// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package atlassiansamplingprocessor // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// PolicyConfig holds the common configuration to all policies.
type PolicyConfig struct {
	SharedPolicyConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// Configs for defining and policy
	AndConfig AndConfig `mapstructure:"and"`

	RootSpansConfig RootSpansConfig `mapstructure:"root_spans"`

	DowngraderConfig DowngraderConfig `mapstructure:"downgrader"`
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
	// Configs for latency filter sampling policy evaluator.
	LatencyConfig `mapstructure:"latency"`
	// Configs for status code filter sampling policy evaluator.
	StatusCodeConfig `mapstructure:"status_code"`
	// Configs for OTTL condition filter sampling policy evaluator
	OTTLConditionConfig `mapstructure:"ottl_condition"`
	// Configs for remote probabilistic sampling policy evaluator
	RemoteProbabilisticConfig `mapstructure:"remote_probabilistic"`
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
	// MinSpans is the minimum number of spans in a Trace for it to be sampled
	MinSpans int32 `mapstructure:"min_spans"`
	// LogSampled indicates whether to emit a log when a trace is sampled.
	// It will log trace ID, services in trace, and span count.
	LogSampled bool `mapstructure:"log_sampled"`
}

type RootSpansConfig struct {
	SubPolicyCfg SharedPolicyConfig `mapstructure:"sub_policy"`
}

type DowngraderConfig struct {
	DowngradeTo  string             `mapstructure:"downgrade_to"`
	SubPolicyCfg SharedPolicyConfig `mapstructure:"sub_policy"`
}

// LatencyConfig holds the configurable settings to create a latency filter sampling policy
// evaluator
type LatencyConfig struct {
	// Lower bound in milliseconds
	ThresholdMs int64 `mapstructure:"threshold_ms"`
}

// StatusCodeConfig holds the configurable settings to create a status code filter sampling
// policy evaluator.
type StatusCodeConfig struct {
	StatusCodes []string `mapstructure:"status_codes"`
}

// OTTLConditionConfig holds the configurable setting to create a OTTL condition filter
// sampling policy evaluator.
type OTTLConditionConfig struct {
	ErrorMode           ottl.ErrorMode `mapstructure:"error_mode"`
	SpanConditions      []string       `mapstructure:"span"`
	SpanEventConditions []string       `mapstructure:"spanevent"`
}

// RemoteProbabilisticConfig holds the configurable settings to create a remote probabilistic
// sampling policy evaluator.
type RemoteProbabilisticConfig struct {
	// HashSalt allows one to configure the hashing salts. This is important in scenarios where multiple layers of collectors
	// have different sampling rates: if they use the same salt all passing one layer may pass the other even if they have
	// different sampling rates, configuring different salts avoids that.
	HashSalt string `mapstructure:"hash_salt"`
	// DefaultRate is the default rate at which traces are going to be sampled if there is an error from the specified rate getter.
	// Defaults to zero, i.e.: no sample. Values greater or equal 100 are treated as "sample all traces".
	DefaultRate float64 `mapstructure:"default_rate"`
	// RateGetterExt is the component id of the rate getter extension to use to fetch the sampling rate at runtime.
	// The extension must implement the RateGetter interface.
	RateGetterExt component.ID `mapstructure:"rate_getter"`
}

// PolicyType indicates the type of sampling policy.
type PolicyType string

const (
	// Probabilistic samples a given percentage of traces.
	Probabilistic PolicyType = "probabilistic"
	// And allows defining a policy, combining the other policies in one
	And PolicyType = "and"
	// Downgrader downgrades a sampled decision
	Downgrader PolicyType = "downgrader"
	// SpanCount sample traces that are have more spans per Trace than a given threshold.
	SpanCount PolicyType = "span_count"
	// RootSpans allows a sub-policy to be defined, and operates the sub-policy only on root spans with no children.
	RootSpans PolicyType = "root_spans"
	// Latency sample traces that are longer than a given threshold.
	Latency PolicyType = "latency"
	// StatusCode sample traces that have a given status code.
	StatusCode PolicyType = "status_code"
	// OTTLCondition sample traces which match user provided OpenTelemetry Transformation Language
	// conditions.
	OTTLCondition PolicyType = "ottl_condition"
	// Threshold retrieves the threshold from sampling.tail.threshold attribute.
	// It compares the threshold to the trace ID.
	Threshold PolicyType = "threshold"
	// RemoteProbabilistic fetches the sampling rate and samples traces based on the returned rate at runtime.
	RemoteProbabilistic PolicyType = "remote_probabilistic"
)
