// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package atlassiansamplingprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "atlassian_sampling_test_cfg.yml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.Equal(t,
		cfg,
		&Config{
			PrimaryCacheSize:   1000,
			SecondaryCacheSize: 100,
			DecisionCacheCfg:   DecisionCacheCfg{SampledCacheSize: 1000, NonSampledCacheSize: 10000},
			PolicyConfig: []PolicyConfig{
				{
					SharedPolicyConfig: SharedPolicyConfig{
						Name: "test-policy-1",
						Type: "probabilistic",
						ProbabilisticConfig: ProbabilisticConfig{
							HashSalt:           "custom-salt",
							SamplingPercentage: 0.1,
						},
					},
				},
				{
					SharedPolicyConfig: SharedPolicyConfig{
						Name: "test-policy-2",
						Type: "and",
					},
					AndConfig: AndConfig{
						SubPolicyCfg: []AndSubPolicyConfig{
							{
								SharedPolicyConfig: SharedPolicyConfig{
									Name: "test-sub-policy-1",
									Type: "probabilistic",
									ProbabilisticConfig: ProbabilisticConfig{
										SamplingPercentage: 0,
									},
								},
							},
							{
								SharedPolicyConfig: SharedPolicyConfig{
									Name: "test-sub-policy-2",
									Type: "probabilistic",
									ProbabilisticConfig: ProbabilisticConfig{
										SamplingPercentage: 100.0,
									},
								},
							},
						},
					},
				},
				{
					SharedPolicyConfig: SharedPolicyConfig{
						Name: "test-policy-3",
						Type: "span_count",
						SpanCountConfig: SpanCountConfig{
							MinSpans: 0,
						},
					},
				},
				{
					SharedPolicyConfig: SharedPolicyConfig{
						Name: "test-policy-4",
						Type: "root_spans",
					},
					RootSpansConfig: RootSpansConfig{
						SubPolicyCfg: SharedPolicyConfig{
							Type: "probabilistic",
							ProbabilisticConfig: ProbabilisticConfig{
								SamplingPercentage: 0,
							},
						},
					},
				},
				{
					SharedPolicyConfig: SharedPolicyConfig{
						Name: "test-policy-5",
						Type: "latency",
						LatencyConfig: LatencyConfig{
							ThresholdMs: 5000,
						},
					},
				},
				{
					SharedPolicyConfig: SharedPolicyConfig{
						Name: "test-policy-6",
						Type: "status_code",
						StatusCodeConfig: StatusCodeConfig{
							StatusCodes: []string{"ERROR", "UNSET"},
						},
					},
				},
				{
					SharedPolicyConfig: SharedPolicyConfig{
						Name: "test-policy-7",
						Type: OTTLCondition,
						OTTLConditionConfig: OTTLConditionConfig{
							ErrorMode:           ottl.IgnoreError,
							SpanConditions:      []string{"attributes[\"test_attr_key_1\"] == \"test_attr_val_1\"", "attributes[\"test_attr_key_2\"] != \"test_attr_val_1\""},
							SpanEventConditions: []string{"name != \"test_span_event_name\"", "attributes[\"test_event_attr_key_2\"] != \"test_event_attr_val_1\""},
						},
					},
				},
				{
					SharedPolicyConfig: SharedPolicyConfig{
						Name: "test-policy-8",
						Type: Threshold,
					},
				},
				{
					SharedPolicyConfig: SharedPolicyConfig{
						Name: "test-policy-9",
						Type: RemoteProbabilistic,
						RemoteProbabilisticConfig: RemoteProbabilisticConfig{
							HashSalt:      "test-salt",
							RateGetterExt: component.MustNewID("test_rate_getter"),
							DefaultRate:   0.01,
						},
					},
				},
			},
		})
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		PrimaryCacheSize   int
		SecondaryCacheSize int
		hasError           bool
	}{
		{100, 10, false},
		{0, 10, true},
		{10, 0, true},
		{100, 50, false},
		{100, 55, true},
	}
	cfg := createDefaultConfig().(*Config)

	for _, tc := range testCases {
		cfg.PrimaryCacheSize = tc.PrimaryCacheSize
		cfg.SecondaryCacheSize = tc.SecondaryCacheSize
		assert.Equal(t, tc.hasError, cfg.Validate() != nil)
	}
}
