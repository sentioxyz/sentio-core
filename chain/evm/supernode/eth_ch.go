package supernode

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	evmrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/utils"
	"sort"
	"strconv"
	"strings"
)

type EthClickHouse struct {
	baseClickhouseService
}

func NewEthClickHouseMiddleware(base baseClickhouseService) jsonrpc.Middleware {
	return jsonrpc.MakeServiceAsMiddleware("eth", &EthClickHouse{
		baseClickhouseService: base,
	})
}

func (s *EthClickHouse) GetBlockHeaderByNumber(
	ctx context.Context,
	blockNumber evmrpc.BlockNumber,
) (*evm.ExtendedHeader, error) {
	if blockNumber < 0 {
		return nil, jsonrpc.CallNextMiddleware
	}
	blocks, err := s.store.QueryBlocks(ctx, fmt.Sprintf("block_number = %d", blockNumber.Int64()))
	if err != nil {
		return nil, err
	}
	if len(blocks) == 0 {
		return nil, ethereum.NotFound
	}
	return &blocks[0], nil
}

func (s *EthClickHouse) GetBlockByNumber(
	ctx context.Context,
	blockNumber evmrpc.BlockNumber,
	withFullTransactions bool,
) (interface{}, error) {
	header, err := s.GetBlockHeaderByNumber(ctx, blockNumber)
	if err != nil {
		return nil, err
	}
	if !withFullTransactions {
		txs, getTxErr := s.store.QueryBlockTxHashes(ctx, uint64(blockNumber))
		if getTxErr != nil {
			return nil, getTxErr
		}
		return &evm.RPCBlockSimpleResponse{
			ExtendedHeader: header,
			Hash:           header.Hash,
			Transactions:   txs,
		}, nil
	} else {
		txs, getTxErr := s.store.QueryTxs(ctx, fmt.Sprintf("block_number = %d", blockNumber.Int64()))
		if getTxErr != nil {
			return nil, getTxErr
		}
		return &evm.RPCBlockResponse{
			ExtendedHeader: header,
			Hash:           header.Hash,
			Transactions: utils.MapSliceNoError(txs, func(tx evm.ExtendedTransaction) evm.RPCTransaction {
				return tx.RPCTransaction
			}),
		}, nil
	}
}

func (s *EthClickHouse) GetBlocksByNumber(
	ctx context.Context,
	blockNumbers []hexutil.Uint64,
) ([]*evm.ExtendedHeader, error) {
	if len(blockNumbers) == 0 {
		return nil, nil
	}
	where := fmt.Sprintf("block_number IN (%s)",
		strings.Join(utils.MapSliceNoError(blockNumbers, func(n hexutil.Uint64) string {
			return strconv.FormatUint(uint64(n), 10)
		}), ","))
	blocks, err := s.store.QueryBlocks(ctx, where)
	if err != nil {
		return nil, err
	}
	// sort by the order in blockNumbers
	numberIndex := make(map[uint64]int)
	for i, num := range blockNumbers {
		numberIndex[uint64(num)] = i
	}
	sort.Slice(blocks, func(i, j int) bool {
		return numberIndex[blocks[i].Number.Uint64()] < numberIndex[blocks[j].Number.Uint64()]
	})
	return utils.WrapPointerForArray(blocks), nil
}

func (s *EthClickHouse) GetBlockReceipts(
	ctx context.Context,
	blockNumber evmrpc.BlockNumber,
) (receipts []evm.ExtendedReceipt, err error) {
	if blockNumber < 0 {
		return nil, jsonrpc.CallNextMiddleware
	}
	where := fmt.Sprintf("block_number = %d", blockNumber.Int64())
	logs, err := s.store.QueryLogs(ctx, where)
	if err != nil {
		return nil, err
	}
	logsMap := make(map[uint64][]*types.Log)
	for i := range logs {
		logsMap[uint64(logs[i].TxIndex)] = append(logsMap[uint64(logs[i].TxIndex)], &logs[i])
	}
	txs, err := s.store.QueryTxs(ctx, where)
	if err != nil {
		return nil, err
	}
	for i := range txs {
		if r := txs[i].ExtendedReceipt; r != nil {
			r.SetLogs(logsMap[uint64(r.TransactionIndex)])
			receipts = append(receipts, *r)
		}
	}
	return receipts, nil
}

