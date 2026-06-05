package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/cache"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/metadata"
)

var testTelem = func() *metadata.TelemetryBuilder {
	tb, _ := metadata.NewTelemetryBuilder(componenttest.NewNopTelemetrySettings())
	return tb
}()
var testCacheName = "testCache"

type heapGetterMock struct {
	heap uint64
}

func (hg *heapGetterMock) GetHeapUsage() uint64 {
	return hg.heap
}

func TestRegulatorAdjustsCacheSize(t *testing.T) {
	t.Parallel()
	hgm := &heapGetterMock{heap: 9000}
	c, err := cache.NewLRUCache[bool](1000, func(pcommon.TraceID, bool) {}, testTelem, testCacheName)
	require.NoError(t, err)
	r, err := NewRegulator(500, 1000, 10000, hgm.GetHeapUsage, c, zap.NewNop())
	require.NoError(t, err)

	// Steady zone (0.85-1.0): no adjustment
	assert.Equal(t, 1000, r.RegulateCacheSize())

	// Grow mode: heap well below target, +2%
	c.Resize(900)
	hgm.heap = 800
	assert.Equal(t, 918, r.RegulateCacheSize()) // 900 * 1.02

	// Moderate pressure (1.0-1.15): proportional shrink
	c.Resize(1000)
	hgm.heap = 11000
	assert.Equal(t, 909, r.RegulateCacheSize()) // 1000 * 10000/11000
	assert.Equal(t, 826, r.RegulateCacheSize()) // 909 * 10000/11000

	// Heap drops to steady zone: no change
	hgm.heap = 9500
	assert.Equal(t, 826, r.RegulateCacheSize())

	// High pressure (1.15-1.3): squared proportional shrink
	c.Resize(1000)
	hgm.heap = 12000
	assert.Equal(t, 694, r.RegulateCacheSize()) // 1000 * (10000/12000)^2

	// Emergency (>1.3): drop to minSize
	c.Resize(1000)
	hgm.heap = 14000
	assert.Equal(t, 500, r.RegulateCacheSize())

	// Recovery: grow gently from min
	hgm.heap = 5000
	assert.Equal(t, 510, r.RegulateCacheSize()) // 500 * 1.02
	assert.Equal(t, 520, r.RegulateCacheSize()) // 510 * 1.02
}

func TestRegulatorClampsToMinMax(t *testing.T) {
	t.Parallel()
	hgm := &heapGetterMock{heap: 5000}
	c, err := cache.NewLRUCache[bool](990, func(pcommon.TraceID, bool) {}, testTelem, testCacheName)
	require.NoError(t, err)
	r, err := NewRegulator(500, 1000, 10000, hgm.GetHeapUsage, c, zap.NewNop())
	require.NoError(t, err)

	// Heap well below target, growth should be clamped to maxSize
	newSize := r.RegulateCacheSize()
	assert.Equal(t, 1000, newSize) // 990 * 1.02 = 1009 → clamped to 1000

	// High pressure shouldn't go below minSize
	hgm.heap = 12500
	c.Resize(600)
	newSize = r.RegulateCacheSize()
	// scale = 10000/12500 = 0.8, scale^2 = 0.64, 600 * 0.64 = 384 → clamped to 500
	assert.Equal(t, 500, newSize)
}

func TestRegulatorValidatesInput(t *testing.T) {
	t.Parallel()
	hgm := &heapGetterMock{heap: 100}
	c, _ := cache.NewLRUCache[bool](100, func(pcommon.TraceID, bool) {}, testTelem, testCacheName)

	// Invalid minSize
	r, err := NewRegulator(-1, 1, 1, hgm.GetHeapUsage, c, zap.NewNop())
	assert.Nil(t, r)
	assert.Error(t, err)

	// Invalid maxSize
	r, err = NewRegulator(0, 0, 1, hgm.GetHeapUsage, c, zap.NewNop())
	assert.Nil(t, r)
	assert.Error(t, err)

	// Invalid targetHeap
	r, err = NewRegulator(1, 10, 0, hgm.GetHeapUsage, c, zap.NewNop())
	assert.Nil(t, r)
	assert.Error(t, err)

	// Invalid sizes
	r, err = NewRegulator(100, 99, 1, hgm.GetHeapUsage, c, zap.NewNop())
	assert.Nil(t, r)
	assert.Error(t, err)

	// Invalid HeapGetter
	r, err = NewRegulator(100, 200, 100, nil, c, zap.NewNop())
	assert.Nil(t, r)
	assert.Error(t, err)

	// Invalid cache
	r, err = NewRegulator(100, 200, 100, hgm.GetHeapUsage, nil, zap.NewNop())
	assert.Nil(t, r)
	assert.Error(t, err)
}

func TestGetHeapUsage(t *testing.T) {
	heap := GetHeapUsage()
	assert.NotZero(t, heap)
}
