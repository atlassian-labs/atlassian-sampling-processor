package atlassiansamplingprocessor // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor"

import (
	"crypto/rand"
	"hash/fnv"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"golang.org/x/sync/errgroup"
)

var seed = []byte("s33d")

type spansGroupedByTraceID map[pcommon.TraceID][]spanAndScope

type groupedTraceData struct {
	id       pcommon.TraceID
	resource pcommon.Resource
	spans    []spanAndScope
}

// egWaiter returns a channel that will send when the group g is finished
func egWaiter(g *errgroup.Group) <-chan error {
	c := make(chan error)
	go func() {
		c <- g.Wait()
	}()
	return c
}

// wgWaiter returns a channel that will send when the waitgroup wg is finished
func wgWaiter(wg *sync.WaitGroup) <-chan struct{} {
	c := make(chan struct{})
	go func() {
		wg.Wait()
		close(c)
	}()
	return c
}

// shardIDForTrace returns a shard number from 0 to n-1 for a given TraceID
func shardIDForTrace(tid pcommon.TraceID, n int) int {
	h := fnv.New32a()
	// Using a seed specific to this processor rules out clashing with any other previous sharding
	// that might be using the same hashing algorithm (e.g. the load balancing exporter).
	_, _ = h.Write(seed)
	_, _ = h.Write(tid[:])
	return int(h.Sum32() % uint32(n)) //nolint G115: integer overflow conversion int
}

// To generate a random span ID for the placeholder trace
func generateRandomSpanID() pcommon.SpanID {
	var id [8]byte
	_, err := rand.Read(id[:])
	if err != nil {
		panic("failed to generate random span ID: " + err.Error())
	}
	return id
}
