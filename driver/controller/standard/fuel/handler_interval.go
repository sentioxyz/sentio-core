package fuel

import (
	"context"
	"math"

	"google.golang.org/protobuf/types/known/timestamppb"

	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentInterval struct {
	controller.BaseHandlerAgent

	IntervalConfig data.IntervalConfig
}

func (a HandlerAgentInterval) Snapshot() any {
	return map[string]any{
		"HandlerID":      a.HandlerID,
		"Range":          a.Range.String(),
		"IntervalConfig": a.IntervalConfig,
	}
}

func (a HandlerAgentInterval) BuildBindingDataList(
	ctx context.Context,
	bd *BlockData,
) ([]standard.BindingDataInner, error) {
	if !data.ContainsInterval(bd.mainData.Intervals, a.IntervalConfig) {
		return nil, nil
	}
	blockPb, size, err := bd.getBlockPb()
	if err != nil {
		return nil, err
	}
	return []standard.BindingDataInner{{
		HandlerType: protos.HandlerType_FUEL_BLOCK,
		TxIndex:     math.MaxInt,
		Data: &protos.Data{
			Value: &protos.Data_FuelBlock_{
				FuelBlock: &protos.Data_FuelBlock{
					Block:     blockPb,
					Timestamp: timestamppb.New(bd.GetBlockTime()),
				},
			},
		},
		DataSize: size,
	}}, nil
}
