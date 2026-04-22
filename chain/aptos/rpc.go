package aptos

import (
	"context"
	"encoding/json"
	"fmt"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/jsonrpc"
)

func NewRPCService(
	slotCache chain.LatestSlotCache[*Slot],
	store Storage,
) []jsonrpc.Middleware {
	return []jsonrpc.Middleware{
		NewMiddlewareV2(NewRPCServiceV2(slotCache, store)),
		NewMiddleware(NewRPCServiceV1(slotCache, store)),
		func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
				switch method {
				case jsonrpc.HTTPRequestMethod:
					// TODO
					return nil, nil
				default:
					return nil, fmt.Errorf("unknown method: %s", method)
				}
			}
		},
	}
}
