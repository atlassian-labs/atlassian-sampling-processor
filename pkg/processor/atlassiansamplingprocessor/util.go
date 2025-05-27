package atlassiansamplingprocessor // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"golang.org/x/sync/errgroup"
)

// groupWaiter returns a channel that will send when the errgroup is finished
func groupWaiter(group *errgroup.Group) <-chan error {
	c := make(chan error)
	go func() {
		c <- group.Wait()
	}()
	return c
}

type spansGroupedByTraceID map[pcommon.TraceID][]spanAndScope

// groupedResourceSpans contains a pcommon.Resource, and all the spans underneath that resource grouped by trace ID
type groupedResourceSpans struct {
	resource              pcommon.Resource
	spansGroupedByTraceID spansGroupedByTraceID
}
