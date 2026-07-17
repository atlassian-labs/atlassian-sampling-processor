// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.

package atlassiansamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDecisionGrouper_NoMappings(t *testing.T) {
	t.Parallel()
	g, err := newDecisionGrouper(nil)
	require.NoError(t, err)
	assert.Nil(t, g)

	g, err = newDecisionGrouper([]DecisionMapping{})
	require.NoError(t, err)
	assert.Nil(t, g)
}

func TestNewDecisionGrouper_InvalidRegex(t *testing.T) {
	t.Parallel()
	_, err := newDecisionGrouper([]DecisionMapping{{Pattern: "([", Value: "x"}})
	require.Error(t, err)
}

func TestDecisionGrouper_Group(t *testing.T) {
	t.Parallel()
	mappings := []DecisionMapping{
		{Pattern: "^(conf|confluence)-.*", Value: "confluence-monolith"},
		{Pattern: "^tempo-.*", Value: "tempo"},
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "matches first rule", input: "confluence-shard-0001", want: "confluence-monolith"},
		{name: "matches first rule alt prefix", input: "conf-shard-42", want: "confluence-monolith"},
		{name: "matches second rule", input: "tempo-ingester-7", want: "tempo"},
		{name: "no match keeps original", input: "frontend", want: "frontend"},
		{name: "empty input no match keeps empty", input: "", want: ""},
	}

	g, err := newDecisionGrouper(mappings)
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, g.group(tt.input))
		})
	}
}

func TestDecisionGrouper_FirstMatchWins(t *testing.T) {
	t.Parallel()
	g, err := newDecisionGrouper([]DecisionMapping{
		{Pattern: "^confluence-", Value: "specific"},
		{Pattern: "^conf", Value: "broad"},
	})
	require.NoError(t, err)
	assert.Equal(t, "specific", g.group("confluence-shard-1"))
	assert.Equal(t, "broad", g.group("config-service"))
}
