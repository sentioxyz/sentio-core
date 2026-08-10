package state

import (
	"fmt"
	"strconv"
	"strings"
)

type IndexerInfo struct {
	IndexerId           uint64 `json:"indexerId" yaml:"indexer_id"`
	IndexerUrl          string `json:"indexerUrl" yaml:"indexer_url"`
	ComputeNodeRpcPort  uint16 `json:"computeNodeRpcPort" yaml:"compute_node_rpc_port"`
	StorageNodeRpcPort  uint16 `json:"storageNodeRpcPort" yaml:"storage_node_rpc_port"`
	ClickhouseProxyPort uint16 `json:"clickhouseProxyPort" yaml:"clickhouse_proxy_port"`
	Signer              string `json:"signer" yaml:"signer"`
}

type ProcessorAllocation struct {
	ProcessorId string `json:"processorId" yaml:"processor_id"`
	IndexerId   uint64 `json:"indexerId" yaml:"indexer_id"`
}

type ProcessorInfo struct {
	ProcessorId             string `json:"processorId" yaml:"processor_id"`
	EntitySchema            string `json:"entitySchema" yaml:"entity_schema"`
	EntitySchemaVersion     int32  `json:"entitySchemaVersion" yaml:"entity_schema_version"`
	TimeseriesSchemaVersion int32  `json:"timeseriesSchemaVersion,omitempty" yaml:"timeseries_schema_version,omitempty"`
}

// DatabaseType mirrors the on-chain Types.DatabaseType enum:
// USER = 0 (user-owned database), PROCESSOR = 1 (processor replica database).
type DatabaseType uint8

const (
	DatabaseTypeUser      DatabaseType = 0
	DatabaseTypeProcessor DatabaseType = 1
)

type TableInfo struct {
	TableId       string `json:"tableId" yaml:"table_id"`
	TableType     string `json:"tableType" yaml:"table_type"`
	SchemaVersion uint32 `json:"schemaVersion,omitempty" yaml:"schema_version,omitempty"`
	SchemaHash    string `json:"schemaHash,omitempty" yaml:"schema_hash,omitempty"`
}

type TableSchemaInfo struct {
	DatabaseId string `json:"databaseId" yaml:"database_id"`
	TableId    string `json:"tableId" yaml:"table_id"`
	Version    uint32 `json:"version" yaml:"version"`
	SchemaHash string `json:"schemaHash" yaml:"schema_hash"`
	SchemaJson string `json:"schemaJson" yaml:"schema_json"`
}

func TableSchemaKey(databaseId, tableId string, version uint32) string {
	return fmt.Sprintf("%s/%s@%d", databaseId, tableId, version)
}

func ParseTableSchemaKey(key string) (databaseId, tableId string, version uint32, err error) {
	versionSeparator := strings.LastIndexByte(key, '@')
	if versionSeparator < 0 || versionSeparator == len(key)-1 {
		return "", "", 0, fmt.Errorf("invalid table schema key %q: missing version", key)
	}

	identity := key[:versionSeparator]
	tableSeparator := strings.IndexByte(identity, '/')
	if tableSeparator <= 0 || tableSeparator == len(identity)-1 {
		return "", "", 0, fmt.Errorf("invalid table schema key %q: missing database or table", key)
	}
	databaseID := identity[:tableSeparator]
	tableID := identity[tableSeparator+1:]
	if strings.ContainsAny(databaseID, "/@") || strings.ContainsAny(tableID, "/@") {
		return "", "", 0, fmt.Errorf("invalid table schema key %q: database or table contains a reserved delimiter", key)
	}

	parsedVersion, parseErr := strconv.ParseUint(key[versionSeparator+1:], 10, 32)
	if parseErr != nil {
		return "", "", 0, fmt.Errorf("invalid table schema key %q version: %w", key, parseErr)
	}
	return databaseID, tableID, uint32(parsedVersion), nil
}

// DatabaseInfo mirrors on-chain Database struct. A database is bound to
// exactly one indexer (IndexerId). For PROCESSOR databases, ProcessorId
// identifies the owning processor.
type DatabaseInfo struct {
	DatabaseId    string       `json:"databaseId" yaml:"database_id"`
	DbType        DatabaseType `json:"dbType" yaml:"db_type"`
	IndexerId     uint64       `json:"indexerId" yaml:"indexer_id"`
	ProcessorId   string       `json:"processorId,omitempty" yaml:"processor_id,omitempty"`
	PendingDelete bool         `json:"pendingDelete" yaml:"pending_delete"`
	Tables        []TableInfo  `json:"tables,omitempty" yaml:"tables,omitempty"`
}
