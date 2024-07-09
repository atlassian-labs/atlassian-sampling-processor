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
			},
		})
}
