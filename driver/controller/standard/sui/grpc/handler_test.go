package grpc

import (
	"context"
	"encoding/json"
	"testing"

	chainsui "sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/common/set"
	suidata "sentioxyz/sentio-core/driver/controller/data/sui"
	suigrpcdata "sentioxyz/sentio-core/driver/controller/data/sui/grpc"
	suihandler "sentioxyz/sentio-core/driver/controller/standard/sui"

	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func grpcTxWithEvent() *chainsui.ExtendedGrpcTransaction {
	return &chainsui.ExtendedGrpcTransaction{
		Checkpoint:       100,
		CheckpointDigest: "ckpt",
		TimestampMs:      1700000000000,
		Epoch:            5,
		TxIndex:          0,
		ExecutedTransaction: &rpcv2.ExecutedTransaction{
			Digest:  proto.String("tx1"),
			Effects: &rpcv2.TransactionEffects{Status: &rpcv2.ExecutionStatus{Success: proto.Bool(true)}},
			Events: &rpcv2.TransactionEvents{Events: []*rpcv2.Event{{
				PackageId: proto.String("0x0000000000000000000000000000000000000000000000000000000000000002"),
				Module:    proto.String("m"),
				EventType: proto.String("0x2::m::E"),
				Sender:    proto.String("0xabc"),
			}}},
		},
	}
}

func newBlockData(txs ...*chainsui.ExtendedGrpcTransaction) *BlockData {
	bd := &BlockData{mainData: suigrpcdata.BlockMainData{Txs: txs}}
	bd.BlockHeader = suidata.SimpleBlock{Checkpoint: 100, Digest: "ckpt", TimestampMS: 1700000000000}
	return bd
}

// matchAnyFilter passes the tx-level CheckGrpcTx (an empty EventFilterV2 matches any event of a
// successful tx) so the binding-serialization is what's under test, not the filtering.
func matchAnyFilter() chainsui.TransactionFilter {
	return chainsui.TransactionFilter{EventFilters: []chainsui.EventFilterV2{{}}}
}

func TestEventHandlerGrpcBinding(t *testing.T) {
	agent := HandlerAgentEvent{suihandler.HandlerAgentEvent{
		Filter:      matchAnyFilter(),
		FetchConfig: chainsui.TransactionFetchConfig{NeedAllEvents: true},
	}}
	result, err := agent.BuildBindingDataList(context.Background(), newBlockData(grpcTxWithEvent()))
	require.NoError(t, err)
	require.Len(t, result, 1)

	se := result[0].Data.GetSuiEvent()
	require.NotNil(t, se)
	assert.Equal(t, uint64(100), se.GetSlot())

	// raw_event is protojson of rpcv2.Event (enum/field names in camelCase, not numbers).
	var ev map[string]any
	require.NoError(t, json.Unmarshal([]byte(se.GetRawEvent()), &ev))
	assert.Equal(t, "0x2::m::E", ev["eventType"])
	assert.Equal(t, "m", ev["module"])

	// raw_transaction is the flattened grpc ExtendedGrpcTransaction shape: the embedded
	// ExecutedTransaction fields at the top level, plus ext*-prefixed header fields.
	var tx map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(se.GetRawTransaction()), &tx))
	assert.Contains(t, tx, "extCheckpoint")
	assert.Contains(t, tx, "digest")
	assert.NotContains(t, tx, "executedTransaction")
}

func TestFunctionHandlerGrpcBinding(t *testing.T) {
	agent := HandlerAgentFunction{suihandler.HandlerAgentFunction{
		Filter:      matchAnyFilter(),
		FetchConfig: chainsui.TransactionFetchConfig{NeedAllEvents: true},
	}}
	result, err := agent.BuildBindingDataList(context.Background(), newBlockData(grpcTxWithEvent()))
	require.NoError(t, err)
	require.Len(t, result, 1)

	sc := result[0].Data.GetSuiCall()
	require.NotNil(t, sc)
	var tx map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(sc.GetRawTransaction()), &tx))
	assert.Contains(t, tx, "extCheckpoint")
	assert.Contains(t, tx, "digest")
	assert.NotContains(t, tx, "executedTransaction")
}

func TestChangeHandlerGrpcBinding(t *testing.T) {
	const objectID = "0x0000000000000000000000000000000000000000000000000000000000000abc"
	oc := &chainsui.ExtendedGrpcChangedObject{
		Checkpoint: 100,
		TxIndex:    3,
		TxDigest:   "txd",
		ChangedObject: &rpcv2.ChangedObject{
			ObjectId:   proto.String(objectID),
			ObjectType: proto.String("0x2::coin::Coin"),
		},
	}
	agent := HandlerAgentChange{suihandler.HandlerAgentChange{
		Filter: chainsui.ObjectChangeFilter{ObjectIDIn: set.New[string](objectID)},
	}}
	bd := newBlockData()
	bd.mainData.ObjectChanges = []*chainsui.ExtendedGrpcChangedObject{oc}

	result, err := agent.BuildBindingDataList(context.Background(), bd)
	require.NoError(t, err)
	require.Len(t, result, 1)

	soc := result[0].Data.GetSuiObjectChange()
	require.NotNil(t, soc)
	assert.Equal(t, "txd", soc.GetTxDigest())
	assert.Equal(t, 3, result[0].TxIndex)
	require.Len(t, soc.GetRawChanges(), 1)

	// raw_changes[0] is the ExtendedGrpcChangedObject shape: header fields + nested changedObject.
	var change map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(soc.GetRawChanges()[0]), &change))
	assert.Contains(t, change, "checkpoint")
	assert.Contains(t, change, "changedObject")
}
