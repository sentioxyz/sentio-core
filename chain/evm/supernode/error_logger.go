package supernode

import (
	"context"
	"encoding/json"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
)

func NewErrLogMiddleware() jsonrpc.Middleware {
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {

			result, err := next(ctx, method, params)
			if err != nil {
				log.Errorfe(err, "Error in calling method %s, params %v", method, string(params))
			}
			return result, err
		}
	}
}
