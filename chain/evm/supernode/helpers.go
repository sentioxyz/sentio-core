package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/rpc"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func logFilter(log *types.Log, args *evm.EthGetLogsArgs) bool {
	if len(args.Addresses) > 0 {
		found := false
		for _, address := range args.Addresses {
			if log.Address == address {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(args.Topics) > 0 {
		for i, topic := range args.Topics {
			if len(log.Topics) <= i {
				return false
			}
			if topic == nil {
				continue
			}
			found := false
			for _, t := range topic {
				if log.Topics[i] == t {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

func ResultsFromNext[T any](ctx context.Context, methodName string, args ...any) ([]T, error) {
	next, err := jsonrpc.NextHandleFromContext(ctx)
	if err != nil {
		return nil, err
	}
	params, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	results, err := next(ctx, methodName, params)
	if err != nil {
		return nil, err
	}
	if r, ok := results.([]T); ok {
		log.Debugf("merged %d results from next", len(r))
		return r, nil
	} else {
		return nil, fmt.Errorf("typ mismatch, next handler should return []%T", results)
	}
}

type baseClickhouseService struct {
	store Storage
}

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
			return nil, fmt.Errorf("miss block header %d for log %d in txn %s", lg.BlockNumber, lg.Index, lg.TxHash.String())
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
