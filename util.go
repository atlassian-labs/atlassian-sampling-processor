package atlassiansamplingprocessor // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor"

import "golang.org/x/sync/errgroup"

// groupWaiter returns a channel that will send when the errgroup is finished
func groupWaiter(group *errgroup.Group) <-chan error {
	c := make(chan error)
	go func() {
		c <- group.Wait()
	}()
	return c
}
