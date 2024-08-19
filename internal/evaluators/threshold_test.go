package evaluators

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

func TestGetRv(t *testing.T) {
	id := pcommon.TraceID{0xff, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	assert.Equal(t, uint64(0x01020304050607), getRv(id))
}

func TestThresholdEvaluator(t *testing.T) {
	t.Parallel()
	evaluator := NewThresholdEvaluator()

	tests := []struct {
		name           string
		traceID        string
		thresholds     []string
		expectedResult Decision
	}{
		{
			name:    "sampled",
			traceID: "0000 0000 0000 0000 007f ffff ffff ffff",
			thresholds: []string{
				"0x7ffffffffffffe",
			},
			expectedResult: Sampled,
		},
		{
			name:    "pending",
			traceID: "0000 0000 0000 0000 007f ffff ffff fffe",
			thresholds: []string{
				"0x7fffffffffffff",
			},
			expectedResult: Pending,
		},
		{
			name:    "shortened threshold, pending",
			traceID: "0000 0000 0000 0000 007f ffff ffff ffff",
			thresholds: []string{
				"0x8",
			},
			expectedResult: Pending,
		},
		{
			name:    "shortened threshold, sampled",
			traceID: "0000 0000 0000 0000 008f ffff ffff ffff",
			thresholds: []string{
				"0x8",
			},
			expectedResult: Sampled,
		},
		{
			name:    "threshold 0",
			traceID: "0000 0000 0000 0000 0000 0000 0000 0000",
			thresholds: []string{
				"0x",
			},
			expectedResult: Sampled,
		},
		{
			name:    "should use smallest threshold",
			traceID: "0000 0000 0000 0000 00fd 70a3 d500 0000",
			thresholds: []string{
				"0xfd70a3d6",
				"0xfd70a3d5",
			},
			expectedResult: Sampled,
		},
		{
			name:    "should use smallest threshold, opposite order",
			traceID: "0000 0000 0000 0000 00fd 70a3 d500 0000",
			thresholds: []string{
				"0xfd70a3d5",
				"0xfd70a3d6",
			},
			expectedResult: Sampled,
		},
		{
			name:    "max threshold",
			traceID: "0000 0000 0000 0000 ffff ffff ffff fffe",
			thresholds: []string{
				"0xffffffffffffff",
			},
			expectedResult: Pending,
		},
		{
			name:    "one invalid threshold still allows other valid ones to work",
			traceID: "0000 0000 0000 0000 ffff ffff ffff ffff",
			thresholds: []string{
				"invalid",
				"0x000",
			},
			expectedResult: Sampled,
		},
		{
			name:    "All invalid results in pending",
			traceID: "0000 0000 0000 0000 ffff ffff ffff ffff",
			thresholds: []string{
				"invalid",
				"00000",
			},
			expectedResult: Pending,
		},
		{
			name:    "Threshold too long",
			traceID: "0000 0000 0000 0000 ffff ffff ffff ffff",
			thresholds: []string{
				"000000000000000",
			},
			expectedResult: Pending,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			traceID := traceIDFromHex(t, test.traceID)
			trace := ptrace.NewTraces()
			for _, thresh := range test.thresholds {
				span := trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.Attributes().PutStr(samplingTailThreshold, thresh)
			}
			decision, err := evaluator.Evaluate(context.Background(), traceID, trace, &tracedata.Metadata{})
			require.NoError(t, err)
			assert.Equal(t, test.expectedResult, decision)
		})
	}
}

func traceIDFromHex(t testing.TB, idStr string) pcommon.TraceID {
	idStr = strings.Replace(idStr, " ", "", -1)
	id := pcommon.NewTraceIDEmpty()
	_, err := hex.Decode(id[:], []byte(idStr))
	require.NoError(t, err)
	return id
}
