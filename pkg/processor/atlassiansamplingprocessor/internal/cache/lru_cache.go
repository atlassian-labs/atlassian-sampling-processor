// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package cache // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/cache"

import (
	"context"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/metadata"
)

// lruCache implements Cache as a simple LRU cache.
type lruCache[V any] struct {
	cache     *lru.Cache[pcommon.TraceID, V]
	telemetry *metadata.TelemetryBuilder
	size      int

	// used for async recording of metrics, which we do for performance reasons (don't record a new metric every call)
	hits   atomic.Int64
	misses atomic.Int64
}

var _ Cache[any] = (*lruCache[any])(nil)

// NewLRUCache returns a new lruCache.
// The size parameter indicates the amount of keys the cache will hold before it
// starts evicting the least recently used key.
func NewLRUCache[V any](size int, onEvicted func(pcommon.TraceID, V), telemetry *metadata.TelemetryBuilder, name string) (Cache[V], error) {
	delegate, err := lru.NewWithEvict[pcommon.TraceID, V](size, onEvicted)
	if err != nil {
		return nil, err
	}

	c := &lruCache[V]{
		cache:     delegate,
		size:      size,
		telemetry: telemetry,
	}

	hitAttr := metric.WithAttributeSet(attribute.NewSet(attribute.String("cache", name), attribute.Bool("hit", true)))
	missAttr := metric.WithAttributeSet(attribute.NewSet(attribute.String("cache", name), attribute.Bool("hit", false)))
	err = telemetry.RegisterProcessorAtlassianSamplingCacheReadsCallback(func(ctx context.Context, o metric.Int64Observer) error {
		o.Observe(c.hits.Load(), hitAttr)
		o.Observe(c.misses.Load(), missAttr)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *lruCache[V]) Get(id pcommon.TraceID) (V, bool) {
	v, ok := c.cache.Get(id)
	if ok {
		c.hits.Add(1)
	} else {
		c.misses.Add(1)
	}
	return v, ok
}

func (c *lruCache[V]) Put(id pcommon.TraceID, v V) {
	_ = c.cache.Add(id, v)
}

func (c *lruCache[V]) Delete(id pcommon.TraceID) {
	c.cache.Remove(id)
}

func (c *lruCache[V]) Values() []V {
	return c.cache.Values()
}

func (c *lruCache[V]) Keys() []pcommon.TraceID {
	return c.cache.Keys()
}

// Size returns the current capacity of the LRU cache
func (c *lruCache[V]) Size() int {
	return c.size
}

// Resize sets a new size for the cache. If the cache size is being reduced,
// the oldest entries will be deleted.
func (c *lruCache[V]) Resize(size int) {
	c.cache.Resize(size)
	c.size = size
}

// Clear deletes all entries in the cache
func (c *lruCache[V]) Clear() {
	c.cache.Purge()
}
