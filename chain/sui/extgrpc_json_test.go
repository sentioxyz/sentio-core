package sui

import (
	"encoding/json"
	"strings"
	"testing"

	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	// flattened: the embedded ChangedObject fields sit at the top level, no
	// "changedObject" wrapper key.
	assert.NotContains(t, s, `"changedObject":`)
	assert.Contains(t, s, `"objectId":`)
	// enum as string name, not number
	assert.Contains(t, s, "CREATED")
	assert.NotContains(t, s, `"idOperation":1`)
	// header under ext* keys (no collision with proto field names)
	assert.Contains(t, s, `"extTxDigest":"5TLHCn2S"`)

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

func TestExtendedGrpcTransactionJSONFlatten(t *testing.T) {
	digest := "5TLHCn2S"
	tx := &ExtendedGrpcTransaction{
		Checkpoint:          285612737,
		CheckpointDigest:    "abc",
		TimestampMs:         1781106452163,
		Epoch:               1154,
		TxIndex:             3,
		ExecutedTransaction: &rpcv2.ExecutedTransaction{Digest: &digest},
	}
	b, err := json.Marshal(tx)
	require.NoError(t, err)
	s := string(b)
	t.Logf("json: %s", s)
	// flattened: the embedded ExecutedTransaction fields sit at the top level,
	// no "executedTransaction" wrapper key.
	assert.Contains(t, s, `"digest":"5TLHCn2S"`)
	assert.NotContains(t, s, `"executedTransaction"`)
	// header under ext* keys (no collision with the proto's own "checkpoint")
	assert.Contains(t, s, `"extCheckpoint":285612737`)
	assert.Contains(t, s, `"extTxIndex":3`)

	// round-trip
	var rt ExtendedGrpcTransaction
	require.NoError(t, json.Unmarshal(b, &rt))
	assert.Equal(t, tx.Checkpoint, rt.Checkpoint)
	assert.Equal(t, tx.CheckpointDigest, rt.CheckpointDigest)
	assert.Equal(t, tx.TimestampMs, rt.TimestampMs)
	assert.Equal(t, tx.Epoch, rt.Epoch)
	assert.Equal(t, tx.TxIndex, rt.TxIndex)
	require.NotNil(t, rt.ExecutedTransaction)
	assert.Equal(t, digest, rt.ExecutedTransaction.GetDigest())
	b2, err := json.Marshal(&rt)
	require.NoError(t, err)
	assert.JSONEq(t, s, string(b2))
}

func TestExtendedGrpcTransactionGetEventSeq(t *testing.T) {
	// no EventIndexes (full event list): the slice position is the on-chain index.
	full := &ExtendedGrpcTransaction{}
	assert.Equal(t, 0, full.GetEventSeq(0))
	assert.Equal(t, 3, full.GetEventSeq(3))

	// filtered: EventIndexes maps each slice position to its original on-chain index.
	pruned := &ExtendedGrpcTransaction{EventIndexes: []int{1, 2, 5}}
	assert.Equal(t, 1, pruned.GetEventSeq(0))
	assert.Equal(t, 2, pruned.GetEventSeq(1))
	assert.Equal(t, 5, pruned.GetEventSeq(2))
	// out of range falls back to the slice position (defensive).
	assert.Equal(t, 9, pruned.GetEventSeq(9))
}

func TestExtendedGrpcTransactionJSONEventIndexes(t *testing.T) {
	digest := "tx1"
	// EventIndexes must survive JSON: the super node prunes (setting it) before
	// serializing the tx back to the driver, where the handler reads it.
	tx := &ExtendedGrpcTransaction{
		Checkpoint:          7,
		EventIndexes:        []int{1, 2},
		ExecutedTransaction: &rpcv2.ExecutedTransaction{Digest: &digest},
	}
	b, err := json.Marshal(tx)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"extEventIndexes":[1,2]`)

	var rt ExtendedGrpcTransaction
	require.NoError(t, json.Unmarshal(b, &rt))
	assert.Equal(t, []int{1, 2}, rt.EventIndexes)

	// nil EventIndexes (full event list) is omitted from the wire form.
	full := &ExtendedGrpcTransaction{Checkpoint: 7, ExecutedTransaction: &rpcv2.ExecutedTransaction{Digest: &digest}}
	fb, err := json.Marshal(full)
	require.NoError(t, err)
	assert.NotContains(t, string(fb), "extEventIndexes")
	var frt ExtendedGrpcTransaction
	require.NoError(t, json.Unmarshal(fb, &frt))
	assert.Nil(t, frt.EventIndexes)
}
