package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"math"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

func NewExtraMiddleware(
	slotCache chain.LatestSlotCache[*evm.Slot],
	rangeStore chain.RangeStore,
	store Storage,
) jsonrpc.Middleware {
	s := ExtraService{
		slotCache:  slotCache,
		rangeStore: rangeStore,
		store:      store,
	}
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			switch method {
			case "sentio_estimateBlockNumberAtDate":
				return jsonrpc.CallMethod(s.EstimateBlockNumberAtDate, ctx, params)
			default:
				return next(ctx, method, params)
			}
		}
	}
}

type ExtraService struct {
	slotCache  chain.LatestSlotCache[*evm.Slot]
	rangeStore chain.RangeStore
	store      Storage
}

// EstimateBlockNumberAtDate Find the smallest block with timestamp >= targetTimestampMs (GE mode) or
// the biggest block with timestamp <= targetTimestampMs (LE mode) in the interval [startBlock,endBlock].
// If there is no block match the condition, null will be returned.
func (s *ExtraService) EstimateBlockNumberAtDate(
	ctx context.Context,
	targetTimestampMs hexutil.Uint64,
	startBlock rpc.BlockNumber,
	endBlock rpc.BlockNumber,
	mode string,
) (*hexutil.Uint64, error) {
	sn, en := uint64(startBlock), uint64(endBlock)
	if endBlock < 0 {
		if endBlock != rpc.LatestBlockNumber {
			return nil, fmt.Errorf("end block number cannot be %s", endBlock.String())
		}
		en = math.MaxUint64
	}
	if startBlock < 0 {
		if startBlock != rpc.EarliestBlockNumber {
			return nil, fmt.Errorf("start block number cannot be %s", startBlock.String())
		}
		sn = 0
	}
	var result *uint64
	var updateResult func(bn uint64, timestampMs uint64)
	switch mode {
	case "LE":
		updateResult = func(bn uint64, timestampMs uint64) {
			if timestampMs <= uint64(targetTimestampMs) {
				if result == nil {
					result = &bn
				} else {
					*result = max(*result, bn)
				}
			}
		}
	case "GE":
		updateResult = func(bn uint64, timestampMs uint64) {
			if timestampMs >= uint64(targetTimestampMs) {
				if result == nil {
					result = &bn
				} else {
					*result = min(*result, bn)
				}
			}
		}
	default:
		return nil, fmt.Errorf("invalid mode %q, should be \"LE\" or \"GE\"", mode)
	}
	_, err := chain.QueryRangeWithCache(
		ctx,
		rg.NewRange(sn, en),
		s.slotCache,
		func(slot *evm.Slot) ([]uint64, error) {
			updateResult(slot.GetNumber(), uint64(slot.Header.GetBlockTime().UnixMilli()))
			return nil, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]uint64, error) {
			// may be do not need to query in clickhouse
			if mode == "LE" && result != nil {
				// has result in latest slot cache, do not need to query in clickhouse
				return nil, nil
			}
			if mode == "GE" {
				if result == nil {
					// that means the timestamp of the latest block is less the target,
					// do not need to query in clickhouse
					return nil, nil
				} else if *queryRange.End+1 < *result {
					// that means the timestamp of block queryRange.R() + 1 is less than the target timestamp,
					// do not need to query in clickhouse
					return nil, nil
				}
				// result always equal to queryRange.R()+1
			}
			// get data range in clickhouse
			curRange, err := s.rangeStore.Get(ctx)
			if err != nil {
				return nil, err
			}
			// if startBlock is earliest, need to adjust the left point of the query range
			if queryRange.Start == 0 && startBlock == rpc.EarliestBlockNumber {
				queryRange = rg.NewRange(curRange.Start, *queryRange.End)
			}
			// if query range is empty, do not need to execute sql in clickhouse
			if queryRange.IsEmpty() {
				return nil, nil
			}
			// query range is not is the current range of clickhouse data, return error
			if !curRange.Include(queryRange) {
				return nil, fmt.Errorf("request range %s not in scope of range store %s", queryRange, curRange)
			}
			// execute the query
			targetTime := time.UnixMilli(int64(targetTimestampMs)).UTC()
			result, err = s.store.QueryEstimateBlockNumberAtDate(
				ctx,
				targetTime,
				queryRange.Start,
				*queryRange.End,
				mode == "LE",
			)
			return nil, err
		},
	)
	return (*hexutil.Uint64)(result), err
}

