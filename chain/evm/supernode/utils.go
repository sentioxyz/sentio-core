package supernode

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

func buildPackedBlocks(
	headers []evm.ExtendedHeader,
	txs []evm.ExtendedTransaction,
	logs []types.Log,
	fullLogs []types.Log,
	traces []evm.ParityTrace,
) ([]*evm.PackedBlock, error) {
	blocks := make(map[uint64]*evm.PackedBlock)
	for i := range headers {
		blocks[headers[i].Number.Uint64()] = &evm.PackedBlock{BlockHeader: &headers[i]}
	}
	for _, lg := range logs {
		block, has := blocks[lg.BlockNumber]
		if !has {
			return nil, errors.Errorf("miss block header %d for log %d in txn %s", lg.BlockNumber, lg.Index, lg.TxHash)
		}
		block.Logs = append(block.Logs, lg)
	}
	fullLogsMap := make(map[common.Hash][]*types.Log)
	for i := range fullLogs {
		lg := &fullLogs[i]
		fullLogsMap[lg.TxHash] = append(fullLogsMap[lg.TxHash], lg)
	}
	for _, tx := range txs {
		block, has := blocks[tx.BlockNumber]
		if !has {
			return nil, errors.Errorf("miss block header %d for txn %s", tx.BlockNumber, tx.Hash.String())
		}
		block.RelevantTransactions = append(block.RelevantTransactions, tx.RPCTransaction)
		if r := tx.ExtendedReceipt; r != nil {
			r.SetLogs(fullLogsMap[tx.Hash])
			block.RelevantTransactionReceipts = append(block.RelevantTransactionReceipts, *r)
		}
	}
	for _, trace := range traces {
		block, has := blocks[trace.BlockNumber]
		if !has {
			return nil, errors.Errorf("miss block header %d for trace in txn %s", trace.BlockNumber, trace.TransactionHash)
		}
		block.Traces = append(block.Traces, trace)
	}
	return utils.GetMapValuesOrderByKey(blocks), nil
}

func queryWithCache[ELEM any](
	ctx context.Context,
	slotCache chain.LatestSlotCache[*evm.Slot],
	blockHash *common.Hash,
	blockNumber *rpc.BlockNumber,
	fromBlock *rpc.BlockNumber,
	toBlock *rpc.BlockNumber,
	maxQueryRangeSize uint64,
	collectFromSlot func(st *evm.Slot) ([]ELEM, error),
	collectFromStore func(ctx context.Context, r rg.Range) ([]ELEM, error),
	cacheMissHashReturn error,
) ([]ELEM, error) {
	if blockHash != nil {
		st, err := slotCache.GetByHash(ctx, blockHash.String())
		if err != nil {
			if errors.Is(err, chain.ErrSlotNotFound) {
				return nil, cacheMissHashReturn
			}
			return nil, err
		}
		return collectFromSlot(st)
	}
	var sn, en uint64
	if blockNumber != nil {
		if *blockNumber >= 0 {
			sn, en = (uint64)(*blockNumber), (uint64)(*blockNumber)
		} else {
			if *blockNumber == rpc.LatestBlockNumber {
				r, err := slotCache.GetRange(ctx)
				if err != nil {
					return nil, err
				}
				sn, en = *r.End, *r.End
			} else {
				return nil, jsonrpc.CallNextMiddleware
			}
		}
	} else {
		if fromBlock == nil {
			fromBlock = utils.WrapPointer(rpc.LatestBlockNumber)
		}
		if toBlock == nil {
			toBlock = utils.WrapPointer(rpc.LatestBlockNumber)
		}
		// slotCache only holds the latest block, other tags fall through to the next handler
		if *fromBlock < 0 && *fromBlock != rpc.LatestBlockNumber {
			return nil, jsonrpc.CallNextMiddleware
		}
		if *toBlock < 0 && *toBlock != rpc.LatestBlockNumber {
			return nil, jsonrpc.CallNextMiddleware
		}
		if *fromBlock >= 0 && *toBlock >= 0 {
			sn, en = (uint64)(*fromBlock), (uint64)(*toBlock)
		} else {
			r, err := slotCache.GetRange(ctx)
			if err != nil {
				return nil, err
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
		}
	}
	// The span cap applies to the REQUESTED range — a caller-visible contract — not to the
	// sub-range left over after the latest-slot cache serves its part: whether the cache covers
	// some blocks is an internal, dynamically changing detail a caller cannot reason about.
	if maxQueryRangeSize > 0 {
		if err := chain.CheckQuerySpan(sn, en, maxQueryRangeSize); err != nil {
			return nil, err
		}
	}
	return chain.QueryRangeWithCache[*evm.Slot, ELEM](
		ctx,
		rg.NewRange(sn, en),
		slotCache,
		collectFromSlot,
		collectFromStore,
	)
}
