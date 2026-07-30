package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

// resolveExRange resolves the from/to pair of an Ex range query into concrete block numbers.
// Unlike queryWithCache it never falls through to the next middleware: the Ex methods must not be
// proxied upstream (real nodes do not implement them), so an unsupported block tag is a hard
// error instead of jsonrpc.CallNextMiddleware.
func resolveExRange(
	ctx context.Context,
	slotCache chain.LatestSlotCache[*evm.Slot],
	fromBlock, toBlock *rpc.BlockNumber,
) (sn, en uint64, err error) {
	if fromBlock == nil {
		fromBlock = utils.WrapPointer(rpc.LatestBlockNumber)
	}
	if toBlock == nil {
		toBlock = utils.WrapPointer(rpc.LatestBlockNumber)
	}
	if *fromBlock < 0 && *fromBlock != rpc.LatestBlockNumber {
		return 0, 0, errors.Errorf("block tag %d is not supported by the Ex query", *fromBlock)
	}
	if *toBlock < 0 && *toBlock != rpc.LatestBlockNumber {
		return 0, 0, errors.Errorf("block tag %d is not supported by the Ex query", *toBlock)
	}
	if *fromBlock >= 0 && *toBlock >= 0 {
		return (uint64)(*fromBlock), (uint64)(*toBlock), nil
	}
	r, err := slotCache.GetRange(ctx)
	if err != nil {
		return 0, 0, err
	}
	if *fromBlock < 0 {
		sn = *r.End
	} else {
		sn = (uint64)(*fromBlock)
	}
	if *toBlock < 0 {
		en = *r.End
	} else {
		en = (uint64)(*toBlock)
	}
	return sn, en, nil
}

// queryWithCacheEx is the Ex counterpart of queryWithCache: it serves the tail of the range from
// the latest-slot cache and the lower remainder from collectFromStore, and additionally returns
// the per-block chain identity (links) of every block it can vouch for. The links always cover a
// contiguous tail sub-range of [fromBlock, toBlock]: the cache part always contributes links;
// the store part contributes links when collectFromStore returns them (ClickHouse-backed super
// node) and marks its sub-range unverifiable when it returns nil links (upstream proxy).
func queryWithCacheEx[ELEM any](
	ctx context.Context,
	slotCache chain.LatestSlotCache[*evm.Slot],
	fromBlock, toBlock *rpc.BlockNumber,
	maxQueryRangeSize uint64,
	resultLimit int,
	collectFromSlot func(st *evm.Slot) ([]ELEM, error),
	collectFromStore func(ctx context.Context, r rg.Range, limit int) ([]ELEM, []evm.BlockLink, error),
) (elems []ELEM, links []evm.BlockLink, err error) {
	sn, en, err := resolveExRange(ctx, slotCache, fromBlock, toBlock)
	if err != nil {
		return nil, nil, err
	}
	// The span cap applies to the REQUESTED range, same contract as queryWithCache.
	if maxQueryRangeSize > 0 {
		if err = chain.CheckQuerySpan(sn, en, maxQueryRangeSize); err != nil {
			return nil, nil, err
		}
	}
	limit := chain.RangeQueryLimit(sn, en, resultLimit)
	interval := rg.NewRange(sn, en)

	var cachedElems []ELEM
	var cachedLinks []evm.BlockLink
	cachedRange, err := slotCache.Traverse(ctx, interval, func(_ context.Context, st *evm.Slot) error {
		es, collectErr := collectFromSlot(st)
		if collectErr != nil {
			return collectErr
		}
		cachedElems = append(cachedElems, es...)
		cachedLinks = append(cachedLinks, evm.SlotBlockLink(st))
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Same range algebra as chain.QueryRangeWithCache: only the FIRST (lowest) uncovered
	// sub-range is queried; an uncovered sub-range above the cache head does not exist yet.
	queryRange := interval.Remove(cachedRange).First()
	if queryRange.IsEmpty() || (!cachedRange.IsEmpty() && queryRange.Start > *cachedRange.End) {
		elems, err = chain.CheckTooManyResults(cachedElems, nil, limit)
		return elems, cachedLinks, err
	}

	storeElems, storeLinks, err := collectFromStore(ctx, queryRange, chain.StoreQueryLimit(limit))
	if err != nil {
		return nil, nil, err
	}
	if storeLinks != nil && uint64(len(storeLinks)) != *queryRange.Size() {
		return nil, nil, errors.Errorf("the store returned %d block links for range %s (want %d)",
			len(storeLinks), queryRange, *queryRange.Size())
	}
	elems = utils.MergeArr(storeElems, cachedElems)
	if storeLinks != nil {
		links = utils.MergeArr(storeLinks, cachedLinks)
	} else {
		links = cachedLinks
	}
	elems, err = chain.CheckTooManyResults(elems, nil, limit)
	return elems, links, err
}

// exResponseFromSlot builds the single-block Ex result of a by-hash query served from the cache.
func exLinkFromSlot(st *evm.Slot) (evm.BlockHashLinkPart, error) {
	return evm.NewBlockHashLinkPart([]evm.BlockLink{evm.SlotBlockLink(st)})
}

// getSlotByHashForEx looks up a by-hash Ex query in the latest-slot cache. A miss is a hard,
// retryable error: the Ex methods must not fall through to the upstream proxy (real nodes do not
// implement them), and callers that can live without identity information should use the plain
// method instead.
func getSlotByHashForEx(
	ctx context.Context,
	slotCache chain.LatestSlotCache[*evm.Slot],
	hash string,
) (*evm.Slot, error) {
	st, err := slotCache.GetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, chain.ErrSlotNotFound) {
			return nil, errors.Errorf(
				"block %s is not in the latest slot cache, retry later or use the plain query instead", hash)
		}
		return nil, err
	}
	return st, nil
}

