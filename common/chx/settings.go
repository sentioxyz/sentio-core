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

func WithLightDeleteTableSettings(settings map[string]string) {
	settings["enable_block_number_column"] = "1"
	settings["enable_block_offset_column"] = "1"
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
