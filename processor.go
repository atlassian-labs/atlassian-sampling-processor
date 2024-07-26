package atlassiansamplingprocessor // import "bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor"

import (
	"context"
	"encoding/binary"
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
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/metadata"
	"bitbucket.org/atlassian/observability-sidecar/pkg/processor/atlassiansamplingprocessor/internal/tracedata"
)

const flushCountKey = "atlassiansampling.flushes"

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
	// sampledDecisionCache holds the set of trace IDs that were not sampled, and the time of decision
	nonSampledDecisionCache cache.Cache[time.Time]
	// incomingTraces is where the traces are put when they first arrive to the component
	incomingTraces chan ptrace.Traces
	// shutdownStart is a chan used to signal to the async goroutine to start shutdown.
	// The deadline time is passed to the chan, so the async goroutine knows when to time out.
	shutdownStart   chan time.Time
	waitGroup       sync.WaitGroup
	flushOnShutdown bool
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

	asp := &atlassianSamplingProcessor{
		next:      next,
		telemetry: telemetry,
		log:       set.Logger,

		incomingTraces:  make(chan ptrace.Traces),
		shutdownStart:   make(chan time.Time),
		flushOnShutdown: cfg.FlushOnShutdown,
	}

	traceData, err := cache.NewLRUCache[*tracedata.TraceData](cfg.MaxTraces, asp.onEvictTrace, telemetry)
	if err != nil {
		return nil, err
	}
	sdc, err := cache.NewLRUCache[time.Time](cfg.SampledCacheSize, asp.onEvictSampled, telemetry)
	if err != nil {
		return nil, err
	}
	nsdc, err := cache.NewLRUCache[time.Time](cfg.NonSampledCacheSize, asp.onEvictNotSampled, telemetry)
	if err != nil {
		return nil, err
	}

	asp.traceData = traceData
	asp.sampledDecisionCache = sdc
	asp.nonSampledDecisionCache = nsdc

	pols, err := newPolicies(cfg.PolicyConfig)
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

func (asp *atlassianSamplingProcessor) Start(_ context.Context, _ component.Host) error {
	if running := asp.started.Swap(true); running {
		return fmt.Errorf("component already started")
	}
	go asp.consumeChan()
	return nil
}

