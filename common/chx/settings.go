package chx

import (
	"context"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
)

func mergeSettings(settings ...map[string]any) map[string]any {
	r := make(map[string]any)
	for _, ss := range settings {
		if ss == nil {
			continue
		}
		for k, v := range ss {
			r[k] = v
		}
	}
	return r
}

func LightDeleteCtx(ctx context.Context, otherSettings ...map[string]any) context.Context {
	settings := mergeSettings(mergeSettings(otherSettings...), map[string]any{
		"alter_update_mode":                     "lightweight",
		"lightweight_delete_mode":               "lightweight_update",
		"enable_lightweight_delete":             "1",
		"allow_experimental_lightweight_update": "1",
	})
	return ckhmanager.ContextMergeSettings(ctx, settings)
}

func InsertCtx(ctx context.Context, uniqToken string, otherSettings ...map[string]any) context.Context {
	settings := mergeSettings(otherSettings...)
	settings["insert_deduplication_token"] = uniqToken
	return ckhmanager.ContextMergeSettings(ctx, settings)
}

func DisableProjectionCtx(ctx context.Context, otherSettings ...map[string]any) context.Context {
	settings := mergeSettings(mergeSettings(otherSettings...), map[string]any{
		"allow_experimental_projection_optimization": "0",
	})
	return ckhmanager.ContextMergeSettings(ctx, settings)
}

func InsertSelectCtx(ctx context.Context, otherSettings ...map[string]any) context.Context {
	settings := mergeSettings(otherSettings...)
	settings["max_partitions_per_insert_block"] = 0
	return ckhmanager.ContextMergeSettings(ctx, settings)
}

// AsyncMutationCtx makes mutation statements (ALTER TABLE ... DELETE/UPDATE) return as soon as the
// mutation is submitted instead of waiting for it to finish, so the statement never hits the
// client-side read timeout no matter how long the mutation takes. Completion must then be tracked
// separately by polling system.mutations.
func AsyncMutationCtx(ctx context.Context, otherSettings ...map[string]any) context.Context {
	settings := mergeSettings(mergeSettings(otherSettings...), map[string]any{
		"mutations_sync": "0",
	})
	return ckhmanager.ContextMergeSettings(ctx, settings)
}

// DisableAggregationInOrderCtx turns off optimize_aggregation_in_order, which the connection
// enables for every query. Aggregating in order is only fast when the GROUP BY key is a prefix of
// the sorting key; when it is not, the aggregation crawls instead: grouping sui v4 balances by
// (checkpoint, tx_index, address, coin_type) against a sorting key of (checkpoint, tx_index,
// address) took 15s over 10k checkpoints with it and 0.09s without.
func DisableAggregationInOrderCtx(ctx context.Context, otherSettings ...map[string]any) context.Context {
	settings := mergeSettings(mergeSettings(otherSettings...), map[string]any{
		"optimize_aggregation_in_order": 0,
	})
	return ckhmanager.ContextMergeSettings(ctx, settings)
}

func WithLightDeleteTableSettings(settings map[string]string) {
	settings["enable_block_number_column"] = "1"
	settings["enable_block_offset_column"] = "1"
}

// SyncReplicaCtx bounds how long a replica may be waited for. It is set here rather than inherited
// because a caller's own read timeout says nothing about how long a replica should be given to
// fetch what it is missing: the entity store, for one, asks for a five second one.
func SyncReplicaCtx(ctx context.Context, otherSettings ...map[string]any) context.Context {
	settings := mergeSettings(mergeSettings(otherSettings...), map[string]any{
		"receive_timeout": syncReplicaTimeoutSeconds,
	})
	return ckhmanager.ContextMergeSettings(ctx, settings)
}

// onClusterDDLTimeoutSeconds is how long the hosts of a cluster are given to run a distributed
// statement before it fails. It is the default of ClickHouse, stated so that it holds whatever the
// connection is set to.
const onClusterDDLTimeoutSeconds = 180

// onClusterCtx is for every ON CLUSTER statement whose effect the caller goes on to rely on: it
// makes the statement return only once every host has run it, and fail otherwise. Both settings
// are put on the statement rather than left to the connection, whose settings may say otherwise.
// With a task timeout of zero the statement returns as soon as it is queued, and an output mode
// that does not throw reports a host that has not run it as a success; either way the caller
// carries on as if a replica had caught up, or an INSERT had been ended, when it has not, and the
// statement then runs later, on whatever is there by then.
func onClusterCtx(ctx context.Context) context.Context {
	return ckhmanager.ContextMergeSettings(ctx, map[string]any{
		"distributed_ddl_output_mode":  "throw",
		"distributed_ddl_task_timeout": onClusterDDLTimeoutSeconds,
	})
}

// alterSyncCtx makes an ALTER of a replicated table return only once every replica has taken the
// change, where it otherwise waits for the replica that runs it only. A change of the structure
// reaches the other replicas through the replication of the table, moments later, and a statement
// one of them answers meanwhile does not find a column that was just added. It builds on
// onClusterCtx, without which an ALTER ... ON CLUSTER may return before any host has run it. The
// price is that the ALTER fails, on a timeout, while a replica is down.
func alterSyncCtx(ctx context.Context) context.Context {
	return ckhmanager.ContextMergeSettings(onClusterCtx(ctx), map[string]any{"alter_sync": 2})
}

func WithProjectionTableSettings(settings map[string]string) {
	settings["lightweight_mutation_projection_mode"] = "'rebuild'"
	// A merge that deduplicates refuses to run on a table with projections unless it is told what
	// to do with them, and the default is to refuse. That leaves OPTIMIZE ... FINAL DEDUPLICATE,
	// which is how duplicate rows are cleaned out of one of these tables by hand, failing with
	// SUPPORT_IS_DISABLED until someone sets this — and MODIFY SETTING does not replicate, so
	// setting it in the moment means remembering ON CLUSTER or leaving the replicas disagreeing.
	// Declaring it here has the table sync put it on every replica of every such table instead.
	// These tables are all plain MergeTree, where nothing but an explicit DEDUPLICATE merges by
	// key, so this changes no merge that runs on its own.
	settings["deduplicate_merge_projection_mode"] = "'rebuild'"
}
