package memory // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/memory"

import (
	"fmt"
	"runtime"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/cache"
)

const increaseMultiplier float64 = 1.02
const decreaseMultiplier float64 = 0.98
const lowerThreshold float64 = 0.9

type HeapGetter func() uint64

type RegulatorI interface {
	RegulateCacheSize() int
}

type Regulator struct {
	minSize      int
	maxSize      int
	targetHeap   uint64
	getHeapUsage HeapGetter
	cache        cache.Sizer
}

func NewRegulator(minSize int, maxSize int, targetHeap uint64, hg HeapGetter, c cache.Sizer) (*Regulator, error) {
	if minSize < 0 || maxSize <= 0 || targetHeap <= 0 {
		return nil, fmt.Errorf("invalid input values")
	}
	if maxSize <= minSize {
		return nil, fmt.Errorf("maxSize must be larger than minSize")
	}
	if hg == nil || c == nil {
		return nil, fmt.Errorf("must not be nil")
	}
	return &Regulator{
		minSize:      minSize,
		maxSize:      maxSize,
		targetHeap:   targetHeap,
		getHeapUsage: hg,
		cache:        c,
	}, nil
}

// RegulateCacheSize decreases the cache size by 2% if the current heap size is above the target heap size.
// It increases the cache size by 2% if the current heap size is less than 90% of the target heap size.
func (r *Regulator) RegulateCacheSize() int {
	currSize := r.cache.Size()
	newSize := currSize
	heap := r.getHeapUsage()

	if heap < uint64(lowerThreshold*float64(r.targetHeap)) {
		// increase cache by 2%
		newSize = int(float64(currSize) * (increaseMultiplier))
	} else if heap > r.targetHeap {
		// reduce cache by 2%
		newSize = int(float64(currSize) * (decreaseMultiplier))
	}

	newSize = r.clamp(newSize)
	if newSize != currSize {
		r.cache.Resize(newSize)
	}
	return newSize
}

// clamp ensures the given value stays within the min/max bounds.
func (r *Regulator) clamp(x int) int {
	if x > r.maxSize {
		return r.maxSize
	}
	if x < r.minSize {
		return r.minSize
	}
	return x
}

func GetHeapUsage() uint64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.HeapAlloc
}
