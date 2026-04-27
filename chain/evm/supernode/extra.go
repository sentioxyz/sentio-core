package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio/chain/chain"
	"sentioxyz/sentio/chain/evm"
	"sentioxyz/sentio/chain/node"
	"sentioxyz/sentio/chain/proxyv3"
	"sentioxyz/sentio/common/number"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

func NewExtraMiddleware(
	slotCache chain.LatestSlotCache[*evm.Slot],
	rangeStore chain.RangeStore,
	base baseClickhouseService,
) jsonrpc.Middleware {
	s := ExtraService{
		slotCache:  slotCache,
		rangeStore: rangeStore,
		base:       base,
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
	base       baseClickhouseService
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
) (*rpc.BlockNumber, error) {
	if endBlock < 0 {
		if endBlock != rpc.LatestBlockNumber {
			return nil, fmt.Errorf("end block number cannot be %s", endBlock.String())
		}
		endBlock = math.MaxInt64
	}
	if startBlock < 0 {
		if startBlock != rpc.EarliestBlockNumber {
			return nil, fmt.Errorf("start block number cannot be %s", startBlock.String())
		}
		// will use the left point of the range in rangeStore as the start block
	}
	var result *rpc.BlockNumber
	var updateResult func(bn rpc.BlockNumber, timestampMs uint64)
	switch mode {
	case "LE":
		updateResult = func(bn rpc.BlockNumber, timestampMs uint64) {
			if timestampMs <= uint64(targetTimestampMs) {
				if result == nil {
					result = &bn
				} else {
					*result = max(*result, bn)
				}
			}
		}
	case "GE":
		updateResult = func(bn rpc.BlockNumber, timestampMs uint64) {
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
	_, err := chain.QueryRangeWithCacheV2(
		ctx,
		number.NewRange(utils.Select(startBlock < 0, 0, number.Number(startBlock)), number.Number(endBlock)),
		s.slotCache,
		func(slot *evm.Slot) ([]uint64, error) {
			slotBlockNumber := rpc.BlockNumber(slot.GetNumber())
			slotTimestampMs := utils.Select(slot.Header.Time >= math.MaxInt32, slot.Header.Time, slot.Header.Time*1000)
			updateResult(slotBlockNumber, slotTimestampMs)
			return nil, nil
		},
		func(ctx context.Context, queryRange number.Range) ([]uint64, error) {
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
				} else if queryRange.R()+1 < number.Number(*result) {
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
			if queryRange.L() == 0 && startBlock == rpc.EarliestBlockNumber {
				queryRange = number.NewRange(curRange.L(), queryRange.R())
			}
			// if query range is empty, do not need to execute sql in clickhouse
			if queryRange.IsEmpty() {
				return nil, nil
			}
			// query range is not is the current range of clickhouse data, return error
			if !curRange.Contains(queryRange) {
				return nil, fmt.Errorf("request range %s not in scope of range store %s", queryRange, curRange)
			}
			// execute the query
			targetTime := time.UnixMilli(int64(targetTimestampMs)).UTC()
			result, err = s.base.store.QueryEstimateBlockNumberAtDate(
				ctx,
				targetTime,
				uint64(queryRange.L()),
				uint64(queryRange.R()),
				mode == "LE",
			)
			return nil, err
		},
	)
	return result, err
}

func NewProxyExtraMiddleware(
	client node.NodeClient,
	cacheDelay time.Duration,
	proxySvr *proxyv3.JSONRPCServiceV2,
) jsonrpc.Middleware {
	s := ProxyExtraService{
		client:     client,
		cacheDelay: cacheDelay,
		proxySvr:   proxySvr,
	}
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
	client     node.NodeClient
	cacheDelay time.Duration
	proxySvr   *proxyv3.JSONRPCServiceV2
}

func (m *ProxyExtraService) getBlockHeader(ctx context.Context, blockNumber rpc.BlockNumber) (*types.Header, error) {
	var disableCache bool
	if blockNumber < 0 || m.cacheDelay < 0 {
		disableCache = true
	} else {
		latest, err := m.client.Latest(ctx)
		if err != nil {
			return nil, err
		}
		blockInterval := m.client.BlockInterval()
		disableCache = blockInterval == 0 || latest.Number < uint64(blockNumber)+uint64(m.cacheDelay/blockInterval)
	}

	raw, err := m.proxySvr.ProxyCall(ctx, "eth_getBlockByNumber", []any{blockNumber, false}, disableCache, nil)
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
		ts := hexutil.Uint64(utils.Select(h.Time >= math.MaxUint32, h.Time, h.Time*1000))
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
