package sui

import (
	"context"
	"math"

	"google.golang.org/protobuf/types/known/timestamppb"

	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentChange struct {
	controller.BaseHandlerAgent

	Filter sui.ObjectChangeFilter
}

func (a HandlerAgentChange) BuildBindingDataList(
	_ context.Context,
	bd *BlockData,
) (result []standard.BindingDataInner, err error) {
	checker := a.Filter.Checker()
	for i, oc := range bd.mainData.ObjectChanges {
		if !checker(oc) {
			continue
		}
		// rawChange is the result of json marshal types.ObjectChange
		// the required fields include:
		// - objectId
		// - digest
		// - version
		// - previousVersion (may not exist)
		// - type (changeType)
		// - owner
		// - objectType
		var rawChange string
		if rawChange, err = bd.getChange(i); err != nil {
			return nil, err
		}
		result = append(result, standard.BindingDataInner{
			HandlerType: protos.HandlerType_SUI_OBJECT_CHANGE,
			TxIndex:     utils.Select(oc.TxIndex < 0, math.MaxInt, oc.TxIndex),
			Data: &protos.Data{
				Value: &protos.Data_SuiObjectChange_{
					SuiObjectChange: &protos.Data_SuiObjectChange{
						RawChanges: []string{rawChange},
						Slot:       bd.GetBlockNumber(),
						TxDigest:   oc.TxDigest.String(),
						Timestamp:  timestamppb.New(bd.GetBlockTime()),
					},
				},
			},
			DataSize: len(rawChange),
		})
	}
	return
}

func (a HandlerAgentChange) Snapshot() any {
	return map[string]any{
		"HandlerID": a.HandlerID,
		"Range":     a.Range.String(),
		"Filter":    a.Filter,
	}
}
