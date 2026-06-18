package subgraph

import (
	"context"

	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data/evm"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/subgraph/manifest"
)

type HandlerAgentEvent struct {
	controller.BaseHandlerAgent
	DataSource *manifest.DataSource

	Filter evm.LogFilter
}

func (a HandlerAgentEvent) GetExtendRequirements(_ context.Context, bd *BlockData) (evm.BlockExtendRequirement, error) {
	txHashSet := set.New[string]()
	checker := a.Filter.BuildChecker(nil, nil)
	for _, log := range bd.mainData.Logs {
		if ok, _ := checker(log); ok {
			txHashSet.Add(log.TxHash.String())
		}
	}
	txHashes := txHashSet.DumpValues()
	return evm.BlockExtendRequirement{
		SpecialTransactions:           txHashes,
		SpecialTransactionReceipts:    txHashes,
		SpecialTransactionReceiptLogs: txHashes,
	}, nil
}

func (a HandlerAgentEvent) BuildTaskDataList(_ context.Context, bd *BlockData) ([]taskData, error) {
	eventAbi := a.DataSource.Mapping.EventHandlers[a.HandlerID.ID].GetABI()
	var r []taskData
	checker := a.Filter.BuildChecker(nil, nil)
	for _, log := range bd.mainData.Logs {
		if ok, _ := checker(log); !ok {
			continue
		}
		if succeed, err := bd.transactionSucceed(log.TxHash.String()); err != nil {
			return nil, fetcher.Permanent(err)
		} else if !succeed {
			continue // tx failed
		}
		event, size, err := bd.buildEvent(log, eventAbi)
		if err != nil {
			return nil, fetcher.Permanent(err)
		}
		r = append(r, taskData{
			callHandlerParam: event,
			dataSource:       a.DataSource,
			handlerID:        a.HandlerID,
			txIndex:          int(log.TxIndex),
			logIndex:         int(log.Index),
			size:             size,
		})
	}
	return r, nil
}

func (a HandlerAgentEvent) Snapshot() any {
	return map[string]any{
		"HandlerID": a.HandlerID,
		"Range":     a.Range.String(),
		"Filter":    a.Filter,
	}
}