func (asp *atlassianSamplingProcessor) Shutdown(parentCtx context.Context) error {
	if !asp.started.Load() {
		return nil
	}

	deadline := time.Now().Add(50 * time.Second)
	ctx, cancel := context.WithDeadline(parentCtx, deadline)
	defer cancel()
	// Need to close chan because if parentCtx is cancelled already, the below select/case never sends the shutdown
	// signal, resulting in the consumeChan() goroutine to block waiting. Closing the channel forces this to release.
	defer close(asp.shutdownStart)

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
	ctx := context.Background()

	for {
		select {
		// Regular operating case
		case nt := <-asp.incomingTraces:
			resourceSpans := nt.ResourceSpans()
			for i := 0; i < resourceSpans.Len(); i++ {
				asp.processTraces(ctx, resourceSpans.At(i))
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
		td := tracedata.NewTraceData(time.Now(), currentTrace)

		// Merge metadata with any metadata in the cache to pass to evaluators
		mergedMetadata := td.Metadata.DeepCopy()

		if cachedData, ok := asp.traceData.Get(id); ok {
			mergedMetadata.MergeWith(cachedData.Metadata)
		}

		// Evaluate the spans against the policies.
		finalDecision := asp.decider.MakeDecision(ctx, id, currentTrace, mergedMetadata)

		switch finalDecision {
		case evaluators.Sampled:
			// Sample, cache decision, and release all data associated with the trace
			asp.sampledDecisionCache.Put(id, time.Now())
			if cachedData, ok := asp.traceData.Get(id); ok {
				asp.releaseSampledTrace(ctx, cachedData.ReceivedBatches)
				asp.traceData.Delete(id)
			}
			asp.releaseSampledTrace(ctx, td.ReceivedBatches)
			asp.telemetry.ProcessorAtlassianSamplingTracesSampled.Add(ctx, 1)
			return
		case evaluators.NotSampled:
			// Cache decision, delete any associated data
			asp.nonSampledDecisionCache.Put(id, time.Now())
			asp.traceData.Delete(id)
			asp.telemetry.ProcessorAtlassianSamplingTracesNotSampled.Add(ctx, 1)
			return
		default:
			// If we have reached here, the sampling decision is still pending, so we put trace data in the cache
			if cachedData, ok := asp.traceData.Get(id); ok {
				cachedData.MergeWith(td)
			} else {
				asp.traceData.Put(id, td)
			}
		}
	}
}

func (asp *atlassianSamplingProcessor) cachedDecision(
	ctx context.Context,
	id pcommon.TraceID,
	resourceSpans ptrace.ResourceSpans,
	spans []spanAndScope,
) bool {
	if _, ok := asp.sampledDecisionCache.Get(id); ok {
		td := ptrace.NewTraces()
		appendToTraces(td, resourceSpans, spans)
		asp.releaseSampledTrace(ctx, td)
		return true
	}
	if _, ok := asp.nonSampledDecisionCache.Get(id); ok {
		return true
	}
	return false
}

func (asp *atlassianSamplingProcessor) releaseSampledTrace(ctx context.Context, td ptrace.Traces) {
	if err := asp.next.ConsumeTraces(ctx, td); err != nil {
		asp.log.Warn(
			"Error sending spans to destination",
			zap.Error(err))
	}
}

func (asp *atlassianSamplingProcessor) flushAll(ctx context.Context) error {
	var err error
	vals := asp.traceData.Values()
	for i, td := range vals {

		// Increment flush count attribute
		rs := td.ReceivedBatches.ResourceSpans()
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
			return fmt.Errorf(""+
				"flush could not complete due to context cancellation, "+
				"only flushed %d out of %d traces: %w", i, len(vals), ctx.Err())
		default:
			err = multierr.Append(err, asp.next.ConsumeTraces(ctx, td.ReceivedBatches))
		}
	}
	return err
}

func (asp *atlassianSamplingProcessor) onEvictTrace(id uint64, td *tracedata.TraceData) {
	ctx := context.Background()
	asp.log.Debug("evicting trace from cache", zap.Uint64("traceID", id))
	asp.telemetry.ProcessorAtlassianSamplingTraceEvictionTime.
		Record(ctx, time.Since(td.Metadata.ArrivalTime).Seconds())
	// Convert back to [16]byte to query. We only need the right 8-bytes from the uint64,
	// since the cache only uses the right 8 bytes.
	idArr := pcommon.NewTraceIDEmpty()
	binary.LittleEndian.PutUint64(idArr[8:16], id)
	// Double check that we didn't actually sample this trace
	if _, ok := asp.sampledDecisionCache.Get(idArr); !ok {
		// Mark as not sampled
		asp.nonSampledDecisionCache.Put(idArr, time.Now())
		asp.telemetry.ProcessorAtlassianSamplingTracesNotSampled.Add(ctx, 1)
		asp.telemetry.ProcessorAtlassianSamplingPolicyDecisions.Add(ctx, 1, metric.WithAttributes(
			attribute.String("policy", "evicted"),
			attribute.String("decision", evaluators.NotSampled.String()),
		))
	}
}

func (asp *atlassianSamplingProcessor) onEvictSampled(id uint64, insertTime time.Time) {
	asp.log.Debug("evicting sampled decision", zap.Uint64("traceID", id))
	asp.telemetry.ProcessorAtlassianSamplingDecisionEvictionTime.
		Record(context.Background(), time.Since(insertTime).Seconds(),
			metric.WithAttributes(attribute.String("decision", "sampled")))
}

func (asp *atlassianSamplingProcessor) onEvictNotSampled(id uint64, insertTime time.Time) {
	asp.log.Debug("evicting not-sampled decision", zap.Uint64("traceID", id))
	asp.telemetry.ProcessorAtlassianSamplingDecisionEvictionTime.
		Record(context.Background(), time.Since(insertTime).Seconds(),
			metric.WithAttributes(attribute.String("decision", "not_sampled")))
}
