package grpc

import (
	"context"

	chainsui "sentioxyz/sentio-core/chain/sui"
	cprotojson "sentioxyz/sentio-core/common/protojson"
	"sentioxyz/sentio-core/driver/controller/standard"
	suihandler "sentioxyz/sentio-core/driver/controller/standard/sui"
	"sentioxyz/sentio-core/processor/protos"

	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// HandlerAgentEvent embeds the json-rpc event agent (reusing its Filter /
// FetchConfig / BaseHandlerAgent / Snapshot) and only overrides binding building
// to read grpc transactions and emit grpc-format raw_event / raw_transaction.
type HandlerAgentEvent struct {
	suihandler.HandlerAgentEvent
}

func (a HandlerAgentEvent) BuildBindingDataList(
	_ context.Context,
	bd *BlockData,
) (result []standard.BindingDataInner, err error) {
	for txIndex, tx := range bd.mainData.Txs {
		if !a.Filter.CheckGrpcTx(tx.ExecutedTransaction) {
			continue
		}
		var rawTxn string
		if rawTxn, err = bd.getTxn(txIndex, a.Filter.EventFilters, a.FetchConfig); err != nil {
			return nil, err
		}
		eventChecker := chainsui.BuildGrpcEventChecker(a.Filter.EventFilters)
		for evIndex, ev := range tx.GetEvents().GetEvents() {
			if !eventChecker(ev) {
				continue
			}
			var rawEvent []byte
			if rawEvent, err = cprotojson.Marshal(ev); err != nil {
				return nil, errors.Wrapf(err, "marshal grpc sui event #%d in tx %d in block %d failed",
					evIndex, txIndex, bd.GetBlockNumber())
			}
			// grpc events carry no on-chain sequence, so attach the event's index
			// within the transaction. The sui event fetch config defaults to
			// allEvents=true, so the events here are the full (unpruned) list and
			// evIndex is the true on-chain index. The SDK reads this `eventSeq` to
			// populate meta.log_index (mirrors the json-rpc event's id.eventSeq).
			if rawEvent, err = sjson.SetBytes(rawEvent, "eventSeq", evIndex); err != nil {
				return nil, errors.Wrapf(err, "set eventSeq for grpc sui event #%d in tx %d in block %d failed",
					evIndex, txIndex, bd.GetBlockNumber())
			}
			result = append(result, standard.BindingDataInner{
				HandlerType:  protos.HandlerType_SUI_EVENT,
				TxIndex:      txIndex,
				TxInnerIndex: evIndex,
				Data: &protos.Data{
					Value: &protos.Data_SuiEvent_{
						SuiEvent: &protos.Data_SuiEvent{
							RawEvent:       string(rawEvent),
							RawTransaction: rawTxn,
							Timestamp:      timestamppb.New(bd.GetBlockTime()),
							Slot:           bd.GetBlockNumber(),
						},
					},
				},
				DataSize: len(rawEvent) + len(rawTxn),
			})
		}
	}
	return
}
