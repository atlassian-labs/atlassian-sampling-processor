package atlassiansamplingprocessor // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor"

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/cache"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/memory"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/metadata"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/priority"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

const flushCountKey = "atlassiansampling.flushes"
const decisionSpanKey = "atlassiansampling.decision"
const memTickerInterval = 10 * time.Second

var (
	sampledAttr    = metric.WithAttributes(attribute.String("decision", "sampled"))
	notSampledAttr = metric.WithAttributes(attribute.String("decision", "not_sampled"))
)

type atlassianSamplingProcessor struct {
	next      consumer.Traces
	telemetry *metadata.TelemetryBuilder
	log       *zap.Logger

	// decider makes the sampling decisions
	decider deciderI

	// traceData holds the map of trace ID to trace Data
	traceData cache.Cache[*tracedata.TraceData]
	// sampledDecisionCache holds the set of trace IDs that were sampled, and the time of decision
	sampledDecisionCache cache.Cache[time.Time]
	// nonSampledDecisionCache holds the set of trace IDs that were not sampled, and a struct that contains the policy that
	// made the not sampled decision and the time the decision was made
	nonSampledDecisionCache cache.Cache[*nsdOutcome]
	// incomingTraces is where the traces are put when they first arrive to the component
	incomingTraces chan ptrace.Traces
	// memRegulator can adjust cache sizes to target a given heap usage.
	// May be nil, in which case the cache sizes will not be adjusted.
	memRegulator *memory.Regulator
	// memTicker controls how often the memRegulator is called
	memTicker *time.Ticker
	// shutdownStart is a chan used to signal to the async goroutine to start shutdown.
	// The deadline time is passed to the chan, so the async goroutine knows when to time out.
	shutdownStart   chan time.Time
	waitGroup       sync.WaitGroup
	flushOnShutdown bool
	compress        bool
	maxFlushes      int
	started         atomic.Bool
}

var _ processor.Traces = (*atlassianSamplingProcessor)(nil)

type nsdOutcome struct {
	decisionTime time.Time
}

func newAtlassianSamplingProcessor(cCfg component.Config, set component.TelemetrySettings, next consumer.Traces) (*atlassianSamplingProcessor, error) {
	cfg, ok := cCfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type provided")
	}

	telemetry, err := metadata.NewTelemetryBuilder(set)
	if err != nil {
		return nil, err
	}

	asp := &atlassianSamplingProcessor{
		next:      next,
		telemetry: telemetry,
		log:       set.Logger,

		incomingTraces:  make(chan ptrace.Traces),
		shutdownStart:   make(chan time.Time),
		flushOnShutdown: cfg.FlushOnShutdown,
		maxFlushes:      cfg.MaxFlushes,
		compress:        cfg.CompressionEnabled,
	}

	primaryCache, err := cache.NewLRUCache[*tracedata.TraceData](
		cfg.PrimaryCacheSize,
		asp.primaryEvictionCallback,
		telemetry)
	if err != nil {
		return nil, err
	}

	asp.traceData = primaryCache

	if cfg.SecondaryCacheSize > 0 {
		secondaryCache, err2 := cache.NewLRUCache[*tracedata.TraceData](
			cfg.SecondaryCacheSize,
			asp.secondaryEvictionCallback,
			telemetry)
		if err2 != nil {
			return nil, err2
		}
		asp.traceData, err2 = cache.NewTieredCache[*tracedata.TraceData](primaryCache, secondaryCache)
		if err2 != nil {
			return nil, err2
		}
	}

	asp.sampledDecisionCache, err = cache.NewLRUCache[time.Time](cfg.SampledCacheSize, asp.onEvictSampled, telemetry)
	if err != nil {
		return nil, err
	}
	asp.nonSampledDecisionCache, err = cache.NewLRUCache[*nsdOutcome](cfg.NonSampledCacheSize, asp.onEvictNotSampled, telemetry)
	if err != nil {
		return nil, err
	}

	asp.memTicker = time.NewTicker(memTickerInterval)
	if cfg.TargetHeapBytes > 0 {
		memRegulator, rErr := memory.NewRegulator(
			cfg.PrimaryCacheSize/2,
			cfg.PrimaryCacheSize,
			cfg.TargetHeapBytes,
			memory.GetHeapUsage,
			primaryCache)
		if rErr != nil {
			return nil, rErr
		}
		asp.memRegulator = memRegulator
	}

	pols, err := newPolicies(cfg.PolicyConfig, set)
	if err != nil {
		return nil, err
	}
	dec := newDecider(pols, set.Logger, telemetry)
	asp.decider = dec

	return asp, nil
}

