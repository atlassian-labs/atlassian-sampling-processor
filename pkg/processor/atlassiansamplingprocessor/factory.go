package atlassiansamplingprocessor // import "github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/atlassian-labs/atlassian-sampling-processor/pkg/processor/atlassiansamplingprocessor/internal/metadata"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability))
}

func createTracesProcessor(ctx context.Context, params processor.Settings, cfg component.Config, next consumer.Traces) (processor.Traces, error) {
	return newAtlassianSamplingProcessor(cfg, params.TelemetrySettings, next)
}
