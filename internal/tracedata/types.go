package tracedata // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"

import (
	"errors"

	"go.opentelemetry.io/collector/pdata/ptrace"
)

type traceFacade interface {
	GetTraces() (ptrace.Traces, error)
	AbsorbTraces(facade traceFacade) error
}

var errorIncompatibleTraceDataType = errors.New("cannot absorb incompatible trace data")
