package ch

import (
	"encoding/json"
	"math/big"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type SlotTransaction interface {
	FromRPCTransaction(blockIndex BlockIndex, tx evm.RPCTransaction, receipt *evm.ExtendedReceipt)
	ToRPCTransaction() (evm.RPCTransaction, *evm.ExtendedReceipt)

	GetBlockIndex() BlockIndex
	GetTxnIndex() TxnIndex
	GetReceiptStatus() bool
}

type TxnIndex struct {
	TransactionHash  string `clickhouse:"transaction_hash" type:"FixedString(66)" index:"bloom_filter GRANULARITY 10"`
	TransactionIndex uint64 `clickhouse:"transaction_index"`
}

type Transaction struct {
	BlockIndex
	TxnIndex
	TransactionType          uint64   `clickhouse:"transaction_type"`
	FromAddress              string   `clickhouse:"from_address" type:"FixedString(42)"`
	ToAddress                *string  `clickhouse:"to_address"   type:"Nullable(FixedString(42))"`
	Nonce                    uint64   `clickhouse:"nonce"`
	GasPrice                 *big.Int `clickhouse:"gas_price"                type:"Nullable(UInt256)"`
	MaxPriorityFeePerGas     *big.Int `clickhouse:"max_priority_fee_per_gas" type:"Nullable(UInt256)"`
	MaxFeePerGas             *big.Int `clickhouse:"max_fee_per_gas"          type:"Nullable(UInt256)"`
	Gas                      *big.Int `clickhouse:"gas"                      type:"Nullable(UInt256)"`
	Value                    string   `clickhouse:"value"`
	Input                    string   `clickhouse:"input"`
	V                        string   `clickhouse:"v"`
	R                        string   `clickhouse:"r"`
	S                        string   `clickhouse:"s"`
	HasReceipt               bool     `clickhouse:"has_receipt"`
	ReceiptCumulativeGasUsed uint64   `clickhouse:"receipt_cumulative_gas_used"`
	ReceiptContractAddress   *string  `clickhouse:"receipt_contract_address" type:"Nullable(FixedString(42))"`
	ReceiptGasUsed           uint64   `clickhouse:"receipt_gas_used"`
	ReceiptStatus            bool     `clickhouse:"receipt_status"`
	ReceiptEffectiveGasPrice *big.Int `clickhouse:"receipt_effective_gas_price" type:"Nullable(UInt256)"`
}

func (t *Transaction) GetInput() []byte {
	return hexutil.MustDecode(t.Input)
}

func (t *Transaction) GetBlockIndex() BlockIndex {
	return t.BlockIndex
}

func (t *Transaction) GetTxnIndex() TxnIndex {
	return t.TxnIndex
}

func (t *Transaction) GetReceiptStatus() bool {
	return t.ReceiptStatus
}

func (t *Transaction) ToRPCTransaction() (txn evm.RPCTransaction, r *evm.ExtendedReceipt) {
	txn = evm.RPCTransaction{
		Type:                 hexutil.Uint64(t.TransactionType),
		Nonce:                evm.LenientHexUint64(t.Nonce),
		GasPrice:             (*hexutil.Big)(t.GasPrice),
		MaxPriorityFeePerGas: (*hexutil.Big)(t.MaxPriorityFeePerGas),
		MaxFeePerGas:         (*hexutil.Big)(t.MaxFeePerGas),
		Gas:                  hexutil.Uint64(utils.ZeroOrUInt64(t.Gas)),
		Value:                (*hexutil.Big)(hexutil.MustDecodeBig(t.Value)),
		Input:                hexutil.MustDecode(t.Input),
		V:                    (*evm.LenientHexBig)(new(big.Int).SetBytes(hexutil.MustDecode(t.V))),
		R:                    (*evm.LenientHexBig)(new(big.Int).SetBytes(hexutil.MustDecode(t.R))),
		S:                    (*evm.LenientHexBig)(new(big.Int).SetBytes(hexutil.MustDecode(t.S))),
		To:                   utils.NullOrFromString(t.ToAddress, common.HexToAddress),
		From:                 common.HexToAddress(t.FromAddress),
		Hash:                 common.HexToHash(t.TransactionHash),
		TransactionIndex:     hexutil.Uint64(t.TransactionIndex),
		BlockHash:            common.HexToHash(t.BlockHash),
		BlockNumber:          strconv.FormatUint(t.BlockNumber, 10),
	}
	if t.HasReceipt {
		r = &evm.ExtendedReceipt{
			BlockHash:         common.HexToHash(t.BlockHash),
			BlockNumber:       (*hexutil.Big)(new(big.Int).SetUint64(t.BlockNumber)),
			TransactionIndex:  hexutil.Uint(t.TransactionIndex),
			TxHash:            common.HexToHash(t.TransactionHash),
			From:              common.HexToAddress(t.FromAddress),
			To:                utils.NullOrFromString(t.ToAddress, common.HexToAddress),
			CumulativeGasUsed: hexutil.Uint64(t.ReceiptCumulativeGasUsed),
			ContractAddress:   utils.NullOrFromString(t.ReceiptContractAddress, common.HexToAddress),
			GasUsed:           hexutil.Uint64(t.ReceiptGasUsed),
			// Logs Bloom need to set by SetLogs method
			// Root was dropped
			EffectiveGasPrice: (*hexutil.Big)(t.ReceiptEffectiveGasPrice),
			Type:              hexutil.Uint64(t.TransactionType),
			Status:            utils.Select[hexutil.Uint64](t.ReceiptStatus, 1, 0),
		}
	}
	return
}

func (t *Transaction) FromRPCTransaction(blockIndex BlockIndex, tx evm.RPCTransaction, r *evm.ExtendedReceipt) {
	defer func() {
		if err := recover(); err != nil {
			log.Fatalf("load from rpc transaction %d/%s failed: %v", blockIndex.BlockNumber, tx.Hash.String(), err)
		}
	}()
	t.BlockIndex = blockIndex
	t.TransactionHash = tx.Hash.String()
	t.TransactionIndex = uint64(tx.TransactionIndex)
	t.TransactionType = uint64(tx.Type)
	t.FromAddress = AddressToLowerString(tx.From)
	t.ToAddress = utils.NullOrConvert(tx.To, AddressToLowerString)
	t.Nonce = uint64(tx.Nonce)
	t.GasPrice = tx.GasPrice.ToInt()
	t.MaxPriorityFeePerGas = tx.MaxPriorityFeePerGas.ToInt()
	t.MaxFeePerGas = tx.MaxFeePerGas.ToInt()
	t.Gas = big.NewInt(int64(tx.Gas))
	t.Value = tx.Value.String()
	t.Input = tx.Input.String()
	bigToStr := func(n *evm.LenientHexBig) string {
		if n == nil {
			return hexutil.Encode([]byte{})
		}
		return hexutil.Encode(n.ToInt().Bytes())
	}
	t.V = bigToStr(tx.V)
	t.R = bigToStr(tx.R)
	t.S = bigToStr(tx.S)
	t.HasReceipt = false
	if r != nil {
		t.HasReceipt = true
		t.ReceiptCumulativeGasUsed = uint64(r.CumulativeGasUsed)
		t.ReceiptContractAddress = utils.NullOrConvert(r.ContractAddress, AddressToLowerString)
		t.ReceiptGasUsed = uint64(r.GasUsed)
		t.ReceiptStatus = r.Status == 1
		t.ReceiptEffectiveGasPrice = r.EffectiveGasPrice.ToInt()
	}
}

type TransactionArbitrum struct {
	Transaction
	L1BlockNumber uint64  `clickhouse:"l1_block_number"`
	GasUsedForL1  *uint64 `clickhouse:"gas_used_for_l1"`
}

func (t *TransactionArbitrum) FromRPCTransaction(blockIndex BlockIndex, tx evm.RPCTransaction, r *evm.ExtendedReceipt) {
	t.Transaction.FromRPCTransaction(blockIndex, tx, r)
	if r != nil {
		if r.L1BlockNumber != nil {
			t.L1BlockNumber = r.L1BlockNumber.ToInt().Uint64()
		}
		t.GasUsedForL1 = (*uint64)(r.GasUsedForL1)
	}
}

func (t *TransactionArbitrum) ToRPCTransaction() (txn evm.RPCTransaction, r *evm.ExtendedReceipt) {
	txn, r = t.Transaction.ToRPCTransaction()
	if r != nil {
		r.L1BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(t.L1BlockNumber))
		if t.GasUsedForL1 != nil {
			r.GasUsedForL1 = (*hexutil.Uint64)(t.GasUsedForL1)
		}
	}
	return
}

