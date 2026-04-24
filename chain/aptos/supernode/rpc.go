package supernode

import (
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/jsonrpc"
)

func NewRPCService(
	slotCache chain.LatestSlotCache[*aptos.Slot],
	clientPool *aptos.ClientPool,
	store Storage,
) []jsonrpc.Middleware {
	return []jsonrpc.Middleware{
		NewMiddlewareV2(NewRPCServiceV2(slotCache, store)),
		NewMiddleware(NewRPCServiceV1(slotCache, store)),
		jsonrpc.NewProxyMiddleware("", clientPool.ClientPool),
	}
}
