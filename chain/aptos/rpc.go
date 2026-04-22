package aptos

import (
	"context"
	"encoding/json"
	"net/http"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/jsonrpc"
)

func NewRPCService(
	slotCache chain.LatestSlotCache[*Slot],
	clientPool *ClientPool,
	store Storage,
) []jsonrpc.Middleware {
	return []jsonrpc.Middleware{
		NewMiddlewareV2(NewRPCServiceV2(slotCache, store)),
		NewMiddleware(NewRPCServiceV1(slotCache, store)),
		func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
				if method != jsonrpc.HTTPRequestMethod {
					return next(ctx, method, params)
				}
				jsonrpc.ProxyHTTP[ClientConfig, *Client](
					ctx,
					method,
					clientPool.ClientPool,
					func(ctx context.Context, cli *Client) (
						resp *http.Response,
						respBody []byte,
						upstream string,
						r clientpool.Result,
					) {
						ctxData := jsonrpc.GetCtxData(ctx)
						upstream = cli.config.GetName()
						r = cli.Use(ctx, "proxy."+method, func(ctx context.Context) (r clientpool.Result) {
							req, err := clientpool.BuildHTTPRequest(
								ctx,
								ctxData.RawReq.Method,
								cli.config.Endpoint,
								ctxData.RawReq.URL.Path,
								ctxData.RawReq.URL.Query(),
								ctxData.RawReqBody,
							)
							if err != nil {
								return clientpool.Result{Err: err, BrokenForTask: true}
							}
							resp, respBody, r = clientpool.SendHTTP(httpClient, req, nil)
							return r
						})
						return
					},
				)
				return nil, nil
			}
		},
	}
}
