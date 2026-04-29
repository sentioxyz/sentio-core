package supernode

import (
	"context"
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
			return nil, fmt.Errorf("miss block header %d for txn %s", tx.BlockNumber, tx.Hash.String())
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
			return nil, fmt.Errorf("miss block header %d for trace in txn %s", trace.BlockNumber, trace.TransactionHash)
		}
		block.Traces = append(block.Traces, trace)
	}
	return utils.GetMapValuesOrderByKey(blocks), nil
}

func queryWithCache[ELEM any](
	ctx context.Context,
	slotCache chain.LatestSlotCache[*evm.Slot],
	hash *common.Hash,
	blockNumber *rpc.BlockNumber,
	fromBlock *hexutil.Uint64,
	toBlock *hexutil.Uint64,
	collectFromSlot func(st *evm.Slot) ([]ELEM, error),
	collectFromStore func(ctx context.Context, r rg.Range) ([]ELEM, error),
	cacheMissHashReturn error,
) ([]ELEM, error) {
	if hash != nil {
		st, err := slotCache.GetByHash(ctx, hash.String())
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
		if *blockNumber != rpc.LatestBlockNumber {
			return nil, jsonrpc.CallNextMiddleware
		}
		r, err := slotCache.GetRange(ctx)
		if err != nil {
			return nil, err
		}
		sn, en = *r.End, *r.End
	} else if fromBlock == nil || toBlock == nil {
		r, err := slotCache.GetRange(ctx)
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
		slotCache,
		collectFromSlot,
		collectFromStore,
	)
}

func checkRange[ELEM any](
	rangeStore chain.RangeStore,
	do func(context.Context, rg.Range) ([]ELEM, error),
) func(context.Context, rg.Range) ([]ELEM, error) {
	return func(ctx context.Context, queryRange rg.Range) ([]ELEM, error) {
		r, err := rangeStore.Get(ctx)
		if err != nil {
			return nil, err
		}
		if !r.Include(queryRange) {
			return nil, errors.Errorf("request range %s not in scope of range store %s", queryRange, r)
		}
		return do(ctx, r)
	}
}
