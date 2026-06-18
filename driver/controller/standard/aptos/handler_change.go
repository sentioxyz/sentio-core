package aptos

import (
	"context"

	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	aptosdata "sentioxyz/sentio-core/driver/controller/data/aptos"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

// HandlerAgentChange only need txn changes, do not need the txn self, so no aptos.FetchTxnConfig in it
type HandlerAgentChange struct {
	controller.BaseHandlerAgent

	Filter aptos.ChangeFilter
}

func (a HandlerAgentChange) BuildBindingDataList(
	_ context.Context,
	bd *BlockData,
) ([]standard.BindingDataInner, error) {
	changes := utils.FilterArr(bd.mainData.Changes, func(c aptosdata.Change) bool {
		return a.Filter.Check(c.WriteSetChange)
	})
	resources := utils.MapSliceNoError(changes, func(c aptosdata.Change) string { return c.Raw })
	if len(resources) == 0 {
		return nil, nil
	}
	return []standard.BindingDataInner{{
		HandlerType: protos.HandlerType_APT_RESOURCE,
		Data: &protos.Data{
			Value: &protos.Data_AptResource_{
				AptResource: &protos.Data_AptResource{
					Version:         int64(bd.GetBlockNumber()),
					RawResources:    resources,
					TimestampMicros: bd.GetBlockTime().UnixMicro(),
				},
			},
		},
		DataSize: utils.StringsLenSum(resources),
	}}, nil
}

func (a HandlerAgentChange) Snapshot() any {
	return map[string]any{
		"HandlerID": a.HandlerID,
		"Range":     a.Range.String(),
		"Filter":    a.Filter,
	}
}
