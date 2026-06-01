package supernode

import (
	"context"
	"encoding/json"
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
// [From, To]. A single QueryRangeWithCache traversal picks the first block per window from the
// latest-slot cache and queries ClickHouse for the range below the cache; results are then
// deduplicated by window keeping the earliest slot, which drops the "fake" cache block of a window
// that straddles the cache/ClickHouse boundary (its real first block lives in ClickHouse).
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

	// seen dedups within the cache traversal (best effort; correctness comes from the final
	// min-slot dedup below).
	seen := make(map[uint64]struct{})
	blocks, err := chain.QueryRangeWithCache[*sol.Slot, sol.Block](
		ctx,
		rg.NewRange(param.From, param.To),
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
			return s.store.QueryBlocksByInterval(ctx, queryRange.Start, *queryRange.End, param.Window, param.Limit+1)
		}),
	)
	if err != nil {
		return nil, err
	}

	// Keep the earliest block per window: a straddling window has a ClickHouse block (earlier) and a
	// cache block (later, the fake); the earlier one wins.
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

	// Only the first (smallest-slot) window can straddle From's left boundary. The nearest
	// non-skipped block before From lies in that window iff the window's first block is before From
	// (i.e. it belongs to an earlier page); in that case the first result is a duplicate "fake"
	// block and must be dropped so each window is reported by exactly one page.
	if len(result) > 0 && param.From > 0 && result[0].GetBlockResult != nil {
		firstKey := param.Window.Key(result[0].Slot, result[0].BlockTime)
		prevKey, found, prevErr := s.previousUnskippedWindowKey(ctx, param.Window, param.From)
		if prevErr != nil {
			return nil, prevErr
		}
		if found && prevKey == firstKey {
			result = result[1:]
		}
	}

	if param.Limit > 0 && len(result) > param.Limit {
		return nil, errors.Errorf("too many interval blocks (> %d) in slot range [%d, %d]",
			param.Limit, param.From, param.To)
	}
	return result, nil
}

// previousUnskippedWindowKey returns the interval-window key of the nearest non-skipped block with
// slot < from, looking in the latest-slot cache (the recent suffix) first and then ClickHouse.
func (s *RPCService) previousUnskippedWindowKey(
	ctx context.Context,
	window sol.IntervalWindow,
	from uint64,
) (key uint64, found bool, err error) {
	if from == 0 {
		return 0, false, nil
	}
	cacheRange, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return 0, false, err
	}
	if !cacheRange.IsEmpty() && cacheRange.Start < from {
		upper := from - 1
		if cacheRange.End != nil && *cacheRange.End < upper {
			upper = *cacheRange.End
		}
		var slot uint64
		var blockTime *solana.UnixTimeSeconds
		var cacheFound bool
		// Traverse is ascending, so the last non-skipped block seen is the nearest one before from.
		if _, err = s.slotCache.Traverse(ctx, rg.NewRange(cacheRange.Start, upper),
			func(ctx context.Context, st *sol.Slot) error {
				if !st.Skipped {
					slot, blockTime, cacheFound = st.SlotNumber, st.BlockTime, true
				}
				return nil
			}); err != nil {
			return 0, false, err
		}
		if cacheFound {
			return window.Key(slot, blockTime), true, nil
		}
		// Everything below from is skipped in the cache; look in ClickHouse below the cache.
		return s.previousUnskippedWindowKeyFromStore(ctx, window, cacheRange.Start)
	}
	return s.previousUnskippedWindowKeyFromStore(ctx, window, from)
}

func (s *RPCService) previousUnskippedWindowKeyFromStore(
	ctx context.Context,
	window sol.IntervalWindow,
	before uint64,
) (uint64, bool, error) {
	slot, blockTime, found, err := s.store.QueryPreviousUnskipped(ctx, before)
	if err != nil || !found {
		return 0, false, err
	}
	return window.Key(slot, blockTime), true, nil
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
			return s.store.FindTransactions(ctx, queryRange.Start, *queryRange.End, param.ProgramIDs, param.Limit+1)
		}),
	)
	if err != nil {
		return nil, err
	}
	if param.Limit > 0 {
		var total int
		for _, b := range result {
			total += len(b.Transactions)
		}
		if total > param.Limit {
			return nil, errors.Errorf("too many transactions (> %d) for programs in slot range [%d, %d]",
				param.Limit, param.From, param.To)
		}
	}
	return result, nil
}

func (s *RPCService) GetContractStartBlock(
	ctx context.Context,
	param sol.GetContractStartBlockParam,
) (sol.GetContractStartBlockResult, error) {
	if param.Latest < param.Start {
		return sol.GetContractStartBlockResult{}, errors.Errorf(
			"latest %d cannot be less than start %d", param.Latest, param.Start)
	}
	programSet := map[string]struct{}{param.Address.String(): {}}
	slots, err := chain.QueryRangeWithCache[*sol.Slot, uint64](
		ctx,
		rg.NewRange(param.Start, param.Latest),
		s.slotCache,
		func(st *sol.Slot) ([]uint64, error) {
			if st.InvokesAnyProgram(programSet) {
				return []uint64{st.SlotNumber}, nil
			}
			return nil, nil
		},
		chain.CheckRange(s.rangeStore, func(ctx context.Context, queryRange rg.Range) ([]uint64, error) {
			slot, found, queryErr := s.store.GetContractStartBlock(ctx, param.Address, queryRange.Start, *queryRange.End)
			if queryErr != nil {
				return nil, queryErr
			}
			if !found {
				return nil, nil
			}
			return []uint64{slot}, nil
		}),
	)
	if err != nil {
		return sol.GetContractStartBlockResult{}, err
	}
	if len(slots) == 0 {
		return sol.GetContractStartBlockResult{Found: false}, nil
	}
	minSlot := slots[0]
	for _, sn := range slots[1:] {
		if sn < minSlot {
			minSlot = sn
		}
	}
	return sol.GetContractStartBlockResult{Slot: minSlot, Found: true}, nil
}
