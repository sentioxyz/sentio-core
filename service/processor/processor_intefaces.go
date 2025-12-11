package processor

import (
	"context"
	commonmodels "sentioxyz/sentio-core/service/common/models"
	"sentioxyz/sentio-core/service/processor/models"
)

// ProcessorFactory defines the interface for creating and updating processors
type ProcessorFactory interface {
	CreateOrUpdateProcessor(
		ctx context.Context,
		identity *commonmodels.Identity,
		project *commonmodels.Project,
		continueFrom int32,
		rollback map[string]uint64,
		numWorkers int32,
		sentioProperties models.SentioProcessorProperties,
		subgraphProperties models.SubgraphProcessorProperties,
		activateProcessor func(ctx context.Context, processor *models.Processor, upgrade bool) error,
	) (p *models.Processor, err error)
}

// ProcessorLifecycleHook defines callbacks for processor lifecycle events.
type ProcessorLifecycleHook interface {
	OnActivate(ctx context.Context, processor *models.Processor) error
	OnStop(ctx context.Context, processor *models.Processor) error
	OnPause(ctx context.Context, processor *models.Processor) error
	OnResume(ctx context.Context, processor *models.Processor) error
}
