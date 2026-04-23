package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"

	"github.com/sentioxyz/fuel-go/types"
)

func NewSuperNode(
	client *fuel.ClientPool,
	ext chain.Dimension[*fuel.Slot],
	slotCache chain.LatestSlotCache[*fuel.Slot],
	store Storage,
) []jsonrpc.Middleware {
	rpcSvr := &RPCService{
		ext:       ext,
		slotCache: slotCache,
		store:     store,
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
				case jsonrpc.HTTPRequestMethod:
					jsonrpc.ProxyHTTP[fuel.ClientConfig, *fuel.Client](
						ctx,
						method,
						client.ClientPool,
						func(ctx context.Context, cli *fuel.Client) (
							resp *http.Response,
							respBody []byte,
							upstream string,
							r clientpool.Result,
						) {
							ctxData := jsonrpc.GetCtxData(ctx)
							cfg := cli.GetConfig()
							upstream = cfg.GetName()
							r = cli.Use(ctx, "proxy."+method, func(ctx context.Context) (r clientpool.Result) {
								req, err := clientpool.BuildHTTPRequest(
									ctx,
									ctxData.RawReq.Method,
									cfg.Endpoint,
									ctxData.RawReq.URL.Path,
									ctxData.RawReq.URL.Query(),
									ctxData.RawReqBody,
								)
								if err != nil {
									return clientpool.Result{Err: err, BrokenForTask: true}
								}
								resp, respBody, r = clientpool.SendHTTP(cli.GetHTTPClient(), req, nil)
								return r
							})
							return
						},
					)
					return nil, nil
				default:
					return next(ctx, method, params)
				}
			}
		},
	}
}

type RPCService struct {
	ext       chain.Dimension[*fuel.Slot]
	slotCache chain.LatestSlotCache[*fuel.Slot]
	store     Storage
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
			slots, err := chain.Load[*fuel.Slot](s.ext, ctx, queryRange)
			if err != nil {
				return nil, err
			}
			return utils.MapSliceNoError(slots, func(st *fuel.Slot) types.Header {
				return st.Header
			}), nil
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

func (s *RPCService) GetTransactions(ctx context.Context, param fuel.GetTransactionsParam) ([]fuel.WrappedTransaction, error) {
	_, logger := log.FromContext(ctx)
	if param.EndHeight < param.StartHeight {
		return nil, fmt.Errorf("end_height cannot less than start_height")
	}
	for _, filter := range param.Filters {
		if filter.IsEmpty() {
			logger.Warn("there is an empty filter, which is equivalent to no filter")
			param.Filters = nil
			break
		}
	}
	return chain.QueryRangeWithCache[*fuel.Slot, fuel.WrappedTransaction](
		ctx,
		rg.NewRange(param.StartHeight, param.EndHeight),
		s.slotCache,
		func(st *fuel.Slot) ([]fuel.WrappedTransaction, error) {
			rawTxns := st.GetTransactions()
			for _, tx := range rawTxns {
				if tx.Status == nil {
					return nil, fmt.Errorf("txn %d/%s in block %d miss status", tx.TransactionIndex, tx.Id.String(), st.Height)
				}
			}
			txns := utils.FilterArr(rawTxns, func(tx fuel.WrappedTransaction) bool {
				return fuel.CheckTransaction(tx, param.Filters)
			})
			for i := range txns {
				txns[i].Status = fuel.BuildTransactionStatus(txns[i].Status, st.Block.Header)
			}
			return txns, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]fuel.WrappedTransaction, error) {
			return s.store.QueryTransactions(ctx, queryRange.Start, *queryRange.End, param.Filters)
		},
	)
}

// GetContractCreateTransaction will return (nil, nil) if contract not created
func (s *RPCService) GetContractCreateTransaction(ctx context.Context, contractID string) (*fuel.WrappedTransaction, error) {
	return s.store.QueryContractCreateTransaction(ctx, contractID)
}
