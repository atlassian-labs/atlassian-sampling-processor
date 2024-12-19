package atlassiansamplingprocessor // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor"

import "golang.org/x/sync/errgroup"

// groupWaiter returns a channel that will send when the errgroup is finished
func groupWaiter(group *errgroup.Group) <-chan error {
	c := make(chan error)
	go func() {
		c <- group.Wait()
	}()
	return c
}
