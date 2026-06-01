package sol

import (
	"sort"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// APIVersion is the version of the super node sol_* protocol.
const APIVersion = 1

// Block is the response of sol_getBlock / an element of sol_getBlocksByInterval. It is
// JSON-compatible with the driver's Block: a skipped slot is encoded with a nil GetBlockResult so
// that Block.Skipped() reports true. sol_getBlock fills only the header; sol_getBlocksByInterval
// additionally fills the transaction signatures.
type Block struct {
	Slot uint64
	*rpc.GetBlockResult
}

// IntervalWindow defines how [from, to] is partitioned into windows for sol_getBlocksByInterval.
// Exactly one of BlockWindow / TimeWindow is set. The target of each window is its first
// non-skipped block.
type IntervalWindow struct {
	// BlockWindow groups slots by intDiv(slot, BlockWindow).
	BlockWindow uint64 `json:"blockWindow,omitempty"`
	// TimeWindow groups slots by intDiv(blockTimeUnixSeconds, TimeWindow/second).
	TimeWindow time.Duration `json:"timeWindow,omitempty"`
}

func (w IntervalWindow) IsBlockWindow() bool {
	return w.BlockWindow > 0
}

func (w IntervalWindow) TimeWindowSeconds() uint64 {
	return uint64(w.TimeWindow / time.Second)
}

// Key returns the window key of a slot with the given block time.
func (w IntervalWindow) Key(slot uint64, blockTime *solana.UnixTimeSeconds) uint64 {
	if w.IsBlockWindow() {
		return slot / w.BlockWindow
	}
	secs := w.TimeWindowSeconds()
	if secs == 0 || blockTime == nil {
		return 0
	}
	return uint64(int64(*blockTime)) / secs
}

// GetBlocksByIntervalParam is the param of sol_getBlocksByInterval: return the first non-skipped
// block of each window within [From, To], at most Limit blocks, in ascending slot order.
//
// GlobalFrom is the lower bound of the whole interval requirement (not just this paged sub-range).
// It lets the super node attribute the window straddling From to the correct page: that window is
// reported here only when it has no earlier non-skipped block in [GlobalFrom, From-1] — otherwise
// its first block lies in an earlier page and reporting it here would duplicate a "fake" block.
type GetBlocksByIntervalParam struct {
	From       uint64         `json:"from"`
	To         uint64         `json:"to"`
	GlobalFrom uint64         `json:"globalFrom"`
	Window     IntervalWindow `json:"window"`
	Limit      int            `json:"limit"`
}

// FindTransactionsParam is the param of sol_findTransactions: return, per block in [From, To], the
// full transactions that invoke any program in ProgramIDs, at most Limit transactions in total.
type FindTransactionsParam struct {
	From       uint64             `json:"from"`
	To         uint64             `json:"to"`
	ProgramIDs []solana.PublicKey `json:"programIds"`
	Limit      int                `json:"limit"`
}

func (p FindTransactionsParam) ProgramSet() map[string]struct{} {
	set := make(map[string]struct{}, len(p.ProgramIDs))
	for _, id := range p.ProgramIDs {
		set[id.String()] = struct{}{}
	}
	return set
}

// WrappedTransaction is a full parsed transaction with its in-block index, returned by
// sol_findTransactions.
type WrappedTransaction struct {
	TransactionIndex uint32                     `json:"transactionIndex"`
	Signature        solana.Signature           `json:"signature"`
	Version          rpc.TransactionVersion     `json:"version"`
	Transaction      *rpc.ParsedTransaction     `json:"transaction"`
	Meta             *rpc.ParsedTransactionMeta `json:"meta"`
}

// ToParsedTransactionResult builds the rpc.GetParsedTransactionResult shape the driver serializes
// as the raw transaction.
func (t WrappedTransaction) ToParsedTransactionResult(
	slot uint64,
	blockTime *solana.UnixTimeSeconds,
) *rpc.GetParsedTransactionResult {
	return &rpc.GetParsedTransactionResult{
		Slot:        slot,
		BlockTime:   blockTime,
		Transaction: t.Transaction,
		Meta:        t.Meta,
		Version:     t.Version,
	}
}

// BlockTransactions groups the matching transactions of one block, returned by sol_findTransactions.
// The block header (hash/parentHash/time) is included so the driver can build the block data
// without a separate getBlock call.
type BlockTransactions struct {
	Slot              uint64                  `json:"slot"`
	Blockhash         solana.Hash             `json:"blockhash"`
	PreviousBlockhash solana.Hash             `json:"previousBlockhash"`
	BlockTime         *solana.UnixTimeSeconds `json:"blockTime"`
	Transactions      []WrappedTransaction    `json:"transactions"`
}

// GetContractStartBlockParam is the param of sol_getContractStartBlock.
type GetContractStartBlockParam struct {
	Address solana.PublicKey `json:"address"`
	Start   uint64           `json:"start"`
	Latest  uint64           `json:"latest"`
}

// GetContractStartBlockResult is the response of sol_getContractStartBlock. Slot is the first slot
// in [Start, Latest] that invokes Address; Found is false when Address never appears.
type GetContractStartBlockResult struct {
	Slot  uint64 `json:"slot"`
	Found bool   `json:"found"`
}

// CollectProgramIDs returns the deduplicated, sorted set of program ids invoked by a parsed
// transaction (top-level and inner instructions). It is the index used for instruction lookups.
func CollectProgramIDs(tx *rpc.ParsedTransaction, meta *rpc.ParsedTransactionMeta) []string {
	if tx == nil {
		return nil
	}
	programs := make(map[string]struct{})
	collect := func(instructions []*rpc.ParsedInstruction) {
		for _, in := range instructions {
			if in != nil {
				programs[in.ProgramId.String()] = struct{}{}
			}
		}
	}
	collect(tx.Message.Instructions)
	if meta != nil {
		for _, inner := range meta.InnerInstructions {
			collect(inner.Instructions)
		}
	}
	result := make([]string, 0, len(programs))
	for p := range programs {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}

func txInvokesAnyProgram(tx ParsedTransactionWithMeta, programs map[string]struct{}) bool {
	for _, p := range CollectProgramIDs(tx.Transaction, tx.Meta) {
		if _, has := programs[p]; has {
			return true
		}
	}
	return false
}
