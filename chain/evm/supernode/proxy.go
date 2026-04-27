package supernode

import (
	"context"
	"encoding/json"
	"github.com/cenkalti/backoff/v4"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio/chain/node"
	"sentioxyz/sentio/chain/proxyv3"
	"strings"
	"time"
)

func NewProxyMiddleware(
	client node.NodeClient,
	cacheDelay time.Duration,
	proxySvr *proxyv3.JSONRPCServiceV2,
) jsonrpc.Middleware {
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
			// detect block number of this request
			var requestBlockNumber rpc.BlockNumber
			switch strings.ToLower(method) {
			case "eth_call", "eth_getbalance", "eth_getcode":
				bn, err := getBlockNumberFromParams(params, "1")
				if err != nil {
					return nil, err
				}
				requestBlockNumber = *bn[0]
			}
			// check if the block number of this request is new enough
			var disableCache bool
			if requestBlockNumber < 0 || cacheDelay < 0 {
				disableCache = true
			} else {
				latest, err := client.Latest(ctx)
				if err != nil {
					return nil, err
				}
				blockInterval := client.BlockInterval()
				disableCache = blockInterval == 0 || latest.Number < uint64(requestBlockNumber)+uint64(cacheDelay/blockInterval)
			}
			if strings.ToLower(method) == "eth_call" {
				// auto retry for eth_call
				const retries = 5
				const retryInterval = time.Second
				const timeoutInitial = time.Second * 5
				const timeoutMultiplier = 1.5
				_, logger := log.FromContext(ctx)
				first := true
				timeout := timeoutInitial
				var result any
				err := backoff.RetryNotify(
					func() (err error) {
						callCtx, cancel := context.WithTimeout(ctx, timeout)
						defer cancel()
						result, err = proxySvr.ProxyCall(callCtx, method, params, !first || disableCache, nil)
						first = false
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
			return proxySvr.ProxyCall(ctx, method, params, disableCache, nil)
		}
	}
}

func NewForcedProxyMiddleware(proxySvr *proxyv3.JSONRPCServiceV2, methods []string) jsonrpc.Middleware {
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
			if utils.IndexOf(methods, method) >= 0 {
				return proxySvr.ProxyCall(ctx, method, params, true, nil)
			}
			return next(ctx, method, params)
		}
	}
}
