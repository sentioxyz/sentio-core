package statemirror

type OnChainKey string

const (
	MappingProcessorAllocations OnChainKey = "ProcessorAllocations"
	MappingProcessorInfos       OnChainKey = "ProcessorInfos"
	MappingIndexerInfos         OnChainKey = "IndexerInfos"
)
