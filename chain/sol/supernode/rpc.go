package supernode

import (
	"sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/common/jsonrpc"
)

func NewSimpleProxyService(client *sol.ClientPool) []jsonrpc.Middleware {
	return []jsonrpc.Middleware{jsonrpc.NewJSONRPCProxyMiddleware(client.ClientPool)}
}
