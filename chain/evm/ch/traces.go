package ch

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/utils"
)

type Trace struct {
	BlockIndex
	TransactionHash     *string  `clickhouse:"transaction_hash" type:"Nullable(FixedString(66))" index:"bloom_filter GRANULARITY 10"`
	TransactionIndex    uint64   `clickhouse:"transaction_index"`
	TraceIndex          uint64   `clickhouse:"trace_index"`
	CallType            string   `clickhouse:"call_type"`
	FromAddress         *string  `clickhouse:"from_address" type:"Nullable(FixedString(42))" index:"bloom_filter"`
	ToAddress           *string  `clickhouse:"to_address"   type:"Nullable(FixedString(42))" index:"bloom_filter"`
	Input               string   `clickhouse:"input"`
	Gas                 *big.Int `clickhouse:"gas"      type:"Nullable(UInt256)"`
	GasUsed             *big.Int `clickhouse:"gas_used" type:"Nullable(UInt256)"`
	Value               string   `clickhouse:"value"`
	Author              *string  `clickhouse:"author" type:"Nullable(FixedString(42))"`
	RewardType          string   `clickhouse:"reward_type"`
	ActionInit          string   `clickhouse:"action_init"`
	ActionAddress       *string  `clickhouse:"action_address"        type:"Nullable(FixedString(42))"`
	ActionRefundAddress *string  `clickhouse:"action_refund_address" type:"Nullable(FixedString(42))"`
	ActionBalance       *big.Int `clickhouse:"action_balance"        type:"Nullable(UInt256)"`
	ResultOutput        string   `clickhouse:"result_output"`
	ResultAddress       *string  `clickhouse:"result_address" type:"Nullable(FixedString(42))"`
	Subtraces           int64    `clickhouse:"sub_traces"`
	TraceAddress        []int64  `clickhouse:"trace_address"`
	OriginType          string   `clickhouse:"origin_type"`
	Error               string   `clickhouse:"error"`
	MethodSig           string   `clickhouse:"method_sig"`
	Type                string   `clickhouse:"type"`
	ReceiptStatus       bool     `clickhouse:"receipt_status"`
}

func (t *Trace) ToTrace() evm.ParityTrace {
	return evm.ParityTrace{
		Action: evm.ParityTraceAction{
			CallType:      t.CallType,
			From:          utils.NullOrFromString(t.FromAddress, common.HexToAddress),
			Gas:           (*hexutil.Big)(t.Gas),
			Input:         hexutil.MustDecode(t.Input),
			To:            utils.EmptyStringIfNil(t.ToAddress),
			Value:         t.Value,
			Author:        utils.NullOrFromString(t.Author, common.HexToAddress),
			RewardType:    t.RewardType,
			Init:          hexutil.MustDecode(t.ActionInit),
			Address:       utils.NullOrFromString(t.ActionAddress, common.HexToAddress),
			RefundAddress: utils.NullOrFromString(t.ActionRefundAddress, common.HexToAddress),
			Balance:       (*hexutil.Big)(t.ActionBalance),
		},
		BlockHash:   common.HexToHash(t.BlockHash),
		BlockNumber: t.BlockNumber,
		Error:       t.Error,
		Result: &evm.ParityTraceResult{
			Address: utils.NullOrFromString(t.ResultAddress, common.HexToAddress),
			GasUsed: (*hexutil.Big)(t.GasUsed),
			Output:  hexutil.MustDecode(utils.Select(t.ResultOutput == "", "0x", t.ResultOutput)),
		},
		Subtraces:           int(t.Subtraces),
		TraceAddress:        utils.MapSliceNoError(t.TraceAddress, func(x int64) int { return int(x) }),
		TransactionHash:     utils.NullOrFromString(t.TransactionHash, common.HexToHash),
		TransactionPosition: t.TransactionIndex,
		Type:                t.Type,
	}
}
