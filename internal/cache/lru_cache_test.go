// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package cache

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/metadata"
)

var testTelem = func() *metadata.TelemetryBuilder {
	tb, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	return tb
}()

func TestSinglePut(t *testing.T) {
	t.Parallel()
	c, err := NewLRUCache[int](2, func(uint64, int) {}, testTelem)
	require.NoError(t, err)
	id := traceIDFromHex(t, "12341234123412341234123412341234")
	c.Put(id, 123)
	v, ok := c.Get(id)
	assert.Equal(t, 123, v)
	assert.True(t, ok)
}

func TestExceedsSizeLimit(t *testing.T) {
	t.Parallel()
	c, err := NewLRUCache[bool](2, func(uint64, bool) {}, testTelem)
	require.NoError(t, err)
	id1 := traceIDFromHex(t, "12341234123412341234123412341231")
	id2 := traceIDFromHex(t, "12341234123412341234123412341232")
	id3 := traceIDFromHex(t, "12341234123412341234123412341233")

	c.Put(id1, true)
	c.Put(id2, true)
	c.Put(id3, true)

	v, ok := c.Get(id1)
	assert.False(t, v)  // evicted
	assert.False(t, ok) // evicted
	v, ok = c.Get(id2)
	assert.True(t, v)
	assert.True(t, ok)
	v, ok = c.Get(id3)
	assert.True(t, v)
	assert.True(t, ok)
}

func TestLeastRecentlyUsedIsEvicted(t *testing.T) {
	t.Parallel()
	c, err := NewLRUCache[bool](2, func(uint64, bool) {}, testTelem)
	require.NoError(t, err)
	id1 := traceIDFromHex(t, "12341234123412341234123412341231")
	id2 := traceIDFromHex(t, "12341234123412341234123412341232")
	id3 := traceIDFromHex(t, "12341234123412341234123412341233")

	c.Put(id1, true)
	c.Put(id2, true)
	v, ok := c.Get(id1) // use id1
	assert.True(t, true, v)
	assert.True(t, true, ok)
	c.Put(id3, true)

	v, ok = c.Get(id1)
	assert.True(t, v)
	assert.True(t, ok)
	v, ok = c.Get(id2)
	assert.False(t, v)  // evicted, returns zero-value
	assert.False(t, ok) // evicted, not OK
	v, ok = c.Get(id3)
	assert.True(t, v)
	assert.True(t, ok)
}

func TestDelete(t *testing.T) {
	t.Parallel()
	c, err := NewLRUCache[bool](2, func(uint64, bool) {}, testTelem)
	require.NoError(t, err)
	id := traceIDFromHex(t, "12341234123412341234123412341231")
	c.Put(id, true)
	_, ok := c.Get(id)
	assert.True(t, ok)
	c.Delete(id)
	res, ok := c.Get(id)
	assert.False(t, ok)
	assert.False(t, res)
}

func TestGetValues(t *testing.T) {
	t.Parallel()
	id1 := traceIDFromHex(t, "11111111111111111111111111111111")
	id2 := traceIDFromHex(t, "22222222222222222222222222222222")
	c, err := NewLRUCache[int](2, func(uint64, int) {}, testTelem)
	require.NoError(t, err)

	c.Put(id1, 1)
	c.Put(id2, 2)
	vals := c.Values()

	assert.Equal(t, []int{1, 2}, vals)
}

func TestOnlyUsesRightHalfTraceID(t *testing.T) {
	t.Parallel()
	c, err := NewLRUCache[int](5, func(uint64, int) {}, testTelem)
	require.NoError(t, err)

	// Same right half
	id1 := traceIDFromHex(t, "00000000000000001111111111111111")
	id2 := traceIDFromHex(t, "ffffffffffffffff1111111111111111")
	c.Put(id1, 1)
	c.Put(id2, 2)

	res, _ := c.Get(id1)
	assert.Equal(t, 2, res, "the put of id2 should overwrite id1's")
}

func TestZeroSizeReturnsError(t *testing.T) {
	t.Parallel()
	c, err := NewLRUCache[bool](0, func(uint64, bool) {}, testTelem)
	assert.Error(t, err)
	assert.Nil(t, c)
}

func TestSize(t *testing.T) {
	t.Parallel()
	c, _ := NewLRUCache[bool](2, func(uint64, bool) {}, testTelem)
	assert.Equal(t, 2, c.Size())
	c.Put(traceIDFromHex(t, "00000000000000001111111111111111"), true)
	assert.Equal(t, 2, c.Size())
	c.Put(traceIDFromHex(t, "00000000000000001111111111111112"), true)
	assert.Equal(t, 2, c.Size())
	c.Put(traceIDFromHex(t, "00000000000000001111111111111113"), true)
	assert.Equal(t, 2, c.Size())
}

func TestResize(t *testing.T) {
	t.Parallel()
	c, _ := NewLRUCache[bool](2, func(uint64, bool) {}, testTelem)
	id1 := traceIDFromHex(t, "00000000000000001111111111111111")
	id2 := traceIDFromHex(t, "00000000000000001111111111111112")
	c.Put(id1, true)
	c.Put(id2, true)
	assert.Equal(t, 2, c.Size())

	c.Resize(1)
	assert.Equal(t, 1, c.Size())
	_, ok := c.Get(id1)
	assert.False(t, ok)
	_, ok = c.Get(id2)
	assert.True(t, ok)

	c.Resize(3)
	assert.Equal(t, 3, c.Size())
	c.Put(id1, true)
	c.Put(id2, true)
	id3 := traceIDFromHex(t, "00000000000000001111111111111113")
	c.Put(id3, true)

	_, ok = c.Get(id1)
	assert.True(t, ok)
	_, ok = c.Get(id2)
	assert.True(t, ok)
	_, ok = c.Get(id3)
	assert.True(t, ok)
}

func traceIDFromHex(t testing.TB, idStr string) pcommon.TraceID {
	id := pcommon.NewTraceIDEmpty()
	_, err := hex.Decode(id[:], []byte(idStr))
	require.NoError(t, err)
	return id
}
