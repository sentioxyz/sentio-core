package state

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestPostgresStoreTableSchemasRoundTripReplaceAndStateKeyIsolation(t *testing.T) {
	dsn := os.Getenv("SENTIO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SENTIO_TEST_POSTGRES_DSN to run the PostgreSQL store integration test")
	}

	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	storeA, err := NewPostgresStore(dsn, "table-schemas-a-"+suffix)
	if err != nil {
		t.Fatalf("NewPostgresStore A: %v", err)
	}
	t.Cleanup(func() { _ = storeA.Close() })
	storeB := &PostgresStore{db: storeA.db, stateKey: "table-schemas-b-" + suffix}

	key := TableSchemaKey("orders", "fills", 1)
	removedKey := TableSchemaKey("orders", "trades", 1)
	first := &PlainState{
		LastBlock: 11,
		TableSchemas: map[string]TableSchemaInfo{
			key: {
				DatabaseId: "orders",
				TableId:    "fills",
				Version:    1,
				SchemaHash: "0x01",
				SchemaJson: `{"table_id":"orders.fills","columns":[]}`,
			},
			removedKey: {
				DatabaseId: "orders",
				TableId:    "trades",
				Version:    1,
				SchemaHash: "0x02",
				SchemaJson: `{"table_id":"orders.trades","columns":[]}`,
			},
		},
	}
	if err := storeA.Save(ctx, first); err != nil {
		t.Fatalf("Save first state: %v", err)
	}
	assertPostgresTableSchemas(t, ctx, storeA, first.LastBlock, first.TableSchemas)

	replacement := &PlainState{
		LastBlock: 12,
		TableSchemas: map[string]TableSchemaInfo{
			key: {
				DatabaseId: "orders",
				TableId:    "fills",
				Version:    1,
				SchemaHash: "0x03",
				SchemaJson: `{"table_id":"orders.fills","partition_by":"day","columns":[]}`,
			},
		},
	}
	if err := storeA.Save(ctx, replacement); err != nil {
		t.Fatalf("Save replacement state: %v", err)
	}
	assertPostgresTableSchemas(t, ctx, storeA, replacement.LastBlock, replacement.TableSchemas)

	isolated := &PlainState{
		LastBlock: 21,
		TableSchemas: map[string]TableSchemaInfo{
			key: {
				DatabaseId: "orders",
				TableId:    "fills",
				Version:    1,
				SchemaHash: "0x99",
				SchemaJson: `{"table_id":"other.orders.fills","columns":[]}`,
			},
		},
	}
	if err := storeB.Save(ctx, isolated); err != nil {
		t.Fatalf("Save isolated state: %v", err)
	}
	assertPostgresTableSchemas(t, ctx, storeB, isolated.LastBlock, isolated.TableSchemas)
	assertPostgresTableSchemas(t, ctx, storeA, replacement.LastBlock, replacement.TableSchemas)
}

func assertPostgresTableSchemas(
	t *testing.T,
	ctx context.Context,
	store *PostgresStore,
	wantLastBlock uint64,
	want map[string]TableSchemaInfo,
) {
	t.Helper()
	got, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.LastBlock != wantLastBlock {
		t.Fatalf("LastBlock = %d, want %d", got.LastBlock, wantLastBlock)
	}
	if len(got.TableSchemas) != len(want) {
		t.Fatalf("TableSchemas length = %d, want %d: %+v", len(got.TableSchemas), len(want), got.TableSchemas)
	}
	for key, wantInfo := range want {
		if gotInfo, ok := got.TableSchemas[key]; !ok || gotInfo != wantInfo {
			t.Fatalf("TableSchemas[%q] = %+v, %v; want %+v", key, gotInfo, ok, wantInfo)
		}
	}
}