type TransactionCronosZkevm struct {
	Transaction
	ReceiptL1BatchNumber  *uint64 `clickhouse:"receipt_l1_batch_number"`
	ReceiptL1BatchTxIndex *uint64 `clickhouse:"receipt_l1_batch_tx_index"`
	ReceiptL2ToL1Logs     string  `clickhouse:"receipt_l2_to_l1_logs_json"`
}

func (t *TransactionCronosZkevm) FromRPCTransaction(
	blockIndex BlockIndex,
	tx evm.RPCTransaction,
	r *evm.ExtendedReceipt,
) {
	t.Transaction.FromRPCTransaction(blockIndex, tx, r)
	if r != nil {
		if r.L1BatchTxIndex != nil {
			t.ReceiptL1BatchNumber = utils.WrapPointer(r.L1BatchTxIndex.ToInt().Uint64())
		}
		if r.L1BatchNumber != nil {
			t.ReceiptL1BatchTxIndex = utils.WrapPointer(r.L1BatchNumber.ToInt().Uint64())
		}
		t.ReceiptL2ToL1Logs = utils.MustJSONMarshal(r.L2ToL1Logs)
	}
}

func (t *TransactionCronosZkevm) ToRPCTransaction() (txn evm.RPCTransaction, r *evm.ExtendedReceipt) {
	txn, r = t.Transaction.ToRPCTransaction()
	if r != nil {
		if t.ReceiptL1BatchNumber != nil {
			r.L1BatchNumber = (*hexutil.Big)(new(big.Int).SetUint64(*t.ReceiptL1BatchNumber))
		}
		if t.ReceiptL1BatchTxIndex != nil {
			r.L1BatchTxIndex = (*hexutil.Big)(new(big.Int).SetUint64(*t.ReceiptL1BatchTxIndex))
		}
		_ = json.Unmarshal([]byte(t.ReceiptL2ToL1Logs), &r.L2ToL1Logs)
	}
	return
}

