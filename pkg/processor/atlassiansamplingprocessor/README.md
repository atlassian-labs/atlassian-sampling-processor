# Atlassian Sampling Processor

This is a tail sampling processor. Some of the code is copied and modified from the upstream collector's
[tailsamplingprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor). 

This processor exists because: 

- The upstream tail sampler has very high memory usage, and does very little to mitigate this.
- The upstream tail sampler has slow decision-making as it tries to gather all spans of a trace before making a decision.
- The upstream tail sampler has no way of prioritising one sampling policy over another.
- The upstream tail sampler lacks features that we want to develop (e.g. support for scaling, ability to look at root spans)

So, the overall goal is to be a tail sampler that is more performant, reliable and feature-rich than the upstream
collector's tail sampler (or even Refinery).

It's called the "Atlassian Sampling Processor", but please refrain from writing any atlassian-specific code, 
as there is a good chance this component will be open sourced.

## Detailed Documentation and Design Rationale

Detailed design documentation for this processor can be found in [DESIGN.md](./DESIGN.md).

## Config 

### `primary_cache_size`
The amount traces with non-low priority that are held in the primary internal LRU cache. 
When this value reaches max, the least-recently-used trace is evicted and considered as "not sampled".

The `primary_cache_size` value should be greater than 0, and should be set to a value that is appropriate for the trace volume.

`primary_cache_size` is the most important value to tune in terms of memory usage. Keep an eye on the
`processor_atlassian_sampling_trace_eviction_time` metric to tune how long you would like your traces to stay pending
in memory before being considered not-sampled.

The primary cache size is initially set to 80% of the `primary_cache_size` value.
It is automatically adjusted depending on heap memory usage at runtime, but will not exceed the `primary_cache_size` value.

### `secondary_cache_size`

The amount of traces with low priority that are held in the secondary internal LRU cache.
When this value reaches max, the least-recently-used trace is evicted and considered as "not sampled".

The `secondary_cache_size` value should be less than 50% of `primary_cache_size`.

If left at 0, there will be no secondary cache, and only the primary cache will be used.

The default value is 0.

__Note: It will overwrite any entries of the same key in either the primary or secondary cache to prevent a key appearing in both primary and secondary. 
If the caller wants to promote an existing key from secondary to primary, they can Put with non-low priority.__


### `decision_cache`

This provides two size config options, `sampled_cache_size` and `non_sampled_cache_size`.
These values configure the size of the decision caches. The decision caches hold a set of trace IDs that have been 
sampled or not sampled respectively. This allows a shortcut to the evaluation of newly arriving spans, if a decision
for their trace has already been made.

It is recommended for this value to be at least an order of magnitude higher than `max_traces`, since the internal
memory usage is much lower (it only stores the trace ID, not all the span data). So, it's valuable to hold 
on to the decision for a longer time than you hold onto any trace data, in case there are any late-arriving spans.

Keep an eye on `processor_atlassian_sampling_decision_eviction_time` to make sure that decisions are lasting an
appropriate amount of time.

### `flush_on_shutdown`

This is `false` by default. When set to true, the `Shutdown()` of the component causes all traces that 
are pending in the trace data cache to be flushed to the next consumer. This is to prevent data loss when 
a node running this component shuts down.

Before being flushed, a resource attribute `atlassiansampling.flushes` is added to the resource spans.
This enables the downstream components to detect which resource spans have been flushed due to shut down, and
routing them accordingly, for example using the `routingconnector`.
Additionally, the attribute counts how many times that it's been flushed and re-ingested, 
enabling the detection of infinite cycling.

### `compression_enabled`

If this is enabled, trace data stored in the primary and secondary caches are marshalled and
compressed using the Snappy algorithm, and decompressed once a sampling decision is made.
The default value is `false`.

### `policies`

`policies` is a list of policies, which configure how the sampling decisions are evaluated against the incoming data.

The order of the policies is important, as the first one that matches a non-pending decision will be 
used as the final decision.

Policies include a `name`, `type`, and then further configuration depending on what the `type` was.

Current supported policy types are: 

- `span_count` - samples the trace if it meets a minimum amount of spans. 
- `probabilistic` - evaluates the hash of the trace ID to a configured percentage.
- `remote_probabilistic` - fetches sampling rates using the specified rate getter at runtime and samples traces
  based on the fetched sampling rate. 
- `and` - combines any number of sampling policies together.
- `root_spans` - specifies a sub-policy to only operate on lone-root-spans, but eagerly converts the sub-policy "pending"
  decisions into "not sampled" decisions. A span considered to be "lone" if there is no other spans present for the same
  trace when it arrives, and it is considered to be a root span if it has no parent ID, or has a parent ID equal to the 
  right 64-bits of the trace ID.
- `latency` - samples traces with duration equal to or greater than threshold_ms. The duration is determined by looking at the earliest start time and latest end time, without taking into consideration what happened in between.
- `status_code` - samples based upon the status code (OK, ERROR or UNSET)
- `ottl_condition` - samples based on given boolean OTTL condition (span and span event).
- `downgrader` - downgrades any "Sampled" decision from the `sub_policy`, to what is specified in `downgrade_to`. 
- `threshold` - inspects span attribute `sampling.tail.threshold`, and makes a Sampled decision if the numerical value of the attribute
  is larger than the rightmost 56-bits of the trace ID. The attribute takes the string form "0x[0-9a-fA-F]{1,14}". If the numerical part 
  of the attribute is less than 14 digits (56-bits) long, it will be right-padded with zeroes as per OTEP-235.

### Example

For a full example of multiple sampling policies being configured, see [the example test file](./testdata/atlassian_sampling_test_cfg.yml).

## Metrics 

Metrics emitted from this component are documented in `documentation.md`.
