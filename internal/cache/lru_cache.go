// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package cache // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/cache"

import (
	"context"
	"encoding/binary"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/metadata"
)

// lruCache implements Cache as a simple LRU cache.
type lruCache[V any] struct {
	cache     *lru.Cache[uint64, V]
	telemetry *metadata.TelemetryBuilder
}

var _ Cache[any] = (*lruCache[any])(nil)

// NewLRUCache returns a new lruCache.
// The size parameter indicates the amount of keys the cache will hold before it
// starts evicting the least recently used key.
func NewLRUCache[V any](size int, onEvicted func(uint64, V), telemetry *metadata.TelemetryBuilder) (Cache[V], error) {
	c, err := lru.NewWithEvict[uint64, V](size, onEvicted)
	if err != nil {
		return nil, err
	}
	return &lruCache[V]{cache: c, telemetry: telemetry}, nil
}

func (c *lruCache[V]) Get(id pcommon.TraceID) (V, bool) {
	v, ok := c.cache.Get(rightHalfTraceID(id))
	c.telemetry.ProcessorAtlassianSamplingCacheReads.
		Add(context.Background(), 1, metric.WithAttributes(attribute.Bool("hit", ok)))
	return v, ok
}

func (c *lruCache[V]) Put(id pcommon.TraceID, v V) {
	_ = c.cache.Add(rightHalfTraceID(id), v)
}

func (c *lruCache[V]) Delete(id pcommon.TraceID) {
	c.cache.Remove(rightHalfTraceID(id))
}

func (c *lruCache[V]) Values() []V {
	return c.cache.Values()
}

func rightHalfTraceID(id pcommon.TraceID) uint64 {
	return binary.LittleEndian.Uint64(id[8:])
}
