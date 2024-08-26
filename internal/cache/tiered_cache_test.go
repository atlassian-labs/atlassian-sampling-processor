package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/priority"
)

var tpgLow = &testPriorityGetter{p: priority.Low}
var tpgHigh = &testPriorityGetter{p: priority.Unspecified}

type testPriorityGetter struct {
	p priority.Priority
}

func (t *testPriorityGetter) GetPriority() priority.Priority {
	return t.p
}

func TestTieredCache(t *testing.T) {
	primary, err := NewLRUCache[*testPriorityGetter](10, nil, testTelem)
	require.NoError(t, err)
	secondary, err := NewLRUCache[*testPriorityGetter](10, nil, testTelem)
	require.NoError(t, err)
	c, err := NewTieredCache[*testPriorityGetter](primary, secondary)
	require.NoError(t, err)
	require.NotNil(t, c)

	id0 := traceIDFromHex(t, "12341234123412341234123412341234")
	c.Put(id0, tpgLow)
	v, ok := c.Get(id0)
	assert.True(t, ok)
	assert.Equal(t, tpgLow, v)

	id1 := traceIDFromHex(t, "56785678567856785678567856785678")
	c.Put(id1, tpgHigh)
	v, ok = c.Get(id1)
	assert.True(t, ok)
	assert.Equal(t, tpgHigh, v)

	idNotExist := traceIDFromHex(t, "abcdabcdabcdabcdabcdabcdabcdabcd")
	v, ok = c.Get(idNotExist)
	assert.False(t, ok)
	assert.Nil(t, v)

	// promote id0 to high priority
	c.Put(id0, tpgHigh)
	v, ok = c.Get(id0)
	assert.True(t, ok)
	assert.Equal(t, tpgHigh, v)

	// demote id1 to low priority
	c.Put(id1, tpgLow)
	v, ok = c.Get(id1)
	assert.True(t, ok)
	assert.Equal(t, tpgLow, v)

	vals := c.Values()
	assert.Equal(t, []*testPriorityGetter{tpgHigh, tpgLow}, vals)

	c.Delete(id0)
	v, ok = c.Get(id0)
	assert.False(t, ok)
	assert.Nil(t, v)

	c.Delete(id1)
	v, ok = c.Get(id1)
	assert.False(t, ok)
	assert.Nil(t, v)

	vals = c.Values()
	assert.Equal(t, 0, len(vals))
}

func TestConstruction_Errors(t *testing.T) {
	c, err := NewLRUCache[*testPriorityGetter](10, nil, testTelem)
	require.NoError(t, err)
	_, err = NewTieredCache[*testPriorityGetter](nil, nil)
	assert.Error(t, err)
	_, err = NewTieredCache[*testPriorityGetter](c, nil)
	assert.Error(t, err)
	_, err = NewTieredCache[*testPriorityGetter](nil, c)
	assert.Error(t, err)
}
