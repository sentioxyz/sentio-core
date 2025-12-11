package repository

import (
	"context"
	commonmodels "sentioxyz/sentio-core/service/common/models"
	"sentioxyz/sentio-core/service/common/repository"
	"sentioxyz/sentio-core/service/processor/models"
	"sentioxyz/sentio-core/service/processor/protos"
)

type ProcessorRepo interface {
	GetProcessor(ctx context.Context, processorID string, withChainState bool) (*models.Processor, error)
	GetProcessors(ctx context.Context, projectID string) ([]models.Processor, error)
	SaveProcessor(ctx context.Context, processor *models.Processor) error
	RemoveProcessor(ctx context.Context, processorID string) error
	ObsoleteProcessor(ctx context.Context, processorID string) error
	PreloadProcessor(ctx context.Context, id string) (*models.Processor, error)

	FindProcessorByVersion(ctx context.Context, projectID string, version int32) (*models.Processor, error)
	FindActiveProcessor(ctx context.Context, projectID string) (*models.Processor, error)
	FindLatestProcessor(ctx context.Context, projectID string) (*models.Processor, error)
	FindReplacingProcessor(ctx context.Context, project *commonmodels.Project) (*models.Processor, error)
	ResolveReferenceProcessor(ctx context.Context, processor *models.Processor) (*models.Processor, error)

	GetProcessorsByProjectAndVersionState(ctx context.Context, projectID string, versionStates ...protos.ProcessorVersionState) ([]*models.Processor, error)
	GetObsoleteProcessors(ctx context.Context, projectID string) ([]models.Processor, error)

	CreateOrUpdateProcessor(
		ctx context.Context,
		project *commonmodels.Project,
		continueFrom int32,
		pause bool,
		identity *commonmodels.Identity,
		clickhouseShardingIndex int32,
		k8sClusterID int32,
		numWorkers int32,
		sentioProperties models.SentioProcessorProperties,
		subgraphProperties models.SubgraphProcessorProperties,
	) (*models.Processor, error)

	ListProcessorUpgradeHistory(ctx context.Context, processorID string) ([]models.ProcessorUpgradeHistory, error)
	GetProcessorUpgradeHistoryByID(ctx context.Context, historyID string, processorID string) (*models.ProcessorUpgradeHistory, error)
	SaveProcessorUpgradeHistory(ctx context.Context, processor *models.Processor) error

	GetProjectByID(ctx context.Context, projectID string) (*commonmodels.Project, error)
	GetProjectVersions(ctx context.Context, projectID string) ([]*models.Processor, error)
	GetProjectVariables(ctx context.Context, projectID string) ([]*commonmodels.ProjectVariable, error)
	PreLoadProject(ctx context.Context, owner, slug string) (*commonmodels.Project, error)

	repository.DBRepoInterface
}

type ChainStateRepo interface {
	GetChainState(ctx context.Context, processorID string, chainID string) (models.ChainState, error)
	GetChainStates(ctx context.Context, processorID string) ([]models.ChainState, error)
	UpdateChainState(ctx context.Context, chainState *models.ChainState) error
	DeleteChainStatesByProcessor(ctx context.Context, processorID string) error
	ListChainsByProjects(ctx context.Context, projectIDList []string, versionStatList []protos.ProcessorVersionState) (map[string][]string, error)
	repository.DBRepoInterface
}
