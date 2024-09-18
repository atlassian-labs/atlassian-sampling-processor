// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package evaluators // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

type ottlConditionEvaluator struct {
	StartFunc
	sampleSpanExpr      BoolExpr[ottlspan.TransformContext]
	sampleSpanEventExpr BoolExpr[ottlspanevent.TransformContext]
	errorMode           ottl.ErrorMode
}

// BoolExpr is an interface that allows matching a context K against a configuration of a match.
type BoolExpr[K any] interface {
	Eval(ctx context.Context, tCtx K) (bool, error)
}

var _ PolicyEvaluator = (*ottlConditionEvaluator)(nil)

// NewOTTLConditionEvaluator looks at the trace data and returns a corresponding SamplingDecision.
func NewOTTLConditionEvaluator(spanConditions, spanEventConditions []string, errMode ottl.ErrorMode) (PolicyEvaluator, error) {
	filter := &ottlConditionEvaluator{
		errorMode: errMode,
	}

	var err error

	if len(spanConditions) == 0 && len(spanEventConditions) == 0 {
		return nil, errors.New("expected at least one OTTL condition to filter on")
	}

	// No logging within the evaluator.
	// We're intentionally not passing the logger from the component to the evaluators, as any error would be propagated up and get logged.
	// One downside of doing this is that we might miss out on some signals from the OTTL library.
	settings := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}

	if len(spanConditions) > 0 {
		if filter.sampleSpanExpr, err = newBoolExprForSpan(spanConditions, standardSpanFuncs(), errMode, settings); err != nil {
			return nil, err
		}
	}

	if len(spanEventConditions) > 0 {
		if filter.sampleSpanEventExpr, err = newBoolExprForSpanEvent(spanEventConditions, standardSpanEventFuncs(), errMode, settings); err != nil {
			return nil, err
		}
	}

	return filter, nil
}

func (oce *ottlConditionEvaluator) Evaluate(ctx context.Context, traceID pcommon.TraceID, currentTrace ptrace.Traces, _ *tracedata.Metadata) (Decision, error) {
	if oce.sampleSpanExpr == nil && oce.sampleSpanEventExpr == nil {
		return Pending, nil
	}

	for i := 0; i < currentTrace.ResourceSpans().Len(); i++ {
		rs := currentTrace.ResourceSpans().At(i)
		resource := rs.Resource()
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			scope := ss.Scope()
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)

				var (
					ok  bool
					err error
				)

				// Now we reach span level and begin evaluation with parsed expr.
				// The evaluation will break when:
				// 1. error happened.
				// 2. "Sampled" decision made.
				// Otherwise, it will keep evaluating and finally exit with "Pending" decision.

				// Span evaluation
				if oce.sampleSpanExpr != nil {
					ok, err = oce.sampleSpanExpr.Eval(ctx, ottlspan.NewTransformContext(span, scope, resource, ss, rs))
					if err != nil {
						return Pending, err
					}
					if ok {
						return Sampled, nil
					}
				}

				// Span event evaluation
				if oce.sampleSpanEventExpr != nil {
					spanEvents := span.Events()
					for l := 0; l < spanEvents.Len(); l++ {
						ok, err = oce.sampleSpanEventExpr.Eval(ctx, ottlspanevent.NewTransformContext(spanEvents.At(l), span, scope, resource, ss, rs))
						if err != nil {
							return Pending, err
						}
						if ok {
							return Sampled, nil
						}
					}
				}
			}
		}
	}
	return Pending, nil
}

// newBoolExprForSpan creates a BoolExpr[ottlspan.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottlspan.TransformContext.
// If a function named `match` is not present in the function map it will be added automatically so that parsing works as expected
func newBoolExprForSpan(conditions []string, functions map[string]ottl.Factory[ottlspan.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings) (BoolExpr[ottlspan.TransformContext], error) {
	parser, err := ottlspan.NewParser(functions, set)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlspan.NewConditionSequence(statements, set, ottlspan.WithConditionSequenceErrorMode(errorMode))
	return &c, nil
}

// newBoolExprForSpanEvent creates a BoolExpr[ottlspanevent.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottlspanevent.TransformContext.
// If a function named `match` is not present in the function map it will be added automatically so that parsing works as expected
func newBoolExprForSpanEvent(conditions []string, functions map[string]ottl.Factory[ottlspanevent.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings) (BoolExpr[ottlspanevent.TransformContext], error) {
	parser, err := ottlspanevent.NewParser(functions, set)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlspanevent.NewConditionSequence(statements, set, ottlspanevent.WithConditionSequenceErrorMode(errorMode))
	return &c, nil
}

func standardSpanFuncs() map[string]ottl.Factory[ottlspan.TransformContext] {
	m := ottlfuncs.StandardConverters[ottlspan.TransformContext]()
	isRootSpanFactory := ottlfuncs.NewIsRootSpanFactory()
	m[isRootSpanFactory.Name()] = isRootSpanFactory
	return m
}

func standardSpanEventFuncs() map[string]ottl.Factory[ottlspanevent.TransformContext] {
	return ottlfuncs.StandardConverters[ottlspanevent.TransformContext]()
}
