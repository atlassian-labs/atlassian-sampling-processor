package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/priority"
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

	assert.Equal(t, 20, c.Size())

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

	assert.Equal(t, []pcommon.TraceID{id0, id1}, c.Keys())

	c.Delete(id0)
	v, ok = c.Get(id0)
	assert.False(t, ok)
	assert.Nil(t, v)

	c.Delete(id1)
	v, ok = c.Get(id1)
	assert.False(t, ok)
	assert.Nil(t, v)

	assert.Equal(t, 0, len(c.Values()))
	assert.Equal(t, 0, len(c.Keys()))
}

func TestTieredClear(t *testing.T) {
	primary, err := NewLRUCache[*testPriorityGetter](10, nil, testTelem)
	require.NoError(t, err)
	secondary, err := NewLRUCache[*testPriorityGetter](10, nil, testTelem)
	require.NoError(t, err)
	c, err := NewTieredCache[*testPriorityGetter](primary, secondary)
	require.NoError(t, err)
	require.NotNil(t, c)

	id0 := traceIDFromHex(t, "12341234123412341234123412341234")
	id1 := traceIDFromHex(t, "56785678567856785678567856785678")
	c.Put(id0, tpgLow)
	c.Put(id1, tpgHigh)

	assert.Equal(t, 20, c.Size())
	assert.Equal(t, 2, len(c.Keys()))
	c.Clear()
	assert.Equal(t, 20, c.Size())
	assert.Equal(t, 0, len(c.Keys()))
	_, ok := c.Get(id0)
	assert.False(t, ok)
	_, ok = c.Get(id1)
	assert.False(t, ok)
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

func TestResize_DoesNothing(t *testing.T) {
	primary, _ := NewLRUCache[*testPriorityGetter](10, nil, testTelem)
	secondary, _ := NewLRUCache[*testPriorityGetter](10, nil, testTelem)
	c, _ := NewTieredCache[*testPriorityGetter](primary, secondary)
	assert.Equal(t, 20, c.Size())
	c.Resize(100)
	assert.Equal(t, 20, c.Size())
}
