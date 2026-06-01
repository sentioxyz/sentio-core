package supernode

import (
	"context"
	"encoding/json"
	"math"
	"sort"

	"github.com/gagliardetto/solana-go"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/common/jsonrpc"
	rg "sentioxyz/sentio-core/common/range"
)

// NewSuperNode builds the middleware chain serving the sol_* data methods from the latest-slot
// cache and ClickHouse. Unrecognized methods fall through to the JSON-RPC proxy so the super node
// still answers raw Solana requests from other callers. The HTTP proxy fallback is appended by the
// launcher (BuildSolMiddlewares), matching the evm/sui super nodes.
// Per-query result caps. When a query in [from, to] would exceed its cap the super node returns an
// error, which the driver treats like any fetch error: it halves the request range and retries. The
// single exception is sol_findTransactions on a single block (from == to), where the range cannot
// be shrunk further, so all matching transactions are returned regardless of the cap.
const (
	maxIntervalBlocks   = 500
	maxFindTransactions = 1000
)

func NewSuperNode(
	client *sol.ClientPool,
	slotCache chain.LatestSlotCache[*sol.Slot],
	rangeStore chain.RangeStore,
	store Storage,
) []jsonrpc.Middleware {
	svc := &RPCService{
		client:     client,
		slotCache:  slotCache,
		rangeStore: rangeStore,
		store:      store,
	}
	return []jsonrpc.Middleware{
		func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
				switch method {
				case "sol_getLatestBlockNumber":
					return svc.GetLatestBlockNumber(ctx)
				case "sol_getBlock":
					return jsonrpc.CallMethod(svc.GetBlock, ctx, params)
				case "sol_getBlocksByInterval":
					return jsonrpc.CallMethod(svc.GetBlocksByInterval, ctx, params)
				case "sol_findTransactions":
					return jsonrpc.CallMethod(svc.FindTransactions, ctx, params)
				case "sol_getContractStartBlock":
					return jsonrpc.CallMethod(svc.GetContractStartBlock, ctx, params)
				default:
					return next(ctx, method, params)
				}
			}
		},
		jsonrpc.NewJSONRPCProxyMiddleware(client.ClientPool),
	}
}

type RPCService struct {
	client     *sol.ClientPool
	slotCache  chain.LatestSlotCache[*sol.Slot]
	rangeStore chain.RangeStore
	store      Storage
}

func (s *RPCService) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	r, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return 0, err
	}
	if r.End == nil {
		return 0, errors.Errorf("latest slot is not ready")
	}
	return *r.End, nil
}

// GetBlock returns the header (without signatures) of a slot, from the cache then ClickHouse.
func (s *RPCService) GetBlock(ctx context.Context, slot uint64) (sol.Block, error) {
	blocks, err := chain.QueryRangeWithCache[*sol.Slot, sol.Block](
		ctx,
		rg.NewSingleRange(slot),
		s.slotCache,
		func(st *sol.Slot) ([]sol.Block, error) {
			return []sol.Block{st.ToBlock(false)}, nil
		},
		chain.CheckRange(s.rangeStore, func(ctx context.Context, queryRange rg.Range) ([]sol.Block, error) {
			block, queryErr := s.store.QueryBlock(ctx, queryRange.Start)
			if queryErr != nil {
				return nil, queryErr
			}
			return []sol.Block{*block}, nil
		}),
	)
	if err != nil {
		return sol.Block{}, err
	}
	if len(blocks) == 0 {
		return sol.Block{}, chain.ErrSlotNotFound
	}
	return blocks[0], nil
}

