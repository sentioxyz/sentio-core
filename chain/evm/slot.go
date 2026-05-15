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

	// HaveTrace indicates whether trace data was successfully loaded for this block.
	// It is false when:
	//   - trace is disabled for this chain (DisableTrace option is set)
	//   - trace loading failed and missTraceDowngrade is configured (error is suppressed to allow GetSlot to succeed)
	//   - trace is temporarily disabled due to a recent failure (disableTraceUntil has not expired)
	// When false, TraceFilter and TraceFilterPacked will return an error for this block.
	HaveTrace bool
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

const featureMissTrace = "MissTrace"

func (s *Slot) Features() []string {
	if !s.HaveTrace {
		return []string{featureMissTrace}
	}
	return nil
}

func (s *Slot) Linked() bool {
	return true
}
