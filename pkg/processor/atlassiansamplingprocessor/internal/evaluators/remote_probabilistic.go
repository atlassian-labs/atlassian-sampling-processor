// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Modifications made by Atlassian Pty Ltd.
// Copyright © 2024 Atlassian US, Inc.
// Copyright © 2024 Atlassian Pty Ltd.
package evaluators // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/big"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

const (
	defaultRemoteHashSalt = "default-hash-seed"
)

type remoteProbabilisticSampler struct {
	rateGetterExt component.ID
	rateGetter    rateGetter
	defaultRate   float64
	hashSalt      string
}

var _ PolicyEvaluator = (*remoteProbabilisticSampler)(nil)

// rateGetter is an interface used by the remote probabilistic policy to fetch sampling rates at runtime.
// Each rateGetter is free to define its own behavior and configuration options, but
// needs to implement the GetFloat64 function that should return the sampling rate to be used.
type rateGetter interface {
	GetFloat64() (float64, error)
}

// NewRemoteProbabilisticSampler creates a policy evaluator that samples traces
// based on the rate returned by the rate getter extension provided.
func NewRemoteProbabilisticSampler(hashSalt string, defaultRate float64, rateGetterExt component.ID) PolicyEvaluator {
	if hashSalt == "" {
		hashSalt = defaultRemoteHashSalt
	}

	return &remoteProbabilisticSampler{
		defaultRate:   defaultRate,
		rateGetterExt: rateGetterExt,
		hashSalt:      hashSalt,
	}
}

// Start looks for any specified extension and keeps a reference to the extension.
// It returns an error if the specified extension is not found or does not implement the RateGetter interface.
func (s *remoteProbabilisticSampler) Start(_ context.Context, host component.Host) error {
	if s.rateGetterExt.String() == "" {
		return nil
	}

	if ext, found := host.GetExtensions()[s.rateGetterExt]; found {
		if rg, ok := ext.(rateGetter); ok {
			s.rateGetter = rg
		} else {
			return fmt.Errorf("specified extension does not implement the rateGetter interface")
		}
	} else {
		return fmt.Errorf("specified rate getter extension with id %q could not be found", s.rateGetterExt)
	}

	return nil
}

// Evaluate uses sampling rate fetched by the rate getter and returns a corresponding SamplingDecision.
// If there is no rate getter specified or if there is an error, it uses the default rate.
func (s *remoteProbabilisticSampler) Evaluate(_ context.Context, traceID pcommon.TraceID, _ ptrace.Traces, _ *tracedata.Metadata) (Decision, error) {
	var err error
	rate := s.defaultRate

	if s.rateGetter != nil {
		remoteRate, remoteErr := s.rateGetter.GetFloat64()
		if remoteErr != nil {
			err = fmt.Errorf("error fetching remote rate: %w", remoteErr)
		} else if remoteRate < 0 {
			err = fmt.Errorf("remote rate is invalid: %f", remoteRate)
		} else {
			rate = remoteRate
		}
	}

	threshold := calculateSamplingThreshold(rate / 100)

	if createHashTraceID(s.hashSalt, traceID[:]) <= threshold {
		return Sampled, err
	}

	return Pending, err
}

// calculateSamplingThreshold converts a ratio into a value between 0 and MaxUint64
func calculateSamplingThreshold(ratio float64) uint64 {
	// Use big.Float and big.Int to calculate threshold because directly convert
	// math.MaxUint64 to float64 will cause digits/bits to be cut off if the converted value
	// doesn't fit into bits that are used to store digits for float64 in Golang
	boundary := new(big.Float).SetInt(new(big.Int).SetUint64(math.MaxUint64))
	res, _ := boundary.Mul(boundary, big.NewFloat(ratio)).Uint64()
	return res
}

// createHashTraceID creates a hash using the FNV-1a algorithm.
func createHashTraceID(salt string, b []byte) uint64 {
	hasher := fnv.New64a()
	// the implementation fnv.Write() never returns an error, see hash/fnv/fnv.go
	_, _ = hasher.Write([]byte(salt))
	_, _ = hasher.Write(b)
	return hasher.Sum64()
}
