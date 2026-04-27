package supernode

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	evmrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"math/big"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio/chain/chain"
	"sentioxyz/sentio/chain/evm"
	"sentioxyz/sentio/common/number"
)

type EthSlotCache struct {
	SlotCache chain.LatestSlotCache[*evm.Slot]
}

func NewEthSlotCacheMiddleware(slotCache chain.LatestSlotCache[*evm.Slot]) jsonrpc.Middleware {
	return jsonrpc.MakeServiceAsMiddleware("eth", &EthSlotCache{SlotCache: slotCache})
}

func (s *EthSlotCache) GetLatestBlockNumber(
	ctx context.Context,
	latestBlockNumberOver uint64,
) (evm.GetLatestBlockNumberResponse, error) {
	jsonrpc.GetCtxData(ctx).NotSlowRequest = true
	resp := evm.GetLatestBlockNumberResponse{APIVersion: evm.APIVersion}
	latest, err := s.SlotCache.Wait(ctx, number.Number(latestBlockNumberOver))
	if err != nil {
		return resp, err
	}
	resp.LatestBlockNumber = uint64(latest)
	return resp, nil
}

func (s *EthSlotCache) BlockNumber(ctx context.Context) (*hexutil.Big, error) {
	r, err := s.SlotCache.GetRange(ctx)
	if err != nil {
		return nil, err
	}
	latest := new(big.Int).SetUint64(uint64(r.R()))
	return (*hexutil.Big)(latest), nil
}

func (s *EthSlotCache) GetBlockHeaderByNumber(
	ctx context.Context,
	blockNumber evmrpc.BlockNumber,
) (*evm.ExtendedHeader, error) {
	st, err := getSlotFromCache(ctx, s.SlotCache, blockNumber)
	if err != nil {
		if errors.Is(err, evm.ErrCacheMissing) {
			return nil, jsonrpc.CallNextMiddleware
		}
		if errors.Is(err, evm.ErrBlockNumberTooBig) {
			return nil, nil
		}
		return nil, err
	}

	return st.Header, nil
}

func buildBlockResponse(st *evm.Slot, withFullTransactions bool) any {
	if !withFullTransactions {
		return &evm.RPCBlockSimpleResponse{
			ExtendedHeader: st.Header,
			Hash:           st.Header.Hash,
			Transactions: utils.MapSliceNoError(st.Block.Transactions, func(txn evm.RPCTransaction) string {
				return txn.Hash.String()
			}),
		}
	}
	txns := st.Block.Transactions
	if txns == nil {
		txns = make([]evm.RPCTransaction, 0)
	}
	return &evm.RPCBlockResponse{
		ExtendedHeader: st.Header,
		Hash:           st.Header.Hash,
		Transactions:   txns,
	}
}

func (s *EthSlotCache) GetBlockByNumber(ctx context.Context, blockNumber evmrpc.BlockNumber,
	withFullTransactions bool) (interface{}, error) {
	st, err := getSlotFromCache(ctx, s.SlotCache, blockNumber)
	if err != nil {
		if errors.Is(err, evm.ErrCacheMissing) {
			return nil, jsonrpc.CallNextMiddleware
		}
		if errors.Is(err, evm.ErrBlockNumberTooBig) {
			return nil, nil
		}
		return nil, err
	}
	return buildBlockResponse(st, withFullTransactions), nil
}

func (s *EthSlotCache) GetBlocksByNumber(ctx context.Context, blockNumbers []hexutil.Uint64) ([]*evm.ExtendedHeader, error) {
	if len(blockNumbers) == 0 {
		return nil, nil
	}
	var numbersToQuery []hexutil.Uint64
	numberToIdx := make(map[hexutil.Uint64][]int)
	results := make([]*evm.ExtendedHeader, len(blockNumbers))
	for i := range blockNumbers {
		n := blockNumbers[i]
		slot, err := getSlotFromCache(ctx, s.SlotCache, evmrpc.BlockNumber(n))
		if errors.Is(err, evm.ErrCacheMissing) {
			numbersToQuery = append(numbersToQuery, n)
			numberToIdx[n] = append(numberToIdx[n], i)
		} else if errors.Is(err, evm.ErrBlockNumberTooBig) {
			results[i] = nil
		} else if err != nil {
			return nil, err
		} else {
			results[i] = slot.Header
		}
	}
	if len(numbersToQuery) > 0 {
		headers, err := ResultsFromNext[*evm.ExtendedHeader](ctx, "eth_getblocksbynumber", numbersToQuery)

		if err != nil {
			return nil, err
		}
		for _, h := range headers {
			indices := numberToIdx[hexutil.Uint64(h.Number.Uint64())]
			if len(indices) == 0 {
				return nil, fmt.Errorf("missing index for block number %d", h.Number.Uint64())
			}
			for _, idx := range indices {
				results[idx] = h
			}
		}

	}
	return results, nil
}

