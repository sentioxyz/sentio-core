package fuel

import (
	"github.com/sentioxyz/fuel-go/types"
	"sentioxyz/sentio-core/chain/chain"
)

type Slot struct {
	*types.Block
}

var _ chain.Slot = (*Slot)(nil)

func NewSlot(block *types.Block) *Slot {
	return &Slot{block}
}

func (s *Slot) GetNumber() uint64 {
	return uint64(s.Header.Height)
}

func (s *Slot) GetHash() string {
	return s.Id.String()
}

func (s *Slot) GetParentHash() string {
	return ""
}

func (s *Slot) Linked() bool {
	return false
}

func (s *Slot) GetTransactions() []WrappedTransaction {
	txns := make([]WrappedTransaction, len(s.Transactions))
	for i, raw := range s.Transactions {
		txns[i] = WrappedTransaction{
			BlockHeight:      uint64(s.Height),
			TransactionIndex: uint64(i),
			Transaction:      raw,
		}
	}
	return txns
}
