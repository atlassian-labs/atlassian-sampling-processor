package evaluators

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

func TestStartRemoteProbabilistic(t *testing.T) {
	t.Parallel()

	// with valid extension
	extId := component.MustNewID("rate_getter")
	ext := &mockRateGetterExtension{rate: 100}
	host := &mockHost{extensions: map[component.ID]component.Component{
		extId: ext,
	}}

	sampler := NewRemoteProbabilisticSampler("", 0, extId)
	err := sampler.Start(context.Background(), host)

	assert.NoError(t, err)

	// without any extension
	extId = component.ID{}

	sampler = NewRemoteProbabilisticSampler("", 0, extId)
	err = sampler.Start(context.Background(), host)

	assert.NoError(t, err)
}

func TestExtensionNotFound(t *testing.T) {
	t.Parallel()

	extId := component.MustNewID("rate_getter")
	host := &mockHost{}

	sampler := NewRemoteProbabilisticSampler("", 0, extId)
	err := sampler.Start(context.Background(), host)

	assert.Error(t, err)
}

func TestExtensionInvalid(t *testing.T) {
	t.Parallel()

	extId := component.MustNewID("not_a_rate_getter")
	ext := &nopExtension{}
	host := &mockHost{extensions: map[component.ID]component.Component{
		extId: ext,
	}}

	sampler := NewRemoteProbabilisticSampler("", 0, extId)
	err := sampler.Start(context.Background(), host)

	assert.Error(t, err)
}

func TestRemoteProbabilisticDefaultRate(t *testing.T) {
	t.Parallel()

	ext := &mockRateGetterExtension{rate: -1, err: errors.New("error from extension")}
	sampler, e := startTestSampler(ext, 100)
	require.NoError(t, e)

	trace := ptrace.NewTraces()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	mergedMetadata := &tracedata.Metadata{}

	decision, err := sampler.Evaluate(context.Background(), traceID, trace, mergedMetadata)

	assert.Error(t, err)
	assert.Equal(t, Sampled, decision)
}

func TestRemoteProbabilisticSampling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		samplingRate         float64
		hashSalt             string
		expectedSamplingRate float64
	}{
		{
			"100%",
			100,
			"",
			100,
		},
		{
			"0%",
			0,
			"",
			0,
		},
		{
			"25%",
			25,
			"",
			25,
		},
		{
			"33%",
			33,
			"",
			33,
		},
		{
			"33% - custom salt",
			33,
			"test-salt",
			33,
		},
		{
			"-%50",
			-50,
			"",
			0,
		},
		{
			"150%",
			150,
			"",
			100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceCount := 100_000

			ext := &mockRateGetterExtension{rate: tt.samplingRate}
			sampler, err := startTestSampler(ext, 0)
			require.NoError(t, err)

			sampled := 0
			for _, traceID := range genRandomTraceIDs(traceCount) {
				trace := ptrace.NewTraces()
				mergedMetadata := &tracedata.Metadata{}

				decision, _ := sampler.Evaluate(context.Background(), traceID, trace, mergedMetadata)

				if decision == Sampled {
					sampled++
				}
			}

			effectiveSamplingRate := float32(sampled) / float32(traceCount) * 100
			assert.InDelta(t, tt.expectedSamplingRate, effectiveSamplingRate, 0.2,
				"Effective sampling rate is %f, expected %f", effectiveSamplingRate, tt.expectedSamplingRate,
			)
		})
	}
}

func startTestSampler(ext component.Component, defaultRate float64) (PolicyEvaluator, error) {
	extId := component.MustNewID("rate_getter")
	host := mockHost{extensions: map[component.ID]component.Component{
		extId: ext,
	}}

	sampler := NewRemoteProbabilisticSampler("", defaultRate, extId)
	err := sampler.Start(context.Background(), host)

	return sampler, err
}

type mockHost struct {
	component.Host

	extensions map[component.ID]component.Component
}

func (h mockHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

type mockRateGetterExtension struct {
	extension.Extension

	rate float64
	err  error
}

func (rg mockRateGetterExtension) GetFloat64() (float64, error) {
	return rg.rate, rg.err
}

type nopExtension struct {
	extension.Extension
}
