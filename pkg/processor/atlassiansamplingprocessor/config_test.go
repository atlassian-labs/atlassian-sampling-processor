// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package atlassiansamplingprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/metadata"

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

	delay, err := time.ParseDuration("5m")
	require.NoError(t, err)

	assert.Equal(t,
		cfg,
		&Config{
			PrimaryCacheSize:   1000,
			SecondaryCacheSize: 100,
			TargetHeapBytes:    100_000_000,
			RegulateCacheDelay: delay,
			DecisionCacheCfg:   DecisionCacheCfg{SampledCacheSize: 1000, NonSampledCacheSize: 10000},
			CompressionEnabled: true,
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
				{
					SharedPolicyConfig: SharedPolicyConfig{
						Name:                        "test-policy-10",
						Type:                        "downgrader",
						EmitSingleSpanForNotSampled: true,
					},
					DowngraderConfig: DowngraderConfig{
						DowngradeTo: "NotSampled",
						SubPolicyCfg: SharedPolicyConfig{
							Type: "probabilistic",
							ProbabilisticConfig: ProbabilisticConfig{
								SamplingPercentage: 0,
							},
						},
					},
				},
			},
		})
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		name          string
		c             *Config
		expectedError error
	}{
		{
			name: "Primary cache 100, secondary cache 10",
			c: &Config{
				PrimaryCacheSize:   100,
				SecondaryCacheSize: 10,
				PolicyConfig:       make([]PolicyConfig, 0),
			},
			expectedError: nil,
		},
		{
			name: "Primary cache 0, secondary cache 10",
			c: &Config{
				PrimaryCacheSize:   0,
				SecondaryCacheSize: 10,
				PolicyConfig:       make([]PolicyConfig, 0),
			},
			expectedError: primaryCacheSizeError,
		},
		{
			name: "Primary cache 10, secondary cache 0",
			c: &Config{
				PrimaryCacheSize:   10,
				SecondaryCacheSize: 0,
				PolicyConfig:       make([]PolicyConfig, 0),
			},
			expectedError: secondaryCacheSizeError,
		},
		{
			name: "Primary cache 100, secondary cache 50",
			c: &Config{
				PrimaryCacheSize:   100,
				SecondaryCacheSize: 50,
				PolicyConfig:       make([]PolicyConfig, 0),
			},
			expectedError: nil,
		},
		{
			name: "Primary cache 100, secondary cache 55",
			c: &Config{
				PrimaryCacheSize:   100,
				SecondaryCacheSize: 55,
				PolicyConfig:       make([]PolicyConfig, 0),
			},
			expectedError: secondaryCacheSizeError,
		},
		{
			name: "No duplicate policy names",
			c: &Config{
				PrimaryCacheSize:   100,
				SecondaryCacheSize: 10,
				PolicyConfig: []PolicyConfig{
					{
						SharedPolicyConfig: SharedPolicyConfig{
							Name: "test-policy-1",
							Type: Probabilistic,
							ProbabilisticConfig: ProbabilisticConfig{
								HashSalt:           "test-hash",
								SamplingPercentage: 0,
							},
						},
					},
					{
						SharedPolicyConfig: SharedPolicyConfig{
							Name: "test-policy-2",
							Type: Probabilistic,
							ProbabilisticConfig: ProbabilisticConfig{
								HashSalt:           "test-hash",
								SamplingPercentage: 0,
							},
						},
					},
				},
			},
			expectedError: nil,
		}, {
			name: "Duplicate policy names",
			c: &Config{
				PrimaryCacheSize:   100,
				SecondaryCacheSize: 10,
				PolicyConfig: []PolicyConfig{
					{
						SharedPolicyConfig: SharedPolicyConfig{
							Name: "test-policy-1",
							Type: Probabilistic,
							ProbabilisticConfig: ProbabilisticConfig{
								HashSalt:           "test-hash",
								SamplingPercentage: 0,
							},
						},
					},
					{
						SharedPolicyConfig: SharedPolicyConfig{
							Name: "test-policy-1",
							Type: Probabilistic,
							ProbabilisticConfig: ProbabilisticConfig{
								HashSalt:           "test-hash",
								SamplingPercentage: 0,
							},
						},
					},
				},
			},
			expectedError: duplicatePolicyName,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.Validate()
			if tc.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err, tc.expectedError)
			}
		})
	}
}
