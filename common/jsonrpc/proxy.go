package jsonrpc

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"net/http"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/utils"
)

const UpstreamHeaderKey = "X-Sentio-Proxy-Endpoint"

func ProxyHTTP[CONFIG clientpool.EntryConfig[CONFIG], CLIENT clientpool.Client](
	ctx context.Context,
	svr string,
	method string,
	clientPool *clientpool.ClientPool[CONFIG, CLIENT],
	fn func(ctx context.Context, src string, cli CLIENT) (resp *http.Response, respBody []byte, r clientpool.Result),
) {
	ctxData := GetCtxData(ctx)
	var resp *http.Response
	var respBody []byte
	var clientName string
	src := utils.Select(svr == "", "proxy", svr+".proxy")
	err := clientPool.UseClient(ctx, src+"."+method, func(ctx context.Context, cli CLIENT) (r clientpool.Result) {
		clientName = cli.GetName()
		resp, respBody, r = fn(ctx, src, cli)
		return r
	})
	if errors.Is(err, clientpool.ErrNoValidClient) {
		http.Error(ctxData.RespWriter, err.Error(), http.StatusInternalServerError)
		return
	}
	for k, vs := range resp.Header {
		for _, v := range vs {
			ctxData.RespWriter.Header().Add(k, v)
		}
	}
	ctxData.RespWriter.Header().Add(UpstreamHeaderKey, clientName)
	ctxData.RespWriter.WriteHeader(resp.StatusCode)
	_, _ = ctxData.RespWriter.Write(respBody)
}

type httpClient interface {
	UseHTTPClient(
		ctx context.Context,
		method string,
		fn func(ctx context.Context, endpoint string, cli *http.Client) clientpool.Result,
	) clientpool.Result

	clientpool.Client
}

func NewProxyMiddleware[CONFIG clientpool.EntryConfig[CONFIG], CLIENT httpClient](
	svr string,
	clientPool *clientpool.ClientPool[CONFIG, CLIENT],
) Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			if method != HTTPRequestMethod {
				return next(ctx, method, params)
			}
			ProxyHTTP[CONFIG, CLIENT](
				ctx,
				svr,
				method,
				clientPool,
				func(ctx context.Context, src string, cli CLIENT) (
					resp *http.Response,
					respBody []byte,
					r clientpool.Result,
				) {
					ctxData := GetCtxData(ctx)
					r = cli.UseHTTPClient(
						ctx,
						src+"."+method,
						func(ctx context.Context, endpoint string, cli *http.Client) (r clientpool.Result) {
							req, err := clientpool.BuildHTTPRequest(
								ctx,
								ctxData.RawReq.Method,
								endpoint,
								ctxData.RawReq.URL.Path,
								ctxData.RawReq.URL.Query(),
								ctxData.RawReq.Header,
								ctxData.RawReqBody,
							)
							if err != nil {
								return clientpool.Result{Err: err, BrokenForTask: true}
							}
							resp, respBody, r = clientpool.SendHTTP(cli, req, nil)
							return r
						},
					)
					return
				},
			)
			return nil, nil
		}
	}
}

type jsonRPCClient interface {
	CallContext(ctx context.Context, result any, src, method string, args ...any) clientpool.Result

	clientpool.Client
}

func ProxyJSONRPCRequest[CONFIG clientpool.EntryConfig[CONFIG], CLIENT jsonRPCClient](
	ctx context.Context,
	svr string,
	method string,
	args []any,
	clientPool *clientpool.ClientPool[CONFIG, CLIENT],
) (json.RawMessage, error) {
	ctxData := GetCtxData(ctx)
	var data json.RawMessage
	var clientName string
	src := utils.Select(svr == "", "proxy", svr+".proxy")
	err := clientPool.UseClient(ctx, src+"."+method, func(ctx context.Context, cli CLIENT) (r clientpool.Result) {
		clientName = cli.GetName()
		return cli.CallContext(ctx, &data, src, method, args...)
	})
	if errors.Is(err, clientpool.ErrNoValidClient) {
		return nil, errors.Errorf("the method %s does not exist/is not available", method)
	}
	ctxData.RespHeaders = http.Header{}
	ctxData.RespHeaders.Set(UpstreamHeaderKey, clientName)
	return data, err
}

func NewJSONRPCProxyMiddleware[CONFIG clientpool.EntryConfig[CONFIG], CLIENT jsonRPCClient](
	svr string,
	clientPool *clientpool.ClientPool[CONFIG, CLIENT],
) Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			args, err := ParseParams(params)
			if err != nil {
				return nil, err
			}
			return ProxyJSONRPCRequest(ctx, svr, method, args, clientPool)
		}
	}
}
