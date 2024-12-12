package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/cache"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/metadata"
)

var testTelem = func() *metadata.TelemetryBuilder {
	tb, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	return tb
}()

type heapGetterMock struct {
	heap uint64
}

func (hg *heapGetterMock) GetHeapUsage() uint64 {
	return hg.heap
}

func TestRegulatorAdjustsCacheSize(t *testing.T) {
	t.Parallel()
	hgm := &heapGetterMock{heap: 100}
	c, err := cache.NewLRUCache[bool](100, func(pcommon.TraceID, bool) {}, testTelem)
	require.NoError(t, err)
	r, err := NewRegulator(50, 100, 1000, hgm.GetHeapUsage, c)
	require.NoError(t, err)

	// heap usage is low, and cache size is at max, so no adjustment is required
	newSize := r.RegulateCacheSize()
	assert.Equal(t, 100, newSize)

	// set heap above target heap
	hgm.heap = 1001
	newSize = r.RegulateCacheSize()
	assert.Equal(t, 98, newSize)
	// Should reduce again because heap still above 1000
	newSize = r.RegulateCacheSize()
	assert.Equal(t, 96, newSize)
	// Target is achieved, and we are above 90% of the heap target, so the size should stay static
	hgm.heap = 950
	newSize = r.RegulateCacheSize()
	assert.Equal(t, 96, newSize)

	// Decrease heap below 90%, size should increase
	hgm.heap = 899
	newSize = r.RegulateCacheSize()
	assert.Equal(t, 97, newSize)
}

func TestRegulatorValidatesInput(t *testing.T) {
	t.Parallel()
	hgm := &heapGetterMock{heap: 100}
	c, _ := cache.NewLRUCache[bool](100, func(pcommon.TraceID, bool) {}, testTelem)

	// Invalid minSize
	r, err := NewRegulator(-1, 1, 1, hgm.GetHeapUsage, c)
	assert.Nil(t, r)
	assert.Error(t, err)

	// Invalid maxSize
	r, err = NewRegulator(0, 0, 1, hgm.GetHeapUsage, c)
	assert.Nil(t, r)
	assert.Error(t, err)

	// Invalid targetHeap
	r, err = NewRegulator(1, 10, 0, hgm.GetHeapUsage, c)
	assert.Nil(t, r)
	assert.Error(t, err)

	// Invalid sizes
	r, err = NewRegulator(100, 99, 1, hgm.GetHeapUsage, c)
	assert.Nil(t, r)
	assert.Error(t, err)

	// Invalid HeapGetter
	r, err = NewRegulator(100, 200, 100, nil, c)
	assert.Nil(t, r)
	assert.Error(t, err)

	// Invalid cache
	r, err = NewRegulator(100, 200, 100, hgm.GetHeapUsage, nil)
	assert.Nil(t, r)
	assert.Error(t, err)
}

func TestGetHeapUsage(t *testing.T) {
	heap := GetHeapUsage()
	assert.NotZero(t, heap)
}
