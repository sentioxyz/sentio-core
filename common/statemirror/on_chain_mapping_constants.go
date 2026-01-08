package statemirror

type OnChainKey string

const (
	MappingProcessorAllocations OnChainKey = "ProcessorAllocations"
	MappingIndexerInfos         OnChainKey = "IndexerInfos"
)

type OnChainValue interface {
	Encode() (string, error)
}

type OnChainValueDecoder[T any] func(string) (T, error)
