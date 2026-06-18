package subgraph

import (
	"context"

	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data/evm"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/subgraph/manifest"
)

type HandlerAgentCall struct {
	controller.BaseHandlerAgent
	DataSource *manifest.DataSource

	Filter evm.TraceFilter
}

func (a HandlerAgentCall) GetExtendRequirements(_ context.Context, bd *BlockData) (evm.BlockExtendRequirement, error) {
	txHashSet := set.New[string]()
	for _, trace := range bd.mainData.Traces {
		if a.Filter.Check(trace) {
			txHashSet.Add(trace.TransactionHash)
		}
	}
	return evm.BlockExtendRequirement{SpecialTransactions: txHashSet.DumpValues()}, nil
}

func (a HandlerAgentCall) BuildTaskDataList(ctx context.Context, bd *BlockData) ([]taskData, error) {
	funcAbi := a.DataSource.Mapping.CallHandlers[a.HandlerID.ID].GetABI()
	var r []taskData
	for _, trace := range bd.mainData.Traces {
		if !a.Filter.Check(trace) {
			continue
		}
		call, size, err := bd.buildCall(trace, funcAbi)
		if err != nil {
			return nil, fetcher.Permanent(err)
		}
		r = append(r, taskData{
			callHandlerParam: call,
			dataSource:       a.DataSource,
			handlerID:        a.HandlerID,
			txIndex:          int(trace.TransactionIndex),
			size:             size,
		})
	}
	return r, nil
}

func (a HandlerAgentCall) Snapshot() any {
	return map[string]any{
		"HandlerID": a.HandlerID,
		"Range":     a.Range.String(),
		"Filter":    a.Filter,
	}
}
