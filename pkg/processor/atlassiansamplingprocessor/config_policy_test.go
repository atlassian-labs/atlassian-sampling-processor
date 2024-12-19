package atlassiansamplingprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/metadata"
)

func TestPolicyCreationFromConfig(t *testing.T) {
	t.Parallel()
	set := componenttest.NewNopTelemetrySettings()
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "atlassian_sampling_test_cfg.yml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	policies, err := newPolicies(cfg.(*Config).PolicyConfig, set)
	require.NoError(t, err)

	assert.Equal(t, "test-policy-1", policies[0].name)
	assert.Equal(t, Probabilistic, policies[0].policyType)
	assert.Equal(t, "test-policy-2", policies[1].name)
	assert.Equal(t, And, policies[1].policyType)
	assert.Equal(t, "test-policy-3", policies[2].name)
	assert.Equal(t, SpanCount, policies[2].policyType)
	assert.Equal(t, "test-policy-4", policies[3].name)
	assert.Equal(t, RootSpans, policies[3].policyType)
	assert.Equal(t, "test-policy-5", policies[4].name)
	assert.Equal(t, Latency, policies[4].policyType)
	assert.Equal(t, "test-policy-6", policies[5].name)
	assert.Equal(t, StatusCode, policies[5].policyType)
	assert.Equal(t, "test-policy-7", policies[6].name)
	assert.Equal(t, OTTLCondition, policies[6].policyType)
	assert.Equal(t, "test-policy-8", policies[7].name)
	assert.Equal(t, Threshold, policies[7].policyType)
	assert.Equal(t, "test-policy-9", policies[8].name)
	assert.Equal(t, RemoteProbabilistic, policies[8].policyType)
	assert.Equal(t, "test-policy-10", policies[9].name)
	assert.Equal(t, Downgrader, policies[9].policyType)

	assert.Equal(t, 10, len(policies), "wrong number of assertions")
}