func (s *EthSlotCache) GetBlockByHash(
	ctx context.Context,
	hash common.Hash,
	withFullTransactions bool,
) (any, error) {
	slot, found, err := chain.GetSlotByHash(ctx, s.SlotCache, hash.String())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, evm.ErrCacheMissing
	}
	return buildBlockResponse(slot, withFullTransactions), nil
}

func (s *EthSlotCache) GetBlockReceipts(
	ctx context.Context,
	numOrHash evmrpc.BlockNumberOrHash,
) ([]evm.ExtendedReceipt, error) {
	var st *evm.Slot
	var err error
	if numOrHash.BlockNumber != nil {
		st, err = getSlotFromCache(ctx, s.SlotCache, *numOrHash.BlockNumber)
	} else {
		var found bool
		st, found, err = chain.GetSlotByHash(ctx, s.SlotCache, numOrHash.BlockHash.String())
		if err == nil && !found {
			err = evm.ErrCacheMissing
		}
	}
	if err != nil {
		if errors.Is(err, evm.ErrCacheMissing) {
			return nil, jsonrpc.CallNextMiddleware
		}
		if errors.Is(err, evm.ErrBlockNumberTooBig) {
			return nil, chain.ErrSlotNotFound
		}
		return nil, err
	}
	return st.Receipts, nil
}

func (s *EthSlotCache) GetLogs(ctx context.Context, args *evm.EthGetLogsArgs) ([]evm.LogWithCustomSerDe, error) {
	slotToLogs := func(slot *evm.Slot) ([]evm.LogWithCustomSerDe, error) {
		var logs []evm.LogWithCustomSerDe
		for _, log := range slot.Logs {
			if logFilter(&log, args) {
				logs = append(logs, evm.LogWithCustomSerDe(log))
			}
		}
		return logs, nil
	}
	if args.BlockHash != nil {
		st, found, err := chain.GetSlotByHash(ctx, s.SlotCache, args.BlockHash.String())
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, evm.ErrCacheMissing
		}
		return slotToLogs(st)
	}

	curRange, err := s.SlotCache.GetRange(ctx)
	if err != nil {
		return nil, err
	}
	fromBlock, toBlock := curRange.R(), curRange.R()
	if args.FromBlock != nil {
		fromBlock = number.Number(*args.FromBlock)
	}
	if args.ToBlock != nil {
		toBlock = number.Number(*args.ToBlock)
	}
	interval := number.NewRange(fromBlock, toBlock)

	slots, uncachedRange, err := getSlotsByRangeFromCache(ctx, s.SlotCache, interval)
	if err != nil {
		if err.Error() == "cache not ready" {
			uncachedRange = interval
		} else {
			return nil, err
		}
	}
	results := make([]evm.LogWithCustomSerDe, 0)

	for _, st := range slots {
		logs, err := slotToLogs(st)
		if err != nil {
			return nil, err
		}
		results = append(results, logs...)
	}

	if !uncachedRange.IsEmpty() {
		from := hexutil.Uint64(uncachedRange.L())
		to := hexutil.Uint64(uncachedRange.R())
		// call next middleware to merge results
		nextResults, err := ResultsFromNext[evm.LogWithCustomSerDe](ctx, "eth_getlogs", &evm.EthGetLogsArgs{
			Addresses: args.Addresses,
			Topics:    args.Topics,
			BlockHash: args.BlockHash,
			FromBlock: &from,
			ToBlock:   &to,
		})

		if err != nil {
			return nil, err
		}
		results = append(results, nextResults...)

	}
	return results, nil
}

