package sol

import (
	"sort"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/pkg/errors"
)

// APIVersion is the version of the super node sol_* protocol. The driver compares it against the
// version reported by sol_getLatestHeader and restarts to upgrade when the super node is newer (see
// GetLatestHeaderResult.CheckAPIVersion), mirroring the sui super node.
const APIVersion = 1

// SimpleBlock is the minimal block header returned by sol_getLatestHeader: just enough to satisfy
// the driver's controller.BlockHeader (slot, hashes, time) without materializing a full block. Its
// JSON keys match Block's so the two header shapes are interchangeable on the wire.
type SimpleBlock struct {
	Slot              uint64                  `json:"slot"`
	Blockhash         solana.Hash             `json:"blockhash"`
	PreviousBlockhash solana.Hash             `json:"previousBlockhash"`
	BlockTime         *solana.UnixTimeSeconds `json:"blockTime"`
}

// NewSimpleBlock builds a SimpleBlock from a (non-skipped) slot.
func NewSimpleBlock(s *Slot) SimpleBlock {
	return SimpleBlock{
		Slot:              s.SlotNumber,
		Blockhash:         s.Blockhash,
		PreviousBlockhash: s.PreviousBlockhash,
		BlockTime:         s.BlockTime,
	}
}

func (b SimpleBlock) GetBlockNumber() uint64     { return b.Slot }
func (b SimpleBlock) GetBlockHash() string       { return b.Blockhash.String() }
func (b SimpleBlock) GetBlockParentHash() string { return b.PreviousBlockhash.String() }
func (b SimpleBlock) GetBlockTime() time.Time    { return b.BlockTime.Time() }

// GetLatestHeaderResult is the response of sol_getLatestHeader. It carries the latest non-skipped
// header (flattened SimpleBlock fields), the earliest slot the caller may index (FirstSlot: 0 when
// the caller may use the BigQuery archival tier, otherwise the start of the ClickHouse range), and
// the super node's APIVersion.
type GetLatestHeaderResult struct {
	SimpleBlock
	FirstSlot  uint64 `json:"firstSlot"`
	APIVersion int    `json:"apiVersion"`
}

// CheckAPIVersion reports whether the super node is newer than this client understands, in which case
// the driver should restart to upgrade. An older/absent version (e.g. a pre-SimpleBlock super node
// that returns 0) is always accepted.
func (r GetLatestHeaderResult) CheckAPIVersion() error {
	if r.APIVersion <= APIVersion {
		return nil
	}
	return errors.Errorf("remote sol api version %d is greater than %d", r.APIVersion, APIVersion)
}

// Block is the response of sol_getBlock / an element of sol_getBlocksByInterval. It is
// JSON-compatible with the driver's Block: a skipped slot is encoded with a nil GetBlockResult so
// that Block.Skipped() reports true. sol_getBlock fills only the header; sol_getBlocksByInterval
// additionally fills the transaction signatures.
type Block struct {
	Slot uint64 `json:"slot"`
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
// block of each window within [From, To], in ascending slot order. The window straddling From's left
// edge is attributed to the page that holds its first block, so it is not reported here when its
// first block lies before From. The super node caps the number of blocks and errors when exceeded.
type GetBlocksByIntervalParam struct {
	From   uint64         `json:"from"`
	To     uint64         `json:"to"`
	Window IntervalWindow `json:"window"`
}

// FindTransactionsParam is the param of sol_findTransactions: return, per block in [From, To], the
// full transactions that invoke any program in ProgramIDs. The super node caps the total number of
// transactions and errors when exceeded (except for a single-block range).
type FindTransactionsParam struct {
	From       uint64             `json:"from"`
	To         uint64             `json:"to"`
	ProgramIDs []solana.PublicKey `json:"programIds"`
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

// PreviousUnskippedBlock is the response of sol_getPreviousUnskippedBlock: the nearest non-skipped
// block with slot strictly less than the requested slot (Found is false when there is none).
type PreviousUnskippedBlock struct {
	Slot      uint64                  `json:"slot"`
	BlockTime *solana.UnixTimeSeconds `json:"blockTime"`
	Found     bool                    `json:"found"`
}

// GetContractStartBlockResult is the response of sol_getContractStartBlock. Slot is the earliest
// block (in the available data) at which the address is invoked as a program; Found is false when
// the address never appears. The caller maps this against its own start/latest range.
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
