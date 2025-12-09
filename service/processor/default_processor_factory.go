package processor

import (
	"context"
	commonmodels "sentioxyz/sentio-core/service/common/models"
	"sentioxyz/sentio-core/service/processor/models"
	"sentioxyz/sentio-core/service/processor/repository"
)

type DefaultProcessorFactory struct {
	processorRepo repository.ProcessorRepo
}

func NewDefaultProcessorFactory(processorRepo repository.ProcessorRepo) *DefaultProcessorFactory {
	return &DefaultProcessorFactory{
		processorRepo,
	}
}

func (d *DefaultProcessorFactory) CreateOrUpdateProcessor(
	ctx context.Context,
	identity *commonmodels.Identity,
	project *commonmodels.Project,
	continueFrom int32,
	rollback map[string]uint64,
	sentioProperties models.SentioProcessorProperties,
	subgraphProperties models.SubgraphProcessorProperties,
	activateProcessor func(ctx context.Context, processor *models.Processor, upgrade bool) error) (p *models.Processor, err error) {
	err = d.processorRepo.WithTransaction(ctx, func(ctx context.Context) error {
		p, err = d.processorRepo.CreateOrUpdateProcessor(
			ctx,
			project,
			continueFrom,
			false,
			identity,
			0,
			0,
			sentioProperties,
			subgraphProperties,
		)
		if err != nil {
			return err
		}
		if err = activateProcessor(ctx, p, continueFrom > 0); err != nil {
			return err
		}
		return nil
	})
	return p, err
}
