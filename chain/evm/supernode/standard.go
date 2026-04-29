package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"strings"
)

type standardService struct {
	slotCache  chain.LatestSlotCache[*evm.Slot]
	rangeStore chain.RangeStore
	store      Storage
}

func NewStandardMiddleware(
	chainID uint64,
	slotCache chain.LatestSlotCache[*evm.Slot],
	rangeStore chain.RangeStore,
	store Storage,
) jsonrpc.Middleware {
	s := standardService{
		slotCache:  slotCache,
		rangeStore: rangeStore,
		store:      store,
	}
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			switch method {
			case "eth_chainId":
				return fmt.Sprintf("0x%x", chainID), nil
			case "eth_getLatestBlockNumber":
				return jsonrpc.CallMethod(s.GetLatestBlockNumber, ctx, params)
			case "eth_blockNumber":
				return jsonrpc.CallMethod(s.BlockNumber, ctx, params)
			case "eth_getBlockHeaderByNumber":
				return jsonrpc.CallMethod(s.GetBlockHeaderByNumber, ctx, params)
			case "eth_getBlockByNumber":
				return jsonrpc.CallMethod(s.GetBlockByNumber, ctx, params)
			case "eth_getBlockByHash":
				return jsonrpc.CallMethod(s.GetBlockByHash, ctx, params)
			case "eth_getBlockReceipts":
				return jsonrpc.CallMethod(s.GetBlockReceipts, ctx, params)
			case "eth_getLogs":
				return jsonrpc.CallMethod(s.GetLogs, ctx, params)
			case "eth_getBlocksPacked":
				return jsonrpc.CallMethod(s.GetBlocksPacked, ctx, params)
			case "eth_getLogsPacked":
				return jsonrpc.CallMethod(s.GetLogsPacked, ctx, params)
			default:
				return next(ctx, method, params)
			}
		}
	}
}

func (s *standardService) GetLatestBlockNumber(
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

func (s *standardService) BlockNumber(ctx context.Context) (hexutil.Uint64, error) {
	r, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return 0, err
	}
	return (hexutil.Uint64)(*r.End), nil
}

func (s *standardService) GetBlockHeaderByNumber(
	ctx context.Context,
	blockNumber rpc.BlockNumber,
) (*evm.ExtendedHeader, error) {
	headers, err := QueryRangeWithCache(ctx, s, nil, &blockNumber, nil, nil,
		func(st *evm.Slot) ([]evm.ExtendedHeader, error) {
			return []evm.ExtendedHeader{*st.Header}, nil
		},
		func(ctx context.Context, r rg.Range) ([]evm.ExtendedHeader, error) {
			return s.store.QueryBlocks(ctx, fmt.Sprintf("block_number = %d", r.Start))
		},
	)
	if err != nil {
		return nil, err
	}
	if len(headers) == 0 {
		return nil, nil
	}
	return &headers[0], nil
}

func (s *standardService) GetBlockByNumber(
	ctx context.Context,
	blockNumber rpc.BlockNumber,
	withFullTransactions bool,
) (*evm.RPCGetBlockResponse, error) {
	responses, err := QueryRangeWithCache(ctx, s, nil, &blockNumber, nil, nil,
		func(st *evm.Slot) ([]evm.RPCGetBlockResponse, error) {
			return []evm.RPCGetBlockResponse{evm.NewRPCGetBlockResponse(st, withFullTransactions)}, nil
		},
		func(ctx context.Context, r rg.Range) ([]evm.RPCGetBlockResponse, error) {
			headers, err := s.store.QueryBlocks(ctx, fmt.Sprintf("block_number = %d", r.Start))
			if err != nil {
				return nil, err
			}
			if len(headers) == 0 {
				return nil, nil
			}
			if !withFullTransactions {
				txs, getTxErr := s.store.QueryBlockTxHashes(ctx, r.Start)
				if getTxErr != nil {
					return nil, getTxErr
				}
				return []evm.RPCGetBlockResponse{{
					ExtendedHeader: &headers[0],
					TxHashes:       utils.MapSliceNoError(txs, common.HexToHash),
				}}, nil
			} else {
				txs, getTxErr := s.store.QueryTxs(ctx, fmt.Sprintf("block_number = %d", r.Start))
				if getTxErr != nil {
					return nil, getTxErr
				}
				return []evm.RPCGetBlockResponse{{
					ExtendedHeader: &headers[0],
					Transactions: utils.MapSliceNoError(txs, func(tx evm.ExtendedTransaction) evm.RPCTransaction {
						return tx.RPCTransaction
					}),
				}}, nil
			}
		},
	)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, nil
	}
	return &responses[0], nil
}

func (s *standardService) GetBlockByHash(
	ctx context.Context,
	hash common.Hash,
	withFullTransactions bool,
) (*evm.RPCGetBlockResponse, error) {
	responses, err := QueryRangeWithCache(ctx, s, &hash, nil, nil, nil,
		func(st *evm.Slot) ([]evm.RPCGetBlockResponse, error) {
			return []evm.RPCGetBlockResponse{evm.NewRPCGetBlockResponse(st, withFullTransactions)}, nil
		},
		nil,
	)
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, nil
	}
	return &responses[0], nil
}

