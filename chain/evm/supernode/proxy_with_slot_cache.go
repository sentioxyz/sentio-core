package supernode

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

func NewProxyWithLatestSlotCacheMiddleware(
	slotCache chain.LatestSlotCache[*evm.Slot],
	client *evm.ClientPool,
) jsonrpc.Middleware {
	svr := proxyWithLatestSlotCacheService{
		slotCache: slotCache,
		client:    client,
	}
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (result any, err error) {
			if slotCache == nil {
				if method == "eth_getLatestBlockNumber" {
					return nil, errors.Errorf("method %s not support", method)
				}
				return next(ctx, method, params)
			}
			switch method {
			case "eth_blockNumber":
				result, err = svr.EthBlockNumber(ctx)
			case "eth_getLatestBlockNumber":
				result, err = jsonrpc.CallMethod(svr.EthGetLatestBlockNumber, ctx, params)
			case "eth_getBlockByNumber":
				result, err = jsonrpc.CallMethod(svr.EthGetBlockByNumber, ctx, params)
			case "eth_getBlockByHash":
				result, err = jsonrpc.CallMethod(svr.EthGetBlockByHash, ctx, params)
			case "eth_getTransactionByHash":
				result, err = jsonrpc.CallMethod(svr.EthGetTransactionByHash, ctx, params)
			case "eth_getTransactionReceipt":
				result, err = jsonrpc.CallMethod(svr.EthGetTransactionReceipt, ctx, params)
			case "eth_getBlockReceipts":
				result, err = jsonrpc.CallMethod(svr.EthGetBlockReceipts, ctx, params)
			case "eth_getLogs":
				result, err = jsonrpc.CallMethod(svr.EthGetLogs, ctx, params)
			case "trace_filter":
				result, err = jsonrpc.CallMethod(svr.TraceFilter, ctx, params)
			default:
				return next(ctx, method, params)
			}
			if err == nil || !errors.Is(err, jsonrpc.CallNextMiddleware) {
				return result, err
			}
			return next(ctx, method, params)
		}
	}
}

type proxyWithLatestSlotCacheService struct {
	slotCache chain.LatestSlotCache[*evm.Slot]
	client    *evm.ClientPool
}

func (s *proxyWithLatestSlotCacheService) EthBlockNumber(ctx context.Context) (hexutil.Uint64, error) {
	r, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(*r.End), nil
}

func (s *proxyWithLatestSlotCacheService) EthGetLatestBlockNumber(
	ctx context.Context,
	latestBlockNumberOver uint64,
) (evm.GetLatestBlockNumberResponse, error) {
	jsonrpc.GetCtxData(ctx).NotSlowRequest = true
	resp := evm.GetLatestBlockNumberResponse{APIVersion: evm.APIVersion}
	latest, err := s.slotCache.Wait(ctx, latestBlockNumberOver)
	if err != nil {
		return resp, err
	}
	resp.LatestBlockNumber = latest
	return resp, nil
}

func (s *proxyWithLatestSlotCacheService) findSlotByChecker(
	ctx context.Context,
	checker func(st *evm.Slot) bool,
) (*evm.Slot, error) {
	result, err := s.slotCache.GetByChecker(ctx, checker)
	if err != nil {
		if errors.Is(err, chain.ErrSlotNotFound) {
			return nil, jsonrpc.CallNextMiddleware
		}
		return nil, err
	}
	return result, nil
}

func (s *proxyWithLatestSlotCacheService) EthGetBlockByNumber(
	ctx context.Context,
	blockNumber rpc.BlockNumber,
	withFullTransactions bool,
) (*evm.RPCGetBlockResponse, error) {
	responses, err := queryWithCache(ctx, s.slotCache, nil, &blockNumber, nil, nil,
		func(st *evm.Slot) ([]evm.RPCGetBlockResponse, error) {
			return []evm.RPCGetBlockResponse{evm.NewRPCGetBlockResponse(st, withFullTransactions)}, nil
		},
		func(ctx context.Context, r rg.Range) ([]evm.RPCGetBlockResponse, error) {
			return nil, jsonrpc.CallNextMiddleware
		},
		jsonrpc.CallNextMiddleware,
	)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, nil
	}
	return &responses[0], nil
}

