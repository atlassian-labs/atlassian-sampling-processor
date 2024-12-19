package tracedata // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/tracedata"

import (
	"fmt"

	"github.com/golang/snappy"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type snappyTraceData struct {
	// compressedBlobs stores any compressed data for the trace, each blob contains a marshaled
	// and compressed copy of a ptrace.Traces struct in the order it was appended
	compressedBlobs [][]byte
}

func newSnappyTraceData(traces ptrace.Traces) (*snappyTraceData, error) {
	protoMarshal := ptrace.ProtoMarshaler{}
	marshalData, err := protoMarshal.MarshalTraces(traces)
	if err != nil {
		return nil, err
	}
	compressedBlob := snappy.Encode(nil, marshalData)

	return &snappyTraceData{
		compressedBlobs: [][]byte{compressedBlob},
	}, nil
}

func (td *snappyTraceData) GetTraces() (ptrace.Traces, error) {
	traces := ptrace.NewTraces()
	protoUnmarshal := ptrace.ProtoUnmarshaler{}

	for _, compressed := range td.compressedBlobs {
		decompressed, err := snappy.Decode(nil, compressed)
		if err != nil {
			return ptrace.NewTraces(), fmt.Errorf("snappyTraceData: error decompressing: %w", err)
		}

		other, err := protoUnmarshal.UnmarshalTraces(decompressed)
		if err != nil {
			return ptrace.NewTraces(), fmt.Errorf("snappyTraceData: error unmarshaling proto: %w", err)
		}

		other.ResourceSpans().MoveAndAppendTo(traces.ResourceSpans())
	}

	return traces, nil
}

func (td *snappyTraceData) AbsorbTraces(facade traceFacade) error {
	if other, ok := facade.(*snappyTraceData); ok {
		td.compressedBlobs = append(td.compressedBlobs, other.compressedBlobs...)
		return nil
	}

	return errorIncompatibleTraceDataType
}
