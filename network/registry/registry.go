package registry

import (
	"context"

	"sentioxyz/sentio-core/common/statemirror"
	"sentioxyz/sentio-core/network/state"
)

type DbRegistry interface {
	RetrieveDatabaseInfo(ctx context.Context, database Database) (state.DatabaseInfo, error)
	RetrievePermissionsByAccount(ctx context.Context, address Address) (map[Database]DbAuth, error)
	AccountHasPermission(ctx context.Context, address Address, database Database, action Action) (bool, error)
	RetrieveAllDatabaseInfos(ctx context.Context) (map[Database]state.DatabaseInfo, error)
}

type ProcessorRegistry interface {
	RetrieveProcessorInfo(ctx context.Context, processorId ProcessorId) (state.ProcessorInfo, error)
	RetrieveProcessorAllocations(ctx context.Context, processorId ProcessorId) ([]state.ProcessorAllocation, error)
}

type IndexerRegistry interface {
	RetrieveIndexerInfo(ctx context.Context, indexerId IndexerId) (state.IndexerInfo, error)
	RetrieveAllIndexers(ctx context.Context) (map[IndexerId]state.IndexerInfo, error)
}

type Registry interface {
	DbRegistry
	ProcessorRegistry
	IndexerRegistry
}

type registry struct {
	mirror statemirror.Mirror
	DbRegistry
	ProcessorRegistry
	IndexerRegistry
}

func NewRegistry(m statemirror.Mirror) Registry {
	return &registry{
		mirror:            m,
		DbRegistry:        NewDbRegistry(m),
		ProcessorRegistry: NewProcessorRegistry(m),
		IndexerRegistry:   NewIndexerRegistry(m),
	}
}