func (asp *atlassianSamplingProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (asp *atlassianSamplingProcessor) Start(ctx context.Context, host component.Host) error {
	if running := asp.started.Swap(true); running {
		return fmt.Errorf("component already started")
	}

	if err := asp.decider.Start(ctx, host); err != nil {
		return fmt.Errorf("failed to start decider")
	}

	go asp.consumeChan()
	return nil
}

func (asp *atlassianSamplingProcessor) Shutdown(parentCtx context.Context) error {
	if !asp.started.Load() {
		return nil
	}
	asp.log.Info("shutdown of atlassianSamplingProcessor initiated")
	defer asp.log.Info("shutdown of atlassianSamplingProcessor finished")

	deadline := time.Now().Add(50 * time.Second)
	ctx, cancel := context.WithDeadline(parentCtx, deadline)
	defer cancel()
	// Need to close chan because if parentCtx is cancelled already, the below select/case never sends the shutdown
	// signal, resulting in the consumeChan() goroutine to block waiting. Closing the channel forces this to release.
	defer close(asp.shutdownStart)

	asp.memTicker.Stop()

	select {
	case asp.shutdownStart <- deadline:
		asp.waitGroup.Wait()
		asp.started.Store(false)
	case <-ctx.Done():
		return fmt.Errorf("failed to wait for consumer goroutine to acknowledge shutdown: %w", ctx.Err())
	}

	var err error
	if asp.flushOnShutdown {
		err = asp.flushAll(ctx)
	}

	return err
}

// ConsumeTraces implements the processing interface.
// This puts on a channel for another goroutine to read, since we want consumption
// to be synchronized, avoiding tricky code and race conditions.
// Note for future: If the synchronicity becomes a bottleneck, this component
// can be sharded similar to how the batch processor is sharded.
func (asp *atlassianSamplingProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	start := time.Now()
	defer func() {
		asp.telemetry.ProcessorAtlassianSamplingChanBlockingTime.Record(ctx, time.Since(start).Nanoseconds())
	}()

	asp.incomingTraces <- td
	return nil
}

// ConsumeTracesAsync reads the traces from a channel.
func (asp *atlassianSamplingProcessor) consumeChan() {
	asp.waitGroup.Add(1)
	defer asp.waitGroup.Done()
	defer asp.log.Info("consumeChan() finished")
	ctx := context.Background()

	for {
		select {
		// Regular operating case
		case nt := <-asp.incomingTraces:
			resourceSpans := nt.ResourceSpans()
			for i := 0; i < resourceSpans.Len(); i++ {
				asp.processTraces(ctx, resourceSpans.At(i))
			}
		case <-asp.memTicker.C:
			// If ticker signals, call the memory regulator
			if asp.memRegulator != nil {
				size := asp.memRegulator.RegulateCacheSize()
				asp.telemetry.ProcessorAtlassianSamplingPrimaryCacheSize.Record(ctx, int64(size))
			}
		// If shutdown is signaled, process any pending traces and return
		case deadline := <-asp.shutdownStart:
			shutdownCtx, cancel := context.WithDeadline(ctx, deadline)
			for {
				select {
				case nt := <-asp.incomingTraces:
					resourceSpans := nt.ResourceSpans()
					for i := 0; i < resourceSpans.Len(); i++ {
						asp.processTraces(shutdownCtx, resourceSpans.At(i))
					}
				case <-shutdownCtx.Done():
					cancel()
					asp.log.Warn("context cancelled before we could process all incoming traces", zap.Error(shutdownCtx.Err()))
					return
				default:
					cancel()
					return
				}
			}
		}
	}
}

func (asp *atlassianSamplingProcessor) processTraces(ctx context.Context, resourceSpans ptrace.ResourceSpans) {
	idToSpanAndScope := groupSpansByTraceKey(resourceSpans)
	for id, spans := range idToSpanAndScope {
		if asp.cachedDecision(ctx, id, resourceSpans, spans) {
			continue
		}

		currentTrace := ptrace.NewTraces()
		appendToTraces(currentTrace, resourceSpans, spans)

		td, err := tracedata.NewTraceData(time.Now(), currentTrace, priority.Unspecified, asp.compress)
		if err != nil {
			asp.log.Error(
				"failed to create trace data",
				zap.Error(err),
				zap.String("traceID", hex.EncodeToString(id[:])),
				zap.Int("spanCount", currentTrace.SpanCount()),
				zap.Bool("compress", asp.compress),
			)
			asp.telemetry.ProcessorAtlassianSamplingInternalErrorDroppedSpans.
				Add(ctx, int64(currentTrace.SpanCount()))
			continue
		}

		// Merge metadata with any metadata in the cache to pass to evaluators
		mergedMetadata := td.Metadata.DeepCopy()

		if cachedData, ok := asp.traceData.Get(id); ok {
			mergedMetadata.MergeWith(cachedData.Metadata)
		}

		// Evaluate the spans against the policies.
		finalDecision, pol := asp.decider.MakeDecision(ctx, id, currentTrace, mergedMetadata)

		// Reset LastLowPriorityDecisionName if the trace is promoted to a higher priority by any higher tier policy.
		if mergedMetadata.LastLowPriorityDecisionName != nil && finalDecision != evaluators.LowPriority {
			mergedMetadata.LastLowPriorityDecisionName = nil
		}

		switch finalDecision {
		case evaluators.Sampled:
			// Sample, cache decision, and release all data associated with the trace
			asp.sampledDecisionCache.Put(id, time.Now())
			if cachedData, ok := asp.traceData.Get(id); ok {
				cachedTraces, err := cachedData.GetTraces()
				if err != nil {
					asp.log.Error("failed to retrieve cached traces",
						zap.Error(err),
						zap.String("traceID", hex.EncodeToString(id[:])),
					)
					asp.telemetry.
						ProcessorAtlassianSamplingInternalErrorDroppedSpans.Add(ctx, int64(cachedData.Metadata.SpanCount))
				} else {
					asp.sendSampledTraceData(ctx, cachedTraces)
				}
				asp.traceData.Delete(id)
			}
			asp.sendSampledTraceData(ctx, currentTrace)
			asp.telemetry.ProcessorAtlassianSamplingTracesSampled.Add(ctx, 1)
		case evaluators.NotSampled:
			asp.releaseNotSampledTrace(ctx, id)
		default:
			if finalDecision == evaluators.LowPriority {
				td.Metadata.Priority = priority.Low
				// Set LastLowPriorityDecisionPolicyMetadata when it's empty
				if mergedMetadata.LastLowPriorityDecisionName == nil {
					mergedMetadata.LastLowPriorityDecisionName = &pol.name
				}
				td.Metadata.LastLowPriorityDecisionName = mergedMetadata.LastLowPriorityDecisionName
			}
			// If we have reached here, the sampling decision is still pending, so we put trace data in the cache
			// Priority of the metadata will affect the cache tier
			if cachedTd, ok := asp.traceData.Get(id); ok {
				err := cachedTd.AbsorbTraceData(td) // chooses higher priority in merge
				if err != nil {
					asp.log.Error(
						"failed to merge into cached traces",
						zap.Error(err),
						zap.String("traceID", hex.EncodeToString(id[:])),
						zap.Int32("spanCount", td.Metadata.SpanCount),
					)
					asp.telemetry.ProcessorAtlassianSamplingInternalErrorDroppedSpans.
						Add(ctx, int64(td.Metadata.SpanCount))
				}
				td = cachedTd // td is now the initial, incoming td + the cached td
			}
			asp.traceData.Put(id, td)
		}
	}
}

// cachedDecision checks to see if a decision has already been made for this trace,
// and sends/drops if a pre-existing decision exists.
// If the incoming span is a decision span, and the decision conflicts with the current decision caches,
// then the decision span will be ignored.
func (asp *atlassianSamplingProcessor) cachedDecision(
	ctx context.Context,
	id pcommon.TraceID,
	resourceSpans ptrace.ResourceSpans,
	spans []spanAndScope,
) bool {
	decision, isDecisionSpan := resourceSpans.Resource().Attributes().Get(decisionSpanKey)

	// check decision caches
	if _, ok := asp.sampledDecisionCache.Get(id); ok {
		// export if not decision span
		if !isDecisionSpan {
			td := ptrace.NewTraces()
			appendToTraces(td, resourceSpans, spans)
			asp.sendSampledTraceData(ctx, td)
		}
		return true
	}
	if _, ok := asp.nonSampledDecisionCache.Get(id); ok {
		return true
	}

	if !isDecisionSpan {
		return false
	}

	// This is a decision span, handle it
	if decision.Bool() {
		if td, ok := asp.traceData.Get(id); ok {
			cachedData, err := td.GetTraces()
			if err != nil {
				asp.log.Error("error getting trace", zap.Error(err))
			}
			asp.sendSampledTraceData(ctx, cachedData)
			asp.traceData.Delete(id)
		}
		asp.sampledDecisionCache.Put(id, time.Now())
	} else {
		asp.releaseNotSampledTrace(ctx, id)
	}
	return true
}

// sendSampledTraceData sends the given trace data to the next component in the pipeline.
// It removes the flush count attr to indicate this is not data flushed on shutdown.
// This function specifically does NOT emit metrics, alter decision caches, or delete trace data from caches.
// This is because it can receive data for traces that have pre-existing, cached decisions.
// The mentioned actions are left to the caller.
func (asp *atlassianSamplingProcessor) sendSampledTraceData(ctx context.Context, td ptrace.Traces) {
	rs := td.ResourceSpans()
	for i := 0; i < rs.Len(); i++ {
		_ = rs.At(i).Resource().Attributes().Remove(flushCountKey)
	}
	if err := asp.next.ConsumeTraces(ctx, td); err != nil {
		asp.log.Warn(
			"Error sending spans to destination",
			zap.Error(err))
	}
}

// releaseNotSampledTrace removes references to traces that have been decided to be not sampled.
// It also caches the non sampled decision, and increments count of traces not sampled.
func (asp *atlassianSamplingProcessor) releaseNotSampledTrace(ctx context.Context, id pcommon.TraceID) {
	asp.traceData.Delete(id)
	asp.nonSampledDecisionCache.Put(id, &nsdOutcome{
		decisionTime: time.Now(),
	})
	asp.telemetry.ProcessorAtlassianSamplingTracesNotSampled.Add(ctx, 1)
}

func (asp *atlassianSamplingProcessor) flushAll(ctx context.Context) error {
	var err error
	defer func() {
		asp.telemetry.ProcessorAtlassianSamplingFlushes.
			Add(ctx, 1,
				metric.WithAttributes(attribute.Bool("success", err == nil)))
	}()

	// prepare decisions spans, which are decisions encoded as spans
	decisionTraces := ptrace.NewTraces()
	sampledDecisionRs := decisionTraces.ResourceSpans().AppendEmpty()
	sampledDecisionRs.Resource().Attributes().PutBool(decisionSpanKey, true)
	sampledDecisionSpans := sampledDecisionRs.ScopeSpans().AppendEmpty().Spans()
	sampledIDs := asp.sampledDecisionCache.Keys()
	asp.sampledDecisionCache.Clear()
	sampledDecisionSpans.EnsureCapacity(len(sampledIDs))
	for _, id := range sampledIDs {
		s := sampledDecisionSpans.AppendEmpty()
		s.SetTraceID(id)
		s.SetName("decision")
	}

	nonSampledDecisionRs := decisionTraces.ResourceSpans().AppendEmpty()
	nonSampledDecisionRs.Resource().Attributes().PutBool(decisionSpanKey, false)
	nonSampledDecisionSpans := nonSampledDecisionRs.ScopeSpans().AppendEmpty().Spans()
	nonSampledIDs := asp.nonSampledDecisionCache.Keys()
	asp.nonSampledDecisionCache.Clear()
	nonSampledDecisionSpans.EnsureCapacity(len(nonSampledIDs))
	for _, id := range nonSampledIDs {
		s := nonSampledDecisionSpans.AppendEmpty()
		s.SetTraceID(id)
		s.SetName("decision")
	}

	// flush decision spans
	err = multierr.Append(err, asp.next.ConsumeTraces(ctx, decisionTraces))

	// flush cached trace data
	vals := asp.traceData.Values()
	for i, td := range vals {
		cachedTraces, errGetTraces := td.GetTraces()
		if errGetTraces != nil {
			asp.log.Error(
				"failed to retrieve cached traces",
				zap.Error(errGetTraces),
				zap.Int32("spanCount", td.Metadata.SpanCount),
			)
			asp.telemetry.ProcessorAtlassianSamplingInternalErrorDroppedSpans.Add(
				ctx,
				int64(td.Metadata.SpanCount),
			)

			err = multierr.Append(err, errGetTraces)
			continue
		}
		// Increment flush count attribute.
		// Must be on resource attributes because routing connector only supports routing on resource attributes,
		// and we must keep decisions spans from being batched in the same resource as actual trace data.
		rs := cachedTraces.ResourceSpans()
		for j := 0; j < rs.Len(); j++ {
			var flushes int64 = 0
			rsAttrs := rs.At(j).Resource().Attributes()
			if v, ok := rsAttrs.Get(flushCountKey); ok {
				flushes = v.Int()
			}
			flushes++
			rsAttrs.PutInt(flushCountKey, flushes)
		}
		select {
		case <-ctx.Done():
			err = multierr.Append(err, fmt.Errorf(
				"flush could not complete due to context cancellation, "+
					"only flushed %d out of %d traces: %w", i, len(vals), ctx.Err()))
			return err
		default:
			err = multierr.Append(err, asp.next.ConsumeTraces(ctx, cachedTraces))
		}
	}
	return err
}

func (asp *atlassianSamplingProcessor) primaryEvictionCallback(id pcommon.TraceID, td *tracedata.TraceData) {
	asp.cacheEvictionCallback("primary", id, td)
}

func (asp *atlassianSamplingProcessor) secondaryEvictionCallback(id pcommon.TraceID, td *tracedata.TraceData) {
	// This trace is only being truly evicted if the priority is still low.
	// If it's priority is not low, it's actually just being promoted into the primary cache and deleted from this one.
	if td.Metadata.Priority == priority.Low {
		asp.cacheEvictionCallback("secondary", id, td)
	}
}

func (asp *atlassianSamplingProcessor) cacheEvictionCallback(cacheName string, id pcommon.TraceID, td *tracedata.TraceData) {
	ctx := context.Background()
	// Check that we didn't actually sample this trace
	_, sampled := asp.sampledDecisionCache.Get(id)

	if !sampled {
		asp.releaseNotSampledTrace(ctx, id)
		asp.telemetry.ProcessorAtlassianSamplingPolicyDecisions.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("policy", "evicted"),
				attribute.String("decision", evaluators.NotSampled.String()),
				attribute.String("cache", cacheName),
			))

		// Only record eviction time when it was a trace that was not sampled, to avoid
		// metrics being polluted with explicit cache deletions from traces being sampled and exported.
		asp.telemetry.ProcessorAtlassianSamplingTraceEvictionTime.
			Record(ctx, time.Since(td.Metadata.ArrivalTime).Seconds(),
				metric.WithAttributes(attribute.String("cache", cacheName)))
	}
}

func (asp *atlassianSamplingProcessor) onEvictSampled(_ pcommon.TraceID, insertTime time.Time) {
	if asp.started.Load() {
		// only record eviction time if processor is running / not shutting down
		asp.telemetry.ProcessorAtlassianSamplingDecisionEvictionTime.
			Record(context.Background(), time.Since(insertTime).Seconds(), sampledAttr)
	}
}

func (asp *atlassianSamplingProcessor) onEvictNotSampled(_ pcommon.TraceID, outcome *nsdOutcome) {
	if asp.started.Load() {
		// only record eviction time if processor is running / not shutting down
		asp.telemetry.ProcessorAtlassianSamplingDecisionEvictionTime.
			Record(context.Background(), time.Since(outcome.decisionTime).Seconds(), notSampledAttr)
	}
}
