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

// DatabaseType mirrors the on-chain Types.DatabaseType enum:
// USER = 0 (user-owned database), PROCESSOR = 1 (processor replica database).
type DatabaseType uint8

const (
	DatabaseTypeUser      DatabaseType = 0
	DatabaseTypeProcessor DatabaseType = 1
)

type TableInfo struct {
	TableId   string `json:"tableId" yaml:"table_id"`
	TableType string `json:"tableType" yaml:"table_type"`
}

// DatabaseInfo mirrors on-chain Database struct. A database is bound to
// exactly one indexer (IndexerId). For PROCESSOR databases, ProcessorId
// identifies the owning processor and Owner is the zero address.
type DatabaseInfo struct {
	DatabaseId  string       `json:"databaseId" yaml:"database_id"`
	DbType      DatabaseType `json:"dbType" yaml:"db_type"`
	Creator     string       `json:"creator" yaml:"creator"`
	Owner       string       `json:"owner" yaml:"owner"`
	IndexerId   uint64       `json:"indexerId" yaml:"indexer_id"`
	ProcessorId string       `json:"processorId,omitempty" yaml:"processor_id,omitempty"`
	Operators   []string     `json:"operators" yaml:"operators"`
	Tables      []TableInfo  `json:"tables" yaml:"tables"`
}
