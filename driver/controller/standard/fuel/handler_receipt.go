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

type HandlerAgentReceipt struct {
	controller.BaseHandlerAgent

	Filters []fuel.TransactionFilter
}

func (a HandlerAgentReceipt) Snapshot() any {
	return map[string]any{
		"HandlerID": a.HandlerID,
		"Range":     a.Range.String(),
		"Filters":   a.Filters,
	}
}

func (a HandlerAgentReceipt) BuildBindingDataList(
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
		receiptIndexes := make(map[int]bool)
		receipts := fuel.GetTxnReceipt(tx.Status)
		for _, filter := range a.Filters {
			if filter.LogFilter != nil {
				for _, receiptIndex := range filter.LogFilter.Check(receipts) {
					receiptIndexes[receiptIndex] = true
				}
			}
			if filter.ReceiptTransferFilter != nil {
				for _, receiptIndex := range filter.ReceiptTransferFilter.Check(receipts) {
					receiptIndexes[receiptIndex] = true
				}
			}
		}
		for _, receiptIndex := range utils.GetOrderedMapKeys(receiptIndexes) {
			result = append(result, standard.BindingDataInner{
				HandlerType:  protos.HandlerType_FUEL_RECEIPT,
				TxIndex:      int(tx.TransactionIndex),
				TxInnerIndex: receiptIndex,
				Data: &protos.Data{
					Value: &protos.Data_FuelReceipt_{
						FuelReceipt: &protos.Data_FuelReceipt{
							Transaction:  bd.getTxPb(i),
							ReceiptIndex: int64(receiptIndex),
							Timestamp:    timestamppb.New(bd.GetBlockTime()),
						},
					},
				},
				DataSize: 1000,
			})
		}
	}
	return result, nil
}
