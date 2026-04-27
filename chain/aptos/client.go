package aptos

import (
	"context"
	"fmt"
	"github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/clientpool/ex"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/utils"
	"strconv"
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

var httpClient = https.NewClient(https.WithTimeout(time.Minute))

type Client struct {
	name       string
	config     ClientConfig
	httpClient *http.Client
	stat       *ex.StatWinManager
}

func NewClient(config ClientConfig) *Client {
	return &Client{
		name:       clientpool.BuildPublicName(config.Endpoint),
		config:     config,
		httpClient: httpClient,
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
	req, err := clientpool.BuildHTTPRequest(callCtx, "GET", c.config.Endpoint, "/v1", nil, nil, nil)
	if err != nil {
		return clientpool.Block{}, clientpool.Result{Err: err, Broken: true}
	}
	var result aptos.NodeInfo
	_, _, r := clientpool.SendHTTP(c.httpClient, req, &result)
	if r.Err != nil {
		return clientpool.Block{}, r
	}
	return clientpool.Block{
		Number:    result.BlockHeight(),
		Timestamp: time.UnixMicro(int64(result.LedgerTimestamp())),
	}, r
}

func (c *Client) _getBlock(ctx context.Context, bn uint64, withTxs bool) (api.Block, clientpool.Result) {
	callCtx, cancel := context.WithTimeout(ctx, c.config.GetBlockTimeout)
	defer cancel()
	params := make(url.Values)
	params.Set("with_transactions", strconv.FormatBool(withTxs))
	path := fmt.Sprintf("/v1/blocks/by_height/%d", bn)
	req, err := clientpool.BuildHTTPRequest(callCtx, "GET", c.config.Endpoint, path, params, nil, nil)
	if err != nil {
		return api.Block{}, clientpool.Result{Err: err, Broken: true}
	}
	var result *api.Block
	_, _, r := clientpool.SendHTTP(c.httpClient, req, &result)
	if result == nil && r.Err == nil {
		r.Err = errors.Errorf("block %d not found", bn)
	}
	if r.Err != nil {
		return api.Block{}, r
	}
	return *result, r
}

func (c *Client) GetBlock(ctx context.Context, src string, bn uint64, withTxs bool) (api.Block, clientpool.Result) {
	var block api.Block
	method := src + utils.Select(withTxs, ".getBlockWithTxs", ".getBlock")
	r := c.Use(ctx, method, func(ctx context.Context) (r clientpool.Result) {
		block, r = c._getBlock(ctx, bn, withTxs)
		return r
	})
	return block, r
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

func (c *Client) UseHTTPClient(
	ctx context.Context,
	method string,
	fn func(ctx context.Context, endpoint string, cli *http.Client) clientpool.Result,
) clientpool.Result {
	return c.Use(ctx, method, func(ctx context.Context) (r clientpool.Result) {
		return fn(ctx, c.config.Endpoint, c.httpClient)
	})
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