func (s *standardService) GetLogsEx(
	ctx context.Context,
	args *evm.EthGetLogsArgs,
) (resp evm.EthGetLogsExResponse, err error) {
	checker := args.Checker()
	if args.BlockHash != nil {
		st, hashErr := getSlotByHashForEx(ctx, s.slotCache, args.BlockHash.String())
		if hashErr != nil {
			return resp, hashErr
		}
		resp.Logs = utils.FilterArr(st.Logs, checker)
		resp.BlockHashLinkPart, err = exLinkFromSlot(st)
		return resp, err
	}
	logs, links, err := queryWithCacheEx(ctx, s.slotCache, args.FromBlock, args.ToBlock,
		maxQueryRangeSize, maxLogs,
		func(st *evm.Slot) ([]types.Log, error) {
			return utils.FilterArr(st.Logs, checker), nil
		},
		func(ctx context.Context, r rg.Range, limit int) ([]types.Log, []evm.BlockLink, error) {
			storeRange, getErr := s.rangeStore.Get(ctx)
			if getErr != nil {
				return nil, nil, getErr
			}
			if !storeRange.Include(r) {
				return nil, nil, errors.Errorf("request range %s not in scope of range store %s", r, storeRange)
			}
			blockWheres := fmt.Sprintf("block_number >= %d AND block_number <= %d", r.Start, *r.End)
			where := strings.Join(append(s.filterLogSQL(args), blockWheres), " AND ")
			logs, queryErr := s.store.QueryLogs(ctx, where, limit)
			if queryErr != nil {
				return nil, nil, queryErr
			}
			links, queryErr := s.store.QueryBlockLinks(ctx, r.Start, *r.End)
			if queryErr != nil {
				return nil, nil, queryErr
			}
			// topics filtering condition is not strict enough, need post-filtering
			return utils.FilterArr(logs, checker), links, nil
		},
	)
	if err != nil {
		return resp, err
	}
	if logs == nil {
		logs = make([]types.Log, 0)
	}
	resp.Logs = logs
	resp.BlockHashLinkPart, err = evm.NewBlockHashLinkPart(links)
	return resp, err
}

func (s *standardService) TraceFilterEx(
	ctx context.Context,
	args *evm.TraceFilterArgs,
) (resp evm.TraceFilterExResponse, err error) {
	checker := args.Checker()
	traces, links, err := queryWithCacheEx(ctx, s.slotCache, args.FromBlock, args.ToBlock,
		maxQueryRangeSize, maxTraces,
		func(st *evm.Slot) ([]evm.ParityTrace, error) {
			if !st.HaveTrace {
				return nil, errors.Errorf("trace invalid in block %d", st.GetNumber())
			}
			return utils.FilterArr(st.Traces, checker), nil
		},
		func(ctx context.Context, r rg.Range, limit int) ([]evm.ParityTrace, []evm.BlockLink, error) {
			storeRange, getErr := s.rangeStore.Get(ctx)
			if getErr != nil {
				return nil, nil, getErr
			}
			if !storeRange.Include(r) {
				return nil, nil, errors.Errorf("request range %s not in scope of range store %s", r, storeRange)
			}
			blockWheres := fmt.Sprintf("block_number >= %d AND block_number <= %d", r.Start, *r.End)
			where := strings.Join(append(s.filterTraceSQL(args), blockWheres), " AND ")
			traces, queryErr := s.store.QueryTraces(ctx, where, limit)
			if queryErr != nil {
				return nil, nil, queryErr
			}
			links, queryErr := s.store.QueryBlockLinks(ctx, r.Start, *r.End)
			if queryErr != nil {
				return nil, nil, queryErr
			}
			return utils.FilterArr(traces, checker), links, nil
		},
	)
	if err != nil {
		return resp, err
	}
	if traces == nil {
		traces = make([]evm.ParityTrace, 0)
	}
	resp.Traces = traces
	resp.BlockHashLinkPart, err = evm.NewBlockHashLinkPart(links)
	return resp, err
}

