package fuel

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentTransaction struct {
	controller.BaseHandlerAgent

	Filters []fuel.TransactionFilter
}

func (a HandlerAgentTransaction) Snapshot() any {
	return map[string]any{
		"HandlerID": a.HandlerID,
		"Range":     a.Range.String(),
		"Filters":   a.Filters,
	}
}

func (a HandlerAgentTransaction) BuildBindingDataList(
	ctx context.Context,
	bd *BlockData,
) (result []standard.BindingDataInner, err error) {
	for i, tx := range bd.mainData.Txs {
		ok := utils.HasAny(a.Filters, func(filter fuel.TransactionFilter) bool {
			return filter.Check(tx.Transaction)
		})
		if !ok {
			continue
		}
		result = append(result, standard.BindingDataInner{
			HandlerType: protos.HandlerType_FUEL_TRANSACTION,
			TxIndex:     int(tx.TransactionIndex),
			Data: &protos.Data{
				Value: &protos.Data_FuelTransaction_{
					FuelTransaction: &protos.Data_FuelTransaction{
						Transaction: bd.getTxPb(i),
						Timestamp:   timestamppb.New(bd.GetBlockTime()),
					},
				},
			},
			DataSize: 1000,
		})
	}
	return result, nil
}
