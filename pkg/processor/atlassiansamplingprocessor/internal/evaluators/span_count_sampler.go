// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package evaluators // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"
	"encoding/hex"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

const serviceNameKey = "service.name"

type spanCount struct {
	StartFunc
	minSpans int32
	log      *zap.Logger
}

var _ PolicyEvaluator = (*spanCount)(nil)

// NewSpanCount creates a policy evaluator sampling traces with more than one span per trace
// log may be nil
func NewSpanCount(minSpans int32, log *zap.Logger) PolicyEvaluator {
	return &spanCount{
		minSpans: minSpans,
		log:      log,
	}
}

// Evaluate return Sampled if the trace has more than or equal to the minSpans, else it returns Pending.
func (sc *spanCount) Evaluate(_ context.Context, id pcommon.TraceID, currentTrace ptrace.Traces, mergedMetadata *tracedata.Metadata) (Decision, error) {
	if mergedMetadata.SpanCount >= sc.minSpans {
		if sc.log != nil {
			sc.log.Info("Ay Carumba! Ginormous trace incoming. "+
				"From the current batch, the services are listed.",
				zap.String("traceID", hex.EncodeToString(id[:])),
				zap.Int32("spanCount", mergedMetadata.SpanCount),
				zap.Strings("services", getServices(currentTrace)))
		}
		return Sampled, nil
	}

	return Pending, nil
}

// getServices returns a slice of service names from the given trace
func getServices(td ptrace.Traces) []string {
	rs := td.ResourceSpans()
	m := make(map[string]bool)
	for i := 0; i < rs.Len(); i++ {
		service, ok := rs.At(i).Resource().Attributes().Get(serviceNameKey)
		if !ok {
			continue
		}
		m[service.Str()] = true
	}
	services := make([]string, 0, len(m))
	for s := range m {
		services = append(services, s)
	}
	slices.Sort(services)
	return services
}