func (s *standardService) GetBlockReceipts(
	ctx context.Context,
	numOrHash rpc.BlockNumberOrHash,
) ([]evm.ExtendedReceipt, error) {
	return QueryRangeWithCache(ctx, s, numOrHash.BlockHash, numOrHash.BlockNumber, nil, nil,
		func(st *evm.Slot) ([]evm.ExtendedReceipt, error) {
			return st.Receipts, nil
		},
		func(ctx context.Context, r rg.Range) ([]evm.ExtendedReceipt, error) {
			where := fmt.Sprintf("block_number = %d", r.Start)
			logs, queryLogErr := s.store.QueryLogs(ctx, where)
			if queryLogErr != nil {
				return nil, queryLogErr
			}
			logsMap := make(map[uint64][]*types.Log)
			for i := range logs {
				logsMap[uint64(logs[i].TxIndex)] = append(logsMap[uint64(logs[i].TxIndex)], &logs[i])
			}
			txs, queryTxErr := s.store.QueryTxs(ctx, where)
			if queryTxErr != nil {
				return nil, queryTxErr
			}
			var receipts []evm.ExtendedReceipt
			for i := range txs {
				if receipt := txs[i].ExtendedReceipt; receipt != nil {
					receipt.SetLogs(logsMap[uint64(receipt.TransactionIndex)])
					receipts = append(receipts, *receipt)
				}
			}
			return receipts, nil
		},
	)
}

func (s *standardService) GetLogs(ctx context.Context, args *evm.EthGetLogsArgs) ([]evm.LogWithCustomSerDe, error) {
	filterAndConvert := func(raw []types.Log) (result []evm.LogWithCustomSerDe) {
		for _, log := range raw {
			if logFilter(&log, args) {
				result = append(result, evm.LogWithCustomSerDe(log))
			}
		}
		return result
	}
	return QueryRangeWithCache(ctx, s, args.BlockHash, nil, args.FromBlock, args.ToBlock,
		func(st *evm.Slot) ([]evm.LogWithCustomSerDe, error) {
			return filterAndConvert(st.Logs), nil
		},
		func(ctx context.Context, r rg.Range) ([]evm.LogWithCustomSerDe, error) {
			where := fmt.Sprintf("block_number >= %d AND block_number <= %d", r.Start, *r.End)
			if len(args.Addresses) > 0 {
				origin := utils.MapSliceNoError(args.Addresses, common.Address.Hex)
				lower := utils.MapSliceNoError(origin, strings.ToLower)
				addresses := set.SmartNew[string](origin, lower).DumpValues()
				where += fmt.Sprintf(" AND address IN ('%s')", strings.Join(addresses, "','"))
			}
			for i, set := range args.Topics {
				if len(set) == 0 {
					continue
				}
				arg := make([]string, len(set))
				for j, v := range set {
					arg[j] = strings.ToLower(v.Hex())
				}
				// type of topics is Array(String), and Array indices are 1-based in clickhouse, so need to use i+1 here
				where += fmt.Sprintf(" AND topics[%d] IN ('%s')", i+1, strings.Join(arg, "','"))
			}
			logs, err := s.store.QueryLogs(ctx, where)
			if err != nil {
				return nil, err
			}
			// topics filtering condition is not strict enough, need post-filtering
			return filterAndConvert(logs), nil
		},
	)
}

func (s *standardService) GetBlocksPacked(
	ctx context.Context,
	fromBlock hexutil.Uint64,
	toBlock hexutil.Uint64,
	needTransaction bool,
	needReceipt bool,
	needReceiptLogs bool,
	needTraces bool,
) ([]*evm.PackedBlock, error) {
	// TODO

	slots, uncachedRange, err := getSlotsFromCache(ctx, s.slotCache, nil, &fromBlock, &toBlock)
	if err != nil {
		return nil, err
	}

	results := make([]*evm.PackedBlock, 0)
	for _, st := range slots {
		block := evm.PackedBlock{BlockHeader: st.Header}
		if needTransaction {
			block.RelevantTransactions = st.Block.Transactions
		}
		if needReceipt {
			block.RelevantTransactionReceipts = st.Receipts
		}
		if needReceiptLogs {
			for _, receipt := range block.RelevantTransactionReceipts {
				for _, rl := range receipt.Logs {
					block.Logs = append(block.Logs, *rl)
				}
			}
		}
		if needTraces {
			block.Traces = st.Traces
		}
		results = append(results, &block)
	}

	if !uncachedRange.IsEmpty() {
		nextResults, err := ResultsFromNext[*evm.PackedBlock](ctx, "eth_getBlocksPacked",
			fromBlock,
			toBlock,
			needTransaction,
			needReceipt,
			needReceiptLogs,
			needTraces,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, nextResults...)
	}
	return results, nil
}

