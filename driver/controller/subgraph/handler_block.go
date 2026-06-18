package subgraph

import (
	"context"
	"math"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/data/evm"
	"sentioxyz/sentio-core/driver/controller/fetcher"
	"sentioxyz/sentio-core/driver/subgraph/manifest"
)

type HandlerAgentBlock struct {
	controller.BaseHandlerAgent
	DataSource *manifest.DataSource

	IntervalConfig data.IntervalConfig
	Once           bool
}

func (a HandlerAgentBlock) GetExtendRequirements(context.Context, *BlockData) (evm.BlockExtendRequirement, error) {
	return evm.BlockExtendRequirement{}, nil
}

func (a HandlerAgentBlock) BuildTaskDataList(_ context.Context, bd *BlockData) ([]taskData, error) {
	if a.Once {
		if bd.GetBlockNumber() != a.Range.StartBlock {
			return nil, nil
		}
	} else {
		if !data.ContainsInterval(bd.mainData.Intervals, a.IntervalConfig) {
			return nil, nil
		}
	}
	block, err := bd.buildBlock()
	if err != nil {
		return nil, fetcher.Permanent(err)
	}
	return []taskData{{
		callHandlerParam: block,
		dataSource:       a.DataSource,
		handlerID:        a.HandlerID,
		txIndex:          utils.Select(a.Once, -1, math.MaxInt),
		size:             len(bd.BlockHeader.Raw),
	}}, nil
}

func (a HandlerAgentBlock) Snapshot() any {
	sn := map[string]any{
		"HandlerID": a.HandlerID,
		"Range":     a.Range.String(),
	}
	if a.Once {
		sn["Once"] = true
	} else {
		sn["IntervalConfig"] = a.IntervalConfig
	}
	return sn
}
