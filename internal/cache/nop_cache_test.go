// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.
package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNopCache(t *testing.T) {
	c := NewNopDecisionCache[bool]()
	id := traceIDFromHex(t, "12341234123412341234123412341234")
	c.Put(id, true)
	v, ok := c.Get(id)
	assert.False(t, v)
	assert.False(t, ok)
	c.Delete(id)
	assert.Equal(t, []bool{}, c.Values())
}
