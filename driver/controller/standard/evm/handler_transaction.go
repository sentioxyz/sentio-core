package evm

import (
	"context"

	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data/evm"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type HandlerAgentTransaction struct {
	controller.BaseHandlerAgent

	FetchConfig *protos.EthFetchConfig
}

func (a HandlerAgentTransaction) GetExtendRequirements(
	_ context.Context,
	d *BlockData,
) (r evm.BlockExtendRequirement, err error) {
	if !a.Range.Contains(d.GetBlockNumber()) {
		return r, nil
	}
	r.AllTransactions = true
	r.AllTransactionReceipts = a.FetchConfig.GetTransactionReceipt()
	r.AllTransactionReceiptLogs = a.FetchConfig.GetTransactionReceiptLogs()
	return r, nil
}

func (a HandlerAgentTransaction) BuildBindingDataList(
	ctx context.Context,
	d *BlockData,
) (r []standard.BindingDataInner, err error) {
	for txIndex, txHash := range d.BlockHeader.TxHashes {
		rawTransaction := d.getTransactionJSON(txHash)
		rawBlock := new(d.getHeaderJSON())
		size := len(rawTransaction) + len(*rawBlock)
		var rawReceipt *string
		if a.FetchConfig.GetTransactionReceipt() {
			rawReceipt = new(d.getReceiptJSON(txHash, a.FetchConfig.GetTransactionReceiptLogs()))
			size += len(*rawReceipt)
		}
		data := standard.BindingDataInner{
			HandlerType: protos.HandlerType_ETH_TRANSACTION,
			TxIndex:     txIndex,
			Data: &protos.Data{
				Value: &protos.Data_EthTransaction_{
					EthTransaction: &protos.Data_EthTransaction{
						Timestamp:             timestamppb.New(d.GetBlockTime()),
						RawTransaction:        rawTransaction,
						RawBlock:              rawBlock,
						RawTransactionReceipt: rawReceipt,
					},
				},
			},
			DataSize: size,
		}
		r = append(r, data)
	}
	return
}
func (a HandlerAgentTransaction) Snapshot() any {
	return map[string]any{
		"HandlerID":   a.HandlerID,
		"Range":       a.Range.String(),
		"FetchConfig": a.FetchConfig,
	}
}