func NewProxyExtraMiddleware(client *evm.ClientPool) jsonrpc.Middleware {
	s := ProxyExtraService{client: client}
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			if method == "sentio_estimateBlockNumberAtDate" {
				return jsonrpc.CallMethod(s.EstimateBlockNumberAtDate, ctx, params)
			}
			return next(ctx, method, params)
		}
	}
}

type ProxyExtraService struct {
	client *evm.ClientPool
}

func (m *ProxyExtraService) getBlockHeader(ctx context.Context, blockNumber rpc.BlockNumber) (*types.Header, error) {
	raw, err := jsonrpc.ProxyJSONRPCRequest(
		ctx,
		"proxy",
		"eth_getBlockByNumber",
		[]any{blockNumber, false},
		m.client.ClientPool,
	)
	if err != nil {
		return nil, err
	}
	var h *types.Header
	err = json.Unmarshal(raw, &h)
	if err == nil && h == nil {
		return nil, fmt.Errorf("header of block %s not found", blockNumber.String())
	}
	return h, err
}

func (m *ProxyExtraService) EstimateBlockNumberAtDate(
	ctx context.Context,
	targetTimestampMs hexutil.Uint64,
	startBlock rpc.BlockNumber,
	endBlock rpc.BlockNumber,
	mode string,
) (*rpc.BlockNumber, error) {
	_, logger := log.FromContext(ctx)
	// check arguments
	if endBlock < 0 && endBlock != rpc.LatestBlockNumber {
		return nil, fmt.Errorf("end block number cannot be %s", endBlock.String())
	}
	if startBlock < 0 && startBlock != rpc.EarliestBlockNumber {
		return nil, fmt.Errorf("start block number cannot be %s", startBlock.String())
	}
	if mode != "LE" && mode != "GE" {
		return nil, fmt.Errorf("invalid mode %q, should be \"LE\" or \"GE\"", mode)
	}
	// compare function between block timestamp and targetTimestampMs
	cmpTime := func(h *types.Header) int {
		ts := hexutil.Uint64(evm.ExtendedHeader{Header: *h}.GetBlockTime().UnixMilli())
		if ts == targetTimestampMs {
			return 0
		} else if ts < targetTimestampMs {
			return -1
		} else {
			return 1
		}
	}
	// get actually block of start/end block
	start, err := m.getBlockHeader(ctx, startBlock)
	if err != nil {
		return nil, err
	}
	if mode == "LE" && cmpTime(start) > 0 {
		// target < start, so no result
		return nil, nil
	}
	if mode == "GE" && cmpTime(start) >= 0 {
		// target <= start, so result is start
		return utils.WrapPointer(rpc.BlockNumber(start.Number.Int64())), nil
	}
	end, err := m.getBlockHeader(ctx, endBlock)
	if err != nil {
		return nil, err
	}
	if mode == "GE" && cmpTime(end) < 0 {
		// end < target, so no result
		return nil, nil
	}
	if mode == "LE" && cmpTime(end) <= 0 {
		// end <= target, so result is end
		return utils.WrapPointer(rpc.BlockNumber(end.Number.Int64())), nil
	}
	// now result must be in [start,end], because for
	// LE mode: start <= target and target < end
	// GE mode: start < target and target <= end
	if start.Number.Cmp(end.Number) == 0 {
		return utils.WrapPointer(rpc.BlockNumber(start.Number.Int64())), nil
	}
	// use bin search to find the result.
	// use 0 and utils.MinP2(end.Number.Uint64()) as low and high, will make the block number need to
	// call getBlockHeader stable, hits the cache as much as possible.
	r, _, err := utils.BinarySearch(0, utils.MinP2(end.Number.Uint64()), func(x uint64) (bool, error) {
		if x <= start.Number.Uint64() {
			return false, nil
		}
		if x >= end.Number.Uint64() {
			return true, nil
		}
		h, getErr := m.getBlockHeader(ctx, rpc.BlockNumber(x))
		if getErr != nil {
			return false, getErr
		}
		logger.Debugf("bin search in [%s,%s] with mode %s and target %d got block %d with ts %d",
			startBlock.String(), endBlock.String(), mode, targetTimestampMs, x, h.Time)
		if mode == "LE" {
			return cmpTime(h) > 0, nil
		} else {
			return cmpTime(h) >= 0, nil
		}
	})
	if err != nil {
		return nil, err
	}
	var result rpc.BlockNumber
	if mode == "LE" {
		result = rpc.BlockNumber(r - 1)
	} else {
		result = rpc.BlockNumber(r)
	}
	logger.Debugf("bin search in [%s,%s] with mode %s and target %d got result %d",
		startBlock.String(), endBlock.String(), mode, targetTimestampMs, result)
	return &result, nil
}
