# Design

This document describes detailed rationale behind the decision we made while developing this processor.

It also contains information about how to run in a production environment.

## High Level Path

This section describes, in order, the path a trace takes when consumed by this processor.

1. `ConsumeTraces()` is invoked. This blocks on send to an unbuffered `chan`, and then returns.
2. `consumeChan()` reads the `chan` and processes the traces.
3. The data is organised by trace ID, and the main loop in `processor.go` processes the data one trace ID at a time.
4. The decision caches are accessed to determine if a sampling decision has already been made for the current trace ID. 
If a prior decision exists, this allows us to streamline the processing. When the cache indicates that the trace has 
already been sampled, the data is promptly forwarded to the next component. Conversely, if the cache indicates that 
the trace has not been sampled, the data is discarded without additional processing.
5. We then assess the sampling decision. The evaluation uses data from the current trace along with any metadata 
stored from previous evaluations of the same trace. Note that the cached trace data itself isn't re-evaluated, only 
the cached metadata is. Policies are applied in the order they are configured, and the first policy to provide a "decisive" 
result (i.e. not "Pending") is adopted. If all policies return "Pending", the final decision is "Pending". 
6. If the decision is "Sampled" or "NotSampled", the data is either forwarded or discarded, respectively. 
7. If the decision is "Pending" or "LowPriority", the data is placed into the cache. The cache operates on a 
Least Recently Used (LRU) basis, meaning that adding new data to the cache may involve evicting the 
least recently accessed trace (i.e. the trace that last received a new span the longest time ago). When a trace is 
evicted, it is considered "not sampled" and added to the decision cache.

## Synchronized Goroutine

The main operation of the processor is executed as a single goroutine, synchronized through an unbuffered channel.

In the collector architecture, receivers typically function as servers that accept and process data using multiple goroutines. 
Consequently, processors like this one are invoked concurrently through the `ConsumeTraces()` method. 
To ensure synchronization, the processor sends data to a channel, which is then received by a 
dedicated goroutine (`consumeChan()`). This design guarantees that all data is processed by a single goroutine. 
It draws inspiration from the core collector's batch processor.

The decision to synchronize is primarily driven by the need to maintain the integrity of internal 
caches while keeping the design simple. Allowing concurrent access to cached trace data would complicate the 
code significantly and potentially lead to bugs, as experienced in the upstream tail sampling processor.

This is, of course, a trade-off. The processing throughput is limited by the capacity of a single goroutine, 
creating a potential bottleneck. This can be alleviated by deploying more instances of the processor with reduced 
memory allocation per instance (e.g., more nodes, each with less memory). If the bottleneck becomes a significant issue, 
a future enhancement could involve sharding the processor. This would involve splitting the processing workload by trace 
ID and maintaining separate caches and states for each shard.

## Policies, and Policy Evaluation

Policy implementations typically return "Pending" by default when the data does not specifically meet their criteria.

When a policy returns "Sampled" it has indicated to sample the trace. When a policy returns "NotSampled" this indicates 
to immediately drop the trace, and purge any remnants of it from the caches.

Given that policies act this way, the order they operate is important. If they were to disagree on such a decision,
then the first evaluation that's non-pending will win.

A "LowPriority" decision is the same as pending, but it indicates to the processor that the data can be considered
low priority, and held in caches for a lower amount of time. An example of this is if a trace contains 
only a single root span - it's likely that it's not an important trace, and we can mark it as low priority if it doesn't 
match another policy.

### Essential policies for configuration

"Default-Low-Rate" using the `probabilistic` policy: This policy introduces a degree of randomness to the sampling process, 
ensuring that a diverse range of traces is sampled at a baseline rate.

"Max-Spans" using the `span_count` policy: Given that the cache is constrained by the number of traces it 
can hold, there's a risk of memory overload if a few traces accumulate an excessive number of spans. 
This issue surfaces when a trace continually receives new spans, preventing it from being evicted. 
To mitigate this, limit the number of spans per trace. If retaining some of these traces is desirable, 
consider combining this policy with a probabilistic sampler that runs beforehand.

### Policy Evaluation Design Decisions

The design decisions on policy evaluations were mainly driven by alleviating the shortcomings of the policy system
in the contrib tail sampling processor.

The contrib's tail sampler is designed to collect spans in memory and wait for a predetermined period, known as the 
"decision wait" time, which defaults to 30 seconds. Its intention is to apply sampling policies to entire traces, 
with the assumption that all spans from a given trace will be collected in-memory after the decision wait time elapses.

