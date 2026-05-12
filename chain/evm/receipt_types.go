package evm

import (
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"google.golang.org/protobuf/types/known/structpb"

	"sentioxyz/sentio-core/common/utils"
)

type L2ToL1Log struct {
	BlockNumber    *hexutil.Big  `json:"blockNumber"`
	BlockHash      common.Hash   `json:"blockHash"`
	L1BatchNumber  *hexutil.Big  `json:"l1BatchNumber"`
	L1BatchTxIndex *hexutil.Big  `json:"txIndexInL1Batch"`
	Index          *hexutil.Big  `json:"logIndex"`
	TxHash         common.Hash   `json:"transactionHash"`
	TxIndex        *hexutil.Big  `json:"transactionIndex"`
	TxLogIndex     *hexutil.Big  `json:"transactionLogIndex"`
	ShardID        *hexutil.Big  `json:"shardId"`
	IsService      bool          `json:"isService"`
	Sender         hexutil.Bytes `json:"sender"`
	Key            hexutil.Bytes `json:"key"`
	Value          hexutil.Bytes `json:"value"`
}

type ExtendedReceipt struct {
	BlockHash         common.Hash     `json:"blockHash,omitempty"`
	BlockNumber       *hexutil.Big    `json:"blockNumber,omitempty"`
	TransactionIndex  hexutil.Uint    `json:"transactionIndex"`
	TxHash            common.Hash     `json:"transactionHash"`
	From              common.Address  `json:"from"`
	To                *common.Address `json:"to"`
	CumulativeGasUsed hexutil.Uint64  `json:"cumulativeGasUsed"`
	ContractAddress   *common.Address `json:"contractAddress"`
	GasUsed           hexutil.Uint64  `json:"gasUsed"`
	Logs              []*types.Log    `json:"logs"`
	Bloom             types.Bloom     `json:"logsBloom"`
	Root              hexutil.Bytes   `json:"root,omitempty"`
	EffectiveGasPrice *hexutil.Big    `json:"effectiveGasPrice,omitempty"`
	Type              hexutil.Uint64  `json:"type"`
	Status            hexutil.Uint64  `json:"status"`

	// For arbitrum.
	L1BlockNumber *hexutil.Big    `json:"l1BlockNumber,omitempty"`
	GasUsedForL1  *hexutil.Uint64 `json:"gasUsedForL1,omitempty"`

	// For cronos zkevm.
	L1BatchTxIndex *hexutil.Big `json:"l1BatchTxIndex,omitempty"`
	L1BatchNumber  *hexutil.Big `json:"l1BatchNumber,omitempty"`
	L2ToL1Logs     []*L2ToL1Log `json:"l2ToL1Logs,omitempty"`

	// For zircuit.
	L1Fee      *hexutil.Big `json:"l1Fee,omitempty"`
	L1GasPrice *hexutil.Big `json:"l1GasPrice,omitempty"`
	L1GasUsed  *hexutil.Big `json:"l1GasUsed,omitempty"`
}

var logTyp = reflect.TypeOf(types.Log{})

func (h *ExtendedReceipt) SetLogs(logs []*types.Log) {
	if len(logs) > 0 {
		h.Logs = logs
		h.Bloom = types.CreateBloom(&types.Receipt{Logs: logs})
	}
}

func (h ExtendedReceipt) MarshalStructpb() *structpb.Value {
	fields := map[string]*structpb.Value{
		"from":              structpb.NewStringValue(h.From.String()),
		"type":              structpb.NewStringValue(h.Type.String()),
		"status":            structpb.NewStringValue(h.Status.String()),
		"cumulativeGasUsed": structpb.NewStringValue(h.CumulativeGasUsed.String()),
		"logsBloom":         structpb.NewStringValue(hexutil.Encode(h.Bloom[:])),
		"transactionHash":   structpb.NewStringValue(h.TxHash.Hex()),
		"gasUsed":           structpb.NewStringValue(h.GasUsed.String()),
		"blockHash":         structpb.NewStringValue(h.BlockHash.Hex()),
		"transactionIndex":  structpb.NewStringValue(h.TransactionIndex.String()),
	}
	appendStringField := func(fieldName string, val any) {
		if !reflect.ValueOf(val).IsNil() {
			fields[fieldName] = structpb.NewStringValue(val.(fmt.Stringer).String())
		}
	}
	appendStringField("to", h.To)
	appendStringField("contractAddress", h.ContractAddress)
	appendStringField("blockNumber", h.BlockNumber)
	appendStringField("l1BlockNumber", h.L1BlockNumber)
	appendStringField("gasUsedForL1", h.GasUsedForL1)
	if len(h.Logs) > 0 {
		logs := make([]*structpb.Value, len(h.Logs))
		for i, log := range h.Logs {
			logs[i] = structpb.NewStructValue(utils.ConvertToStructpb(log, logTyp))
		}
		fields["logs"] = structpb.NewListValue(&structpb.ListValue{Values: logs})
	}
	appendStringField("root", h.Root)
	appendStringField("effectiveGasPrice", h.EffectiveGasPrice)
	appendStringField("l1BatchNumber", h.L1BatchNumber)
	appendStringField("l1BatchTxIndex", h.L1BatchTxIndex)
	if h.L2ToL1Logs != nil {
		fields["l1BatchTxIndex"] = utils.MarshalStructpb(h.L2ToL1Logs)
	}
	appendStringField("l1Fee", h.L1Fee)
	appendStringField("l1GasPrice", h.L1GasPrice)
	appendStringField("l1GasUsed", h.L1GasUsed)
	return structpb.NewStructValue(&structpb.Struct{Fields: fields})
}
