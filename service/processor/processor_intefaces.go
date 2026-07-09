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
		sentioNetworkProperties models.SentioNetworkProperties,
		activateProcessor func(ctx context.Context, processor *models.Processor, upgrade bool) error,
	) (p *models.Processor, err error)
}

// ProcessorLifecycleHook defines callbacks for processor lifecycle events.
type ProcessorLifecycleHook interface {
	// PreActivate runs before a processor version is activated, i.e. before its
	// driver job is (re)started. It is the gate point for admission checks such
	// as a security scan of the uploaded code.
	//
	// Returning a non-nil error aborts activation: the driver is NOT started
	// and currently-running versions are left untouched (so a bad new version
	// cannot take down a good running one). It is not treated as a hard failure
	// of the enclosing request — the caller keeps the persisted processor row so
	// it can be reviewed. Implementations that want the held processor to be
	// visible (paused, annotated, notified) must do so before returning the
	// error. Returning nil allows activation to proceed normally.
	PreActivate(ctx context.Context, processor *models.Processor) error
	OnActivate(ctx context.Context, processor *models.Processor) error
	OnStop(ctx context.Context, processor *models.Processor) error
	OnPause(ctx context.Context, processor *models.Processor) error
	OnResume(ctx context.Context, processor *models.Processor) error
}
