package supernode

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"io"
	"net/http"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	"testing"
)

func Test_rpc(t *testing.T) {
	log.ManuallySetLevel(zapcore.DebugLevel)
	log.BindFlag()

	ctx, cancel := context.WithCancel(context.Background())
	g, gctx := errgroup.WithContext(ctx)

	// prepare client pool
	cli := sol.NewClientPool("client")
	g.Go(func() error {
		ch := make(chan clientpool.PoolConfig[sol.ClientConfig], 1)
		ch <- clientpool.PoolConfig[sol.ClientConfig]{
			ClientConfigs: []clientpool.ClientConfig[sol.ClientConfig]{
				{
					Config: sol.ClientConfig{
						Endpoint: "https://solana-rpc.publicnode.com",
					},
				},
			},
		}
		cli.Start(gctx, ch)
		return nil
	})

	addr := "127.0.0.1:18889"
	h := jsonrpc.NewHandler("test", true, false, nil, nil, "")
	h.RegisterMiddleware(NewSimpleProxyService("", cli)...)

	g.Go(func() error {
		return jsonrpc.ListenAndServe(gctx, ":18889", h)
	})

	t.Run("proxy.getTransaction", func(t *testing.T) {
		body := `{
		    "method": "getTransaction",
		    "params": [
		        "5mNLAzyBCtrFmDoXvUpaQ7TpiCje2VbGFUr1dYQ2tBPt72FSXUa1mrqdNxFEqaZbNQtQUG8dWLbvLBpZwgyFNscd",
		        {
		            "encoding": "jsonParsed",
		            "maxSupportedTransactionVersion": 0
		        }
		    ],
		    "id": 1,
		    "jsonrpc": "2.0"
		}`
		resp, err := http.Post("http://"+addr, "application/json", bytes.NewReader([]byte(body)))
		assert.NoError(t, err)
		for k, vs := range resp.Header {
			log.Infof("getTransaction got header: %s = %s", k, vs)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NoError(t, err)
		var buf bytes.Buffer
		assert.NoError(t, json.Indent(&buf, raw, "", "\t"))
		log.Infof("getTransaction got body: %s", buf.String())
	})

	b, _ := json.MarshalIndent(cli.Snapshot(), "", "\t")
	log.Infof("client: %s", string(b))

	cancel()
	_ = g.Wait()
}
