package atlassiansamplingprocessor // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor"

import (
	"errors"
	"time"

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

	// RegulateCacheDelay is the amount of time after which the processor starts regulating cache sizes based on the set TargetHeapBytes (if specified).
	// It is optional and defaults to 0s.
	// This can be used to avoid regulating cache sizes on an initial empty cache, as it can cause unnecessary cache size adjustments and high memory usage.
	RegulateCacheDelay time.Duration `mapstructure:"regulate_cache_delay"`

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
	duplicatePolicyName     = errors.New("duplicate policy names found in sampling policy config")
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

	err := validateUniquePolicyNames(cfg.PolicyConfig)
	if err != nil {
		errors = multierr.Append(errors, err)
	}

	return errors
}

func validateUniquePolicyNames(policies []PolicyConfig) error {
	policyNames := make(map[string]bool)
	for _, policy := range policies {
		if _, exists := policyNames[policy.Name]; exists {
			return duplicatePolicyName
		}
		policyNames[policy.Name] = true
	}
	return nil
}
