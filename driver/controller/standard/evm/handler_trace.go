package evm

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data/evm"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentTrace struct {
	controller.BaseHandlerAgent

	FetchConfig *protos.EthFetchConfig
	Filter      evm.TraceFilter
}

func (a HandlerAgentTrace) GetExtendRequirements(
	_ context.Context,
	d *BlockData,
) (evm.BlockExtendRequirement, error) {
	var r evm.BlockExtendRequirement
	if !a.Range.Contains(d.GetBlockNumber()) {
		return r, nil
	}
	if !a.FetchConfig.GetTransaction() &&
		!a.FetchConfig.GetTransactionReceipt() &&
		!a.FetchConfig.GetTransactionReceiptLogs() {
		return r, nil
	}
	txnSet := make(map[string]bool)
	for _, trace := range d.mainData.Traces {
		if a.Filter.Check(trace) {
			txnSet[trace.TransactionHash] = true
		}
	}
	for txnHash := range txnSet {
		if a.FetchConfig.GetTransaction() {
			r.SpecialTransactions = append(r.SpecialTransactions, txnHash)
		}
		if a.FetchConfig.GetTransactionReceipt() {
			r.SpecialTransactionReceipts = append(r.SpecialTransactionReceipts, txnHash)
		}
		if a.FetchConfig.GetTransactionReceiptLogs() {
			r.SpecialTransactionReceiptLogs = append(r.SpecialTransactionReceiptLogs, txnHash)
		}
	}
	return r, nil
}

func (a HandlerAgentTrace) BuildBindingDataList(
	ctx context.Context,
	d *BlockData,
) (r []standard.BindingDataInner, err error) {
	for _, trace := range d.mainData.Traces {
		if !a.Filter.Check(trace) {
			continue
		}
		rawTrace := string(trace.Raw)
		var rawBlock, rawTransaction, rawReceipt *string
		if a.FetchConfig.GetBlock() {
			s := d.getHeaderJSON()
			rawBlock = &s
		}
		if a.FetchConfig.GetTransaction() {
			s := d.getTransactionJSON(trace.TransactionHash)
			rawTransaction = &s
		}
		if a.FetchConfig.GetTransactionReceipt() {
			s := d.getReceiptJSON(trace.TransactionHash, a.FetchConfig.GetTransactionReceiptLogs())
			rawReceipt = &s
		}
		dataSize := len(rawTrace)
		if rawBlock != nil {
			dataSize += len(*rawBlock)
		}
		if rawTransaction != nil {
			dataSize += len(*rawTransaction)
		}
		if rawReceipt != nil {
			dataSize += len(*rawReceipt)
		}
		data := standard.BindingDataInner{
			HandlerType: protos.HandlerType_ETH_TRACE,
			TxIndex:     int(trace.TransactionIndex),
			Data: &protos.Data{
				Value: &protos.Data_EthTrace_{
					EthTrace: &protos.Data_EthTrace{
						Timestamp:             timestamppb.New(d.GetBlockTime()),
						RawTrace:              rawTrace,
						RawBlock:              rawBlock,
						RawTransaction:        rawTransaction,
						RawTransactionReceipt: rawReceipt,
					},
				},
			},
			DataSize: dataSize,
		}
		r = append(r, data)
	}
	return
}

func (a HandlerAgentTrace) Snapshot() any {
	return map[string]any{
		"HandlerID":   a.HandlerID,
		"Range":       a.Range.String(),
		"Filter":      a.Filter,
		"FetchConfig": a.FetchConfig,
	}
}
