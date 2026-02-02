package ckhmanager

func NewConnSettingsMacro() map[string]any {
	var (
		maxPartitionSizeToDrop uint64 = 536870912000
		maxTableSizeToDrop     uint64 = 536870912000
	)
	return map[string]interface{}{
		"optimize_aggregation_in_order":                       1,
		"max_ast_depth":                                       50000,
		"max_partition_size_to_drop":                          maxPartitionSizeToDrop, // 500GB
		"max_table_size_to_drop":                              maxTableSizeToDrop,     // 500GB
		"union_default_mode":                                  "ALL",
		"connect_timeout_with_failover_ms":                    120000,
		"query_cache_system_table_handling":                   "save",
		"output_format_native_write_json_as_string":           1,
		"allow_push_predicate_ast_for_distributed_subqueries": 0,
		"enable_json_type":                                    1,
		"query_cache_nondeterministic_function_handling":      "ignore",
		"max_partitions_per_insert_block":                     10240, // 10240 partitions per insert block
		"secondary_indices_enable_bulk_filtering":             0,
	}
}

const (
	ExternalTcpProxyField    = "external_tcp_proxy"
	InternalTcpProxyField    = "internal_tcp_proxy"
	ExternalTcpField         = "external_tcp_addr"
	InternalTcpField         = "internal_tcp_addr"
	ExternalTcpReplicasField = "external_tcp_replicas"
	InternalTcpReplicasField = "internal_tcp_replicas"
)

const (
	ClickhouseSettings_ProxyAuthKey = "SQL_x_auth_token"
)
