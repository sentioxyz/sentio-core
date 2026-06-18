package sol

import (
	"context"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"

	solcore "sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

const (
	// targetKeepBytes is the buffered-data backpressure target for the main-data fetchers.
	// BlockMainData.Size() estimates real memory use (full transactions can be large), so this is
	// sized in bytes, not in block/transaction counts.
	targetKeepBytes = 100 * 1024 * 1024 // 100MB

	// targetQueryBytes is the per-query data-size target the fetcher grows a range toward, so each
	// query fetches roughly this much data.
	targetQueryBytes = 10 * 1024 * 1024 // 10MB

	// intervalMinQuerySize is the smallest interval fetch range. It matches the super node's
	// per-query block cap: GetBlocksByInterval returns at most one block per slot, so a range this
	// size can always fit under the cap, letting the fetcher shrink down to it on an over-cap error.
	intervalMinQuerySize = 500
)

// DataRequirement is the data demand of all handlers of one processor.
type DataRequirement struct {
	// Tx is the instruction handlers' requirements (one per handler, merged into disjoint ranges
	// when building the fetchers).
	Tx []TransactionRequirement
	// Interval is the interval handlers' requirements.
	Interval []data.IntervalRequirement
}

func (r DataRequirement) Snapshot() any {
	txs := make([]any, len(r.Tx))
	for i, t := range r.Tx {
		txs[i] = t.Snapshot()
	}
	intervals := make([]any, len(r.Interval))
	for i, iv := range r.Interval {
		intervals[i] = iv.Snapshot()
	}
	return map[string]any{
		"tx":        txs,
		"intervals": intervals,
	}
}

// BlockMainData is the per-block data produced by the main-data fetchers and consumed directly by
// the handlers; it already carries everything needed to build the binding data, so the block-data
// phase no longer fetches anything.
type BlockMainData struct {
	// Slot / Blockhash / PreviousBlockhash / BlockTime are the block header, always set when data
	// is present, so the block data can be built without a separate getBlock call.
	Slot              uint64
	Blockhash         string
	PreviousBlockhash string
	BlockTime         *solana.UnixTimeSeconds
	// Intervals are the interval configs this block is a target of (interval handler).
	Intervals []data.IntervalConfig
	// Block is the block header with signatures (interval handler), nil when not an interval target.
	Block *Block
	// Transactions are the full transactions invoking the requested programs (instruction handler).
	Transactions []solcore.WrappedTransaction
}

func (d BlockMainData) IsEmpty() bool {
	return len(d.Intervals) == 0 && d.Block == nil && len(d.Transactions) == 0
}

func (d BlockMainData) Size() int {
	size := 0
	if d.Block != nil && d.Block.GetBlockResult != nil {
		size += len(d.Block.Signatures) * 90
	}
	for _, tx := range d.Transactions {
		size += estimateTransactionSize(tx)
	}
	return size
}

func estimateTransactionSize(tx solcore.WrappedTransaction) int {
	size := 256 // base overhead for signature/version/envelope
	if tx.Transaction != nil {
		for _, in := range tx.Transaction.Message.Instructions {
			size += len(in.Data) + len(in.Accounts)*45
		}
	}
	if tx.Meta != nil {
		for _, inner := range tx.Meta.InnerInstructions {
			for _, in := range inner.Instructions {
				size += len(in.Data) + len(in.Accounts)*45
			}
		}
	}
	return size
}

func windowsOf(cfg data.IntervalConfig) (backfill, watching solcore.IntervalWindow) {
	if cfg.BlockInterval != nil {
		return solcore.IntervalWindow{BlockWindow: cfg.BlockInterval.Backfill},
			solcore.IntervalWindow{BlockWindow: cfg.BlockInterval.Watching}
	}
	return solcore.IntervalWindow{TimeWindow: cfg.TimeInterval.Backfill},
		solcore.IntervalWindow{TimeWindow: cfg.TimeInterval.Watching}
}

func BuildIntervalFetcher(
	name string,
	req data.IntervalRequirement,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client Client,
) controller.Fetcher[BlockMainData] {
	backfill, watching := windowsOf(req.IntervalConfig)
	return fetcher.NewFetcher[BlockMainData](
		name,
		req,
		controller.BlockRange{
			StartBlock: max(currentBlockNumber, req.StartBlock),
			EndBlock:   req.EndBlock,
		},
		latest,
		// minQuerySize is intervalMinQuerySize (not maxQuerySize) so the fetcher can shrink down to
		// a range the super node's block cap can always satisfy when it returns an over-cap error.
		intervalMinQuerySize,
		10000,
		targetKeepBytes,
		targetQueryBytes,
		time.Second*15,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			result := make(map[uint64]BlockMainData)
			// Builds the interval handler's binding data from each window's first block. Only block
			// header fields are used (slot, blockhash, parent hash, time). NOTE: blocks served from
			// the BigQuery archival tier (slots below the ClickHouse range) carry NO transaction
			// signatures — a deliberate BigQuery cost optimization (see GetBlocksByInterval and
			// the archival store). That is fine here because interval binding data is header-only; do not
			// add a dependency on b's transaction signatures in this path.
			add := func(blocks []Block) {
				for i := range blocks {
					b := blocks[i]
					if b.Skipped() {
						continue
					}
					bd := result[b.Slot]
					if !data.ContainsInterval(bd.Intervals, req.IntervalConfig) {
						bd.Intervals = append(bd.Intervals, req.IntervalConfig)
					}
					bd.Slot = b.Slot
					bd.Blockhash = b.GetBlockHash()
					bd.PreviousBlockhash = b.GetBlockParentHash()
					bd.BlockTime = b.BlockTime
					bd.Block = &b
					result[b.Slot] = bd
				}
			}
			// The super node caps the result and errors when exceeded; that error propagates and the
			// fetcher halves the range, so no client-side saturation check is needed.
			backfillBlocks, err := client.GetBlocksByInterval(ctx, start, end, backfill)
			if err != nil {
				return nil, err
			}
			add(backfillBlocks)
			// The finer "watching" window only applies near the head. Like data.QueryInterval, decide
			// by time: the range is "in watching" when end's block time is within WatchingDelay of the
			// latest. end may itself be skipped, so look up the nearest non-skipped block at or before
			// end (slot < end+1) for its time; if there is none we conservatively run the watching
			// query and rely on the per-block time filter below.
			if watching != backfill {
				inWatching := true
				endBlock, getErr := client.GetPreviousUnskippedBlock(ctx, end+1)
				if getErr != nil {
					return nil, getErr
				}
				if endBlock.Found && endBlock.BlockTime != nil {
					inWatching = latest.GetBlockTime().Sub(endBlock.BlockTime.Time()) < controller.WatchingDelay
				}
				if inWatching {
					watchingBlocks, err := client.GetBlocksByInterval(ctx, start, end, watching)
					if err != nil {
						return nil, err
					}
					// Keep only blocks whose own time is within WatchingDelay of the latest.
					cutoff := latest.GetBlockTime().Add(-controller.WatchingDelay)
					inWatch := make([]Block, 0, len(watchingBlocks))
					for _, b := range watchingBlocks {
						if !b.Skipped() && b.BlockTime != nil && b.BlockTime.Time().After(cutoff) {
							inWatch = append(inWatch, b)
						}
					}
					add(inWatch)
				}
			}
			return result, nil
		},
	)
}

