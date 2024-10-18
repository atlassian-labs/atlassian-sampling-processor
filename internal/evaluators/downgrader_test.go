package evaluators

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

func TestDowngrader(t *testing.T) {
	dngrder, err := NewDowngrader(NotSampled, &staticTestEvaluator{d: Sampled})
	require.NoError(t, err)
	require.NoError(t, dngrder.Start(context.Background(), componenttest.NewNopHost()))

	d, err := dngrder.Evaluate(context.Background(), testTraceID, ptrace.NewTraces(), &tracedata.Metadata{})
	assert.NoError(t, err)
	assert.Equal(t, NotSampled, d)

	dngrder, err = NewDowngrader(NotSampled, &staticTestEvaluator{d: Pending})
	require.NoError(t, err)
	d, err = dngrder.Evaluate(context.Background(), testTraceID, ptrace.NewTraces(), &tracedata.Metadata{})
	assert.NoError(t, err)
	assert.Equal(t, Pending, d)

	dngrder, err = NewDowngrader(LowPriority, &staticTestEvaluator{d: Sampled})
	require.NoError(t, err)
	d, err = dngrder.Evaluate(context.Background(), testTraceID, ptrace.NewTraces(), &tracedata.Metadata{})
	assert.NoError(t, err)
	assert.Equal(t, LowPriority, d)

	dngrder, err = NewDowngrader(LowPriority, &staticTestEvaluator{d: Sampled, e: errors.New("test err")})
	require.NoError(t, err)
	d, err = dngrder.Evaluate(context.Background(), testTraceID, ptrace.NewTraces(), &tracedata.Metadata{})
	assert.Error(t, err)
	assert.Equal(t, Unspecified, d)
}
