package sol

import (
	"sort"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// APIVersion is the version of the super node sol_* protocol.
const APIVersion = 1

// Block is the response of sol_getBlock. It is JSON-compatible with the driver's Block: a skipped
// slot is encoded with a nil GetBlockResult so that the driver's Block.Skipped() reports true.
type Block struct {
	Slot uint64
	*rpc.GetBlockResult
}

// ParsedBlock is the response of sol_getBlockTransactions: the parsed transactions of a single slot.
type ParsedBlock struct {
	BlockTime    *solana.UnixTimeSeconds     `json:"blockTime"`
	Transactions []ParsedTransactionWithMeta `json:"transactions"`
}

// FindTransactionsParam is the param of sol_findTransactions: find the signatures of every
// transaction in [FromBlock, ToBlock] that references Address.
type FindTransactionsParam struct {
	FromBlock uint64           `json:"fromBlock"`
	ToBlock   uint64           `json:"toBlock"`
	Address   solana.PublicKey `json:"address"`
	Limit     int              `json:"limit"`
}

// GetContractStartBlockParam is the param of sol_getContractStartBlock.
type GetContractStartBlockParam struct {
	Address solana.PublicKey `json:"address"`
	Start   uint64           `json:"start"`
	Latest  uint64           `json:"latest"`
}

// GetContractStartBlockResult is the response of sol_getContractStartBlock. Slot is the first slot
// in [Start, Latest] that references Address; Found is false when Address never appears.
type GetContractStartBlockResult struct {
	Slot  uint64 `json:"slot"`
	Found bool   `json:"found"`
}

// CollectAccountKeys returns the deduplicated, sorted set of account public keys referenced by a
// parsed transaction (top-level and inner instruction program ids and accounts plus the message
// account keys). It mirrors the addresses that getSignaturesForAddress would match, and is used to
// index transactions for address lookups.
func CollectAccountKeys(tx *rpc.ParsedTransaction, meta *rpc.ParsedTransactionMeta) []string {
	if tx == nil {
		return nil
	}
	keys := make(map[string]struct{})
	add := func(k solana.PublicKey) {
		keys[k.String()] = struct{}{}
	}
	for _, ak := range tx.Message.AccountKeys {
		add(ak.PublicKey)
	}
	collect := func(instructions []*rpc.ParsedInstruction) {
		for _, in := range instructions {
			if in == nil {
				continue
			}
			add(in.ProgramId)
			for _, a := range in.Accounts {
				add(a)
			}
		}
	}
	collect(tx.Message.Instructions)
	if meta != nil {
		for _, inner := range meta.InnerInstructions {
			collect(inner.Instructions)
		}
	}
	result := make([]string, 0, len(keys))
	for k := range keys {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// InvolvesAddress reports whether the given parsed transaction references address.
func InvolvesAddress(tx *rpc.ParsedTransaction, meta *rpc.ParsedTransactionMeta, address solana.PublicKey) bool {
	target := address.String()
	for _, k := range CollectAccountKeys(tx, meta) {
		if k == target {
			return true
		}
	}
	return false
}
