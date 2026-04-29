package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
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
			case "eth_getBlocksPacked":
				return jsonrpc.CallMethod(s.GetBlocksPacked, ctx, params)
			case "eth_getLogs":
				return jsonrpc.CallMethod(s.GetLogs, ctx, params)
			case "eth_getLogsPacked":
				return jsonrpc.CallMethod(s.GetLogsPacked, ctx, params)
			case "trace_filter":
				return jsonrpc.CallMethod(s.GetLogsPacked, ctx, params)
			case "trace_filterPacked":
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
	headers, err := queryWithCache(ctx, s.slotCache, nil, &blockNumber, nil, nil,
		func(st *evm.Slot) ([]evm.ExtendedHeader, error) {
			return []evm.ExtendedHeader{*st.Header}, nil
		},
		checkRange(s.rangeStore, func(ctx context.Context, r rg.Range) ([]evm.ExtendedHeader, error) {
			return s.store.QueryBlocks(ctx, fmt.Sprintf("block_number = %d", r.Start))
		}),
		jsonrpc.CallNextMiddleware,
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
	responses, err := queryWithCache(ctx, s.slotCache, nil, &blockNumber, nil, nil,
		func(st *evm.Slot) ([]evm.RPCGetBlockResponse, error) {
			return []evm.RPCGetBlockResponse{evm.NewRPCGetBlockResponse(st, withFullTransactions)}, nil
		},
		checkRange(s.rangeStore, func(ctx context.Context, r rg.Range) ([]evm.RPCGetBlockResponse, error) {
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
		}),
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

func (s *standardService) GetBlockByHash(
	ctx context.Context,
	hash common.Hash,
	withFullTransactions bool,
) (*evm.RPCGetBlockResponse, error) {
	responses, err := queryWithCache(ctx, s.slotCache, &hash, nil, nil, nil,
		func(st *evm.Slot) ([]evm.RPCGetBlockResponse, error) {
			return []evm.RPCGetBlockResponse{evm.NewRPCGetBlockResponse(st, withFullTransactions)}, nil
		},
		nil,
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

func (s *standardService) GetBlockReceipts(
	ctx context.Context,
	numOrHash rpc.BlockNumberOrHash,
) ([]evm.ExtendedReceipt, error) {
	return queryWithCache(ctx, s.slotCache, numOrHash.BlockHash, numOrHash.BlockNumber, nil, nil,
		func(st *evm.Slot) ([]evm.ExtendedReceipt, error) {
			return st.Receipts, nil
		},
		checkRange(s.rangeStore, func(ctx context.Context, r rg.Range) ([]evm.ExtendedReceipt, error) {
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
		}),
		jsonrpc.CallNextMiddleware,
	)
}

func (s *standardService) queryPackedBlockAppendPart(
	ctx context.Context,
	blockWhere string,
	needTransaction bool,
	needReceipt bool,
	needReceiptLogs bool,
) (headers []evm.ExtendedHeader, txs []evm.ExtendedTransaction, fullLogs []types.Log, err error) {
	headers, err = s.store.QueryBlocks(ctx, blockWhere)
	if err != nil {
		return
	}
	if len(headers) == 0 {
		return
	}
	if !needTransaction {
		return
	}
	txs, err = s.store.QueryTxs(ctx, blockWhere)
	if err != nil {
		return
	}
	if !needReceipt {
		for i := range txs {
			txs[i].ExtendedReceipt = nil
		}
		return
	}
	if !needReceiptLogs {
		return
	}
	fullLogs, err = s.store.QueryLogs(ctx, blockWhere)
	return
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
	return queryWithCache(ctx, s.slotCache, nil, nil, &fromBlock, &toBlock,
		func(st *evm.Slot) ([]*evm.PackedBlock, error) {
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
			return []*evm.PackedBlock{&block}, nil
		},
		checkRange(s.rangeStore, func(ctx context.Context, r rg.Range) ([]*evm.PackedBlock, error) {
			where := fmt.Sprintf("block_number >= %d AND block_number <= %d", r.Start, *r.End)
			headers, txs, fullLogs, err := s.queryPackedBlockAppendPart(
				ctx, where, needTransaction, needReceipt, needReceiptLogs)
			var traces []evm.ParityTrace
			if needTraces {
				traces, err = s.store.QueryTraces(ctx, where)
				if err != nil {
					return nil, err
				}
			}
			return buildPackedBlocks(headers, txs, fullLogs, fullLogs, traces)
		}),
		ErrCacheMissing,
	)
}

func (s *standardService) filterLogSQL(args *evm.EthGetLogsArgs) []string {
	var wheres []string
	if len(args.Addresses) > 0 {
		origin := utils.MapSliceNoError(args.Addresses, common.Address.Hex)
		lower := utils.MapSliceNoError(origin, strings.ToLower)
		addresses := set.SmartNew[string](origin, lower).DumpValues()
		wheres = append(wheres, fmt.Sprintf("address IN ('%s')", strings.Join(addresses, "','")))
	}
	for i, items := range args.Topics {
		if len(items) == 0 {
			continue
		}
		arg := make([]string, len(items))
		for j, v := range items {
			arg[j] = strings.ToLower(v.Hex())
		}
		// type of topics is Array(String), and Array indices are 1-based in clickhouse, so need to use i+1 here
		wheres = append(wheres, fmt.Sprintf("topics[%d] IN ('%s')", i+1, strings.Join(arg, "','")))
	}
	return wheres
}

func (s *standardService) GetLogs(ctx context.Context, args *evm.EthGetLogsArgs) ([]types.Log, error) {
	checker := args.Checker()
	return queryWithCache(ctx, s.slotCache, args.BlockHash, nil, args.FromBlock, args.ToBlock,
		func(st *evm.Slot) ([]types.Log, error) {
			return utils.FilterArr(st.Logs, checker), nil
		},
		checkRange(s.rangeStore, func(ctx context.Context, r rg.Range) ([]types.Log, error) {
			blockWheres := fmt.Sprintf("block_number >= %d AND block_number <= %d", r.Start, *r.End)
			where := strings.Join(append(s.filterLogSQL(args), blockWheres), " AND ")
			logs, err := s.store.QueryLogs(ctx, where)
			if err != nil {
				return nil, err
			}
			// topics filtering condition is not strict enough, need post-filtering
			return utils.FilterArr(logs, checker), nil
		}),
		jsonrpc.CallNextMiddleware,
	)
}

func (s *standardService) GetLogsPacked(
	ctx context.Context,
	args *evm.EthGetLogsArgs,
	needTransaction bool,
	needReceipt bool,
	needReceiptLogs bool,
) ([]*evm.PackedBlock, error) {
	checker := args.Checker()
	return queryWithCache(ctx, s.slotCache, args.BlockHash, nil, args.FromBlock, args.ToBlock,
		func(st *evm.Slot) ([]*evm.PackedBlock, error) {
			logs := utils.FilterArr(st.Logs, checker)
			if len(logs) == 0 {
				return nil, nil
			}
			blk := evm.MakePackedBlock(st, logs, nil, needTransaction, needReceipt, needReceiptLogs)
			return []*evm.PackedBlock{blk}, nil
		},
		checkRange(s.rangeStore, func(ctx context.Context, r rg.Range) ([]*evm.PackedBlock, error) {
			blockWheres := fmt.Sprintf("block_number >= %d AND block_number <= %d", r.Start, *r.End)
			where := strings.Join(append(s.filterLogSQL(args), blockWheres), " AND ")
			logs, err := s.store.QueryLogs(ctx, where)
			if err != nil {
				return nil, err
			}
			logs = utils.FilterArr(logs, checker) // topics filtering condition is not strict enough, need post-filtering
			if len(logs) == 0 {
				return nil, nil
			}

			blockNumbers := set.New[uint64]()
			for _, log := range logs {
				blockNumbers.Add(log.BlockNumber)
			}
			if blockNumbers.Size() > 1000 {
				blockWheres = fmt.Sprintf("%s AND block_number IN (%s)", blockWheres, s.store.QueryLogsBlockSQL(where))
			} else {
				blockWheres = fmt.Sprintf("%s AND block_number IN (%s)",
					blockWheres, strings.Join(utils.MapSliceNoError(blockNumbers.DumpValues(), utils.UIntFormatter(10)), ","))
			}

			headers, txs, fullLogs, err := s.queryPackedBlockAppendPart(
				ctx, where, needTransaction, needReceipt, needReceiptLogs)
			// because topics filtering condition is not strict enough, so headers got maybe more than needed, need post-filtering
			headers = utils.FilterArr(headers, func(header evm.ExtendedHeader) bool {
				return blockNumbers.Contains(header.Number.Uint64())
			})
			return buildPackedBlocks(headers, txs, logs, fullLogs, nil)
		}),
		ErrCacheMissing,
	)
}

func (s *standardService) filterTraceSQL(args *evm.TraceFilterArgs) []string {
	var wheres []string
	if len(args.FromAddress) > 0 {
		addresses := utils.MapSliceNoError(args.FromAddress, func(addr common.Address) string {
			return strings.ToLower(addr.Hex())
		})
		wheres = append(wheres, fmt.Sprintf("lower(from_address) in ('%s')", strings.Join(addresses, "','")))
	}
	if len(args.ToAddress) > 0 {
		addresses := utils.MapSliceNoError(args.ToAddress, func(addr string) string {
			return strings.ToLower(addr)
		})
		wheres = append(wheres, fmt.Sprintf("lower(to_address) in ('%s')", strings.Join(addresses, "','")))
	}
	return wheres
}

func (s *standardService) TraceFilter(ctx context.Context, args *evm.TraceFilterArgs) ([]evm.ParityTrace, error) {
	checker := args.Checker()
	return queryWithCache(ctx, s.slotCache, nil, nil, args.FromBlock, args.ToBlock,
		func(st *evm.Slot) ([]evm.ParityTrace, error) {
			return utils.FilterArr(st.Traces, checker), nil
		},
		checkRange(s.rangeStore, func(ctx context.Context, r rg.Range) ([]evm.ParityTrace, error) {
			blockWheres := fmt.Sprintf("block_number >= %d AND block_number <= %d", r.Start, *r.End)
			where := strings.Join(append(s.filterTraceSQL(args), blockWheres), " AND ")
			traces, err := s.store.QueryTraces(ctx, where)
			if err != nil {
				return nil, err
			}
			return utils.FilterArr(traces, checker), nil
		}),
		jsonrpc.CallNextMiddleware,
	)
}

func (s *standardService) TraceFilterPacked(
	ctx context.Context,
	args *evm.TraceFilterArgs,
	needTransaction bool,
	needReceipt bool,
	needReceiptLogs bool,
) ([]*evm.PackedBlock, error) {
	checker := args.Checker()
	return queryWithCache(ctx, s.slotCache, nil, nil, args.FromBlock, args.ToBlock,
		func(st *evm.Slot) ([]*evm.PackedBlock, error) {
			traces := utils.FilterArr(st.Traces, checker)
			if len(traces) == 0 {
				return nil, nil
			}
			blk := evm.MakePackedBlock(st, nil, traces, needTransaction, needReceipt, needReceiptLogs)
			return []*evm.PackedBlock{blk}, nil
		},
		checkRange(s.rangeStore, func(ctx context.Context, r rg.Range) ([]*evm.PackedBlock, error) {
			blockWheres := fmt.Sprintf("block_number >= %d AND block_number <= %d", r.Start, *r.End)

			where := strings.Join(append(s.filterTraceSQL(args), blockWheres), " AND ")
			traces, err := s.store.QueryTraces(ctx, where)
			if err != nil {
				return nil, err
			}
			traces = utils.FilterArr(traces, checker)
			if len(traces) == 0 {
				return nil, nil
			}

			blockNumbers := set.New[uint64]()
			for _, trace := range traces {
				blockNumbers.Add(trace.BlockNumber)
			}
			if blockNumbers.Size() > 1000 {
				blockWheres = fmt.Sprintf("%s AND block_number IN (%s)", blockWheres, s.store.QueryLogsBlockSQL(where))
			} else {
				blockWheres = fmt.Sprintf("%s AND block_number IN (%s)",
					blockWheres, strings.Join(utils.MapSliceNoError(blockNumbers.DumpValues(), utils.UIntFormatter(10)), ","))
			}

			headers, txs, fullLogs, err := s.queryPackedBlockAppendPart(
				ctx, where, needTransaction, needReceipt, needReceiptLogs)
			// because topics filtering condition is not strict enough, so headers got maybe more than needed, need post-filtering
			headers = utils.FilterArr(headers, func(header evm.ExtendedHeader) bool {
				return blockNumbers.Contains(header.Number.Uint64())
			})
			return buildPackedBlocks(headers, txs, nil, fullLogs, traces)
		}),
		ErrCacheMissing,
	)
}
