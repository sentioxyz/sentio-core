package supernode

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"math"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio/chain/chain"
	"sentioxyz/sentio/chain/evm"
	"sentioxyz/sentio/chain/proxyv3"
	"sentioxyz/sentio/common/number"
)

func NewProxyWithLatestSlotCacheMiddleware(
	slotCache chain.LatestSlotCache[*evm.Slot],
	proxySvr *proxyv3.JSONRPCServiceV2,
) jsonrpc.Middleware {
	svr := proxyWithLatestSlotCacheService{
		slotCache: slotCache,
		proxySvr:  proxySvr,
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
	proxySvr  *proxyv3.JSONRPCServiceV2
}

func (s *proxyWithLatestSlotCacheService) EthBlockNumber(ctx context.Context) (hexutil.Uint64, error) {
	r, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(r.R()), nil
}

func (s *proxyWithLatestSlotCacheService) EthGetLatestBlockNumber(
	ctx context.Context,
	latestBlockNumberOver uint64,
) (evm.GetLatestBlockNumberResponse, error) {
	jsonrpc.GetCtxData(ctx).NotSlowRequest = true
	resp := evm.GetLatestBlockNumberResponse{APIVersion: evm.APIVersion}
	latest, err := s.slotCache.Wait(ctx, number.Number(latestBlockNumberOver))
	if err != nil {
		return resp, err
	}
	resp.LatestBlockNumber = uint64(latest)
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

func (s *proxyWithLatestSlotCacheService) findSlotByChecker(
	ctx context.Context,
	checker func(ctx context.Context, st *evm.Slot) (bool, error),
) (*evm.Slot, error) {
	result, found, err := chain.GetSlotByChecker(ctx, s.slotCache, checker)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, jsonrpc.CallNextMiddleware
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
	st, err := s.findSlotByChecker(ctx, func(ctx context.Context, st *evm.Slot) (bool, error) {
		return st.Header.Hash == hash, nil
	})
	return buildBlockResponse(st, withFullTransactions), err
}

func (s *proxyWithLatestSlotCacheService) EthGetTransactionByHash(
	ctx context.Context,
	hash common.Hash,
) (any, error) {
	var result any
	_, err := s.findSlotByChecker(ctx, func(ctx context.Context, st *evm.Slot) (bool, error) {
		for _, tx := range st.Block.Transactions {
			if tx.Hash == hash {
				result = tx
				return true, nil
			}
		}
		return false, nil
	})
	return result, err
}

func (s *proxyWithLatestSlotCacheService) EthGetTransactionReceipt(
	ctx context.Context,
	hash common.Hash,
) (any, error) {
	var result any
	_, err := s.findSlotByChecker(ctx, func(ctx context.Context, st *evm.Slot) (bool, error) {
		for _, re := range st.Receipts {
			if re.TxHash == hash {
				result = re
				return true, nil
			}
		}
		return false, nil
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
		st, err = s.findSlotByChecker(ctx, func(ctx context.Context, st *evm.Slot) (bool, error) {
			return st.Header.Hash == *numOrHash.BlockHash, nil
		})
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
		st, err := s.findSlotByChecker(ctx, func(ctx context.Context, st *evm.Slot) (bool, error) {
			return st.Header.Hash == *args.BlockHash, nil
		})
		if err != nil {
			return nil, err
		}
		var logs []evm.LogWithCustomSerDe
		for _, log := range st.Logs {
			if logFilter(&log, args) {
				logs = append(logs, evm.LogWithCustomSerDe(log))
			}
		}
		return logs, nil
	}

	// determine the query range
	var fromBlock, toBlock number.Number = 0, math.MaxUint64
	if args.FromBlock != nil {
		fromBlock = number.Number(*args.FromBlock)
	}
	if args.ToBlock != nil {
		toBlock = number.Number(*args.ToBlock)
	}
	queryRange := number.NewRange(fromBlock, toBlock)

	// try to query in latest slot cache
	var logs []evm.LogWithCustomSerDe
	curRange, err := s.slotCache.Traverse(ctx, queryRange, func(ctx context.Context, st *evm.Slot) error {
		for _, log := range st.Logs {
			if logFilter(&log, args) {
				logs = append(logs, evm.LogWithCustomSerDe(log))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if curRange.Contains(queryRange) {
		// latest slot cache include the whole query range, so logs is the result
		return logs, nil
	}
	if curRange.R() < queryRange.R() {
		return nil, errors.Errorf("block range greater than %d", curRange.R())
	}
	// proxy to the external endpoints
	proxyRange := queryRange.Sub(curRange).GetFirstRange()
	proxyArgs := *args
	proxyArgs.FromBlock = utils.WrapPointer(hexutil.Uint64(proxyRange.L()))
	proxyArgs.ToBlock = utils.WrapPointer(hexutil.Uint64(proxyRange.R()))
	var proxyLogs []evm.LogWithCustomSerDe
	err = s.proxySvr.ProxyCallAndConvert(ctx, "eth_getLogs", &proxyArgs, true, &proxyLogs)
	if err != nil {
		return nil, err
	}
	return append(proxyLogs, logs...), nil
}

func (s *proxyWithLatestSlotCacheService) TraceFilter(
	ctx context.Context,
	args *evm.TraceFilterArgs,
) (any, error) {
	if args.After != nil || args.Count != nil {
		return nil, jsonrpc.CallNextMiddleware
	}

	// determine the query range
	var fromBlock, toBlock number.Number = 0, math.MaxUint64
	if args.FromBlock != nil {
		fromBlock = number.Number(*args.FromBlock)
	}
	if args.ToBlock != nil {
		toBlock = number.Number(*args.ToBlock)
	}
	queryRange := number.NewRange(fromBlock, toBlock)

	// try to query in latest slot cache
	var traces []evm.ParityTrace
	curRange, err := s.slotCache.Traverse(ctx, queryRange, func(ctx context.Context, st *evm.Slot) error {
		for _, trace := range st.Traces {
			if traceFilter(&trace, args) {
				traces = append(traces, trace)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if curRange.Contains(queryRange) {
		// latest slot cache include the whole query range, so logs is the result
		return traces, nil
	}
	if curRange.R() < queryRange.R() {
		return nil, errors.Errorf("block range greater than %d", curRange.R())
	}
	// proxy to the external endpoints
	proxyRange := queryRange.Sub(curRange).GetFirstRange()
	proxyArgs := *args
	proxyArgs.FromBlock = utils.WrapPointer(hexutil.Uint64(proxyRange.L()))
	proxyArgs.ToBlock = utils.WrapPointer(hexutil.Uint64(proxyRange.R()))
	var proxyTraces []evm.ParityTrace
	err = s.proxySvr.ProxyCallAndConvert(ctx, "trace_filter", &proxyArgs, true, &proxyTraces)
	if err != nil {
		return nil, err
	}
	return append(proxyTraces, traces...), nil
}
