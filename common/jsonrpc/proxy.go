package jsonrpc

import (
	"context"
	"github.com/pkg/errors"
	"net/http"
	"sentioxyz/sentio-core/chain/clientpool"
)

const UpstreamHeaderKey = "X-Sentio-Proxy-Endpoint"

func ProxyHTTP[CONFIG clientpool.EntryConfig[CONFIG], CLIENT clientpool.Client](
	ctx context.Context,
	method string,
	clientPool *clientpool.ClientPool[CONFIG, CLIENT],
	fn func(ctx context.Context, cli CLIENT) (resp *http.Response, respBody []byte, upstream string, r clientpool.Result),
) {
	ctxData := GetCtxData(ctx)
	var resp *http.Response
	var respBody []byte
	var upstream string
	err := clientPool.UseClient(ctx, "proxy."+method, func(ctx context.Context, cli CLIENT) (r clientpool.Result) {
		resp, respBody, upstream, r = fn(ctx, cli)
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
	ctxData.RespWriter.Header().Add(UpstreamHeaderKey, upstream)
	ctxData.RespWriter.WriteHeader(resp.StatusCode)
	_, _ = ctxData.RespWriter.Write(respBody)
}
