package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"sentioxyz/sentio-core/common/jsonrpc"
)

func NewVersionMiddleware(chainID uint64) jsonrpc.Middleware {
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
			switch method {
			case "eth_version":
				return fmt.Sprintf("%d", chainID), nil
			case "eth_chainId":
				return fmt.Sprintf("0x%x", chainID), nil
			}
			return next(ctx, method, params)
		}
	}
}
