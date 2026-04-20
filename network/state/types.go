package state

type IndexerInfo struct {
	IndexerId           uint64 `json:"indexerId" yaml:"indexer_id"`
	IndexerUrl          string `json:"indexerUrl" yaml:"indexer_url"`
	ComputeNodeRpcPort  uint16 `json:"computeNodeRpcPort" yaml:"compute_node_rpc_port"`
	StorageNodeRpcPort  uint16 `json:"storageNodeRpcPort" yaml:"storage_node_rpc_port"`
	ClickhouseProxyPort uint16 `json:"clickhouseProxyPort" yaml:"clickhouse_proxy_port"`
}

type ProcessorAllocation struct {
	ProcessorId string `json:"processorId" yaml:"processor_id"`
	IndexerId   uint64 `json:"indexerId" yaml:"indexer_id"`
}

type ProcessorInfo struct {
	ProcessorId         string `json:"processorId" yaml:"processor_id"`
	EntitySchema        string `json:"entitySchema" yaml:"entity_schema"`
	EntitySchemaVersion int32  `json:"entitySchemaVersion" yaml:"entity_schema_version"`
}

type DatabaseAllocation struct {
	IndexerId    uint64 `json:"indexerId" yaml:"indexer_id"`
	ReplicaIndex uint32 `json:"replicaIndex" yaml:"replica_index"`
}

type TableInfo struct {
	TableId string `json:"tableId" yaml:"table_id"`
	Schema  string `json:"schema" yaml:"schema"`
}

type DatabaseInfo struct {
	DatabaseId  string               `json:"databaseId" yaml:"database_id"`
	Owner       string               `json:"owner" yaml:"owner"`
	Operators   []string             `json:"operators" yaml:"operators"`
	Allocations []DatabaseAllocation `json:"allocations" yaml:"allocations"`
	Tables      []TableInfo          `json:"tables" yaml:"tables"`
}
