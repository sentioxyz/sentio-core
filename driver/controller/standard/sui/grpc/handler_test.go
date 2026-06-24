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
	// grpc events carry no on-chain sequence, so the binding reports the index in
	// the EventSeq proto field (not stuffed into raw_event).
	assert.EqualValues(t, 0, se.GetEventSeq())
	assert.NotContains(t, ev, "eventSeq")

	// raw_transaction is the flattened grpc ExtendedGrpcTransaction shape: the embedded
	// ExecutedTransaction fields at the top level, plus ext*-prefixed header fields.
	var tx map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(se.GetRawTransaction()), &tx))
	assert.Contains(t, tx, "extCheckpoint")
	assert.Contains(t, tx, "digest")
	assert.NotContains(t, tx, "executedTransaction")
}

func TestEventHandlerGrpcEventSeq(t *testing.T) {
	mkEvent := func(mod string) *rpcv2.Event {
		return &rpcv2.Event{
			PackageId: proto.String("0x0000000000000000000000000000000000000000000000000000000000000002"),
			Module:    proto.String(mod),
			EventType: proto.String("0x2::m::E"),
			Sender:    proto.String("0xabc"),
		}
	}
	tx := &chainsui.ExtendedGrpcTransaction{
		ExecutedTransaction: &rpcv2.ExecutedTransaction{
			Digest:  proto.String("tx1"),
			Effects: &rpcv2.TransactionEffects{Status: &rpcv2.ExecutionStatus{Success: proto.Bool(true)}},
			Events:  &rpcv2.TransactionEvents{Events: []*rpcv2.Event{mkEvent("m0"), mkEvent("m1")}},
		},
	}
	agent := HandlerAgentEvent{suihandler.HandlerAgentEvent{
		Filter:      matchAnyFilter(),
		FetchConfig: chainsui.TransactionFetchConfig{NeedAllEvents: true},
	}}
	result, err := agent.BuildBindingDataList(context.Background(), newBlockData(tx))
	require.NoError(t, err)
	require.Len(t, result, 2)

	// each event's index within the tx is reported via EventSeq and used as the
	// binding's TxInnerIndex.
	for i, r := range result {
		assert.Equal(t, i, r.TxInnerIndex)
		assert.EqualValues(t, i, r.Data.GetSuiEvent().GetEventSeq())
	}
}

func TestEventHandlerGrpcEventSeqPruned(t *testing.T) {
	mkEvent := func(sender string) *rpcv2.Event {
		return &rpcv2.Event{
			PackageId: proto.String("0x0000000000000000000000000000000000000000000000000000000000000002"),
			Module:    proto.String("m"),
			EventType: proto.String("0x2::m::E"),
			Sender:    proto.String(sender),
		}
	}
	full := &chainsui.ExtendedGrpcTransaction{
		ExecutedTransaction: &rpcv2.ExecutedTransaction{
			Digest:  proto.String("tx1"),
			Effects: &rpcv2.TransactionEffects{Status: &rpcv2.ExecutionStatus{Success: proto.Bool(true)}},
			Events: &rpcv2.TransactionEvents{Events: []*rpcv2.Event{
				mkEvent("s0"), mkEvent("keep"), mkEvent("keep"),
			}},
		},
	}
	// allEvents=false: only the matching events survive, and PruneGrpcTransaction
	// records their original on-chain indices.
	keep := "keep"
	fc := chainsui.TransactionFetchConfig{NeedAllEvents: false}
	filters := []chainsui.EventFilterV2{{Sender: &keep}}
	pruned := fc.PruneGrpcTransaction(full, filters)
	require.Equal(t, []int{1, 2}, pruned.EventIndexes)
	require.Len(t, pruned.GetEvents().GetEvents(), 2)

	agent := HandlerAgentEvent{suihandler.HandlerAgentEvent{
		Filter:      chainsui.TransactionFilter{EventFilters: filters},
		FetchConfig: fc,
	}}
	result, err := agent.BuildBindingDataList(context.Background(), newBlockData(pruned))
	require.NoError(t, err)
	require.Len(t, result, 2)

	// the binding reports the original on-chain index (1, 2) via EventSeq, not the
	// pruned slice position (0, 1).
	for i, wantSeq := range []int{1, 2} {
		assert.Equal(t, wantSeq, result[i].TxInnerIndex)
		assert.EqualValues(t, wantSeq, result[i].Data.GetSuiEvent().GetEventSeq())
	}
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

	// raw_changes[0] is the flattened ExtendedGrpcChangedObject shape: the embedded
	// ChangedObject fields at the top level, plus ext*-prefixed header fields.
	var change map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(soc.GetRawChanges()[0]), &change))
	assert.Contains(t, change, "extCheckpoint")
	assert.Contains(t, change, "objectId")
	assert.NotContains(t, change, "changedObject")
}
