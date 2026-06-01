package supernode

import (
	"context"
	"encoding/json"
	"sort"

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
// [From, To]. It merges the ClickHouse part (older slots) and the latest-slot cache (recent slots);
// for a window straddling the boundary the earlier (ClickHouse) block wins.
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
	cacheRange, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return nil, err
	}

	// The cache covers [cacheStart, To]; below cacheStart is served from ClickHouse.
	cacheStart := param.To + 1
	if !cacheRange.IsEmpty() && cacheRange.Start <= param.To {
		cacheStart = max(cacheRange.Start, param.From)
	}

	targetByWindow := make(map[uint64]uint64) // window key -> target slot
	blockBySlot := make(map[uint64]sol.Block)

	// ClickHouse part: [From, cacheStart-1]
	if cacheStart > param.From {
		targets, chErr := chain.CheckRange(s.rangeStore, func(ctx context.Context, qr rg.Range) ([]uint64, error) {
			return s.store.QueryIntervalTargetSlots(ctx, qr.Start, *qr.End, param.Window, param.Limit+1)
		})(ctx, rg.NewRange(param.From, cacheStart-1))
		if chErr != nil {
			return nil, chErr
		}
		blocks, qErr := s.store.QueryBlocks(ctx, targets)
		if qErr != nil {
			return nil, qErr
		}
		for _, b := range blocks {
			key := param.Window.Key(b.Slot, b.BlockTime)
			if _, has := targetByWindow[key]; !has {
				targetByWindow[key] = b.Slot
				blockBySlot[b.Slot] = b
			}
		}
	}

	// Cache part: [cacheStart, To]
	if cacheStart <= param.To {
		if _, err = s.slotCache.Traverse(ctx, rg.NewRange(cacheStart, param.To),
			func(ctx context.Context, st *sol.Slot) error {
				if st.Skipped {
					return nil
				}
				key := param.Window.Key(st.SlotNumber, st.BlockTime)
				if cur, has := targetByWindow[key]; !has || st.SlotNumber < cur {
					targetByWindow[key] = st.SlotNumber
					blockBySlot[st.SlotNumber] = st.ToBlock(true)
				}
				return nil
			}); err != nil {
			return nil, err
		}
	}

	slots := make([]uint64, 0, len(targetByWindow))
	for _, slot := range targetByWindow {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
	result := make([]sol.Block, 0, len(slots))
	for _, slot := range slots {
		result = append(result, blockBySlot[slot])
	}
	if param.Limit > 0 && len(result) > param.Limit {
		return nil, errors.Errorf("too many interval blocks (> %d) in slot range [%d, %d]",
			param.Limit, param.From, param.To)
	}
	return result, nil
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