func BuildBlockMainDataFetcher(
	namePrefix string,
	req DataRequirement,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client Client,
) controller.Fetcher[BlockMainData] {
	req.Tx = MergeTxRequirements(currentBlockNumber, req.Tx)
	req.Interval = data.MergeIntervalRequirements(req.Interval)
	var fetchers []controller.Fetcher[BlockMainData]
	for i, r := range req.Tx {
		fetchers = append(fetchers, BuildTxFetcher(
			namePrefix+fmt.Sprintf("TxFetcher#%d", i), r, currentBlockNumber, latest, client))
	}
	for i, r := range req.Interval {
		fetchers = append(fetchers, BuildIntervalFetcher(
			namePrefix+fmt.Sprintf("IntervalFetcher#%d", i), r, currentBlockNumber, latest, client))
	}
	return fetcher.MergeIsomorphicFetchers(
		namePrefix+"MainDataFetcher",
		req,
		fetchers,
		func(bn uint64, from []BlockMainData) (result BlockMainData, has bool, _ error) {
			result.Slot = bn
			for _, box := range from {
				if box.Blockhash != "" {
					result.Blockhash = box.Blockhash
					result.PreviousBlockhash = box.PreviousBlockhash
				}
				if box.BlockTime != nil {
					result.BlockTime = box.BlockTime
				}
				result.Intervals = append(result.Intervals, box.Intervals...)
				if box.Block != nil {
					result.Block = box.Block
				}
				result.Transactions = append(result.Transactions, box.Transactions...)
			}
			return result, !result.IsEmpty(), nil
		})
}
