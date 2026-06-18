package evm

import (
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/utils"
)

type BlockExtendData struct {
	Transactions map[string]evm.RPCTransaction
	Receipts     map[string]evm.ExtendedReceipt
	Traces       map[string][]Trace
}

type BlockExtendRequirement struct {
	AllTransactions     bool
	SpecialTransactions []string

	AllTransactionReceipts     bool
	SpecialTransactionReceipts []string

	AllTransactionReceiptLogs     bool
	SpecialTransactionReceiptLogs []string

	AllTraces bool
}

func (r *BlockExtendRequirement) Merge(a BlockExtendRequirement) {
	r.AllTransactions = r.AllTransactions || a.AllTransactions
	r.AllTransactionReceipts = r.AllTransactionReceipts || a.AllTransactionReceipts
	r.AllTransactionReceiptLogs = r.AllTransactionReceiptLogs || a.AllTransactionReceiptLogs
	r.AllTraces = r.AllTraces || a.AllTraces
	r.SpecialTransactions = append(r.SpecialTransactions, a.SpecialTransactions...)
	r.SpecialTransactionReceipts = append(r.SpecialTransactionReceipts, a.SpecialTransactionReceipts...)
	r.SpecialTransactionReceiptLogs = append(r.SpecialTransactionReceiptLogs, a.SpecialTransactionReceiptLogs...)
	r.Trim()
}

func (r *BlockExtendRequirement) IsEmpty() bool {
	return !r.AllTransactions && len(r.SpecialTransactions) == 0 &&
		!r.AllTransactionReceipts && len(r.SpecialTransactionReceipts) == 0 &&
		!r.AllTransactionReceiptLogs && len(r.SpecialTransactionReceiptLogs) == 0 &&
		!r.AllTraces
}

func (r *BlockExtendRequirement) Trim() {
	if r.AllTransactions {
		r.SpecialTransactions = nil
	} else {
		r.SpecialTransactions = utils.GetMapKeys(utils.BuildSet(r.SpecialTransactions))
	}

	if r.AllTransactionReceipts {
		r.SpecialTransactionReceipts = nil
	} else {
		r.SpecialTransactionReceipts = utils.GetMapKeys(utils.BuildSet(r.SpecialTransactionReceipts))
	}

	if r.AllTransactionReceiptLogs {
		r.SpecialTransactionReceiptLogs = nil
	} else {
		r.SpecialTransactionReceiptLogs = utils.GetMapKeys(utils.BuildSet(r.SpecialTransactionReceiptLogs))
	}
}
