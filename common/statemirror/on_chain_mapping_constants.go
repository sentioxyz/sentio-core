package statemirror

type OnChainKey string

const (
	MappingProcessorAllocations OnChainKey = "ProcessorAllocations"
	MappingProcessorInfos       OnChainKey = "ProcessorInfos"
	MappingIndexerInfos         OnChainKey = "IndexerInfos"
	MappingDatabases            OnChainKey = "Databases"
	MappingTableSchemas         OnChainKey = "TableSchemas"
	MappingDatabasePermissions  OnChainKey = "DatabasePermissions"
	MappingOperators            OnChainKey = "Operators"
)