func (s *EthSlotCache) GetBlocksPacked(
	ctx context.Context,
	fromBlock hexutil.Uint64,
	toBlock hexutil.Uint64,
	needTransaction bool,
	needReceipt bool,
	needReceiptLogs bool,
	needTraces bool,
) ([]*evm.PackedBlock, error) {
	converter := func(slot *evm.Slot) ([]*evm.PackedBlock, error) {
		block := evm.PackedBlock{BlockHeader: slot.Header}
		if needTransaction {
			block.RelevantTransactions = slot.Block.Transactions
		}
		if needReceipt {
			block.RelevantTransactionReceipts = slot.Receipts
		}
		if needReceiptLogs {
			for _, receipt := range block.RelevantTransactionReceipts {
				for _, rl := range receipt.Logs {
					block.Logs = append(block.Logs, *rl)
				}
			}
		}
		if needTraces {
			block.Traces = slot.Traces
		}
		return []*evm.PackedBlock{&block}, nil
	}

	results := make([]*evm.PackedBlock, 0)
	interval := number.NewRange(number.Number(fromBlock), number.Number(toBlock))
	slots, uncachedRange, err := getSlotsByRangeFromCache(ctx, s.SlotCache, interval)
	if err != nil {
		if err.Error() == "cache not ready" {
			uncachedRange = interval
		} else {
			return nil, err
		}
	}

	for _, st := range slots {
		packedBlocks, err := converter(st)
		if err != nil {
			return nil, err
		}
		results = append(results, packedBlocks...)
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

func (s *EthSlotCache) GetLogsPacked(ctx context.Context, args *evm.EthGetLogsArgs,
	needTransaction, needReceipt, needReceiptLogs bool) ([]*evm.PackedBlock, error) {
	slotToLogsPacked := func(slot *evm.Slot) ([]*evm.PackedBlock, error) {
		var logs []types.Log
		for _, log := range slot.Logs {
			if logFilter(&log, args) {
				logs = append(logs, log)
			}
		}
		return []*evm.PackedBlock{evm.MakePackedBlock(slot, logs, nil, needTransaction, needReceipt, needReceiptLogs)}, nil
	}
	if args.FromBlock == nil || args.ToBlock == nil {
		slot, found, err := chain.GetSlotByHash(ctx, s.SlotCache, args.BlockHash.String())
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil // may be should return evm.ErrCacheMissing
		}
		return slotToLogsPacked(slot)
	}

	fromBlock := number.Number(*args.FromBlock)
	toBlock := number.Number(*args.ToBlock)
	interval := number.NewRange(fromBlock, toBlock)

	slots, uncachedRange, err := getSlotsByRangeFromCache(ctx, s.SlotCache, interval)
	if err != nil {
		if err.Error() == "cache not ready" {
			uncachedRange = interval
		} else {
			return nil, err
		}
	}

	results := make([]*evm.PackedBlock, 0)
	for _, st := range slots {
		packedBlocks, err := slotToLogsPacked(st)
		if err != nil {
			return nil, err
		}
		results = append(results, packedBlocks...)
	}

	if !uncachedRange.IsEmpty() {
		from := hexutil.Uint64(uncachedRange.L())
		to := hexutil.Uint64(uncachedRange.R())
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

func getSlotsByRangeFromCache(ctx context.Context, slotCache chain.LatestSlotCache[*evm.Slot], interval number.Range) ([]*evm.Slot, number.Range, error) {
	slots := make([]*evm.Slot, 0)
	uncachedRange := number.EmptyRange
	_, err := chain.QueryRangeWithCacheV2(ctx, interval, slotCache,
		func(slot *evm.Slot) ([]*evm.Slot, error) {
			slots = append(slots, slot)
			return slots, nil
		},
		func(ctx context.Context, queryRange number.Range) (results []*evm.Slot, err error) {
			if !queryRange.IsEmpty() {
				uncachedRange = queryRange
			}
			return nil, nil
		},
	)
	return slots, uncachedRange, err
}

func getSlotFromCache(ctx context.Context, slotCache chain.LatestSlotCache[*evm.Slot], sn evmrpc.BlockNumber) (*evm.Slot, error) {
	cacheRange, err := slotCache.GetRange(ctx)
	if err != nil {
		return nil, err
	}
	if sn < 0 {
		if sn != evmrpc.LatestBlockNumber {
			return nil, fmt.Errorf("%w: unsupported block number tag %s", jsonrpc.CallNextMiddleware, sn.String())
		}
		sn = evmrpc.BlockNumber(cacheRange.R())
	}
	if number.Number(sn) > cacheRange.R() {
		return nil, evm.ErrBlockNumberTooBig
	}
	if !cacheRange.ContainsNumber(number.Number(sn)) {
		return nil, evm.ErrCacheMissing
	}
	st, err := slotCache.GetByNumber(ctx, number.Number(sn))
	if errors.Is(err, chain.ErrSlotNotFound) {
		return nil, evm.ErrCacheMissing
	}
	if err != nil {
		return nil, err
	}
	return st, nil
}
