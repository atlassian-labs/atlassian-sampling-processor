package atlassiansamplingprocessor // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor"

import (
	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// PolicyConfig sets the tail-based sampling policy which makes a sampling decision
	// for a given trace when requested.
	PolicyConfig []PolicyConfig `mapstructure:"policies"`

	// MaxTraces configures the internal cache size for cached trace data.
	// When the cache reaches its limit, the cache evicts the least recently used trace.
	MaxTraces int `mapstructure:"max_traces"`

	DecisionCacheCfg `mapstructure:"decision_cache"`
}

var _ component.Config = (*Config)(nil)

func createDefaultConfig() component.Config {
	return &Config{
		MaxTraces: 1000,
		DecisionCacheCfg: DecisionCacheCfg{
			SampledCacheSize:    10000,
			NonSampledCacheSize: 10000,
		},
		PolicyConfig: make([]PolicyConfig, 0),
	}
}

type DecisionCacheCfg struct {
	// SampledCacheSize specifies the size of the cache that holds the sampled trace IDs.
	// This value will be the maximum amount of trace IDs that the cache can hold before overwriting previous IDs.
	// For effective use, this value should be at least an order of magnitude higher than Config.MaxTraces.
	// By default, 10x the Config.MaxTraces value will be used.
	SampledCacheSize int `mapstructure:"sampled_cache_size"`
	// NonSampledCacheSize specifies the size of the cache that holds the non-sampled trace IDs.
	// This value will be the maximum amount of trace IDs that the cache can hold before overwriting previous IDs.
	// For effective use, this value should be at least an order of magnitude higher than Config.MaxTraces.
	// By default, 10x the Config.MaxTraces value will be used.
	NonSampledCacheSize int `mapstructure:"non_sampled_cache_size"`
}
