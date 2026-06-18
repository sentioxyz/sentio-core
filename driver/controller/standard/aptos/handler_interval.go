package aptos

import (
	"context"

	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	aptosdata "sentioxyz/sentio-core/driver/controller/data/aptos"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentInterval struct {
	controller.BaseHandlerAgent

	IntervalConfig data.IntervalConfig
	FetchConfig    aptosdata.AccountResourceFilter
}

func (a HandlerAgentInterval) BuildBindingDataList(
	_ context.Context,
	bd *BlockData,
) ([]standard.BindingDataInner, error) {
	if !data.ContainsInterval(bd.mainData.Intervals, a.IntervalConfig) {
		return nil, nil
	}
	if a.FetchConfig.NeedNothing() {
		// is a contract move interval handler, need a APT_CALL binding data
		rawTxn, err := bd.getRawTxn(aptos.TransactionFetchConfig{
			NeedAllEvents:       true,
			ChangeResourceTypes: move.TypeSet{move.MustBuildType("")},
		}, nil)
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
		}}, nil
	}
	// is a account move interval handler, need a APT_RESOURCE binding data
	ars := utils.FilterArr(bd.accountResources, a.FetchConfig.Check)
	resources := utils.MapSliceNoError(ars, func(ar aptosdata.AccountResource) string { return ar.Raw })
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

func (a HandlerAgentInterval) Snapshot() any {
	return map[string]any{
		"HandlerID":      a.HandlerID,
		"Range":          a.Range.String(),
		"IntervalConfig": a.IntervalConfig,
		"FetchConfig":    a.FetchConfig,
	}
}
