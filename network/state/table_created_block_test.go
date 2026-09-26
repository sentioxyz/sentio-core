package state

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"sentioxyz/sentio-core/common/statemirror"
)

func TestTableInfoCreatedBlockOverwrittenOnRecreate(t *testing.T) {
	ctx := context.Background()
	state := &PlainState{Databases: map[string]DatabaseInfo{"db_1": {DatabaseId: "db_1"}}}
	if err := state.UpsertDatabaseTable(ctx, "db_1", TableInfo{TableId: "t", TableType: "user", CreatedBlock: 100}); err != nil {
		t.Fatalf("UpsertDatabaseTable create: %v", err)
	}
	if err := state.DeleteDatabaseTable(ctx, "db_1", "t"); err != nil {
		t.Fatalf("DeleteDatabaseTable: %v", err)
	}
	if err := state.UpsertDatabaseTable(ctx, "db_1", TableInfo{TableId: "t", TableType: "user", CreatedBlock: 250}); err != nil {
		t.Fatalf("UpsertDatabaseTable recreate: %v", err)
	}
	tables := state.Databases["db_1"].Tables
	if len(tables) != 1 || tables[0].CreatedBlock != 250 {
		t.Fatalf("tables after recreate = %+v, want one table created at block 250", tables)
	}
}

func TestTableInfoCreatedBlockIsMirrored(t *testing.T) {
	ctx := context.Background()
	mirror, err := statemirror.NewFileMirror(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileMirror: %v", err)
	}
	state, err := NewStateMirrored(ctx, &PlainState{Databases: map[string]DatabaseInfo{"db_1": {DatabaseId: "db_1", IndexerId: 7}}}, mirror)
	if err != nil {
		t.Fatalf("NewStateMirrored: %v", err)
	}
	if err := state.UpsertDatabaseTable(ctx, "db_1", TableInfo{TableId: "t", TableType: "user", CreatedBlock: 4242}); err != nil {
		t.Fatalf("UpsertDatabaseTable: %v", err)
	}
	raw, ok, err := mirror.Get(ctx, statemirror.MappingDatabases, "db_1")
	if err != nil || !ok {
		t.Fatalf("mirror.Get(db_1) = ok %v, err %v", ok, err)
	}
	var got DatabaseInfo
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("decode mirrored database: %v", err)
	}
	if len(got.Tables) != 1 || got.Tables[0].CreatedBlock != 4242 {
		t.Fatalf("mirrored tables = %+v, want createdBlock 4242", got.Tables)
	}
}

func TestTableInfoWithoutCreatedBlockDecodesAsZero(t *testing.T) {
	// A mirror or store row written before the upgrade has no createdBlock
	// field; zero means "created before the upgrade" (design E5).
	var got DatabaseInfo
	if err := json.Unmarshal([]byte(`{"databaseId":"db_1","indexerId":7,"tables":[{"tableId":"t","tableType":"user"}]}`), &got); err != nil {
		t.Fatalf("decode legacy database: %v", err)
	}
	if got.Tables[0].CreatedBlock != 0 {
		t.Fatalf("legacy createdBlock = %d, want 0", got.Tables[0].CreatedBlock)
	}
	encoded, err := json.Marshal(TableInfo{TableId: "t", TableType: "user"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if string(encoded) != `{"tableId":"t","tableType":"user"}` {
		t.Fatalf("zero createdBlock must be omitted to keep legacy bytes, got %s", encoded)
	}
}

func TestFileStoreTableCreatedBlockRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := NewFileStore(filepath.Join(t.TempDir(), "state.yaml"))
	want := &PlainState{Databases: map[string]DatabaseInfo{
		"db_1": {DatabaseId: "db_1", Tables: []TableInfo{{TableId: "t", TableType: "user", CreatedBlock: 99}}},
	}}
	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("FileStore.Save: %v", err)
	}
	got, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("FileStore.Load: %v", err)
	}
	if block := got.Databases["db_1"].Tables[0].CreatedBlock; block != 99 {
		t.Fatalf("loaded createdBlock = %d, want 99", block)
	}
}
