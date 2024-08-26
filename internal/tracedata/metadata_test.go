package tracedata

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestMetadataDeepCopy(t *testing.T) {
	t.Parallel()
	sc := &atomic.Int64{}
	sc.Store(1)
	m1 := &Metadata{
		ArrivalTime:       time.Now(),
		EarliestStartTime: 1,
		LatestEndTime:     2,
		SpanCount:         sc,
	}

	m2 := m1.DeepCopy()

	// m1 and m2 should have the same values
	assert.Equal(t, m1, m2)
	// m1 and m2 should be 2 different pointers
	assert.NotSame(t, m1, m2)

	// Updating the copy should not affect the original metadata
	m2.SpanCount.Store(2)
	m2.ArrivalTime = time.Now()
	m2.EarliestStartTime = 3
	m2.LatestEndTime = 4

	assert.NotEqual(t, m1.ArrivalTime, m2.ArrivalTime)
	assert.Equal(t, int64(1), m1.SpanCount.Load())
	assert.Equal(t, pcommon.Timestamp(1), m1.EarliestStartTime)
	assert.Equal(t, pcommon.Timestamp(2), m1.LatestEndTime)
}
