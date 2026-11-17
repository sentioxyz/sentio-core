package supernode

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	lru "github.com/sentioxyz/golang-lru"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
)

// BlockNumberByHashStore resolves the number of a stored block from its hash.
type BlockNumberByHashStore interface {
	// QueryBlockNumberByHash returns nil if no stored block has the hash.
	QueryBlockNumberByHash(ctx context.Context, hash common.Hash) (*uint64, error)
}

// nativeBlockHashCacheSize bounds the block hashes remembered as resolved from the store. A caller
// usually sends many calls for the same block, so remembering them saves most store lookups.
const nativeBlockHashCacheSize = 4096

const (
	serverErrorCode        = -32000
	invalidParamsErrorCode = -32602
)

type nativeStateService struct {
	nativeFrom uint64
	slotCache  chain.LatestSlotCache[*evm.Slot]
	store      BlockNumberByHashStore
	// resolved holds hash => number of blocks found in the store. Only hits are kept: an unknown
	// hash may belong to a block that has not been stored yet.
	resolved *lru.Cache[common.Hash, uint64]
}

// NewNativeStateOnlyMiddleware rejects the state queries (eth_call, eth_getBalance, ...) of a request
// carrying the jsonrpc.NativeStateOnlyLabel source label when they read the state at a block outside
// the native range [nativeFrom, +inf) of the chain (see evm.NetworkOptions.NativeFrom). It must stand
// in front of every middleware that proxies requests to the chain nodes.
//
// A block given by its hash is resolved with the latest slot cache and then the store, both of which
// may be nil. A hash neither of them knows is rejected as well: the chain node cannot tell it from an
// old block without looking it up where the old blocks are kept, which is exactly the load the label
// is meant to keep away. With nativeFrom == 0 the whole chain is native and nothing is rejected.
func NewNativeStateOnlyMiddleware(
	nativeFrom uint64,
	slotCache chain.LatestSlotCache[*evm.Slot],
	store BlockNumberByHashStore,
) jsonrpc.Middleware {
	if nativeFrom == 0 {
		return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
			return next
		}
	}
	resolved, _ := lru.New[common.Hash, uint64](nativeBlockHashCacheSize) // only fails for size <= 0
	s := nativeStateService{nativeFrom: nativeFrom, slotCache: slotCache, store: store, resolved: resolved}
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			if err := s.check(ctx, method, params); err != nil {
				return nil, err
			}
			return next(ctx, method, params)
		}
	}
}

func (s *nativeStateService) check(ctx context.Context, method string, params json.RawMessage) error {
	ctxData := jsonrpc.GetCtxData(ctx)
	if ctxData == nil || !ctxData.ReqSrc.BoolLabel(jsonrpc.NativeStateOnlyLabel) {
		return nil
	}
	args, err := jsonrpc.ParseParams(params)
	if err != nil {
		return nil // not ours to judge, the handler of the method reports it
	}
	bp, isStateMethod, err := evm.StateMethodBlockParameter(method, args)
	if !isStateMethod {
		return nil
	}
	if err != nil {
		return jsonrpc.NewJSONError(invalidParamsErrorCode, err.Error(), nil)
	}
	if bp.BlockHash != nil {
		return s.checkBlockHash(ctx, method, *bp.BlockHash)
	}
	switch bn := *bp.BlockNumber; {
	case bn == rpc.EarliestBlockNumber:
		return s.reject(method, "the earliest block")
	case bn >= 0 && uint64(bn) < s.nativeFrom:
		return s.reject(method, fmt.Sprintf("block %d", uint64(bn)))
	}
	return nil // a native block number, or a tag that follows the head of the chain
}

func (s *nativeStateService) checkBlockHash(ctx context.Context, method string, hash common.Hash) error {
	number, found, err := s.resolveBlockHash(ctx, hash)
	if err != nil {
		_, logger := log.FromContext(ctx)
		logger.Warnfe(err, "resolve block hash %s for the method %s failed", hash, method)
		return jsonrpc.NewJSONError(
			serverErrorCode,
			fmt.Sprintf("the method %s is not available at block %s: the block cannot be resolved for now", method, hash),
			nil,
		)
	}
	if !found {
		return jsonrpc.NewJSONError(
			serverErrorCode,
			fmt.Sprintf("the method %s is not available at block %s: unknown block", method, hash),
			nil,
		)
	}
	if number < s.nativeFrom {
		return s.reject(method, fmt.Sprintf("block %d", number))
	}
	return nil
}

func (s *nativeStateService) resolveBlockHash(ctx context.Context, hash common.Hash) (uint64, bool, error) {
	if s.slotCache != nil {
		st, err := s.slotCache.GetByHash(ctx, hash.String())
		if err == nil {
			return st.GetNumber(), true, nil
		}
		if !errors.Is(err, chain.ErrSlotNotFound) {
			return 0, false, err
		}
	}
	if number, has := s.resolved.Get(hash); has {
		return number, true, nil
	}
	if s.store == nil {
		return 0, false, nil
	}
	number, err := s.store.QueryBlockNumberByHash(ctx, hash)
	if err != nil || number == nil {
		return 0, false, err
	}
	s.resolved.Add(hash, *number)
	return *number, true, nil
}

func (s *nativeStateService) reject(method, block string) error {
	return jsonrpc.NewJSONError(
		serverErrorCode,
		fmt.Sprintf("the method %s is not available at %s: only blocks from %d on are served for this request",
			method, block, s.nativeFrom),
		nil,
	)
}
