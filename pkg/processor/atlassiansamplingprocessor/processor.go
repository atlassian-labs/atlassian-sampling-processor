package atlassiansamplingprocessor // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor"

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
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/cache"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/evaluators"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/memory"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/metadata"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/priority"
	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

const (
	flushCountKey     = "atlassiansampling.flushes"
	decisionSpanKey   = "atlassiansampling.decision"
	memTickerInterval = 10 * time.Second
	traceIDLoggingKey = "traceId"
	shardIDKey        = "shardid"
)

var (
	sampledAttr    = metric.WithAttributes(attribute.String("decision", "sampled"))
	notSampledAttr = metric.WithAttributes(attribute.String("decision", "not_sampled"))
	evictionAttrs  = metric.WithAttributes(
		attribute.String("policy", "evicted"),
		attribute.String("decision", evaluators.NotSampled.String()))
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
	nonSampledDecisionCache cache.Cache[time.Time]
	// shards are the channels which the sharded goroutines read incoming data from
	shards []chan []*groupedTraceData
	// memRegulator can adjust cache sizes to target a given heap usage.
	// May be nil, in which case the cache sizes will not be adjusted.
	memRegulator memory.RegulatorI
	// memTicker controls how often the memRegulator is called
	memTicker *time.Ticker
	// globalLock protects the caches for operations that affect data cross-shard, e.g. resizing the cache.
	globalLock *sync.RWMutex
	// regulatorStartTime is the time at which the regulator starts being used to regulate cache sizes
	regulatorStartTime time.Time
	// shutdown is a chan used to signal to the async goroutines to start shutdown. This is
	// broadcast to the async goroutines by closing the chan.
	shutdown  chan struct{}
	waitGroup *sync.WaitGroup

	flushOnShutdown bool
	compress        bool
	started         atomic.Bool
}

var _ processor.Traces = (*atlassianSamplingProcessor)(nil)

func newAtlassianSamplingProcessor(cCfg component.Config, set component.TelemetrySettings, next consumer.Traces) (*atlassianSamplingProcessor, error) {
	cfg, ok := cCfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type provided")
	}

	telemetry, err := metadata.NewTelemetryBuilder(set)
	if err != nil {
		return nil, err
	}

	shards := make([]chan []*groupedTraceData, cfg.Shards)
	for i := 0; i < cfg.Shards; i++ {
		shards[i] = make(chan []*groupedTraceData, cfg.PreprocessBufferSize)
	}

	asp := &atlassianSamplingProcessor{
		next:      next,
		telemetry: telemetry,
		log:       set.Logger,

		shards:     shards,
		globalLock: &sync.RWMutex{},

		shutdown:        make(chan struct{}),
		waitGroup:       &sync.WaitGroup{},
		flushOnShutdown: cfg.FlushOnShutdown,
		compress:        cfg.CompressionEnabled,
	}

	// Start with 60% of max cache size, the memory regulator will adjust the cache size as needed
	initialPrimaryCacheSize := int(0.6 * float64(cfg.PrimaryCacheSize))

	primaryCache, err := cache.NewLRUCache[*tracedata.TraceData](
		initialPrimaryCacheSize,
		asp.primaryEvictionCallback,
		telemetry,
		"primary")
	if err != nil {
		return nil, err
	}

	asp.traceData = primaryCache

	if cfg.SecondaryCacheSize > 0 {
		secondaryCache, err2 := cache.NewLRUCache[*tracedata.TraceData](
			cfg.SecondaryCacheSize,
			asp.secondaryEvictionCallback,
			telemetry,
			"secondary")
		if err2 != nil {
			return nil, err2
		}
		asp.traceData, err2 = cache.NewTieredCache[*tracedata.TraceData](primaryCache, secondaryCache)
		if err2 != nil {
			return nil, err2
		}
	}

	asp.sampledDecisionCache, err = cache.NewLRUCache[time.Time](cfg.SampledCacheSize, asp.onEvictSampled, telemetry, "sampled_decision")
	if err != nil {
		return nil, err
	}
	asp.nonSampledDecisionCache, err = cache.NewLRUCache[time.Time](cfg.NonSampledCacheSize, asp.onEvictNotSampled, telemetry, "nonsampled_decision")
	if err != nil {
		return nil, err
	}

	asp.memTicker = time.NewTicker(memTickerInterval)
	if cfg.TargetHeapBytes > 0 {
		memRegulator, rErr := memory.NewRegulator(
			cfg.PrimaryCacheSize/4,
			cfg.PrimaryCacheSize,
			cfg.TargetHeapBytes,
			memory.GetHeapUsage,
			primaryCache)
		if rErr != nil {
			return nil, rErr
		}

		asp.memRegulator = memRegulator
		asp.regulatorStartTime = time.Now().Add(cfg.RegulateCacheDelay)
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

	for i := 0; i < len(asp.shards); i++ {
		asp.waitGroup.Add(1)
		go func() {
			defer asp.waitGroup.Done()
			defer asp.log.Info("shardListener() finished", zap.Int(shardIDKey, i))
			asp.shardListener(i)
		}()
	}
	return nil
}

