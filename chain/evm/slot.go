package evm

import (
	"github.com/ethereum/go-ethereum/core/types"
	"sentioxyz/sentio-core/chain/chain"
)

type Slot struct {
	Header   *ExtendedHeader
	Block    *RPCBlock
	Logs     []types.Log
	Traces   []ParityTrace
	Receipts []ExtendedReceipt
}

var _ chain.Slot = (*Slot)(nil)

func (s *Slot) GetNumber() uint64 {
	return s.Header.Number.Uint64()
}

func (s *Slot) GetHash() string {
	return s.Header.Hash.String()
}

func (s *Slot) GetParentHash() string {
	return s.Header.ParentHash.String()
}

func (s *Slot) Linked() bool {
	return true
}
