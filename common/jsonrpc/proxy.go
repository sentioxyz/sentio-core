package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/utils"

	"github.com/pkg/errors"
)

const UpstreamHeaderKey = "X-Sentio-Proxy-Endpoint"

func ProxyHTTP[CONFIG clientpool.EntryConfig[CONFIG], CLIENT clientpool.Client](
	ctx context.Context,
	svr string,
	urlPath string,
	clientPool *clientpool.ClientPool[CONFIG, CLIENT],
	fn func(ctx context.Context, src string, cli CLIENT) (resp *http.Response, respBody []byte, r clientpool.Result),
) error {
	ctxData := GetCtxData(ctx)
	var resp *http.Response
	var respBody []byte
	src := utils.Select(svr == "", "proxy", svr+".proxy")
	r := clientPool.UseClient(ctx, src+"#"+urlPath, func(ctx context.Context, cli CLIENT) (r clientpool.Result) {
		resp, respBody, r = fn(ctx, src, cli)
		return r
	})
	if errors.Is(r.Err, clientpool.ErrNoValidClient) {
		http.Error(ctxData.RespWriter, r.Err.Error(), http.StatusInternalServerError)
		return r.Err
	}
	if resp == nil {
		if r.Err == nil {
			// unreachable
			r.Err = errors.Errorf("no http response from upstream without error")
		}
		http.Error(ctxData.RespWriter, r.Err.Error(), http.StatusInternalServerError)
		return r.Err
	}
	for k, vs := range resp.Header {
		for _, v := range vs {
			ctxData.RespWriter.Header().Add(k, v)
		}
	}
	ctxData.RespWriter.Header().Add(UpstreamHeaderKey, r.ClientName)
	ctxData.RespWriter.WriteHeader(resp.StatusCode)
	_, _ = ctxData.RespWriter.Write(respBody)
	return nil
}

type httpClient interface {
	UseHTTPClient(
		ctx context.Context,
		svr string,
		src string,
		url *url.URL,
		fn func(ctx context.Context, endpoint string, cli *http.Client) clientpool.Result,
	) clientpool.Result

	clientpool.Client
}

func NewHTTPProxyMiddleware[CONFIG clientpool.EntryConfig[CONFIG], CLIENT httpClient](
	svr string,
	clientPool *clientpool.ClientPool[CONFIG, CLIENT],
) Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			if method != HTTPRequestMethod {
				return next(ctx, method, params)
			}
			ctxData := GetCtxData(ctx)
			return nil, ProxyHTTP[CONFIG, CLIENT](
				ctx,
				svr,
				ctxData.RawReq.URL.String(),
				clientPool,
				func(ctx context.Context, src string, cli CLIENT) (
					resp *http.Response,
					respBody []byte,
					r clientpool.Result,
				) {
					r = cli.UseHTTPClient(
						ctx,
						svr,
						src,
						ctxData.RawReq.URL,
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
		}
	}
}

type jsonRPCClient interface {
	CallContext(ctx context.Context, result any, src, method string, args ...any) clientpool.Result

	clientpool.Client
}

func ProxyJSONRPCRequest[CONFIG clientpool.EntryConfig[CONFIG], CLIENT jsonRPCClient](
	ctx context.Context,
	method string,
	args []any,
	clientPool *clientpool.ClientPool[CONFIG, CLIENT],
) (json.RawMessage, error) {
	ctxData := GetCtxData(ctx)
	var data json.RawMessage
	const src = "proxy"
	r := clientPool.UseClient(
		ctx,
		src+"."+method,
		func(ctx context.Context, cli CLIENT) (r clientpool.Result) {
			return cli.CallContext(ctx, &data, src, method, args...)
		},
		clientpool.WithoutTags[CONFIG](clientpool.MethodNotSupportedTag(method)),
		// A method-authority endpoint rejecting the method means no other endpoint should be
		// probed for it: give the caller a method-not-found response right away.
		clientpool.InterruptWithTags[CONFIG](clientpool.MethodNotSupportedByAuthorityTag(method)),
	)
	if errors.Is(r.Err, clientpool.ErrInterrupted) {
		// distinct wording from the ErrNoValidClient case so authority rejections are
		// identifiable in responses and logs
		return nil, NewJSONError(
			MethodNotFoundErrorCode,
			fmt.Sprintf("the method %s is rejected as not supported", method),
			nil,
		)
	}
	if errors.Is(r.Err, clientpool.ErrNoValidClient) {
		return nil, NewJSONError(
			MethodNotFoundErrorCode,
			fmt.Sprintf("the method %s does not exist/is not available", method),
			nil,
		)
	}
	ctxData.RespHeaders = http.Header{}
	ctxData.RespHeaders.Set(UpstreamHeaderKey, r.ClientName)
	return data, r.Err
}

func NewJSONRPCProxyMiddleware[CONFIG clientpool.EntryConfig[CONFIG], CLIENT jsonRPCClient](
	clientPool *clientpool.ClientPool[CONFIG, CLIENT],
) Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			if GetCtxData(ctx).WebsocketSession != nil {
				return next(ctx, method, params)
			}
			args, err := ParseParams(params)
			if err != nil {
				return nil, err
			}
			return ProxyJSONRPCRequest(ctx, method, args, clientPool)
		}
	}
}
