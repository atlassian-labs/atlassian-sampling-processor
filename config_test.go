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
			MaxTraces:        100,
			DecisionCacheCfg: DecisionCacheCfg{SampledCacheSize: 1000, NonSampledCacheSize: 10000},
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
			},
		})
}
