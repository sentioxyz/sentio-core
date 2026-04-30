package sui

import (
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/sui/types"
)

type SlotCheckpointInfo struct {
	SequenceNumber     uint64       `json:"sequence_number"`
	Digest             string       `json:"digest"`
	TransactionDigests []string     `json:"transaction_digests,omitempty"`
	TimestampMs        types.Number `json:"timestamp_ms"`
}

type Slot struct {
	SlotCheckpointInfo
	Transactions   []types.TransactionResponseV1 `json:"transactions,omitempty"`
	GrpcCheckpoint *rpcv2.Checkpoint             `json:"grpc_checkpoint,omitempty"`
}

var _ chain.Slot = (*Slot)(nil)

func (s *Slot) GetNumber() uint64 {
	return s.SequenceNumber
}

func (s *Slot) GetHash() string {
	return s.Digest
}

func (s *Slot) GetParentHash() string {
	return ""
}

func (s *Slot) Linked() bool {
	return false
}
