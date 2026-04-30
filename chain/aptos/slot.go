package aptos

import (
	"github.com/aptos-labs/aptos-go-sdk/api"
)

type Slot api.Block

func (s *Slot) GetNumber() uint64 {
	return s.BlockHeight
}

func (s *Slot) GetHash() string {
	return s.BlockHash
}

func (s *Slot) GetParentHash() string {
	return ""
}

func (s *Slot) Linked() bool {
	return false
}
