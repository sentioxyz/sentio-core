package sui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
)

func TestExtendedGrpcChangedObjectJSON(t *testing.T) {
	op := rpcv2.ChangedObject_CREATED
	oid := "0x0000000000000000000000000000000000000000000000000000000000000006"
	obj := &ExtendedGrpcChangedObject{
		Checkpoint:       285612737,
		CheckpointDigest: "abc",
		TimestampMs:      1781106452163,
		Epoch:            1154,
		TxIndex:          3,
		TxDigest:         "5TLHCn2S",
		ChangedObject:    &rpcv2.ChangedObject{ObjectId: &oid, IdOperation: &op},
	}
	b, err := json.Marshal(obj)
	require.NoError(t, err)
	s := string(b)
	t.Logf("json: %s", s)
	// nested, not flattened
	assert.Contains(t, s, `"changedObject":`)
	// enum as string name, not number
	assert.Contains(t, s, "CREATED")
	assert.NotContains(t, s, `"idOperation":1`)
	// header present
	assert.Contains(t, s, `"txDigest":"5TLHCn2S"`)

	// round-trip
	var rt ExtendedGrpcChangedObject
	require.NoError(t, json.Unmarshal(b, &rt))
	assert.Equal(t, obj.Checkpoint, rt.Checkpoint)
	assert.Equal(t, obj.TxDigest, rt.TxDigest)
	require.NotNil(t, rt.ChangedObject)
	assert.Equal(t, oid, rt.ChangedObject.GetObjectId())
	assert.Equal(t, op, rt.ChangedObject.GetIdOperation())
	b2, err := json.Marshal(&rt)
	require.NoError(t, err)
	assert.JSONEq(t, s, string(b2))
	_ = strings.TrimSpace
}

func TestExtendedGrpcTransactionJSONNil(t *testing.T) {
	// nil embedded proto must not panic and round-trips
	tx := &ExtendedGrpcTransaction{Checkpoint: 1, Epoch: 2}
	b, err := json.Marshal(tx)
	require.NoError(t, err)
	var rt ExtendedGrpcTransaction
	require.NoError(t, json.Unmarshal(b, &rt))
	assert.Equal(t, uint64(1), rt.Checkpoint)
	assert.Nil(t, rt.ExecutedTransaction)
}