// GetBlocksByInterval returns the first non-skipped block (with signatures) of each window within
// [From, To], at most Limit blocks. A single QueryRangeWithCache traversal picks the first block per
// window from the latest-slot cache and queries ClickHouse for the range below the cache; results
// are deduplicated by window keeping the earliest slot. The window straddling From's left boundary
// is dropped when its first block lies before From (it belongs to an earlier page); this is decided
// after the cache/ClickHouse merge because that window's first block may live in either layer.
//
// For a block window the scan is left-extended to the window start so the boundary window's true
// first block is found in the same scan and dropped by the From filter — no extra query. For a time
// window the window start is not a slot, so the boundary is decided by comparing the first result
// with the nearest non-skipped block before From.
func (s *RPCService) GetBlocksByInterval(
	ctx context.Context,
	param sol.GetBlocksByIntervalParam,
) ([]sol.Block, error) {
	if param.To < param.From {
		return nil, errors.Errorf("to %d cannot be less than from %d", param.To, param.From)
	}
	if param.Window.BlockWindow == 0 && param.Window.TimeWindow == 0 {
		return nil, errors.Errorf("interval window is empty")
	}

	scanFrom := param.From
	if param.Window.IsBlockWindow() {
		scanFrom = param.From / param.Window.BlockWindow * param.Window.BlockWindow
	}

	// seen dedups within the cache traversal (best effort; correctness comes from the final
	// min-slot dedup below).
	seen := make(map[uint64]struct{})
	blocks, err := chain.QueryRangeWithCache[*sol.Slot, sol.Block](
		ctx,
		rg.NewRange(scanFrom, param.To),
		s.slotCache,
		func(st *sol.Slot) ([]sol.Block, error) {
			if st.Skipped {
				return nil, nil
			}
			key := param.Window.Key(st.SlotNumber, st.BlockTime)
			if _, has := seen[key]; has {
				return nil, nil
			}
			seen[key] = struct{}{}
			return []sol.Block{st.ToBlock(true)}, nil
		},
		chain.CheckRange(s.rangeStore, func(ctx context.Context, queryRange rg.Range) ([]sol.Block, error) {
			// maxIntervalBlocks+1 lets the merge detect an over-cap range even after the boundary
			// window (first result) is dropped below.
			return s.store.QueryBlocksByInterval(ctx, queryRange.Start, *queryRange.End, param.Window, maxIntervalBlocks+1)
		}),
	)
	if err != nil {
		return nil, err
	}

	// Keep the earliest block per window across the cache/ClickHouse merge.
	blockByWindow := make(map[uint64]sol.Block)
	for _, b := range blocks {
		if b.GetBlockResult == nil {
			continue
		}
		key := param.Window.Key(b.Slot, b.BlockTime)
		if cur, has := blockByWindow[key]; !has || b.Slot < cur.Slot {
			blockByWindow[key] = b
		}
	}
	result := make([]sol.Block, 0, len(blockByWindow))
	for _, b := range blockByWindow {
		result = append(result, b)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Slot < result[j].Slot })

	if param.Window.IsBlockWindow() {
		// The left-extended scan computed the boundary window's true first block; drop it (and any
		// other extended block) when it precedes From — it belongs to an earlier page.
		filtered := result[:0]
		for _, b := range result {
			if b.Slot >= param.From {
				filtered = append(filtered, b)
			}
		}
		result = filtered
	} else if len(result) > 0 && param.From > 0 && result[0].GetBlockResult != nil {
		// Time window: the first window straddles From's left edge iff the nearest non-skipped block
		// before From is in the same window; if so it belongs to an earlier page.
		prevSlot, prevTime, found, prevErr := s.previousUnskippedBlock(ctx, param.From)
		if prevErr != nil {
			return nil, prevErr
		}
		if found && param.Window.Key(prevSlot, prevTime) == param.Window.Key(result[0].Slot, result[0].BlockTime) {
			result = result[1:]
		}
	}

	// Too many interval blocks: signal the caller to shrink the range (the minimum fetch range is
	// maxIntervalBlocks, where at most one block per slot can be a target, so this is reachable).
	if len(result) > maxIntervalBlocks {
		return nil, errors.Errorf("too many interval blocks (> %d) in slot range [%d, %d]",
			maxIntervalBlocks, param.From, param.To)
	}
	return result, nil
}

