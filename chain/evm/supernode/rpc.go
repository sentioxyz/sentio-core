package supernode

import (
	"fmt"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	"strconv"
)

var (
	ErrCacheMissing      = errors.New("cache missing")
	ErrBlockNumberTooBig = errors.New("block number too big")
)

func NewSimpleProxyService(
	client *evm.ClientPool,
	slotCache chain.LatestSlotCache[*evm.Slot],
	forcedProxyMethods []string,
) []jsonrpc.Middleware {
	return []jsonrpc.Middleware{
		NewForcedProxyMiddleware(client, forcedProxyMethods),
		NewCustomFunctionProxyMiddleware(client),
		NewProxyExtraMiddleware(client),
		NewSubscribeMiddleware(slotCache),
		NewProxyWithLatestSlotCacheMiddleware(slotCache, client),
		NewProxyMiddleware(client),
	}
}

func NewRPCServiceV2(
	network string,
	chainID string,
	networkOptions *evm.NetworkOptions,
	slotCache chain.LatestSlotCache[*evm.Slot],
	store evm.Storage,
	rangeStore chain.RangeStore,
	client *evm.ClientPool,
	forcedProxyMethods []string,
) []jsonrpc.Middleware {

	chainIDNum, err := strconv.ParseUint(chainID, 0, 64)
	if err != nil {
		panic(fmt.Errorf("chainID %q is not a number", chainID))
	}

	return []jsonrpc.Middleware{
		NewForcedProxyMiddleware(client, forcedProxyMethods),
		NewCustomFunctionProxyMiddleware(client),
		NewExtraMiddleware(slotCache, rangeStore, store),
		NewSubscribeMiddleware(slotCache),
		NewEthSlotCacheMiddleware(chainIDNum, slotCache, rangeStore, client),
		//NewTraceSlotCacheMiddleware(slotCache, networkOptions),
		//NewRangeCheckMiddleware(rangeStore),
		//NewEthClickHouseMiddleware(base),
		//NewTraceClickHouseMiddleware(base),
		NewProxyMiddleware(client),
	}
}
