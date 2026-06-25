package sui

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentFunction struct {
	controller.BaseHandlerAgent

	Filter      sui.TransactionFilter
	FetchConfig sui.TransactionFetchConfig
}

func (a HandlerAgentFunction) BuildBindingDataList(
	_ context.Context,
	bd *BlockData,
) (result []standard.BindingDataInner, err error) {
	for txIndex, tx := range bd.mainData.Txs {
		if !a.Filter.Check(tx) {
			continue
		}
		var rawTxn string
		if rawTxn, err = bd.getTxn(txIndex, nil, a.FetchConfig); err != nil {
			return nil, err
		}
		result = append(result, standard.BindingDataInner{
			// On-chain checkpoint position, not the mainData.Txs slice index — see
			// the note in handler_event.go's BuildBindingDataList.
			TxIndex:     tx.TransactionPosition,
			HandlerType: protos.HandlerType_SUI_CALL,
			Data: &protos.Data{
				Value: &protos.Data_SuiCall_{
					SuiCall: &protos.Data_SuiCall{
						RawTransaction: rawTxn,
						Timestamp:      timestamppb.New(bd.GetBlockTime()),
						Slot:           tx.Checkpoint.Uint64(),
					},
				},
			},
			DataSize: len(rawTxn),
		})
	}
	return
}

func (a HandlerAgentFunction) Snapshot() any {
	return map[string]any{
		"HandlerID":   a.HandlerID,
		"Range":       a.Range.String(),
		"Filter":      a.Filter,
		"FetchConfig": a.FetchConfig,
	}
}
