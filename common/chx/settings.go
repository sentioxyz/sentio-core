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

func WithLightDeleteTableSettings(settings map[string]string) {
	settings["enable_block_number_column"] = "1"
	settings["enable_block_offset_column"] = "1"
}

func WithProjectionTableSettings(settings map[string]string) {
	settings["lightweight_mutation_projection_mode"] = "'rebuild'"
}
