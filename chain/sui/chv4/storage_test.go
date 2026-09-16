package chv4

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/objectx"
)

// Exercise the real storage scanner with repeated physical rows. MergeTree's
// ordering key does not prevent an ingestion retry from inserting them twice.
type objectChangeTestConn struct {
	chx.Conn
	objects []Object
}

func (*objectChangeTestConn) GetCluster() string  { return "" }
func (*objectChangeTestConn) GetDatabase() string { return "test" }
func (c *objectChangeTestConn) Query(context.Context, string, ...any) (driver.Rows, error) {
	return &objectChangeTestRows{objects: c.objects}, nil
}

type objectChangeTestRows struct {
	driver.Rows
	objects []Object
	next    int
}

func (r *objectChangeTestRows) Next() bool { return r.next < len(r.objects) }
func (*objectChangeTestRows) Close() error { return nil }
func (r *objectChangeTestRows) Scan(dest ...any) error {
	filter := objectx.HasTag("clickhouse").And(objectx.NoTag("clickhouse", "json")).
		And(objectx.NoTag("clickhouse", "package"))
	values := objectx.CollectFieldValues(r.objects[r.next], filter)
	for i := range dest {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(values[i]))
	}
	r.next++
	return nil
}

func queryObjectChangeTestRows(t *testing.T, objects []Object, limit int) ([]*sui.ExtendedGrpcChangedObject, error) {
	t.Helper()
	storage := NewStorage(chx.New(&objectChangeTestConn{objects: objects}), nil)
	return storage.queryObjectChanges(context.Background(), func(*sui.ExtendedGrpcChangedObject) bool {
		return true
	}, limit, "checkpoint = ?", uint64(314319388))
}

func objectChangeTestRow() Object {
	return Object{
		Checkpoint: 314319388, CheckpointDigest: "checkpoint", Timestamp: time.UnixMilli(1787556272284),
		TransactionIndex: TransactionIndex{TxIndex: 1, TxDigest: "transaction"},
		ChangeType:       "mutated", ObjectID: "0x1", ObjectVersion: 976007682, ObjectDigest: "output",
		OwnerKind: "OBJECT", OwnerAddress: "0x2", PreObjectVersion: 962311257, PreDigest: "input",
		PreOwnerKind: "OBJECT", PreOwnerAddress: "0x2", ObjectType: "0x3::obligation::Obligation",
	}
}

func TestQueryObjectChangesDeduplicatesPhysicalRows(t *testing.T) {
	first := objectChangeTestRow()
	other := first
	other.ObjectID = "0x4"
	later := first
	later.TxIndex++
	later.TxDigest = "later-transaction"
	later.ObjectVersion++
	// Copies of a row need not be adjacent. A later transaction changing the
	// same object in the same checkpoint remains a separate change.
	rows, err := queryObjectChangeTestRows(t, []Object{first, other, first, other, later}, 0)
	assert.NoError(t, err)
	assert.Equal(t, []*sui.ExtendedGrpcChangedObject{
		first.ToChangedObject(), other.ToChangedObject(), later.ToChangedObject(),
	}, rows)
}

func TestQueryObjectChangesRejectsConflictingDuplicates(t *testing.T) {
	for _, field := range []string{"owner", "version", "transaction", "checkpoint"} {
		t.Run(field, func(t *testing.T) {
			first := objectChangeTestRow()
			conflict := first
			switch field {
			case "owner":
				conflict.OwnerAddress = "0x5"
			case "version":
				conflict.ObjectVersion++
			case "transaction":
				conflict.TxDigest = "different-transaction"
			case "checkpoint":
				conflict.CheckpointDigest = "different-checkpoint"
			}
			_, err := queryObjectChangeTestRows(t, []Object{first, conflict}, 0)
			assert.ErrorContains(t, err, "conflicting object changes")
		})
	}
}

func TestQueryObjectChangesDuplicatesStillCountTowardScanLimit(t *testing.T) {
	first := objectChangeTestRow()
	_, err := queryObjectChangeTestRows(t, []Object{first, first}, 2)
	assert.Error(t, err, "deduplication must not hide an incomplete raw-row scan")
}
