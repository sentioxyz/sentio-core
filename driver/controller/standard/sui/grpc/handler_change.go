package grpc

import (
	"context"

	"sentioxyz/sentio-core/driver/controller/standard"
	suihandler "sentioxyz/sentio-core/driver/controller/standard/sui"
	"sentioxyz/sentio-core/processor/protos"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type HandlerAgentChange struct {
	suihandler.HandlerAgentChange
}

func (a HandlerAgentChange) BuildBindingDataList(
	_ context.Context,
	bd *BlockData,
) (result []standard.BindingDataInner, err error) {
	checker := a.Filter.CheckerGrpc()
	for i, oc := range bd.mainData.ObjectChanges {
		if !checker(oc.ChangedObject) {
			continue
		}
		var rawChange string
		if rawChange, err = bd.getChange(i); err != nil {
			return nil, err
		}
		result = append(result, standard.BindingDataInner{
			HandlerType: protos.HandlerType_SUI_OBJECT_CHANGE,
			TxIndex:     int(oc.TxIndex),
			Data: &protos.Data{
				Value: &protos.Data_SuiObjectChange_{
					SuiObjectChange: &protos.Data_SuiObjectChange{
						RawChanges: []string{rawChange},
						Slot:       bd.GetBlockNumber(),
						TxDigest:   oc.TxDigest,
						Timestamp:  timestamppb.New(bd.GetBlockTime()),
					},
				},
			},
			DataSize: len(rawChange),
		})
	}
	return
}
