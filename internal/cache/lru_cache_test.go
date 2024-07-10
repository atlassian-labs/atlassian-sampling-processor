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

func TestSinglePut(t *testing.T) {
	tb, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	c, err := NewLRUCache[int](2, func(uint64, int) {}, tb)
	require.NoError(t, err)
	id, err := traceIDFromHex("12341234123412341234123412341234")
	require.NoError(t, err)
	c.Put(id, 123)
	v, ok := c.Get(id)
	assert.Equal(t, 123, v)
	assert.True(t, ok)
}

func TestExceedsSizeLimit(t *testing.T) {
	tb, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	c, err := NewLRUCache[bool](2, func(uint64, bool) {}, tb)
	require.NoError(t, err)
	id1, err := traceIDFromHex("12341234123412341234123412341231")
	require.NoError(t, err)
	id2, err := traceIDFromHex("12341234123412341234123412341232")
	require.NoError(t, err)
	id3, err := traceIDFromHex("12341234123412341234123412341233")
	require.NoError(t, err)

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
	tb, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	c, err := NewLRUCache[bool](2, func(uint64, bool) {}, tb)
	require.NoError(t, err)
	id1, err := traceIDFromHex("12341234123412341234123412341231")
	require.NoError(t, err)
	id2, err := traceIDFromHex("12341234123412341234123412341232")
	require.NoError(t, err)
	id3, err := traceIDFromHex("12341234123412341234123412341233")
	require.NoError(t, err)

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
	tb, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	c, err := NewLRUCache[bool](2, func(uint64, bool) {}, tb)
	require.NoError(t, err)
	id, _ := traceIDFromHex("12341234123412341234123412341231")
	c.Put(id, true)
	_, ok := c.Get(id)
	assert.True(t, ok)
	c.Delete(id)
	res, ok := c.Get(id)
	assert.False(t, ok)
	assert.False(t, res)
}

func TestOnlyUsesRightHalfTraceID(t *testing.T) {
	tb, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	c, err := NewLRUCache[int](5, func(uint64, int) {}, tb)
	require.NoError(t, err)

	// Same right half
	id1, _ := traceIDFromHex("00000000000000001111111111111111")
	id2, _ := traceIDFromHex("ffffffffffffffff1111111111111111")
	c.Put(id1, 1)
	c.Put(id2, 2)

	res, _ := c.Get(id1)
	assert.Equal(t, 2, res, "the put of id2 should overwrite id1's")
}

func TestZeroSizeReturnsError(t *testing.T) {
	tb, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	c, err := NewLRUCache[bool](0, func(uint64, bool) {}, tb)
	assert.Error(t, err)
	assert.Nil(t, c)
}

func traceIDFromHex(idStr string) (pcommon.TraceID, error) {
	id := pcommon.NewTraceIDEmpty()
	_, err := hex.Decode(id[:], []byte(idStr))
	return id, err
}