func (s *proxyWithLatestSlotCacheService) EthGetLogsEx(
	ctx context.Context,
	args *evm.EthGetLogsArgs,
) (resp evm.EthGetLogsExResponse, err error) {
	checker := args.Checker()
	if args.BlockHash != nil {
		st, hashErr := getSlotByHashForEx(ctx, s.slotCache, args.BlockHash.String())
		if hashErr != nil {
			return resp, hashErr
		}
		resp.Logs = utils.FilterArr(st.Logs, checker)
		resp.BlockHashLinkPart, err = exLinkFromSlot(st)
		return resp, err
	}
	logs, links, err := queryWithCacheEx(ctx, s.slotCache, args.FromBlock, args.ToBlock, 0, 0,
		func(st *evm.Slot) ([]types.Log, error) {
			return utils.FilterArr(st.Logs, checker), nil
		},
		func(ctx context.Context, r rg.Range, _ int) ([]types.Log, []evm.BlockLink, error) {
			// This super node has no local store: proxy the plain query upstream for the
			// sub-range below the cache. The upstream cannot vouch for block identities,
			// so this sub-range contributes no links.
			fromBlock, toBlock := (rpc.BlockNumber)(r.Start), (rpc.BlockNumber)(*r.End)
			proxyArgs := *args
			proxyArgs.FromBlock = &fromBlock
			proxyArgs.ToBlock = &toBlock
			proxyResult, proxyErr := jsonrpc.ProxyJSONRPCRequest(
				ctx,
				"eth_getLogs",
				[]any{&proxyArgs},
				s.client.ClientPool,
			)
			if proxyErr != nil {
				return nil, nil, proxyErr
			}
			var result []types.Log
			if proxyErr = json.Unmarshal(proxyResult, &result); proxyErr != nil {
				return nil, nil, proxyErr
			}
			return result, nil, nil
		},
	)
	if err != nil {
		return resp, err
	}
	if logs == nil {
		logs = make([]types.Log, 0)
	}
	resp.Logs = logs
	resp.BlockHashLinkPart, err = evm.NewBlockHashLinkPart(links)
	return resp, err
}

func (s *proxyWithLatestSlotCacheService) TraceFilterEx(
	ctx context.Context,
	args *evm.TraceFilterArgs,
) (resp evm.TraceFilterExResponse, err error) {
	checker := args.Checker()
	traces, links, err := queryWithCacheEx(ctx, s.slotCache, args.FromBlock, args.ToBlock, 0, 0,
		func(st *evm.Slot) ([]evm.ParityTrace, error) {
			if !st.HaveTrace {
				return nil, errors.Errorf("trace invalid in block %d", st.GetNumber())
			}
			return utils.FilterArr(st.Traces, checker), nil
		},
		func(ctx context.Context, r rg.Range, _ int) ([]evm.ParityTrace, []evm.BlockLink, error) {
			fromBlock, toBlock := (rpc.BlockNumber)(r.Start), (rpc.BlockNumber)(*r.End)
			proxyArgs := *args
			proxyArgs.FromBlock = &fromBlock
			proxyArgs.ToBlock = &toBlock
			proxyResult, proxyErr := jsonrpc.ProxyJSONRPCRequest(
				ctx,
				"trace_filter",
				[]any{&proxyArgs},
				s.client.ClientPool,
			)
			if proxyErr != nil {
				return nil, nil, proxyErr
			}
			var result []evm.ParityTrace
			if proxyErr = json.Unmarshal(proxyResult, &result); proxyErr != nil {
				return nil, nil, proxyErr
			}
			return result, nil, nil
		},
	)
	if err != nil {
		return resp, err
	}
	if traces == nil {
		traces = make([]evm.ParityTrace, 0)
	}
	resp.Traces = traces
	resp.BlockHashLinkPart, err = evm.NewBlockHashLinkPart(links)
	return resp, err
}
