package supernode

import (
	"context"
	"encoding/json"
	"net/http"
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
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
		func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
				if method != jsonrpc.HTTPRequestMethod {
					return next(ctx, method, params)
				}
				jsonrpc.ProxyHTTP[aptos.ClientConfig, *aptos.Client](
					ctx,
					method,
					clientPool.ClientPool,
					func(ctx context.Context, cli *aptos.Client) (
						resp *http.Response,
						respBody []byte,
						upstream string,
						r clientpool.Result,
					) {
						ctxData := jsonrpc.GetCtxData(ctx)
						cfg := cli.GetConfig()
						upstream = cfg.GetName()
						r = cli.Use(ctx, "proxy."+method, func(ctx context.Context) (r clientpool.Result) {
							req, err := clientpool.BuildHTTPRequest(
								ctx,
								ctxData.RawReq.Method,
								cfg.Endpoint,
								ctxData.RawReq.URL.Path,
								ctxData.RawReq.URL.Query(),
								ctxData.RawReqBody,
							)
							if err != nil {
								return clientpool.Result{Err: err, BrokenForTask: true}
							}
							resp, respBody, r = clientpool.SendHTTP(cli.GetHTTPClient(), req, nil)
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
