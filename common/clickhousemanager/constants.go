package ckhmanager

import (
	"flag"
	"time"
)

var (
	readTimeout  = flag.Duration("clickhouse-read-timeout", time.Minute, "ClickHouse read timeout")
	dialTimeout  = flag.Duration("clickhouse-dial-timeout", time.Minute, "ClickHouse dial timeout")
	maxIdleConns = flag.Int("clickhouse-max-idle-conns", 30, "ClickHouse max idle connection")
	maxOpenConns = flag.Int("clickhouse-max-open-conns", 100, "ClickHouse max open connection")
)

func newConnSettingsMacro() map[string]any {
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
