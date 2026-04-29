package supernode

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	evmrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	rg "sentioxyz/sentio-core/common/range"
)

func getSlotsFromCache(
	ctx context.Context,
	slotCache chain.LatestSlotCache[*evm.Slot],
	blockHash *common.Hash,
	fromBlock *hexutil.Uint64,
	toBlock *hexutil.Uint64,
) ([]*evm.Slot, rg.Range, error) {

	if blockHash != nil {
		st, err := slotCache.GetByHash(ctx, blockHash.String())
		if err != nil {
			if errors.Is(err, chain.ErrSlotNotFound) {
				return nil, rg.EmptyRange, ErrCacheMissing
			}
			return nil, rg.EmptyRange, err
		}
		return []*evm.Slot{st}, rg.EmptyRange, nil
	}

	curRange, err := slotCache.GetRange(ctx)
	if err != nil {
		return nil, rg.EmptyRange, err
	}
	fb, tb := *curRange.End, *curRange.End
	if fromBlock != nil {
		fb = (uint64)(*fromBlock)
	}
	if toBlock != nil {
		tb = (uint64)(*toBlock)
	}
	interval := rg.NewRange(fb, tb)
	slots := make([]*evm.Slot, 0)
	uncachedRange := rg.EmptyRange
	_, err = chain.QueryRangeWithCache(ctx, interval, slotCache,
		func(slot *evm.Slot) ([]*evm.Slot, error) {
			slots = append(slots, slot)
			return slots, nil
		},
		func(ctx context.Context, queryRange rg.Range) (results []*evm.Slot, err error) {
			if !queryRange.IsEmpty() {
				uncachedRange = queryRange
			}
			return nil, nil
		},
	)
	return slots, uncachedRange, err
}

func getSlotFromCache(
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
