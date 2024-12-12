package tracedata // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"

import "go.opentelemetry.io/collector/pdata/ptrace"

type uncompressedTraceData struct {
	// traces stores trace data received for the trace
	traces ptrace.Traces
}

func newUncompressedTraceData(traces ptrace.Traces) *uncompressedTraceData {
	return &uncompressedTraceData{
		traces: traces,
	}
}

func (td *uncompressedTraceData) GetTraces() (ptrace.Traces, error) {
	return td.traces, nil
}

func (td *uncompressedTraceData) AbsorbTraces(facade traceFacade) error {
	if other, ok := facade.(*uncompressedTraceData); ok {
		other.traces.ResourceSpans().MoveAndAppendTo(td.traces.ResourceSpans())
		return nil
	}

	return errorIncompatibleTraceDataType
}
