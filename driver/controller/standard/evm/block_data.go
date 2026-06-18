package evm

import (
	"encoding/json"

	"sentioxyz/sentio-core/driver/controller"
	evmExtend "sentioxyz/sentio-core/driver/controller/data/evm"
)

type BlockData struct {
	evmExtend.BlockHeader

	mainData   evmExtend.BlockMainData
	extendData evmExtend.BlockExtendData

	headerJSON          string
	txnJSON             map[string]string
	receiptJSON         map[string]string
	receiptWithLogsJSON map[string]string

	taskList      []controller.Task
	taskTotalSize int
	dataSource    string

	checkpointData map[string]string
}

func (b *BlockData) DataSource() string {
	return b.dataSource
}

func (b *BlockData) CheckpointData() map[string]string {
	return b.checkpointData
}

func (b *BlockData) Size() int {
	return b.taskTotalSize
}

func (b *BlockData) GetTaskList() []controller.Task {
	return b.taskList
}

func (b *BlockData) getHeaderJSON() string {
	if b.headerJSON == "" {
		b.headerJSON = string(b.BlockHeader.Raw)
	}
	return b.headerJSON
}

func (b *BlockData) getTransactionJSON(txHash string) string {
	if b.txnJSON == nil {
		b.txnJSON = make(map[string]string)
	}
	if r, has := b.txnJSON[txHash]; has {
		return r
	}
	if tx, has := b.extendData.Transactions[txHash]; !has {
		return ""
	} else {
		r, _ := json.Marshal(tx)
		b.txnJSON[txHash] = string(r)
		return b.txnJSON[txHash]
	}
}

func (b *BlockData) getReceiptJSON(txHash string, withLogs bool) string {
	var cache map[string]string
	if withLogs {
		if b.receiptWithLogsJSON == nil {
			b.receiptWithLogsJSON = make(map[string]string)
		}
		cache = b.receiptWithLogsJSON
	} else {
		if b.receiptJSON == nil {
			b.receiptJSON = make(map[string]string)
		}
		cache = b.receiptJSON
	}
	if pb, has := cache[txHash]; has {
		return pb
	}
	if receipt, has := b.extendData.Receipts[txHash]; !has {
		return ""
	} else {
		if !withLogs {
			receipt.Logs = nil
		}
		r, _ := json.Marshal(receipt)
		cache[txHash] = string(r)
		return cache[txHash]
	}
}
