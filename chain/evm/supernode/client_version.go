package supernode

import (
	"context"
	"encoding/json"

	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/version"
)

// SuperNodeClientName is the client name the super node reports through
// node-identity methods such as web3_clientVersion.
const SuperNodeClientName = "sentio-super-node"

// NewClientVersionMiddleware answers node-identity methods locally. The super node is
// the server the caller is connected to, so forwarding these to an upstream endpoint
// would report a random upstream's identity — and a different one per request as the
// client pool rotates. Placed after the forced-proxy middleware so an explicit
// forced-proxy configuration can still override it.
func NewClientVersionMiddleware() jsonrpc.Middleware {
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			if method == "web3_clientVersion" {
				return version.ClientVersion(SuperNodeClientName), nil
			}
			return next(ctx, method, params)
		}
	}
}