This approach presented several challenges for us:

* A significant portion of our data is "garbage" that could be discarded sooner. For instance, "lonely" root spans often have a high probability of being irrelevant.
* Many of our critical traces arrive over a duration exceeding 30 to 60 seconds, making the "decision time" awkward.
* There is no mechanism to differentiate between "important" and "garbage" data, resulting in all traces being retained in memory for a minimum duration.
* Once a decision is made, it is not cached, which led to incomplete traces if a trace continued to arrive over an extended period.
* Traces cannot be efficiently compressed in memory, because they may need to be read several times after being put into the cache.

To address these issues, we decided that policies within this processor should be evaluated immediately upon receiving 
spans, rather than waiting for the entire trace to be collected.

However, this introduces a significant tradeoff: policies must evaluate based solely on the spans that have arrived, 
not the complete trace. They rely on the currently available trace data and metadata from previously collected spans, 
such as count, earliest start time, and latest end time.

This decision has implications for policy design:

* Policies cannot consider the relationships between spans, as they do not have access to the full trace.
* Decisions cannot be made based on trace completeness, such discarding traces that consist of less than a certain number of spans (e.g., 10 spans), 
because it is uncertain whether all data from a trace has been received.

As previously mentioned, the policy evaluators are unable to "go back" and examine spans stored in the cache; 
they can only access cached metadata from the cache. This restriction is in place for two key reasons:

1. **Cache Compression:** It allows for the compression of spans within the cache. We are guaranteed that a compressed 
blob is decompressed at most once, which occurs in the case it is sampled and transmitted. 
Restricting the reading of cached spans allows for this optimisation.

2. **Performance Efficiency:** It ensures that policy evaluation remains efficient, adhering to O(n) complexity, 
where n represents the number of spans in the current arriving batch. This prevents the policies from becoming slow.

## Caches

There are four LRU caches that make up the state of this processor.
They all have trace ID as their key, and can all be configured by maximum number of entries.

### 1. Primary Trace Data Cache

The primary trace data cache stores trace data and metadata that are awaiting a sampling decision. 
As this cache represents the largest source of memory consumption, careful configuration is essential. 
Once a trace is evicted from this cache, it is considered not sampled.

If the `target_heap_bytes` is configured, the cache regulator becomes active on the primary trace data cache.
This periodically observes the heap usage of the golang runtime, and reduces the cache size by 2% if it is above
the target.

### 2. Secondary Trace Data Cache

The secondary trace data cache is optional and is only enabled if the `secondary_cache_size` configuration option is specified. 
Functionally similar to the primary cache, it is specifically designated for "LowPriority" data. 
Traces deemed "LowPriority" by a sampling policy are stored here. This cache should have a
shorter eviction time than the primary cache, allowing lower-priority data to be removed from memory more swiftly.

### 3. Sampled Decision Cache

This is a cache that holds trace IDs that are sampled. This is used to remember decisions made prior for a trace. 

### 4. Non Sampled Decision Cache

This is the same as the sampled decision cache, except it holds trace IDs that were not sampled.

## Compression of cached data

Compression is optionally enabled via `compression_enabled` config option. It will convert the trace data 
into compressed blobs before placing it into the cache.

This significantly saves memory but adds to processing time and CPU usage. 
Given the single goroutine bottleneck, this should be carefully enabled.

## Shutdown Flushing

The config option `flush_on_shutdown`, can be used to flush all cached data (including decision data) upon the shutdown
of the processor. 

This is useful for the scenario where a node is being scaled down or replaced. Without this option,
all cached data is lost when the processor is shutdown.

Flushed data is given specific resource attributes, so it can be routed differently than regular, sampled data.
This is ordinarily used in conjunction with the `routingconnector`, so the data can be flushed back to the balancing
layer for re-processing.

The decision caches are also flushed. This data is encoded as "decision spans". 
Decision spans have the same trace ID as the decision they represent, so it gets routed to the same node as the actual span data.
When the processor receives a decision span (it sees the attribute), it recognises it as such, and populates the 
decision cache. Note that this means you can actually "send" a decision to the processor. Decision spans are given their trace ID 
so they get routed to the correct node by the load balancing layer.

Given the LRU nature of all the caches, data is flushed in order, oldest first, newest last. This is intended 
to preserve the rough existing order. Data that's older will arrive first and so be evicted first.

## Metrics 

All metrics emitted by this processor are documented in `documentation.md`.