func (asp *atlassianSamplingProcessor) Shutdown(parentCtx context.Context) error {
	start := time.Now()
	if !asp.started.Load() {
		return nil
	}
	asp.log.Info("shutdown of atlassianSamplingProcessor initiated")
	defer func() {
		asp.log.Info("shutdown of atlassianSamplingProcessor finished", zap.Duration("shutdown.duration", time.Since(start)))
	}()

	deadline := time.Now().Add(50 * time.Second)
	ctx, cancel := context.WithDeadline(parentCtx, deadline)
	defer cancel()

	asp.memTicker.Stop()

	// Closing this channel broadcasts a shutdown signal to all shardListeners
	close(asp.shutdown)
	select {
	// Sends when shard listeners are done
	case <-wgWaiter(asp.waitGroup):
		asp.started.Store(false)
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for shard listeners to shutdown: %w", ctx.Err())
	}

	var err error
	if asp.flushOnShutdown {
		err = asp.flushAll(ctx)
	}

	return err
}

// ConsumeTraces implements the processing interface.
// The main work this function does is to split up the received data by trace ID, and then
// split up those trace IDs deterministically by shard. Then, it calls the shard with the arranged data.
func (asp *atlassianSamplingProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	start := time.Now()
	defer func() {
		asp.telemetry.ProcessorAtlassianSamplingChanBlockingTime.Record(ctx, time.Since(start).Nanoseconds())
	}()

	n := len(asp.shards)

	// This is the eventual structure we are building for sending to the shard
	shardedResourceSpans := make([][]*groupedTraceData, n)

	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		resource := resourceSpans.At(i).Resource()
		spansByTraceID := groupSpansByTraceID(resourceSpans.At(i))

		asp.earlyDecisionChecks(ctx, resource, spansByTraceID)

		if len(spansByTraceID) == 0 {
			continue
		}

		// Shard the traces
		for id, spans := range spansByTraceID {
			shardID := shardIDForTrace(id, n)
			shardedResourceSpans[shardID] = append(shardedResourceSpans[shardID], &groupedTraceData{
				id:       id,
				resource: resource,
				spans:    spans,
			})
		}
	}

	// Send to shards in parallel
	eg := &errgroup.Group{}
	for i := range asp.shards {
		if shardedResourceSpans[i] == nil {
			continue
		}

		eg.Go(func() error {
			select {
			case asp.shards[i] <- shardedResourceSpans[i]:
				return nil
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while waiting for shards[%d] to receive: %w", i, ctx.Err())
			}
		})
	}
	return <-egWaiter(eg)
}