func (s *proxyWithLatestSlotCacheService) EthGetBlockByHash(
	ctx context.Context,
	hash common.Hash,
	withFullTransactions bool,
) (*evm.RPCGetBlockResponse, error) {
	responses, err := queryWithCache(ctx, s.slotCache, &hash, nil, nil, nil,
		func(st *evm.Slot) ([]evm.RPCGetBlockResponse, error) {
			return []evm.RPCGetBlockResponse{evm.NewRPCGetBlockResponse(st, withFullTransactions)}, nil
		},
		func(ctx context.Context, r rg.Range) ([]evm.RPCGetBlockResponse, error) {
			return nil, jsonrpc.CallNextMiddleware
		},
		jsonrpc.CallNextMiddleware,
	)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, nil
	}
	return &responses[0], nil
}

func (s *proxyWithLatestSlotCacheService) EthGetTransactionByHash(
	ctx context.Context,
	hash common.Hash,
) (evm.RPCTransaction, error) {
	var result evm.RPCTransaction
	_, err := s.findSlotByChecker(ctx, func(st *evm.Slot) bool {
		for _, tx := range st.Block.Transactions {
			if tx.Hash == hash {
				result = tx
				return true
			}
		}
		return false
	})
	return result, err
}

func (s *proxyWithLatestSlotCacheService) EthGetTransactionReceipt(
	ctx context.Context,
	hash common.Hash,
) (evm.ExtendedReceipt, error) {
	var result evm.ExtendedReceipt
	_, err := s.findSlotByChecker(ctx, func(st *evm.Slot) bool {
		for _, re := range st.Receipts {
			if re.TxHash == hash {
				result = re
				return true
			}
		}
		return false
	})
	return result, err
}

func (s *proxyWithLatestSlotCacheService) EthGetBlockReceipts(
	ctx context.Context,
	numOrHash rpc.BlockNumberOrHash,
) ([]evm.ExtendedReceipt, error) {
	return queryWithCache(ctx, s.slotCache, numOrHash.BlockHash, numOrHash.BlockNumber, nil, nil,
		func(st *evm.Slot) ([]evm.ExtendedReceipt, error) {
			return st.Receipts, nil
		},
		func(ctx context.Context, r rg.Range) ([]evm.ExtendedReceipt, error) {
			return nil, jsonrpc.CallNextMiddleware
		},
		jsonrpc.CallNextMiddleware,
	)
}

func (s *proxyWithLatestSlotCacheService) EthGetLogs(
	ctx context.Context,
	args *evm.EthGetLogsArgs,
) ([]types.Log, error) {
	checker := args.Checker()
	return queryWithCache(ctx, s.slotCache, args.BlockHash, nil, args.FromBlock, args.ToBlock,
		func(st *evm.Slot) ([]types.Log, error) {
			return utils.FilterArr(st.Logs, checker), nil
		},
		func(ctx context.Context, r rg.Range) ([]types.Log, error) {
			proxyArgs := *args
			proxyArgs.FromBlock = (*hexutil.Uint64)(&r.Start)
			proxyArgs.ToBlock = (*hexutil.Uint64)(r.End)
			proxyResult, proxyErr := jsonrpc.ProxyJSONRPCRequest(
				ctx,
				"proxy",
				"eth_getLogs",
				[]any{proxyArgs},
				s.client.ClientPool,
			)
			if proxyErr != nil {
				return nil, proxyErr
			}
			var result []types.Log
			if proxyErr = json.Unmarshal(proxyResult, &result); proxyErr != nil {
				return nil, proxyErr
			}
			return result, nil
		},
		jsonrpc.CallNextMiddleware,
	)
}

func (s *proxyWithLatestSlotCacheService) TraceFilter(
	ctx context.Context,
	args *evm.TraceFilterArgs,
) ([]evm.ParityTrace, error) {
	checker := args.Checker()
	return queryWithCache(ctx, s.slotCache, nil, nil, args.FromBlock, args.ToBlock,
		func(st *evm.Slot) ([]evm.ParityTrace, error) {
			return utils.FilterArr(st.Traces, checker), nil
		},
		func(ctx context.Context, r rg.Range) ([]evm.ParityTrace, error) {
			proxyArgs := *args
			proxyArgs.FromBlock = (*hexutil.Uint64)(&r.Start)
			proxyArgs.ToBlock = (*hexutil.Uint64)(r.End)
			proxyResult, proxyErr := jsonrpc.ProxyJSONRPCRequest(
				ctx,
				"proxy",
				"trace_filter",
				[]any{proxyArgs},
				s.client.ClientPool,
			)
			if proxyErr != nil {
				return nil, proxyErr
			}
			var result []evm.ParityTrace
			if proxyErr = json.Unmarshal(proxyResult, &result); proxyErr != nil {
				return nil, proxyErr
			}
			return result, nil
		},
		jsonrpc.CallNextMiddleware,
	)
}