func (s *standardService) GetLogsPacked(
	ctx context.Context,
	args *evm.EthGetLogsArgs,
	needTransaction bool,
	needReceipt bool,
	needReceiptLogs bool,
) ([]*evm.PackedBlock, error) {
	slots, uncachedRange, err := getSlotsFromCache(ctx, s.slotCache, args.BlockHash, args.FromBlock, args.ToBlock)
	if err != nil {
		return nil, err
	}

	results := make([]*evm.PackedBlock, 0)
	for _, st := range slots {
		var logs []types.Log
		for _, log := range st.Logs {
			if logFilter(&log, args) {
				logs = append(logs, log)
			}
		}
		if len(logs) == 0 {
			continue
		}
		results = append(results, evm.MakePackedBlock(st, logs, nil, needTransaction, needReceipt, needReceiptLogs))
	}

	if !uncachedRange.IsEmpty() {
		from := hexutil.Uint64(uncachedRange.Start)
		to := hexutil.Uint64(*uncachedRange.End)
		nextResults, err := ResultsFromNext[*evm.PackedBlock](ctx, "eth_getLogsPacked", &evm.EthGetLogsArgs{
			Addresses: args.Addresses,
			Topics:    args.Topics,
			BlockHash: args.BlockHash,
			FromBlock: &from,
			ToBlock:   &to,
		}, needTransaction, needReceipt, needReceiptLogs)
		if err != nil {
			return nil, err
		}
		results = append(results, nextResults...)
	}
	return results, nil
}

func (s *standardService) convertBlockNumber(
	ctx context.Context,
	sn rpc.BlockNumber,
) (uint64, error) {
	if sn >= 0 {
		return uint64(sn), nil
	}
	switch sn {
	case rpc.LatestBlockNumber:
		r, err := s.slotCache.GetRange(ctx)
		if err != nil {
			return 0, err
		}
		return *r.End, nil
	case rpc.EarliestBlockNumber:
		return 0, nil
	default:
		return 0, errors.Errorf("unsupported block tag: %s", sn)
	}
}

func QueryRangeWithCache[ELEM any](
	ctx context.Context,
	s *standardService,
	hash *common.Hash,
	blockNumber *rpc.BlockNumber,
	fromBlock *hexutil.Uint64,
	toBlock *hexutil.Uint64,
	collectFromSlot func(st *evm.Slot) ([]ELEM, error),
	collectFromStore func(ctx context.Context, r rg.Range) ([]ELEM, errror),
) ([]ELEM, error) {
	if hash != nil {
		st, err := s.slotCache.GetByHash(ctx, hash.String())
		if err != nil {
			if errors.Is(err, chain.ErrSlotNotFound) {
				return nil, jsonrpc.CallNextMiddleware
			}
			return nil, err
		}
		return collectFromSlot(st), nil
	}

	var sn, en uint64
	if blockNumber != nil {
		if blockNumber != rpc.LatestBlockNumber {
			return nil, jsonrpc.CallNextMiddleware
		}
		r, err := s.slotCache.GetRange(ctx)
		if err != nil {
			return nil, err
		}
		sn, en = *r.End, *r.End
	} else if fromBlock == nil || toBlock == nil {
		r, err := s.slotCache.GetRange(ctx)
		if err != nil {
			return nil, err
		}
		if fromBlock == nil {
			sn = *r.End
		} else {
			sn = (uint64)(*fromBlock)
		}
		if toBlock == nil {
			en = *r.End
		} else {
			en = (uint64)(*toBlock)
		}
	} else {
		sn, en = (uint64)(*fromBlock), (uint64)(*toBlock)
	}

	return chain.QueryRangeWithCache[*evm.Slot, ELEM](
		ctx,
		rg.NewRange(sn, en),
		s.slotCache,
		collectFromSlot,
		func(ctx context.Context, queryRange rg.Range) (results []ELEM, err error) {
			r, getErr := s.rangeStore.Get(ctx)
			if getErr != nil {
				return nil, getErr
			}
			if !r.Include(queryRange) {
				return nil, errors.Errorf("request range %s not in scope of range store %s", queryRange, r)
			}
			return collectFromStore(ctx, queryRange)
		},
	)
}

func (s *standardService) getSlotFromCache(
	ctx context.Context,
	slotCache chain.LatestSlotCache[*evm.Slot],
	sn evmrpc.BlockNumber,
) (*evm.Slot, error) {
	cacheRange, err := slotCache.GetRange(ctx)
	if err != nil {
		return nil, err
	}
	var bn uint64
	if sn < 0 {
		if sn != evmrpc.LatestBlockNumber {
			return nil, fmt.Errorf("%w: unsupported block number tag %s", jsonrpc.CallNextMiddleware, sn.String())
		}
		bn = *cacheRange.End
	} else {
		bn = uint64(sn)
	}
	if bn > *cacheRange.End {
		return nil, ErrBlockNumberTooBig
	}
	if !cacheRange.Contains(bn) {
		return nil, ErrCacheMissing
	}
	var st *evm.Slot
	st, err = slotCache.GetByNumber(ctx, bn)
	if errors.Is(err, chain.ErrSlotNotFound) {
		return nil, ErrCacheMissing
	}
	if err != nil {
		return nil, err
	}
	return st, nil
}