// shardListener reads the traces from the specified channel / shard.
func (asp *atlassianSamplingProcessor) shardListener(shardID int) {
	ctx := context.Background()
	for {
		select {
		// Regular operating case
		case gtdArr := <-asp.shards[shardID]:
			asp.processTraces(ctx, gtdArr)
		case t := <-asp.memTicker.C:
			// If ticker signals, call the memory regulator
			if t.After(asp.regulatorStartTime) && asp.memRegulator != nil {
				// cache size regulation needs global write lock because it will affect data owned by other shards
				asp.globalLock.Lock()
				size := asp.memRegulator.RegulateCacheSize()
				asp.globalLock.Unlock()
				asp.telemetry.ProcessorAtlassianSamplingPrimaryCacheSize.Record(ctx, int64(size))
			}
		// If shutdown is signaled, process any pending traces and return
		case <-asp.shutdown:
			deadline := time.Now().Add(50 * time.Second)
			shutdownCtx, cancel := context.WithDeadline(ctx, deadline)
			for {
				select {
				case gtdArr := <-asp.shards[shardID]:
					asp.processTraces(shutdownCtx, gtdArr)
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

func (asp *atlassianSamplingProcessor) processTraces(ctx context.Context, gtdArr []*groupedTraceData) {
	asp.globalLock.RLock()
	defer asp.globalLock.RUnlock()
	for _, gtd := range gtdArr {
		id := gtd.id
		resource := gtd.resource
		spans := gtd.spans
		if asp.cachedDecision(ctx, id, resource, spans) {
			continue
		}

		currentTrace := ptrace.NewTraces()
		appendToTraces(currentTrace, resource, spans)

		td, err := tracedata.NewTraceData(time.Now(), currentTrace, priority.Unspecified, asp.compress)
		if err != nil {
			asp.reportTraceDataErr(ctx, err, int64(currentTrace.SpanCount()),
				zap.String(traceIDLoggingKey, hex.EncodeToString(id[:])),
				zap.Bool("compress", asp.compress))
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
				cachedTraces, gtErr := cachedData.GetTraces()
				if gtErr != nil {
					asp.reportTraceDataErr(ctx, gtErr, int64(td.Metadata.SpanCount), zap.String(traceIDLoggingKey, hex.EncodeToString(id[:])))
				} else {
					asp.sendSampledTraceData(ctx, cachedTraces)
				}
				asp.traceData.Delete(id)
			}
			asp.sendSampledTraceData(ctx, currentTrace)
			asp.telemetry.ProcessorAtlassianSamplingTracesSampled.Add(ctx, 1)
		case evaluators.NotSampled:
			asp.releaseNotSampledTrace(ctx, id, pol)
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
				tdErr := cachedTd.AbsorbTraceData(td) // chooses higher priority in merge
				if tdErr != nil {
					asp.reportTraceDataErr(ctx, tdErr, int64(td.Metadata.SpanCount), zap.String(traceIDLoggingKey, hex.EncodeToString(id[:])))
				}
				td = cachedTd // td is now the initial, incoming td + the cached td
			}
			asp.traceData.Put(id, td)
		}
	}
}

func (asp *atlassianSamplingProcessor) earlyDecisionChecks(ctx context.Context, resource pcommon.Resource, spansByTraceID spansGroupedByTraceID) {
	for id, spans := range spansByTraceID {
		_, isDecisionSpan := resource.Attributes().Get(decisionSpanKey)
		// decision spans must be dealt with in synchronized path, because they cause cache writes
		if isDecisionSpan {
			continue
		}

		// checks decision caches, exports data if sampled. If decision is in cache,
		// deletes this trace from the map, as it no longer needs later processing.
		decisionFromCache := asp.safeCheckDecisionCaches(id)

		if decisionFromCache == evaluators.Sampled {
			tracesToExport := ptrace.NewTraces()
			appendToTraces(tracesToExport, resource, spans)
			asp.sendSampledTraceData(ctx, tracesToExport)
			delete(spansByTraceID, id)
		} else if decisionFromCache == evaluators.NotSampled {
			delete(spansByTraceID, id)
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
	resource pcommon.Resource,
	spans []spanAndScope,
) bool {
	decisionFromDecisionSpan, isDecisionSpan := resource.Attributes().Get(decisionSpanKey)

	decisionFromCache := asp.safeCheckDecisionCaches(id)
	if decisionFromCache == evaluators.Sampled {
		if !isDecisionSpan {
			// We can palm this expensive operation off to a goroutine, since we know
			// there won't be anymore cache accesses that must be synchronized.
			go func() {
				td := ptrace.NewTraces()
				appendToTraces(td, resource, spans)
				asp.sendSampledTraceData(context.Background(), td)
			}()
		}
		return true
	} else if decisionFromCache == evaluators.NotSampled {
		return true
	}

	if !isDecisionSpan {
		return false
	}

	// This is a decision span, handle it
	if decisionFromDecisionSpan.Bool() {
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
		asp.releaseNotSampledTrace(ctx, id, nil)
	}
	return true
}

// safeCheckDecisionCaches is a concurrency safe check of a decision cache for the given trace ID, because it strictly
// reads the cache and does not write to it. The cache.Cache implementation must also be concurrency safe for this to hold.
func (asp *atlassianSamplingProcessor) safeCheckDecisionCaches(
	id pcommon.TraceID) evaluators.Decision {

	if _, ok := asp.sampledDecisionCache.Get(id); ok {
		return evaluators.Sampled
	}
	if _, ok := asp.nonSampledDecisionCache.Get(id); ok {
		return evaluators.NotSampled
	}
	return evaluators.Unspecified
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
func (asp *atlassianSamplingProcessor) releaseNotSampledTrace(ctx context.Context, id pcommon.TraceID, policy *policy) {
	// Check if EmitSingleSpanForNotSampled is true in the policy config
	if policy != nil && policy.emitSingleSpanForNotSampled {
		// Create a placeholder trace with a single span containing the decision policy name
		notSampledTrace := ptrace.NewTraces()
		rs := notSampledTrace.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", "not-sampled-dummy-service")
		ss := rs.ScopeSpans().AppendEmpty()
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(id)
		span.SetSpanID(generateRandomSpanID())
		span.SetName("TRACE NOT SAMPLED")
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Second)))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		span.Attributes().PutStr("sampling.policy", policy.name)

		// Send the placeholder trace
		asp.sendSampledTraceData(ctx, notSampledTrace)
	}
	asp.nonSampledDecisionCache.Put(id, time.Now())
	asp.traceData.Delete(id)
}

func (asp *atlassianSamplingProcessor) flushAll(ctx context.Context) error {
	group := &errgroup.Group{}
	group.SetLimit(30)

	// Create and export decision spans async
	group.Go(func() error {
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
		return asp.next.ConsumeTraces(ctx, decisionTraces)
	})

	// flush cached trace data
	vals := asp.traceData.Values()
	valsLen := len(vals)
	tracesSent := atomic.Int64{}
	for _, td := range vals {
		select {
		case <-ctx.Done():
			return fmt.Errorf("flush could not complete due to context cancellation. "+
				"%d out of %d traces successffuly sent: %w", tracesSent.Load(), valsLen, context.Cause(ctx))
		default:
			group.Go(
				func() error {
					cachedTraces, errGetTraces := td.GetTraces()
					if errGetTraces != nil {
						asp.reportTraceDataErr(ctx, errGetTraces, int64(td.Metadata.SpanCount))
						return errGetTraces
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
					err := asp.next.ConsumeTraces(ctx, cachedTraces)
					if err == nil {
						tracesSent.Add(1)
					}
					return err
				})
		}
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("context finished while waiting for goroutines to complete, "+
			"%d out of %d traces successffuly sent: %w", tracesSent.Load(), valsLen, context.Cause(ctx))
	case err := <-egWaiter(group):
		if err != nil {
			return fmt.Errorf("error waiting on errgroup, %d out of %d traces successfully sent: %w",
				tracesSent.Load(), valsLen, err)
		}
		return nil
	}
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
	// Check decision caches
	_, sampled := asp.sampledDecisionCache.Get(id)
	_, notSampled := asp.nonSampledDecisionCache.Get(id)

	// If decision does not exist in the cache, this is not an explicit deletion, but a natural eviction
	if !sampled && !notSampled {
		asp.nonSampledDecisionCache.Put(id, time.Now())
		asp.telemetry.ProcessorAtlassianSamplingTracesNotSampled.Add(ctx, 1)
		asp.telemetry.ProcessorAtlassianSamplingPolicyDecisions.
			Add(ctx, 1, evictionAttrs,
				metric.WithAttributeSet(attribute.NewSet(attribute.KeyValue{Key: "cache", Value: attribute.StringValue(cacheName)})))

		// Only record eviction time when it was a trace that was not sampled, to avoid
		// metrics being polluted with explicit cache deletions from traces being sampled and exported.
		asp.telemetry.ProcessorAtlassianSamplingTraceEvictionTime.
			Record(ctx, time.Since(td.Metadata.ArrivalTime).Seconds(),
				metric.WithAttributeSet(attribute.NewSet(attribute.String("cache", cacheName))))
	}
}

func (asp *atlassianSamplingProcessor) onEvictSampled(_ pcommon.TraceID, insertTime time.Time) {
	if asp.started.Load() {
		// only record eviction time if processor is running / not shutting down
		asp.telemetry.ProcessorAtlassianSamplingDecisionEvictionTime.
			Record(context.Background(), time.Since(insertTime).Seconds(), sampledAttr)
	}
}

func (asp *atlassianSamplingProcessor) onEvictNotSampled(_ pcommon.TraceID, decisionTime time.Time) {
	if asp.started.Load() {
		// only record eviction time if processor is running / not shutting down
		asp.telemetry.ProcessorAtlassianSamplingDecisionEvictionTime.
			Record(context.Background(), time.Since(decisionTime).Seconds(), notSampledAttr)
	}
}

func (asp *atlassianSamplingProcessor) reportTraceDataErr(ctx context.Context, err error, spanCount int64, fields ...zap.Field) {
	fields = append(fields, zap.Error(err), zap.Int64("spanCount", spanCount))
	asp.log.Error("failed to perform operation on TraceData", fields...)
	asp.telemetry.ProcessorAtlassianSamplingInternalErrorDroppedSpans.Add(ctx, spanCount)
}
