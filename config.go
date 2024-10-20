package atlassiansamplingprocessor // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"
)

type Config struct {
	// PolicyConfig sets the tail-based sampling policy which makes a sampling decision
	// for a given trace when requested.
	PolicyConfig []PolicyConfig `mapstructure:"policies"`

	// TargetHeapBytes, is the optional target heap size runtime.MemStats.HeapAlloc.
	// If set, the processor may adjust cache sizes dynamically in order to keep within the target.
	// A good starting point to set this is about 75% of overall memory resource allocation.
	TargetHeapBytes uint64 `mapstructure:"target_heap_bytes"`

	// PrimaryCacheSize sets the initial and maximum size of the primary cache that holds non-low priority traces.
	PrimaryCacheSize int `mapstructure:"primary_cache_size"`

	// SecondaryCacheSize defaults to 10% of the primary cache size.
	// It should not more than 50% of the primary cache size
	SecondaryCacheSize int `mapstructure:"secondary_cache_size"`

	DecisionCacheCfg `mapstructure:"decision_cache"`

	// FlushOnShutdown determines whether to flush the pending/cached trace data upon shutdown.
	FlushOnShutdown bool `mapstructure:"flush_on_shutdown"`

	// CompressionEnabled compresses trace data in the primary and secondary caches if enabled
	CompressionEnabled bool `mapstructure:"compression_enabled"`
}

var (
	primaryCacheSizeError   = errors.New("primary_cache_size must be greater than 0")
	secondaryCacheSizeError = errors.New("secondary_cache_size must be greater than 0 and less than 50% of primary_cache_size")
)

var _ component.Config = (*Config)(nil)

func createDefaultConfig() component.Config {
	return &Config{
		PrimaryCacheSize:   1000,
		SecondaryCacheSize: 100,
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

func (cfg *Config) Validate() (errors error) {
	if cfg.PrimaryCacheSize <= 0 {
		errors = multierr.Append(errors, primaryCacheSizeError)
	}

	if cfg.SecondaryCacheSize <= 0 || cfg.SecondaryCacheSize > cfg.PrimaryCacheSize/2 {
		errors = multierr.Append(errors, secondaryCacheSizeError)
	}

	return errors
}
