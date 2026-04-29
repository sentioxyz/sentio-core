package supernode

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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

func (s *proxyWithLatestSlotCacheService) findSlotByNumber(
	ctx context.Context,
	blockNumber rpc.BlockNumber,
) (*evm.Slot, error) {
	var bn number.Number
	if blockNumber < 0 {
		if blockNumber != rpc.LatestBlockNumber {
			return nil, errors.Wrapf(jsonrpc.CallNextMiddleware, "unsupported block tag %s", blockNumber)
		}
		curRange, err := s.slotCache.GetRange(ctx)
		if err != nil {
			return nil, err
		}
		bn = curRange.R()
	} else {
		bn = number.Number(blockNumber)
	}
	slot, err := s.slotCache.GetByNumber(ctx, bn)
	if err != nil {
		if errors.Is(err, chain.ErrSlotNotFound) {
			return nil, jsonrpc.CallNextMiddleware
		}
		return nil, err
	}
	return slot, nil
}

func (s *proxyWithLatestSlotCacheService) findSlotByHash(
	ctx context.Context,
	hash common.Hash,
) (*evm.Slot, error) {
	return s.findSlotByChecker(ctx, func(st *evm.Slot) bool {
		return st.Header.Hash == hash
	})
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
) (any, error) {
	slot, err := s.findSlotByNumber(ctx, blockNumber)
	if err != nil {
		return nil, err
	}
	return buildBlockResponse(slot, withFullTransactions), nil
}

func (s *proxyWithLatestSlotCacheService) EthGetBlockByHash(
	ctx context.Context,
	hash common.Hash,
	withFullTransactions bool,
) (any, error) {
	st, err := s.findSlotByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	return buildBlockResponse(st, withFullTransactions), nil
}

func (s *proxyWithLatestSlotCacheService) EthGetTransactionByHash(
	ctx context.Context,
	hash common.Hash,
) (any, error) {
	var result any
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
) (any, error) {
	var result any
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
) (any, error) {
	var st *evm.Slot
	var err error
	if numOrHash.BlockHash != nil {
		st, err = s.findSlotByHash(ctx, *numOrHash.BlockHash)
	} else {
		st, err = s.findSlotByNumber(ctx, *numOrHash.BlockNumber)
	}
	if err != nil {
		return nil, err
	}
	return st.Receipts, nil
}

func (s *proxyWithLatestSlotCacheService) EthGetLogs(
	ctx context.Context,
	args *evm.EthGetLogsArgs,
) (any, error) {
	if args.BlockHash != nil {
		// query logs in the specified block by args.BlockHash
		st, err := s.findSlotByHash(ctx, *args.BlockHash)
		if err != nil {
			return nil, err
		}
		logs := make([]evm.LogWithCustomSerDe, 0)
		for _, log := range st.Logs {
			if logFilter(&log, args) {
				logs = append(logs, evm.LogWithCustomSerDe(log))
			}
		}
		return logs, nil
	}

	// determine the query range
	curRange, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return nil, err
	}
	var fromBlock, toBlock uint64 = *curRange.End, *curRange.End
	if args.FromBlock != nil {
		fromBlock = (uint64)(*args.FromBlock)
	}
	if args.ToBlock != nil {
		toBlock = (uint64)(*args.ToBlock)
	}

	return chain.QueryRangeWithCache(
		ctx,
		rg.NewRange(fromBlock, toBlock),
		s.slotCache,
		func(st *evm.Slot) ([]evm.LogWithCustomSerDe, error) {
			var logs []evm.LogWithCustomSerDe
			for _, log := range st.Logs {
				if logFilter(&log, args) {
					logs = append(logs, evm.LogWithCustomSerDe(log))
				}
			}
			return logs, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]evm.LogWithCustomSerDe, error) {
			proxyArgs := *args
			proxyArgs.FromBlock = utils.WrapPointer(hexutil.Uint64(queryRange.Start))
			proxyArgs.ToBlock = utils.WrapPointer(hexutil.Uint64(*queryRange.End))
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
			var result []evm.LogWithCustomSerDe
			if proxyErr = json.Unmarshal(proxyResult, &result); proxyErr != nil {
				return nil, proxyErr
			}
			return result, nil
		})
}

func (s *proxyWithLatestSlotCacheService) TraceFilter(
	ctx context.Context,
	args *evm.TraceFilterArgs,
) ([]evm.ParityTrace, error) {
	// determine the query range
	curRange, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return nil, err
	}
	var fromBlock, toBlock uint64 = *curRange.End, *curRange.End
	if args.FromBlock != nil {
		fromBlock = (uint64)(*args.FromBlock)
	}
	if args.ToBlock != nil {
		toBlock = (uint64)(*args.ToBlock)
	}

	return chain.QueryRangeWithCache(
		ctx,
		rg.NewRange(fromBlock, toBlock),
		s.slotCache,
		func(st *evm.Slot) ([]evm.ParityTrace, error) {
			var traces []evm.ParityTrace
			for _, trace := range st.Traces {
				if traceFilter(&trace, args) {
					traces = append(traces, trace)
				}
			}
			return traces, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]evm.ParityTrace, error) {
			proxyArgs := *args
			proxyArgs.FromBlock = utils.WrapPointer(hexutil.Uint64(queryRange.Start))
			proxyArgs.ToBlock = utils.WrapPointer(hexutil.Uint64(*queryRange.End))
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
		})
}