// previousUnskippedBlock returns the nearest non-skipped block (slot and time) with slot < from,
// looking in the latest-slot cache first and then ClickHouse. A single cache traversal is used (and
// its returned range reused) so the cache window cannot slide between reading its range and its
// blocks.
func (s *RPCService) previousUnskippedBlock(
	ctx context.Context,
	from uint64,
) (slot uint64, blockTime *solana.UnixTimeSeconds, found bool, err error) {
	if from == 0 {
		return 0, nil, false, nil
	}
	var cacheSlot uint64
	var cacheTime *solana.UnixTimeSeconds
	var cacheFound bool
	// Traverse clamps [0, from-1] to the cached suffix; ascending order means the last non-skipped
	// block seen is the nearest one before from.
	cachedRange, err := s.slotCache.Traverse(ctx, rg.NewRange(0, from-1),
		func(ctx context.Context, st *sol.Slot) error {
			if !st.Skipped {
				cacheSlot, cacheTime, cacheFound = st.SlotNumber, st.BlockTime, true
			}
			return nil
		})
	if err != nil {
		return 0, nil, false, err
	}
	if cacheFound {
		return cacheSlot, cacheTime, true, nil
	}
	// The cache has no non-skipped block before from; look in ClickHouse below the cached range.
	before := from
	if !cachedRange.IsEmpty() {
		before = cachedRange.Start
	}
	return s.store.QueryPreviousUnskipped(ctx, before)
}

// FindTransactions returns, grouped by block, the transactions in [From, To] that invoke any of the
// given programs, from the cache then ClickHouse.
func (s *RPCService) FindTransactions(
	ctx context.Context,
	param sol.FindTransactionsParam,
) ([]sol.BlockTransactions, error) {
	if param.To < param.From {
		return nil, errors.Errorf("to %d cannot be less than from %d", param.To, param.From)
	}
	// A single-block range cannot be shrunk further, so it returns all matching transactions even
	// beyond the cap; otherwise cap the store query so an over-cap range can be detected.
	singleBlock := param.From == param.To
	storeLimit := maxFindTransactions + 1
	if singleBlock {
		storeLimit = math.MaxInt32
	}
	programSet := param.ProgramSet()
	result, err := chain.QueryRangeWithCache[*sol.Slot, sol.BlockTransactions](
		ctx,
		rg.NewRange(param.From, param.To),
		s.slotCache,
		func(st *sol.Slot) ([]sol.BlockTransactions, error) {
			matching := st.MatchingTransactions(programSet)
			if len(matching) == 0 {
				return nil, nil
			}
			return []sol.BlockTransactions{{
				Slot:              st.SlotNumber,
				Blockhash:         st.Blockhash,
				PreviousBlockhash: st.PreviousBlockhash,
				BlockTime:         st.BlockTime,
				Transactions:      matching,
			}}, nil
		},
		chain.CheckRange(s.rangeStore, func(ctx context.Context, queryRange rg.Range) ([]sol.BlockTransactions, error) {
			return s.store.FindTransactions(ctx, queryRange.Start, *queryRange.End, param.ProgramIDs, storeLimit)
		}),
	)
	if err != nil {
		return nil, err
	}

	if !singleBlock {
		total := 0
		for _, b := range result {
			total += len(b.Transactions)
		}
		// Too many transactions: signal the caller to shrink the range.
		if total > maxFindTransactions {
			return nil, errors.Errorf("too many transactions (> %d) in slot range [%d, %d]",
				maxFindTransactions, param.From, param.To)
		}
	}
	return result, nil
}

// GetContractStartBlock returns the earliest block at which address (a program) appears, over all
// available data. The caller maps this against its own start/latest range (clamp to start, treat an
// appearance after latest or no appearance as out of range). ClickHouse holds the older history, so
// its earliest is the global earliest when present; the cache is consulted only when the program is
// absent from ClickHouse (a brand-new program not yet synced).
func (s *RPCService) GetContractStartBlock(
	ctx context.Context,
	address solana.PublicKey,
) (sol.GetContractStartBlockResult, error) {
	chMin, chFound, err := s.store.EarliestProgramSlot(ctx, address)
	if err != nil {
		return sol.GetContractStartBlockResult{}, err
	}
	if chFound {
		return sol.GetContractStartBlockResult{Slot: chMin, Found: true}, nil
	}
	programSet := map[string]struct{}{address.String(): {}}
	var cacheMin uint64
	var cacheFound bool
	if _, err = s.slotCache.Traverse(ctx, rg.Range{},
		func(ctx context.Context, st *sol.Slot) error {
			if !cacheFound && !st.Skipped && st.InvokesAnyProgram(programSet) {
				cacheMin, cacheFound = st.SlotNumber, true
			}
			return nil
		}); err != nil {
		return sol.GetContractStartBlockResult{}, err
	}
	return sol.GetContractStartBlockResult{Slot: cacheMin, Found: cacheFound}, nil
}
