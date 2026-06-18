package aptos

import (
	"context"

	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentFunction struct {
	controller.BaseHandlerAgent

	Filter      aptos.TransactionFilter
	FetchConfig aptos.TransactionFetchConfig
}

func (a HandlerAgentFunction) BuildBindingDataList(
	_ context.Context,
	bd *BlockData,
) ([]standard.BindingDataInner, error) {
	if bd.mainData.Txn == nil || !a.Filter.Check(*bd.mainData.Txn) {
		return nil, nil
	}
	rawTxn, err := bd.getRawTxn(a.FetchConfig, a.Filter.EventFilters)
	if err != nil {
		return nil, err
	}
	return []standard.BindingDataInner{{
		HandlerType: protos.HandlerType_APT_CALL,
		Data: &protos.Data{
			Value: &protos.Data_AptCall_{
				AptCall: &protos.Data_AptCall{
					RawTransaction: rawTxn,
				},
			},
		},
		DataSize: len(rawTxn),
	}}, nil
}

func (a HandlerAgentFunction) Snapshot() any {
	return map[string]any{
		"HandlerID":   a.HandlerID,
		"Range":       a.Range.String(),
		"Filter":      a.Filter,
		"FetchConfig": a.FetchConfig,
	}
}
