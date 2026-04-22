package aptos

import (
	"github.com/aptos-labs/aptos-go-sdk/api"
)

type Slot api.Block

func (s Slot) GetNumber() uint64 {
	return s.BlockHeight
}

func (s Slot) GetHash() string {
	return s.BlockHash
}

func (s Slot) GetParentHash() string {
	return ""
}

func (s Slot) Linked() bool {
	return false
}

func (s Slot) GetTransactionByVersion(version uint64) *Transaction {
	for _, tx := range s.Transactions {
		if tx.Version() == version {
			ntx := NewTransaction(tx)
			return &ntx
		}
	}
	return nil
}
