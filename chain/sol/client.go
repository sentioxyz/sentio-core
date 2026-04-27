package sol

import (
	"context"
	"fmt"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
	"github.com/pkg/errors"
	"net/http"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/clientpool/ex"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
)

type ClientConfig struct {
	Endpoint         string        `json:"endpoint" yaml:"endpoint"`
	KeepWatch        time.Duration `json:"keep_watch" yaml:"keep_watch"`
	GetLatestTimeout time.Duration `json:"get_latest_timeout" yaml:"get_latest_timeout"`
	GetBlockTimeout  time.Duration `json:"get_block_timeout" yaml:"get_block_timeout"`
}

func (c ClientConfig) Trim() ClientConfig {
	return ClientConfig{
		Endpoint:         strings.TrimSpace(c.Endpoint),
		KeepWatch:        utils.Select(c.KeepWatch == 0, time.Second, c.KeepWatch),
		GetLatestTimeout: utils.Select(c.GetLatestTimeout == 0, time.Second*3, c.GetLatestTimeout),
		GetBlockTimeout:  utils.Select(c.GetBlockTimeout == 0, time.Second*3, c.GetBlockTimeout),
	}
}

func (c ClientConfig) GetName() string {
	return c.Endpoint
}

func (c ClientConfig) Equal(a ClientConfig) bool {
	return c == a
}

type Client struct {
	name       string
	config     ClientConfig
	httpClient *http.Client
	client     *rpc.Client
	stat       *ex.StatWinManager
}

var httpClient = https.NewClient(https.WithTimeout(time.Minute))

func NewClient(config ClientConfig) *Client {
	client := rpc.NewWithCustomRPCClient(
		jsonrpc.NewClientWithOpts(
			config.Endpoint,
			&jsonrpc.RPCClientOpts{HTTPClient: httpClient},
		),
	)
	return &Client{
		name:       clientpool.BuildPublicName(config.Endpoint),
		config:     config,
		httpClient: httpClient,
		client:     client,
		stat:       ex.NewStatWinManager(time.Minute),
	}
}

func (c *Client) Init(ctx context.Context) (clientpool.Block, error) {
	var latest clientpool.Block
	r := c.Use(ctx, "init.getLatest", func(ctx context.Context) (r clientpool.Result) {
		latest, r = c._getLatest(ctx)
		return r
	})
	return latest, r.Err
}

func (c *Client) SubscribeLatest(ctx context.Context, start uint64, ch chan<- clientpool.Block) {
	clientpool.SubscribeUsingGetLatest(
		ctx,
		start,
		c.config.KeepWatch,
		time.Minute*5,
		ch,
		func(ctx context.Context) (clientpool.Block, error) {
			var latest clientpool.Block
			r := c.Use(ctx, "subscribe.getLatest", func(ctx context.Context) (r clientpool.Result) {
				latest, r = c._getLatest(ctx)
				return r
			})
			return latest, r.Err
		},
	)
}

func (c *Client) _getLatest(ctx context.Context) (clientpool.Block, clientpool.Result) {
	callCtx, cancel := context.WithTimeout(ctx, c.config.GetLatestTimeout)
	defer cancel()
	latestNumber, err := c.client.GetSlot(callCtx, rpc.CommitmentFinalized)
	if err != nil {
		return clientpool.Block{}, clientpool.Result{Err: err, Broken: true}
	}
	latestTime, err := c.client.GetBlockTime(callCtx, latestNumber)
	if err != nil {
		return clientpool.Block{}, clientpool.Result{Err: err, Broken: true}
	}
	if latestTime == nil {
		return clientpool.Block{}, clientpool.Result{
			Err:    fmt.Errorf("getBlockTime for the latest block %d got nil", latestNumber),
			Broken: true,
		}
	}
	return clientpool.Block{
		Number:    latestNumber,
		Timestamp: latestTime.Time(),
	}, clientpool.Result{}
}

func IsBrokenError(err error) bool {
	var httpErr *jsonrpc.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Code != http.StatusBadRequest && httpErr.Code != http.StatusNotFound
	}
	var rpcErr *jsonrpc.RPCError
	if errors.As(err, &rpcErr) {
		return false
	}
	return true
}

func IsInvalidMethodError(err error) bool {
	if strings.Contains(strings.ToLower(err.Error()), "method not found") {
		return true
	}
	return false
}

func (c *Client) CallContext(
	ctx context.Context,
	result any,
	src string,
	method string,
	args ...any,
) clientpool.Result {
	return c.Use(ctx, src+"."+method, func(ctx context.Context) (r clientpool.Result) {
		r.Err = c.client.RPCCallForInto(ctx, &result, method, args)
		if r.Err == nil || errors.Is(r.Err, context.Canceled) || errors.Is(r.Err, context.DeadlineExceeded) {
			return r
		}
		r.Broken = IsBrokenError(r.Err)
		r.BrokenForTask = IsInvalidMethodError(r.Err)
		return r
	})
}

func (c *Client) Use(
	ctx context.Context,
	method string,
	fn func(ctx context.Context) clientpool.Result,
) clientpool.Result {
	startAt := time.Now()
	r := fn(ctx)
	c.stat.Record(method, time.Since(startAt), r.Err != nil)
	return r
}

func (c *Client) GetConfig() ClientConfig {
	return c.config
}

func (c *Client) GetHTTPClient() *http.Client {
	return c.httpClient
}

func (c *Client) GetName() string {
	return c.name
}

func (c *Client) Snapshot() any {
	return map[string]any{
		"statistic": c.stat.Snapshot(),
	}
}

type ClientPool struct {
	*clientpool.ClientPool[ClientConfig, *Client]
}

func NewClientPool(name string) *ClientPool {
	return &ClientPool{
		ClientPool: clientpool.NewClientPool(name, NewClient),
	}
}
