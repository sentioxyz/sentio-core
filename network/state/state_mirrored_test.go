package state

import (
	"context"
	"encoding/json"
	"testing"

	"sentioxyz/sentio-core/common/statemirror"
)

func TestStateMirroredTableSchemasAllWritePaths(t *testing.T) {
	ctx := context.Background()
	mirror, err := statemirror.NewFileMirror(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileMirror: %v", err)
	}

	initial := &PlainState{TableSchemas: map[string]TableSchemaInfo{}}
	state, err := NewStateMirrored(ctx, initial, mirror)
	if err != nil {
		t.Fatalf("NewStateMirrored: %v", err)
	}

	infoV1 := TableSchemaInfo{
		DatabaseId: "db_1",
		TableId:    "table_1",
		Version:    1,
		SchemaHash: "0x01",
		SchemaJson: `{"table_id":"table_1"}`,
	}
	keyV1 := TableSchemaKey(infoV1.DatabaseId, infoV1.TableId, infoV1.Version)

	working := state.Inner().Clone()
	if err := working.UpsertTableSchema(ctx, infoV1); err != nil {
		t.Fatalf("working.UpsertTableSchema: %v", err)
	}
	if err := state.ReplaceInner(ctx, working); err != nil {
		t.Fatalf("ReplaceInner add: %v", err)
	}
	assertMirroredTableSchema(t, ctx, mirror, keyV1, infoV1)

	working = state.Inner().Clone()
	delete(working.TableSchemas, keyV1)
	if err := state.ReplaceInner(ctx, working); err != nil {
		t.Fatalf("ReplaceInner delete: %v", err)
	}
	if _, ok, err := mirror.Get(ctx, statemirror.MappingTableSchemas, keyV1); err != nil {
		t.Fatalf("mirror.Get after delete: %v", err)
	} else if ok {
		t.Fatalf("mirror still contains %q after ReplaceInner delete", keyV1)
	}

	if err := state.UpsertTableSchema(ctx, infoV1); err != nil {
		t.Fatalf("StateMirrored.UpsertTableSchema: %v", err)
	}
	assertMirroredTableSchema(t, ctx, mirror, keyV1, infoV1)

	if err := mirror.Apply(ctx, statemirror.MappingTableSchemas, func(context.Context, statemirror.OnChainKey) (*statemirror.StateDiff, error) {
		return &statemirror.StateDiff{Deleted: []string{keyV1}}, nil
	}); err != nil {
		t.Fatalf("remove mirrored fixture: %v", err)
	}
	if err := state.SyncMirror(ctx); err != nil {
		t.Fatalf("SyncMirror: %v", err)
	}
	assertMirroredTableSchema(t, ctx, mirror, keyV1, infoV1)

	infoV2 := infoV1
	infoV2.Version = 2
	infoV2.SchemaHash = "0x02"
	keyV2 := TableSchemaKey(infoV2.DatabaseId, infoV2.TableId, infoV2.Version)
	if err := state.UpsertTableSchema(ctx, infoV2); err != nil {
		t.Fatalf("StateMirrored.UpsertTableSchema v2: %v", err)
	}
	assertMirroredTableSchema(t, ctx, mirror, keyV2, infoV2)
}

func assertMirroredTableSchema(
	t *testing.T,
	ctx context.Context,
	mirror statemirror.Mirror,
	key string,
	want TableSchemaInfo,
) {
	t.Helper()
	raw, ok, err := mirror.Get(ctx, statemirror.MappingTableSchemas, key)
	if err != nil {
		t.Fatalf("mirror.Get(%q): %v", key, err)
	}
	if !ok {
		t.Fatalf("mirror.Get(%q) missed", key)
	}
	var got TableSchemaInfo
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("decode mirror value for %q: %v", key, err)
	}
	if got != want {
		t.Fatalf("mirror value for %q = %+v, want %+v", key, got, want)
	}
}
