# atlassian-sampling-processor 

This repo contains the source code for the Atlassian Sampling Processor, a tail-based sampler
implemented as an OpenTelemetry Collector processor.

Read the documentation at [./pkg/processor/atlassiansamplingprocessor/README.md](./pkg/processor/atlassiansamplingprocessor/README.md).

We developed this processor to circumvent challenges with the Collector's [tailsamplingprocessor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/tailsamplingprocessor/README.md).
For example, we wanted to scale horizontally without data loss, compress spans in memory, order sampling policies by priority,
and make sampling decisions as quickly as possible. 
More info on the design choices can be found in [DESIGN.md](./pkg/processor/atlassiansamplingprocessor/DESIGN.md).
We made these optimisations to dramatically reduce our tail sampling compute cost. 

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

Copyright 2024 Atlassian US, Inc.
Copyright 2024 Atlassian Pty Ltd.
