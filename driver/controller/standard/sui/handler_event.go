package sui

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentEvent struct {
	controller.BaseHandlerAgent

	Filter      sui.TransactionFilter
	FetchConfig sui.TransactionFetchConfig
}

func (a HandlerAgentEvent) BuildBindingDataList(
	_ context.Context,
	bd *BlockData,
) (result []standard.BindingDataInner, err error) {
	for txIndex, tx := range bd.mainData.Txs {
		if !a.Filter.Check(tx) {
			continue
		}
		var rawTxn string
		if rawTxn, err = bd.getTxn(txIndex, a.Filter.EventFilters, a.FetchConfig); err != nil {
			return nil, err
		}
		eventChecker := sui.BuildEventChecker(a.Filter.EventFilters)
		for _, ev := range tx.Events {
			if !eventChecker(ev) {
				continue
			}
			// tx.Events is the filtered/pruned event list, so its slice position is
			// not the event's real index — use the on-chain id.eventSeq instead.
			eventSeq := ev.ID.EventSeq.Uint64()
			var rawEvent []byte
			if rawEvent, err = json.Marshal(ev); err != nil {
				return nil, errors.Wrapf(err, "marshal sui event #%d in tx %d/%s in block %d failed",
					eventSeq, tx.TransactionPosition, tx.Digest.String(), bd.GetBlockNumber())
			}
			result = append(result, standard.BindingDataInner{
				HandlerType:  protos.HandlerType_SUI_EVENT,
				TxIndex:      txIndex,
				TxInnerIndex: int(eventSeq),
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

func (a HandlerAgentEvent) Snapshot() any {
	return map[string]any{
		"HandlerID":   a.HandlerID,
		"Range":       a.Range.String(),
		"Filter":      a.Filter,
		"FetchConfig": a.FetchConfig,
	}
}
