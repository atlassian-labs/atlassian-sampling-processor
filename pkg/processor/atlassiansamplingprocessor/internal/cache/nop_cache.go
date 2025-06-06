// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package cache // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/cache"

import "go.opentelemetry.io/collector/pdata/pcommon"

type nopDecisionCache[V any] struct{}

var _ Cache[any] = (*nopDecisionCache[any])(nil)

func NewNopDecisionCache[V any]() Cache[V] {
	return &nopDecisionCache[V]{}
}

func (n *nopDecisionCache[V]) Get(_ pcommon.TraceID) (V, bool) {
	var v V
	return v, false
}

func (n *nopDecisionCache[V]) Put(_ pcommon.TraceID, _ V) {
}

func (n *nopDecisionCache[V]) Delete(_ pcommon.TraceID) {}

func (n *nopDecisionCache[V]) Values() []V {
	return make([]V, 0)
}

func (n *nopDecisionCache[V]) Keys() []pcommon.TraceID {
	return make([]pcommon.TraceID, 0)
}

func (n *nopDecisionCache[V]) Size() int {
	return 0
}

func (n *nopDecisionCache[V]) Resize(_ int) {}

func (n *nopDecisionCache[V]) Clear() {}
