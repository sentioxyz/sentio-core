package aptos

import (
	"context"

	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentEvent struct {
	controller.BaseHandlerAgent

	Filter      aptos.TransactionFilter
	FetchConfig aptos.TransactionFetchConfig
}

func (a HandlerAgentEvent) BuildBindingDataList(
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
	var result []standard.BindingDataInner
	eventChecker := aptos.BuildEventFilter(a.Filter.EventFilters)
	for i, ev := range bd.mainData.Txn.Events {
		if !eventChecker(ev) {
			continue
		}
		var rawEvent string
		if rawEvent, err = bd.getRawEvent(i); err != nil {
			return nil, err
		}
		result = append(result, standard.BindingDataInner{
			HandlerType:  protos.HandlerType_APT_EVENT,
			TxInnerIndex: int(ev.Index),
			Data: &protos.Data{
				Value: &protos.Data_AptEvent_{
					AptEvent: &protos.Data_AptEvent{
						EventIndex:     ev.Index,
						RawEvent:       rawEvent,
						RawTransaction: rawTxn,
					},
				},
			},
			DataSize: len(rawTxn) + len(rawEvent),
		})
	}
	return result, nil
}

func (a HandlerAgentEvent) Snapshot() any {
	return map[string]any{
		"HandlerID":   a.HandlerID,
		"Range":       a.Range.String(),
		"Filter":      a.Filter,
		"FetchConfig": a.FetchConfig,
	}
}
