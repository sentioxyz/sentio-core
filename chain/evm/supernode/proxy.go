package supernode

import (
	"context"
	"encoding/json"
	"github.com/cenkalti/backoff/v4"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"strings"
	"time"
)

func NewProxyMiddleware(client *evm.ClientPool) jsonrpc.Middleware {
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
			args, err := jsonrpc.ParseParams(params)
			if err != nil {
				return nil, err
			}
			if strings.ToLower(method) == "eth_call" {
				// auto retry for eth_call
				const retries = 5
				const retryInterval = time.Second
				const timeoutInitial = time.Second * 5
				const timeoutMultiplier = 1.5
				_, logger := log.FromContext(ctx)
				timeout := timeoutInitial
				var result any
				err = backoff.RetryNotify(
					func() (err error) {
						callCtx, cancel := context.WithTimeout(ctx, timeout)
						defer cancel()
						result, err = jsonrpc.ProxyJSONRPCRequest(callCtx, "", method, args, client.ClientPool)
						timeout = time.Duration(float64(timeout) * timeoutMultiplier)
						var rpcErr rpc.Error
						if errors.As(err, &rpcErr) {
							return backoff.Permanent(err)
						}
						return err // http error || timeout || canceled
					},
					backoff.WithContext(backoff.WithMaxRetries(backoff.NewConstantBackOff(retryInterval), retries), ctx),
					func(err error, duration time.Duration) {
						logger.Warnfe(err, "eth_call failed, will retry after %s", duration.String())
					})
				return result, err
			}
			// proxy the request
			return jsonrpc.ProxyJSONRPCRequest(ctx, "", method, args, client.ClientPool)
		}
	}
}

func NewForcedProxyMiddleware(client *evm.ClientPool, methods []string) jsonrpc.Middleware {
	methodSet := set.New[string](methods...)
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
			if methodSet.Contains(method) {
				args, err := jsonrpc.ParseParams(params)
				if err != nil {
					return nil, err
				}
				return jsonrpc.ProxyJSONRPCRequest(ctx, "", method, args, client.ClientPool)
			}
			return next(ctx, method, params)
		}
	}
}
