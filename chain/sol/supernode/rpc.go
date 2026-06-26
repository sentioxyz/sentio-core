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
	// maxQuerySpan caps the slot span (To - From) of a single range query, independent of how many
	// blocks/transactions it returns: a very wide range forces ClickHouse to scan many granules even
	// when the result is small, so the super node rejects it and the driver shrinks the range.
	maxQuerySpan = 100000
)

// NewSuperNode wires the sol_* handlers. store is the ClickHouse-backed Storage; bqStore is an
// optional lower-priority Storage (BigQuery) consulted only for slots below the ClickHouse range
// (older archival history). Pass bqStore == nil to disable the BigQuery tier (original behavior).
func NewSuperNode(
	client *sol.ClientPool,
	slotCache chain.LatestSlotCache[*sol.Slot],
	rangeStore chain.RangeStore,
	store Storage,
	bqStore Storage,
) []jsonrpc.Middleware {
	svc := &RPCService{
		client:     client,
		slotCache:  slotCache,
		rangeStore: rangeStore,
		store:      store,
		bqStore:    bqStore,
	}
	return []jsonrpc.Middleware{
		func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
				switch method {
				case "sol_getLatestHeader":
					return jsonrpc.CallMethod(svc.GetLatestHeader, ctx, params)
				case "sol_getBlock":
					return jsonrpc.CallMethod(svc.GetBlock, ctx, params)
				case "sol_getBlocksByInterval":
					return jsonrpc.CallMethod(svc.GetBlocksByInterval, ctx, params)
				case "sol_findTransactions":
					return jsonrpc.CallMethod(svc.FindTransactions, ctx, params)
				case "sol_getContractStartBlock":
					return jsonrpc.CallMethod(svc.GetContractStartBlock, ctx, params)
				case "sol_getPreviousUnskippedBlock":
					return jsonrpc.CallMethod(svc.GetPreviousUnskippedBlock, ctx, params)
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
	// bqStore is the optional lowest-priority store (BigQuery) for slots below the ClickHouse range.
	// nil when the BigQuery tier is not configured.
	bqStore Storage
}

// bqBlockLoader returns a single-slot block loader backed by BigQuery for use as the fallback of
// CheckRangeWithFallback in GetBlock, or nil when the BigQuery tier is disabled.
func (s *RPCService) bqBlockLoader() func(context.Context, rg.Range) ([]sol.Block, error) {
	if s.bqStore == nil {
		return nil
	}
	return func(ctx context.Context, queryRange rg.Range) ([]sol.Block, error) {
		block, err := s.bqStore.QueryBlock(ctx, queryRange.Start)
		if err != nil {
			return nil, err
		}
		return []sol.Block{*block}, nil
	}
}

// bqIntervalLoader returns the BigQuery fallback for GetBlocksByInterval, or nil when disabled.
//
// NOTE: blocks served from the BigQuery tier (for the slot range below ClickHouse) carry headers
// only — no transaction signatures — as a deliberate BigQuery cost optimization (attaching them
// scans whole Transactions DAY partitions; see bq.Store.QueryBlocksByInterval). Interval/sampling
// callers on the archival range must not rely on per-block signatures.
func (s *RPCService) bqIntervalLoader(
	param sol.GetBlocksByIntervalParam,
	limit int,
) func(context.Context, rg.Range) ([]sol.Block, error) {
	if s.bqStore == nil {
		return nil
	}
	return func(ctx context.Context, queryRange rg.Range) ([]sol.Block, error) {
		return s.bqStore.QueryBlocksByInterval(ctx, queryRange.Start, *queryRange.End, param.Window, limit)
	}
}

// bqFindTxLoader returns the BigQuery fallback for FindTransactions, or nil when disabled.
func (s *RPCService) bqFindTxLoader(
	param sol.FindTransactionsParam,
	storeLimit int,
) func(context.Context, rg.Range) ([]sol.BlockTransactions, error) {
	if s.bqStore == nil {
		return nil
	}
	return func(ctx context.Context, queryRange rg.Range) ([]sol.BlockTransactions, error) {
		return s.bqStore.FindTransactions(ctx, queryRange.Start, *queryRange.End, param.ProgramIDs, storeLimit)
	}
}

// permissionGated is implemented by a Storage that restricts access per caller (the BigQuery
// archival tier). The super node uses it to decide, without running a query, whether the caller may
// reach the older history below the ClickHouse range.
type permissionGated interface {
	CheckPermission(ctx context.Context) error
}

// callerMayUseArchive reports whether the caller in ctx may use the BigQuery archival tier: the tier
// must be configured and, when it gates access per caller, must permit this caller.
func (s *RPCService) callerMayUseArchive(ctx context.Context) bool {
	if s.bqStore == nil {
		return false
	}
	if pg, ok := s.bqStore.(permissionGated); ok {
		return pg.CheckPermission(ctx) == nil
	}
	return true
}

// firstSlot is the earliest slot the caller may index: 0 when it may use the BigQuery archival tier
// (the full history is reachable), otherwise the start of the ClickHouse range (the oldest slot the
// caller can actually read). The driver uses it to clamp the agent's start block.
func (s *RPCService) firstSlot(ctx context.Context) (uint64, error) {
	if s.callerMayUseArchive(ctx) {
		return 0, nil
	}
	r, err := s.rangeStore.Get(ctx)
	if err != nil {
		return 0, err
	}
	return r.Start, nil
}

// GetLatestHeader blocks until the latest-slot cache holds a non-skipped block with slot strictly
// greater than slotGt, then returns its header as a SimpleBlock (no signatures), along with the
// caller's first indexable slot and the super node's APIVersion. This long-poll lets the driver
// subscribe by waiting instead of polling (data.SubscribeUsingWaiting). The head slot may itself be
// skipped, so the latest non-skipped block at or below the cache head is returned; when the newest
// slots are all skipped (or none is yet beyond slotGt) it waits for the cache to advance before
// re-checking, so it never busy-loops.
func (s *RPCService) GetLatestHeader(ctx context.Context, slotGt uint64) (sol.GetLatestHeaderResult, error) {
	jsonrpc.GetCtxData(ctx).NotSlowRequest = true
	first, err := s.firstSlot(ctx)
	if err != nil {
		return sol.GetLatestHeaderResult{}, err
	}
	waitGt := slotGt
	for {
		latest, err := s.slotCache.Wait(ctx, waitGt)
		if err != nil {
			return sol.GetLatestHeaderResult{}, err
		}
		head, err := s.latestUnskipped(ctx)
		if err != nil {
			return sol.GetLatestHeaderResult{}, err
		}
		if head != nil && head.SlotNumber > slotGt {
			return sol.GetLatestHeaderResult{
				SimpleBlock: sol.NewSimpleBlock(head),
				FirstSlot:   first,
				APIVersion:  sol.APIVersion,
			}, nil
		}
		waitGt = latest
	}
}

// latestUnskipped returns the newest non-skipped slot in the latest-slot cache (nil when the cache
// is empty or holds only skipped slots). The cache is dense and ascending, so the last non-skipped
// slot seen during a traversal is the newest one.
func (s *RPCService) latestUnskipped(ctx context.Context) (*sol.Slot, error) {
	var head *sol.Slot
	_, err := s.slotCache.Traverse(ctx, rg.Range{}, func(ctx context.Context, st *sol.Slot) error {
		if !st.Skipped {
			head = st
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return head, nil
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
		chain.CheckRangeWithFallback(s.rangeStore,
			func(ctx context.Context, queryRange rg.Range) ([]sol.Block, error) {
				block, queryErr := s.store.QueryBlock(ctx, queryRange.Start)
				if queryErr != nil {
					return nil, queryErr
				}
				return []sol.Block{*block}, nil
			},
			s.bqBlockLoader(),
		),
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
	if param.To-param.From > maxQuerySpan {
		return nil, errors.Errorf("slot span %d (> %d) is too large in range [%d, %d]",
			param.To-param.From, maxQuerySpan, param.From, param.To)
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
		chain.CheckRangeWithFallback(s.rangeStore,
			func(ctx context.Context, queryRange rg.Range) ([]sol.Block, error) {
				// maxIntervalBlocks+2: one extra so an over-cap range yields > maxIntervalBlocks results,
				// plus one more because the boundary window (first result) may be dropped below.
				return s.store.QueryBlocksByInterval(ctx, queryRange.Start, *queryRange.End, param.Window, maxIntervalBlocks+2)
			},
			s.bqIntervalLoader(param, maxIntervalBlocks+2),
		),
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

// GetPreviousUnskippedBlock returns the nearest non-skipped block (slot and time) with slot < slot.
// Callers use it to learn the chain time "as of" a slot even when that slot itself is skipped (pass
// slot+1 to include the slot itself).
func (s *RPCService) GetPreviousUnskippedBlock(
	ctx context.Context,
	slot uint64,
) (sol.PreviousUnskippedBlock, error) {
	prevSlot, blockTime, found, err := s.previousUnskippedBlock(ctx, slot)
	if err != nil {
		return sol.PreviousUnskippedBlock{}, err
	}
	return sol.PreviousUnskippedBlock{Slot: prevSlot, BlockTime: blockTime, Found: found}, nil
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
	chSlot, chTime, chFound, err := s.store.QueryPreviousUnskipped(ctx, before)
	if err != nil {
		return 0, nil, false, err
	}
	if chFound || s.bqStore == nil {
		return chSlot, chTime, chFound, nil
	}
	// Not found in ClickHouse (the requested point is below the ClickHouse range): fall back to the
	// older BigQuery history.
	return s.bqStore.QueryPreviousUnskipped(ctx, before)
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
	if param.To-param.From > maxQuerySpan {
		return nil, errors.Errorf("slot span %d (> %d) is too large in range [%d, %d]",
			param.To-param.From, maxQuerySpan, param.From, param.To)
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
		chain.CheckRangeWithFallback(s.rangeStore,
			func(ctx context.Context, queryRange rg.Range) ([]sol.BlockTransactions, error) {
				return s.store.FindTransactions(ctx, queryRange.Start, *queryRange.End, param.ProgramIDs, storeLimit)
			},
			s.bqFindTxLoader(param, storeLimit),
		),
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

// GetContractStartBlock returns the earliest block at which address (a program) appears at or after
// startFrom, over all data the caller may reach. startFrom is the caller's floor (its agent start
// block): the result is never below it, and it bounds how far back the super node looks.
//
// The BigQuery archival tier only holds slots below the ClickHouse range start, so it is consulted
// only when startFrom is below that start (startFrom < range.Start) — a caller floored at or above
// the ClickHouse range (e.g. one without BigQuery access, whose floor is the range start) never
// triggers a BigQuery query, which also avoids a permission-denied error for such callers. ClickHouse
// holds the rest of the history, so its earliest is the global earliest at/above the range start; the
// cache is consulted only when the program is absent from ClickHouse (a brand-new program not yet
// synced). The caller still maps the result against its own latest (an appearance after latest or no
// appearance is treated as out of range).
func (s *RPCService) GetContractStartBlock(
	ctx context.Context,
	address solana.PublicKey,
	startFrom uint64,
) (sol.GetContractStartBlockResult, error) {
	r, err := s.rangeStore.Get(ctx)
	if err != nil {
		return sol.GetContractStartBlockResult{}, err
	}

	earliest, found, err := s.store.EarliestProgramSlot(ctx, address)
	if err != nil {
		return sol.GetContractStartBlockResult{}, err
	}
	switch {
	case found:
		// ClickHouse has it. Only when the program appears at the very lower bound might it extend
		// below into the (older, costlier) archive; above the bound chMin is already the global earliest.
		if earliest <= r.Start {
			bqMin, bqFound, bqErr := s.archiveEarliest(ctx, address, r, startFrom)
			if bqErr != nil {
				return sol.GetContractStartBlockResult{}, bqErr
			}
			if bqFound && bqMin < earliest {
				earliest = bqMin
			}
		}
	default:
		// Absent from ClickHouse: the program may live entirely in the older archive, or be brand-new
		// and not yet synced (still only in the latest-slot cache).
		if earliest, found, err = s.archiveEarliest(ctx, address, r, startFrom); err != nil {
			return sol.GetContractStartBlockResult{}, err
		}
		if !found {
			if earliest, found, err = s.cacheEarliest(ctx, address); err != nil {
				return sol.GetContractStartBlockResult{}, err
			}
		}
	}

	if !found {
		return sol.GetContractStartBlockResult{}, nil
	}
	// The caller indexes from startFrom at the earliest, so never report a slot below it.
	return sol.GetContractStartBlockResult{Slot: max(earliest, startFrom), Found: true}, nil
}

// archiveEarliest returns the earliest BigQuery (archival) slot at which address appears. It skips
// the query — returning (0, false) — when the archive cannot help this request: it is disabled, the
// ClickHouse range is empty, or the caller's floor is at/above the range start (so nothing below it
// is wanted, and the floored caller may not have archive permission anyway). The archive only holds
// slots below the ClickHouse range start.
func (s *RPCService) archiveEarliest(
	ctx context.Context,
	address solana.PublicKey,
	r rg.Range,
	startFrom uint64,
) (uint64, bool, error) {
	if s.bqStore == nil || r.IsEmpty() || startFrom >= r.Start {
		return 0, false, nil
	}
	return s.bqStore.EarliestProgramSlot(ctx, address)
}

// cacheEarliest scans the latest-slot cache for the earliest non-skipped slot invoking address, used
// when the program is not yet in ClickHouse (brand-new). The cache is dense and ascending, so the
// first match is the earliest.
func (s *RPCService) cacheEarliest(ctx context.Context, address solana.PublicKey) (uint64, bool, error) {
	programSet := map[string]struct{}{address.String(): {}}
	var earliest uint64
	var found bool
	_, err := s.slotCache.Traverse(ctx, rg.Range{}, func(ctx context.Context, st *sol.Slot) error {
		if !found && !st.Skipped && st.InvokesAnyProgram(programSet) {
			earliest, found = st.SlotNumber, true
		}
		return nil
	})
	return earliest, found, err
}
