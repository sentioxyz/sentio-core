package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio/chain/chain"
	"sentioxyz/sentio/chain/evm"
	"sentioxyz/sentio/chain/node"
	"sentioxyz/sentio/chain/proxyv3"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func NewSimpleProxyService(
	network string,
	client node.NodeClient,
	cacheDelay time.Duration,
	slotCache chain.LatestSlotCache[*evm.Slot],
	proxySvr *proxyv3.JSONRPCServiceV2,
	forcedProxyMethods []string,
) []jsonrpc.Middleware {
	return []jsonrpc.Middleware{
		NewCustomFunctionProxyMiddleware(proxySvr),
		NewProxyExtraMiddleware(client, cacheDelay, proxySvr),
		NewSubscribeMiddleware(slotCache),
		NewForcedProxyMiddleware(proxySvr, forcedProxyMethods),
		NewProxyWithLatestSlotCacheMiddleware(slotCache, proxySvr),
		NewProxyMiddleware(client, cacheDelay, proxySvr),
	}
}

func NewRPCServiceV2(
	network string,
	chainID string,
	networkOptions *evm.NetworkOptions,
	slotCache chain.LatestSlotCache[*evm.Slot],
	store evm.Storage,
	rangeStore chain.RangeStore,
	client node.NodeClient,
	cacheDelay time.Duration,
	proxySvr *proxyv3.JSONRPCServiceV2,
	forcedProxyMethods []string,
) []jsonrpc.Middleware {

	base := baseClickhouseService{
		store: store,
	}

	chainIDNum, err := strconv.ParseUint(chainID, 0, 64)
	if err != nil {
		panic(fmt.Errorf("chainID %q is not a number", chainID))
	}

	return []jsonrpc.Middleware{
		NewErrLogMiddleware(),
		NewVersionMiddleware(chainIDNum),
		NewCustomFunctionProxyMiddleware(proxySvr),
		NewForcedProxyMiddleware(proxySvr, forcedProxyMethods),
		NewExtraMiddleware(slotCache, rangeStore, base),
		NewSubscribeMiddleware(slotCache),
		NewEthSlotCacheMiddleware(slotCache),
		NewTraceSlotCacheMiddleware(slotCache, networkOptions),
		NewRangeCheckMiddleware(rangeStore),
		NewEthClickHouseMiddleware(base),
		NewTraceClickHouseMiddleware(base),
		NewProxyMiddleware(client, cacheDelay, proxySvr),
	}
}

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
	store evm.Storage
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
