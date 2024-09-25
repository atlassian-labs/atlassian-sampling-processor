package cache // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/cache"

import (
	"fmt"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/priority"
)

type tieredCache[V priority.Getter] struct {
	primary   Cache[V]
	secondary Cache[V]
}

var _ Cache[priority.Getter] = (*tieredCache[priority.Getter])(nil)

func NewTieredCache[V priority.Getter](primary Cache[V], secondary Cache[V]) (Cache[V], error) {
	if primary == nil || secondary == nil {
		return nil, fmt.Errorf("input cache was nil")
	}
	return &tieredCache[V]{
		primary:   primary,
		secondary: secondary,
	}, nil
}

func (tc *tieredCache[V]) Get(id pcommon.TraceID) (V, bool) {
	if v, ok := tc.primary.Get(id); ok {
		return v, ok
	}
	return tc.secondary.Get(id)
}

// Put inserts a key into the secondary cache if c.GetPriority() is priority.Low.
// Otherwise, it inserts the key into the primary cache.
// It will overwrite any entries of the same key in either the primary or secondary cache,
// to prevent a key appearing in both primary and secondary.
// If the caller wants to promote an existing key from secondary to primary, they can Put with non-low priority.
func (tc *tieredCache[V]) Put(id pcommon.TraceID, v V) {
	if v.GetPriority() == priority.Low {
		tc.primary.Delete(id)
		tc.secondary.Put(id, v)
	} else {
		tc.secondary.Delete(id)
		tc.primary.Put(id, v)
	}
}

func (tc *tieredCache[V]) Delete(id pcommon.TraceID) {
	tc.primary.Delete(id)
	tc.secondary.Delete(id)
}

func (tc *tieredCache[V]) Values() []V {
	return slices.Concat(tc.primary.Values(), tc.secondary.Values())
}

func (tc *tieredCache[V]) Size() int {
	return tc.primary.Size() + tc.secondary.Size()
}

// Resize is not supported on tieredCache
func (tc *tieredCache[V]) Resize(_ int) {}
