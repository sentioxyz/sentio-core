package sol

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"sentioxyz/sentio-core/chain/chain"
)

// ParsedTransactionWithMeta is one transaction inside a parsed block, as returned by the
// jsonParsed/full form of the getBlock RPC. Version distinguishes legacy from versioned (v0)
// transactions and is required to fully reconstruct the transaction.
type ParsedTransactionWithMeta struct {
	Transaction *rpc.ParsedTransaction     `json:"transaction"`
	Meta        *rpc.ParsedTransactionMeta `json:"meta"`
	Version     rpc.TransactionVersion     `json:"version"`
}

// Slot is the sentio-core representation of a Solana slot (block) carrying the full parsed
// transactions, so the super node can both persist it to ClickHouse and serve it from the
// latest-slot cache. A skipped slot is represented with Skipped=true and no transactions.
type Slot struct {
	SlotNumber        uint64                      `json:"slot"`
	Skipped           bool                        `json:"skipped"`
	Blockhash         solana.Hash                 `json:"blockhash"`
	PreviousBlockhash solana.Hash                 `json:"previousBlockhash"`
	ParentSlot        uint64                      `json:"parentSlot"`
	BlockHeight       *uint64                     `json:"blockHeight"`
	BlockTime         *solana.UnixTimeSeconds     `json:"blockTime"`
	Transactions      []ParsedTransactionWithMeta `json:"transactions"`
}

var _ chain.Slot = (*Slot)(nil)

func (s *Slot) GetNumber() uint64 {
	return s.SlotNumber
}

func (s *Slot) GetHash() string {
	if s.Skipped {
		return ""
	}
	return s.Blockhash.String()
}

func (s *Slot) GetParentHash() string {
	if s.Skipped {
		return ""
	}
	return s.PreviousBlockhash.String()
}

func (s *Slot) Features() []string {
	return nil
}

// Linked returns false because Solana slots are frequently skipped, so the parent-hash chain
// across consecutive slot numbers is not contiguous and must not be link-checked.
func (s *Slot) Linked() bool {
	return false
}

// ToBlock builds the sol_getBlock response (block header + transaction signatures) for this slot.
// A skipped slot yields a Block with a nil GetBlockResult, matching the driver's Block.Skipped().
func (s *Slot) ToBlock() Block {
	if s.Skipped {
		return Block{Slot: s.SlotNumber}
	}
	return Block{
		Slot: s.SlotNumber,
		GetBlockResult: &rpc.GetBlockResult{
			Blockhash:         s.Blockhash,
			PreviousBlockhash: s.PreviousBlockhash,
			ParentSlot:        s.ParentSlot,
			BlockTime:         s.BlockTime,
			BlockHeight:       s.BlockHeight,
			Signatures:        s.Signatures(),
		},
	}
}

// ToParsedBlock builds the sol_getBlockTransactions response for this slot.
func (s *Slot) ToParsedBlock() ParsedBlock {
	return ParsedBlock{
		BlockTime:    s.BlockTime,
		Transactions: s.Transactions,
	}
}

// Signatures returns the first signature of every transaction in slot order.
func (s *Slot) Signatures() []solana.Signature {
	sigs := make([]solana.Signature, 0, len(s.Transactions))
	for _, tx := range s.Transactions {
		if tx.Transaction != nil && len(tx.Transaction.Signatures) > 0 {
			sigs = append(sigs, tx.Transaction.Signatures[0])
		}
	}
	return sigs
}
