// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package cache // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/cache"

import (
	"context"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/metadata"
)

// pre compute attributes for performance
var (
	trueAttr  = metric.WithAttributeSet(attribute.NewSet(attribute.Bool("hit", true)))
	falseAttr = metric.WithAttributeSet(attribute.NewSet(attribute.Bool("hit", false)))
)

// lruCache implements Cache as a simple LRU cache.
type lruCache[V any] struct {
	cache     *lru.Cache[pcommon.TraceID, V]
	telemetry *metadata.TelemetryBuilder
	size      int

	// we store the attributes here, to avoid new allocations on the hot path
	cacheNameAttr metric.MeasurementOption
}

var _ Cache[any] = (*lruCache[any])(nil)

// NewLRUCache returns a new lruCache.
// The size parameter indicates the amount of keys the cache will hold before it
// starts evicting the least recently used key.
func NewLRUCache[V any](size int, onEvicted func(pcommon.TraceID, V), telemetry *metadata.TelemetryBuilder, name string) (Cache[V], error) {
	c, err := lru.NewWithEvict[pcommon.TraceID, V](size, onEvicted)
	if err != nil {
		return nil, err
	}
	return &lruCache[V]{cache: c, size: size, telemetry: telemetry,
		cacheNameAttr: metric.WithAttributeSet(attribute.NewSet(attribute.String("cache", name)))}, nil
}

func (c *lruCache[V]) Get(id pcommon.TraceID) (V, bool) {
	v, ok := c.cache.Get(id)
	if ok {
		c.telemetry.ProcessorAtlassianSamplingCacheReads.
			Add(context.Background(), 1, trueAttr, c.cacheNameAttr)
	} else {
		c.telemetry.ProcessorAtlassianSamplingCacheReads.
			Add(context.Background(), 1, falseAttr, c.cacheNameAttr)
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
