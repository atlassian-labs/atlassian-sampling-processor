# atlassian-sampling-processor 

<img src="atlassian-sampling-geometric.svg" alt="Atlassian Sampling Processor Logo - Geometric" width="150" height="150">
<img src="atlassian-sampling-flow.svg" alt="Atlassian Sampling Processor Logo - Flow" width="150" height="150">
<img src="atlassian-sampling-minimal.svg" alt="Atlassian Sampling Processor Logo - Minimal" width="150" height="150">

This repo contains the source code for the Atlassian Sampling Processor, a tail-based sampler
implemented as an OpenTelemetry Collector processor.

Read the documentation at [./pkg/processor/atlassiansamplingprocessor/README.md](./pkg/processor/atlassiansamplingprocessor/README.md).

We developed this processor to circumvent challenges with the Collector's [tailsamplingprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/tailsamplingprocessor/README.md).
For example, we wanted to scale horizontally without data loss, compress spans in memory, order sampling policies by priority,
and make sampling decisions as quickly as possible. 
More info on the design choices can be found in [DESIGN.md](./pkg/processor/atlassiansamplingprocessor/DESIGN.md).
We made these optimisations to dramatically reduce our tail sampling compute cost. 

## Logo Design

The three logo variations above represent the core concepts of the Atlassian Sampling Processor:

**Geometric Variation**: Features a funnel shape that visualizes the sampling/filtering process - many data points enter at the top (represented by dots), fewer are processed in the middle, and only the most relevant traces emerge at the bottom. The Atlassian blue gradient represents the processing pipeline, while the green accent indicates efficiency optimization.

**Flow Variation**: Uses flowing curves to represent the dynamic nature of trace processing and the intelligent routing of data streams. The gradient from blue to green symbolizes the transformation from raw telemetry data to optimized, sampled output. The circular nodes represent decision points where sampling policies are evaluated.

**Minimal Variation**: Takes a clean, hub-and-spoke approach with scattered input dots converging into a central processing core, then emerging as a single optimized output stream. This design emphasizes the processor's ability to handle distributed trace data and make quick, intelligent sampling decisions.

All designs use Atlassian's signature blue (#0052CC, #4C9AFF) combined with green (#36B37E) to represent efficiency and optimization - key goals of this tail-sampling processor.

The commits under `/pkg` are synced from a closed source monorepo. Most of the development is done closed source and then synced here.
That being said, we still accept contributions that we can later sync back to our upstream version. Also see [CONTRIBUTING.md](./CONTRIBUTING.md).

## Contributors

A big thank you to all contributors who have made this repository possible through their dedication and hard work.

* [James Moessis](https://github.com/jamesmoessis)
* [Jason Lee](https://github.com/jsonlsy)
* [Santosh Balaranganathan](https://github.com/san-san)
* [David Li](https://github.com/davidlee88)
* [Jordan Bertasso](https://github.com/jordanbertasso)
* Michael Yoo

## License

Apache-2.0 License. See [LICENSE](./LICENSE).
