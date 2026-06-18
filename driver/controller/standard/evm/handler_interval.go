package evm

import (
	"context"
	"encoding/json"
	"math"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/data/evm"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentInterval struct {
	controller.BaseHandlerAgent

	FetchConfig    *protos.EthFetchConfig
	IntervalConfig data.IntervalConfig
}

func (a HandlerAgentInterval) GetExtendRequirements(
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
	if !data.ContainsInterval(d.mainData.Intervals, a.IntervalConfig) {
		return r, nil
	}
	if a.FetchConfig.GetTransaction() {
		r.AllTransactions = true
	}
	if a.FetchConfig.GetTransactionReceipt() {
		r.AllTransactionReceipts = true
	}
	if a.FetchConfig.GetTransactionReceiptLogs() {
		r.AllTransactionReceiptLogs = true
	}
	if a.FetchConfig.GetTrace() {
		r.AllTraces = true
	}
	return r, nil
}

func (a HandlerAgentInterval) BuildBindingDataList(
	_ context.Context,
	d *BlockData,
) ([]standard.BindingDataInner, error) {
	if !data.ContainsInterval(d.mainData.Intervals, a.IntervalConfig) {
		return nil, nil
	}
	rawBlock := d.getHeaderJSON()
	if a.FetchConfig.GetTransaction() || a.FetchConfig.GetTransactionReceipt() || a.FetchConfig.GetTrace() {
		// Splice transactions / receipts / traces (each already raw JSON) into the header
		// object to build the composite raw_block. Header fields are kept as raw bytes.
		var block map[string]json.RawMessage
		if err := json.Unmarshal(d.BlockHeader.Raw, &block); err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal block header %d", d.GetBlockNumber())
		}
		if a.FetchConfig.GetTransaction() {
			txns := make([]json.RawMessage, 0, len(d.BlockHeader.TxHashes))
			for _, txHash := range d.BlockHeader.TxHashes {
				txns = append(txns, json.RawMessage(d.getTransactionJSON(txHash)))
			}
			raw, err := json.Marshal(txns)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to marshal transactions for block %d", d.GetBlockNumber())
			}
			block["transactions"] = raw
		}
		if a.FetchConfig.GetTransactionReceipt() {
			receipts := make([]json.RawMessage, 0, len(d.BlockHeader.TxHashes))
			for _, txHash := range d.BlockHeader.TxHashes {
				receipts = append(receipts, json.RawMessage(d.getReceiptJSON(txHash, a.FetchConfig.GetTransactionReceiptLogs())))
			}
			raw, err := json.Marshal(receipts)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to marshal transaction receipts for block %d", d.GetBlockNumber())
			}
			block["transactionReceipts"] = raw
		}
		if a.FetchConfig.GetTrace() {
			var traces []json.RawMessage
			for _, txHash := range d.BlockHeader.TxHashes {
				for _, trace := range d.extendData.Traces[txHash] {
					traces = append(traces, json.RawMessage(trace.Raw))
				}
			}
			raw, err := json.Marshal(traces)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to marshal traces for block %d", d.GetBlockNumber())
			}
			block["traces"] = raw
		}
		raw, err := json.Marshal(block)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal block %d", d.GetBlockNumber())
		}
		rawBlock = string(raw)
	}
	return []standard.BindingDataInner{{
		HandlerType: protos.HandlerType_ETH_BLOCK,
		TxIndex:     math.MaxInt,
		Data: &protos.Data{
			Value: &protos.Data_EthBlock_{
				EthBlock: &protos.Data_EthBlock{
					RawBlock: rawBlock,
				},
			},
		},
		DataSize: len(rawBlock),
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
