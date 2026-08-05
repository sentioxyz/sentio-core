package supernode

import (
	"context"
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/jsonrpc"
)

func NewRPCService(
	slotCache chain.LatestSlotCache[*aptos.Slot],
	clientPool *aptos.ClientPool,
	store Storage,
) []jsonrpc.Middleware {
	prober := func(ctx context.Context, address string, txVersion uint64) (bool, error) {
		has, r := clientPool.HasAccountResources(ctx, "supernode", address, txVersion)
		return has, r.Err
	}
	return []jsonrpc.Middleware{
		NewMiddlewareV2(NewRPCServiceV2(slotCache, store, prober)),
		NewMiddleware(NewRPCServiceV1(slotCache, store)),
		jsonrpc.NewHTTPProxyMiddleware("", clientPool.ClientPool),
	}
}
