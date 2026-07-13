package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	fuelGo "github.com/sentioxyz/fuel-go"
	"github.com/sentioxyz/fuel-go/types"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

func NewSuperNode(
	client *fuel.ClientPool,
	slotCache chain.LatestSlotCache[*fuel.Slot],
	rangeStore chain.RangeStore,
	store Storage,
) []jsonrpc.Middleware {
	rpcSvr := &RPCService{
		client:     client,
		slotCache:  slotCache,
		rangeStore: rangeStore,
		store:      store,
	}
	return []jsonrpc.Middleware{
		func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
				switch method {
				case "fuel_getLatestHeight":
					return rpcSvr.GetLatestHeight(ctx)
				case "fuel_getLatestHeader":
					return jsonrpc.CallMethod(rpcSvr.GetLatestHeader, ctx, params)
				case "fuel_getBlockHeader":
					return jsonrpc.CallMethod(rpcSvr.GetBlockHeader, ctx, params)
				case "fuel_getTransactions":
					return jsonrpc.CallMethod(rpcSvr.GetTransactions, ctx, params)
				case "fuel_getContractCreateTransaction":
					return jsonrpc.CallMethod(rpcSvr.GetContractCreateTransaction, ctx, params)
				default:
					return next(ctx, method, params)
				}
			}
		},
		jsonrpc.NewHTTPProxyMiddleware("", client.ClientPool),
	}
}

type RPCService struct {
	client     *fuel.ClientPool
	slotCache  chain.LatestSlotCache[*fuel.Slot]
	rangeStore chain.RangeStore
	store      Storage
}

func (s *RPCService) GetLatestHeight(ctx context.Context) (uint64, error) {
	r, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return 0, err
	}
	return *r.End, nil
}

func (s *RPCService) GetLatestHeader(ctx context.Context, blockHeightGt uint64) (fuel.GetLatestBlockResponse, error) {
	jsonrpc.GetCtxData(ctx).NotSlowRequest = true
	resp := fuel.GetLatestBlockResponse{APIVersion: fuel.APIVersion}
	latest, err := s.slotCache.Wait(ctx, blockHeightGt)
	if err != nil {
		return resp, err
	}
	latestSlot, err := s.slotCache.GetByNumber(ctx, latest)
	if err != nil {
		return resp, err
	}
	resp.Header = latestSlot.Block.Header
	return resp, nil
}

func (s *RPCService) GetBlockHeader(ctx context.Context, height uint64) (types.Header, error) {
	headers, err := chain.QueryRangeWithCache[*fuel.Slot, types.Header](
		ctx,
		rg.NewSingleRange(height),
		s.slotCache,
		func(st *fuel.Slot) ([]types.Header, error) {
			return []types.Header{st.Header}, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]types.Header, error) {
			// here queryRange always be [height,height]
			opt := fuelGo.GetBlockOption{WithHeader: true}
			var block *types.Block
			err := s.client.UseClient(ctx, fmt.Sprintf("proxy.GetBlockHeader/%d", height),
				func(ctx context.Context, cli *fuel.Client) (r clientpool.Result) {
					block, r = cli.GetBlock(ctx, "proxy.GetBlockHeader", height, opt)
					r.BrokenForTask = r.Err != nil // always retry using other client
					return r
				},
			).Err
			if err != nil {
				return nil, err
			}
			return []types.Header{block.Header}, nil
		},
	)
	if err != nil {
		return types.Header{}, err
	}
	if len(headers) == 0 {
		return types.Header{}, chain.ErrSlotNotFound
	}
	return headers[0], err
}

// maxQuerySpan / maxTransactions bound a single fuel_getTransactions query: the block span is
// capped independently of how many records it matches, and a multi-block query returning more than
// maxTransactions fails with chain.NewTooManyResultsError so the caller shrinks the range and
// retries (single-block queries are exempt: they cannot be shrunk further). The span cap matches
// the typical block partition sizing of the backing table (intDiv(block_height, N), see the schema
// manager), so one query scans at most about one partition; the record cap budgets a response of
// roughly 1 MiB (a transaction is typically a few hundred bytes) and sits at 2x the per-query
// record target of the driver's transaction fetcher.
const (
	maxQuerySpan    = 500000
	maxTransactions = 2000
)

func (s *RPCService) GetTransactions(
	ctx context.Context,
	param fuel.GetTransactionsParam,
) ([]fuel.WrappedTransaction, error) {
	_, logger := log.FromContext(ctx)
	if err := chain.CheckQuerySpan(param.StartHeight, param.EndHeight, maxQuerySpan); err != nil {
		return nil, err
	}
	for _, filter := range param.Filters {
		if filter.IsEmpty() {
			logger.Warn("there is an empty filter, which is equivalent to no filter")
			param.Filters = nil
			break
		}
	}
	limit := chain.RangeQueryLimit(param.StartHeight, param.EndHeight, maxTransactions)
	result, err := chain.QueryRangeWithCache[*fuel.Slot, fuel.WrappedTransaction](
		ctx,
		rg.NewRange(param.StartHeight, param.EndHeight),
		s.slotCache,
		func(st *fuel.Slot) ([]fuel.WrappedTransaction, error) {
			txs := utils.FilterArr(st.GetTransactions(), func(tx fuel.WrappedTransaction) bool {
				return fuel.CheckTransaction(tx, param.Filters)
			})
			for i := range txs {
				txs[i].Status = fuel.BuildTransactionStatus(txs[i].Status, st.Block.Header)
			}
			return txs, nil
		},
		chain.CheckRange(s.rangeStore, func(ctx context.Context, queryRange rg.Range) ([]fuel.WrappedTransaction, error) {
			return s.store.QueryTransactions(ctx, queryRange.Start, *queryRange.End, param.Filters, chain.StoreQueryLimit(limit))
		}),
	)
	return chain.CheckTooManyResults(result, err, limit)
}

// GetContractCreateTransaction will return (nil, nil) if contract not created
func (s *RPCService) GetContractCreateTransaction(
	ctx context.Context,
	contractID string,
) (*fuel.WrappedTransaction, error) {
	return s.store.QueryContractCreateTransaction(ctx, contractID)
}
