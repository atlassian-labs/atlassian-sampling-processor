package memory // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/memory"

import (
	"fmt"
	"runtime"

	"go.uber.org/zap"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/cache"
)

// Pressure mode thresholds as ratios of heap usage to target heap.
const (
	// growThreshold: below this ratio, the cache is allowed to grow gently.
	growThreshold float64 = 0.85
	// moderatePressureThreshold: above this ratio, shrink proportionally.
	moderatePressureThreshold float64 = 1.0
	// highPressureThreshold: above this ratio, shrink aggressively with a direct estimate.
	highPressureThreshold float64 = 1.15
	// emergencyThreshold: above this ratio, drop to minSize immediately.
	emergencyThreshold float64 = 1.3

	// growMultiplier is the gentle growth rate applied when heap is well below target.
	growMultiplier float64 = 1.02
)

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
	log          *zap.Logger
}

func NewRegulator(minSize int, maxSize int, targetHeap uint64, hg HeapGetter, c cache.Sizer, log *zap.Logger) (*Regulator, error) {
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
		log:          log,
	}, nil
}

// RegulateCacheSize uses a modal proportional controller to adjust cache size based on heap pressure.
// The controller operates in distinct modes based on the ratio of current heap usage to the target:
func (r *Regulator) RegulateCacheSize() int {
	currSize := r.cache.Size()
	heap := r.getHeapUsage()

	ratio := float64(heap) / float64(r.targetHeap)

	var newSize int
	switch {
	case ratio > emergencyThreshold:
		// Emergency: drop to minimum immediately
		newSize = r.minSize
	case ratio > highPressureThreshold:
		scale := (float64(r.targetHeap) / float64(heap))
		newSize = int(float64(currSize) * scale * scale)
	case ratio > moderatePressureThreshold:
		newSize = int(float64(currSize) * float64(r.targetHeap) / float64(heap))
	case ratio < growThreshold:
		newSize = int(float64(currSize) * growMultiplier)
	default:
		newSize = currSize
	}

	newSize = r.clamp(newSize)
	if newSize != currSize {
		r.cache.Resize(newSize)
		r.log.Info("memory regulator: cache size updated",
			zap.Int("old_size", currSize), zap.Int("new_size", newSize))
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