func (s *EthClickHouse) GetLogs(ctx context.Context, args *evm.EthGetLogsArgs) ([]evm.LogWithCustomSerDe, error) {
	where := "1"
	if len(args.Addresses) > 0 {
		origin := utils.MapSliceNoError(args.Addresses, common.Address.Hex)
		lower := utils.MapSliceNoError(origin, strings.ToLower)
		addresses := utils.GetMapKeys(utils.BuildSet(append(origin, lower...)))
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
	if args.BlockHash != nil {
		where += fmt.Sprintf(" AND block_hash = '%s'", strings.ToLower(args.BlockHash.Hex()))
	}
	if args.FromBlock != nil {
		fromBlock := number.Number(*args.FromBlock)
		where += fmt.Sprintf(" AND block_number >= %d", fromBlock)
	}
	if args.ToBlock != nil {
		toBlock := number.Number(*args.ToBlock)
		where += fmt.Sprintf(" AND block_number <= %d", toBlock)
	}
	if where == "1" {
		return nil, errors.Errorf("can't get logs without any filter")
	}

	logs, err := s.store.QueryLogs(ctx, where)
	if err != nil {
		return nil, err
	}
	// topics filtering condition is not strict enough, need post-filtering
	logs = utils.FilterArr(logs, func(log types.Log) bool {
		return logFilter(&log, args)
	})
	return utils.MapSliceNoError(logs, func(lg types.Log) evm.LogWithCustomSerDe {
		return evm.LogWithCustomSerDe(lg)
	}), nil
}

func (s *EthClickHouse) GetBlocksPacked(
	ctx context.Context,
	fromBlock hexutil.Uint64,
	toBlock hexutil.Uint64,
	needTransaction bool,
	needReceipt bool,
	needReceiptLogs bool,
	needTraces bool,
) (blocks []*evm.PackedBlock, err error) {
	where := fmt.Sprintf("block_number >= %d AND block_number <= %d", fromBlock, toBlock)

	var headers []evm.ExtendedHeader
	var txs []evm.ExtendedTransaction
	var fullLogs []types.Log
	var traces []evm.ParityTrace
	headers, err = s.store.QueryBlocks(ctx, where)
	if err != nil {
		return nil, err
	}
	if len(headers) == 0 {
		return nil, nil
	}
	if needTransaction {
		txs, err = s.store.QueryTxs(ctx, where)
		if err != nil {
			return nil, err
		}
		if !needReceipt {
			for i := range txs {
				txs[i].ExtendedReceipt = nil
			}
		} else if needReceiptLogs {
			fullLogs, err = s.store.QueryLogs(ctx, where)
			if err != nil {
				return nil, err
			}
		}
	}
	if needTraces {
		traces, err = s.store.QueryTraces(ctx, where)
		if err != nil {
			return nil, err
		}
	}
	return buildPackedBlocks(headers, txs, fullLogs, fullLogs, traces)
}

func (s *EthClickHouse) GetLogsPacked(
	ctx context.Context,
	args *evm.EthGetLogsArgs,
	needTransaction,
	needReceipt,
	needReceiptLogs bool,
) ([]*evm.PackedBlock, error) {
	if args.FromBlock == nil || args.ToBlock == nil {
		return nil, fmt.Errorf("fromBlock and toBlock are both required")
	}

	blockWheres := fmt.Sprintf("block_number >= %d AND block_number <= %d", *args.FromBlock, *args.ToBlock)
	where := blockWheres
	if len(args.Addresses) > 0 {
		origin := utils.MapSliceNoError(args.Addresses, common.Address.Hex)
		lower := utils.MapSliceNoError(origin, strings.ToLower)
		addresses := utils.GetMapKeys(utils.BuildSet(append(origin, lower...)))
		where += fmt.Sprintf(" AND address IN ('%s')", strings.Join(addresses, "','"))
	}
	if len(args.Topics) > 0 {
		var topics []string
		for _, topic := range args.Topics {
			if len(topic) > 0 {
				topics = append(topics, fmt.Sprintf("'%s'", strings.ToLower(topic[0].Hex())))
			}
		}
		where += fmt.Sprintf(" AND hasAny(topics, [%s])", strings.Join(topics, ","))
	}
	if args.BlockHash != nil {
		where += fmt.Sprintf(" AND block_hash = '%s'", strings.ToLower(args.BlockHash.Hex()))
	}

	var headers []evm.ExtendedHeader
	var txs []evm.ExtendedTransaction
	var logs []types.Log
	var fullLogs []types.Log
	var err error

	logs, err = s.store.QueryLogs(ctx, where)
	if err != nil {
		return nil, err
	}
	// topics filtering condition is not strict enough, need post-filtering
	logs = utils.FilterArr(logs, func(log types.Log) bool {
		return logFilter(&log, args)
	})
	if len(logs) == 0 {
		return nil, nil
	}

	blockNumbers := utils.BuildSet(utils.MapSliceNoError(logs, func(lg types.Log) uint64 {
		return lg.BlockNumber
	}))
	if len(blockNumbers) > 100 {
		blockWheres = fmt.Sprintf("%s AND block_number IN (%s)",
			blockWheres, s.store.QueryLogsBlockSQL(where))
	} else {
		blockWheres = fmt.Sprintf("%s AND block_number IN (%s)",
			blockWheres, strings.Join(utils.MapSliceNoError(utils.GetMapKeys(blockNumbers), utils.UIntFormatter(10)), ","))
	}

	headers, err = s.store.QueryBlocks(ctx, blockWheres)
	if err != nil {
		return nil, err
	}
	// because topics filtering condition is not strict enough, so headers got maybe more than needed, need post-filtering
	headers = utils.FilterArr(headers, func(header evm.ExtendedHeader) bool {
		return blockNumbers[header.Number.Uint64()]
	})
	if needTransaction {
		txs, err = s.store.QueryTxs(ctx, blockWheres)
		if err != nil {
			return nil, err
		}
		if !needReceipt {
			for i := range txs {
				txs[i].ExtendedReceipt = nil
			}
		} else if needReceiptLogs {
			fullLogs, err = s.store.QueryLogs(ctx, blockWheres)
			if err != nil {
				return nil, err
			}
		}
	}
	return buildPackedBlocks(headers, txs, logs, fullLogs, nil)
}