type TransactionOptimism struct {
	Transaction
	ReceiptL1Fee      *big.Int `clickhouse:"receipt_l1_fee"`
	ReceiptL1GasPrice *big.Int `clickhouse:"receipt_l1_gas_price"`
	ReceiptL1GasUsed  *big.Int `clickhouse:"receipt_l1_gas_used"`
}

func (t *TransactionOptimism) FromRPCTransaction(
	blockIndex BlockIndex,
	tx evm.RPCTransaction,
	r *evm.ExtendedReceipt,
) {
	t.Transaction.FromRPCTransaction(blockIndex, tx, r)
	if r != nil {
		if r.L1Fee != nil {
			t.ReceiptL1Fee = (*big.Int)(r.L1Fee)
		}
		if r.L1GasPrice != nil {
			t.ReceiptL1GasPrice = (*big.Int)(r.L1GasPrice)
		}
		if r.L1GasUsed != nil {
			t.ReceiptL1GasUsed = (*big.Int)(r.L1GasUsed)
		}
	}
}

func (t *TransactionOptimism) ToRPCTransaction() (txn evm.RPCTransaction, r *evm.ExtendedReceipt) {
	txn, r = t.Transaction.ToRPCTransaction()
	if r != nil {
		if t.ReceiptL1Fee != nil {
			r.L1Fee = (*hexutil.Big)(t.ReceiptL1Fee)
		}
		if t.ReceiptL1GasPrice != nil {
			r.L1GasPrice = (*hexutil.Big)(t.ReceiptL1GasPrice)
		}
		if t.ReceiptL1GasUsed != nil {
			r.L1GasUsed = (*hexutil.Big)(t.ReceiptL1GasUsed)
		}
	}
	return
}
