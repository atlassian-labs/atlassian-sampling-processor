package evaluators // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/internal/ptraceutil"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

const (
	samplingTailThreshold        = "sampling.tail.threshold" // defined on https://hello.atlassian.net/l/cp/fo1Z7fnm
	maxThreshold          uint64 = 0xff_ffff_ffff_ffff       // 2^56 - 1
	threshAttrMaxLen             = 14                        // 14 hex characters -> 56 bits = 7 bytes
)

type thresholdEvaluator struct {
	StartFunc
}

func NewThresholdEvaluator() PolicyEvaluator {
	return &thresholdEvaluator{}
}

// Evaluate uses the consistent threshold-based sampling algorithm defined in OTEP-235.
// It looks for the `sampling.tail.threshold` attribute, using the smallest
// value it finds across the spans, which corresponds to the largest sampling probability.
// It then performs a sampling decision by comparing this threshold
// value with the right 56-bits (7 bytes) of the trace ID.
func (t *thresholdEvaluator) Evaluate(_ context.Context, id pcommon.TraceID, currentTrace ptrace.Traces, _ *tracedata.Metadata) (Decision, error) {
	minThreshold := maxThreshold
	for item := range ptraceutil.TraceIterator(currentTrace) {
		span := item.Span
		threshold, ok := getThreshold(span)
		if !ok {
			continue
		}
		if threshold < minThreshold {
			minThreshold = threshold
		}
	}

	if minThreshold == maxThreshold {
		return Pending, nil // Didn't find any span with threshold set
	}

	rv := getRv(id)
	if rv >= minThreshold {
		return Sampled, nil
	}
	return Pending, nil
}

// getThreshold inspects the span attributes for the sampling.tail.threshold attribute.
// If there is a valid value, it returns it as an integer, with true. Otherwise, it returns (0, false).
func getThreshold(span ptrace.Span) (uint64, bool) {
	v, ok := span.Attributes().Get(samplingTailThreshold)
	if !ok {
		return 0, false
	}

	s := v.Str()
	if !strings.HasPrefix(s, "0x") {
		return 0, false
	}

	s = s[2:] // chop off '0x'
	if len(s) > threshAttrMaxLen {
		return 0, false
	}
	// right pad with zeroes in accordance with OTEP-235
	s += strings.Repeat("0", threshAttrMaxLen-len(s))

	threshold, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0, false
	}

	return threshold, true
}

// getRv retrieves the rightmost 7 bytes of the trace ID and returns their numerical value as uint64.
// Adapted from binary.BigEndian.Uint64().
func getRv(id pcommon.TraceID) uint64 {
	return uint64(id[15]) | uint64(id[14])<<8 | uint64(id[13])<<16 |
		uint64(id[12])<<24 | uint64(id[11])<<32 | uint64(id[10])<<40 | uint64(id[9])<<48
}
