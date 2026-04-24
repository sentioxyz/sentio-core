package fuel

import (
	"context"
	"net/http"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/clientpool/ex"
	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"time"

	"github.com/pkg/errors"
	fuelGo "github.com/sentioxyz/fuel-go"
	"github.com/sentioxyz/fuel-go/types"
)

type ClientConfig struct {
	Endpoint         string        `json:"endpoint" yaml:"endpoint"`
	KeepWatch        time.Duration `json:"keep_watch" yaml:"keep_watch"`
	GetLatestTimeout time.Duration `json:"get_latest_timeout" yaml:"get_latest_timeout"`
	GetBlockTimeout  time.Duration `json:"get_block_timeout" yaml:"get_block_timeout"`
}

func (c ClientConfig) Trim() ClientConfig {
	return ClientConfig{
		Endpoint:         c.Endpoint,
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

var debugFuelClient = envconf.LoadBool("SENTIO_DEBUG_FUEL_CLIENT", false)

type Client struct {
	name       string
	config     ClientConfig
	httpClient *http.Client
	client     *fuelGo.Client
	stat       *ex.StatWinManager
}

var httpClient = https.NewClient(https.WithTimeout(time.Minute))

func NewClient(config ClientConfig) *Client {
	opts := []fuelGo.Options{fuelGo.WithHTTPClient(httpClient)}
	if debugFuelClient {
		_, logger := log.FromContext(context.Background(), "endpoint", config.Endpoint)
		opts = append(opts, fuelGo.WithLogger(logger))
	}
	return &Client{
		name:       clientpool.BuildPublicName(config.Endpoint),
		config:     config,
		httpClient: httpClient,
		client:     fuelGo.NewClient(config.Endpoint, opts...),
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

func IsQueryErrors(err error) bool {
	var queryErrors fuelGo.QueryErrors
	return errors.As(err, &queryErrors)
}

func (c *Client) _getLatest(ctx context.Context) (clientpool.Block, clientpool.Result) {
	callCtx, cancel := context.WithTimeout(ctx, c.config.GetLatestTimeout)
	defer cancel()
	latest, err := c.client.GetLatestBlockHeader(callCtx)
	if err != nil {
		return clientpool.Block{}, clientpool.Result{Err: err, Broken: !IsQueryErrors(err)}
	}
	return clientpool.Block{
		Number:    uint64(latest.Height),
		Hash:      latest.Id.String(),
		Timestamp: latest.Time.Time,
	}, clientpool.Result{}
}

func (c *Client) _checkBlock(bn uint64, block *types.Block) error {
	if block == nil {
		return errors.Errorf("block %d not found", bn)
	}
	for j, txn := range block.Transactions {
		if txn.Status == nil {
			return errors.Errorf("transaction %d/%s in block %d miss status", j, txn.Id.String(), block.Height)
		}
	}
	return nil
}

func (c *Client) _getBlock(
	ctx context.Context,
	bn uint64,
	opt fuelGo.GetBlockOption,
) (*types.Block, clientpool.Result) {
	callCtx, cancel := context.WithTimeout(ctx, c.config.GetBlockTimeout)
	defer cancel()
	height := types.U32(bn)
	req := types.QueryBlockParams{Height: &height}
	blk, err := c.client.GetBlock(callCtx, req, opt)
	if err != nil {
		return nil, clientpool.Result{Err: err, Broken: !IsQueryErrors(err)}
	}
	if err = c._checkBlock(bn, blk); err != nil {
		return nil, clientpool.Result{Err: err, BrokenForTask: true}
	}
	return blk, clientpool.Result{}
}

func (c *Client) _getBlocks(
	ctx context.Context,
	bns []uint64,
	opt fuelGo.GetBlockOption,
) ([]*types.Block, clientpool.Result) {
	callCtx, cancel := context.WithTimeout(ctx, c.config.GetBlockTimeout)
	defer cancel()
	req := utils.MapSliceNoError(bns, func(bn uint64) types.QueryBlockParams {
		height := types.U32(bn)
		return types.QueryBlockParams{Height: &height}
	})
	blocks, err := c.client.GetBlocks(callCtx, req, opt)
	if err != nil {
		return nil, clientpool.Result{Err: err, Broken: !IsQueryErrors(err)}
	}
	for i, block := range blocks {
		if err = c._checkBlock(bns[i], block); err != nil {
			return nil, clientpool.Result{Err: err, BrokenForTask: true}
		}
	}
	return blocks, clientpool.Result{}
}

func (c *Client) GetBlock(
	ctx context.Context,
	src string,
	bn uint64,
	opt fuelGo.GetBlockOption,
) (*types.Block, clientpool.Result) {
	var block *types.Block
	r := c.Use(ctx, src+".getBlock", func(ctx context.Context) (r clientpool.Result) {
		block, r = c._getBlock(ctx, bn, opt)
		return r
	})
	return block, r
}

func (c *Client) GetBlocks(
	ctx context.Context,
	src string,
	bns []uint64,
	opt fuelGo.GetBlockOption,
) ([]*types.Block, clientpool.Result) {
	var blocks []*types.Block
	r := c.Use(ctx, src+".getBlocks", func(ctx context.Context) (r clientpool.Result) {
		blocks, r = c._getBlocks(ctx, bns, opt)
		return r
	})
	return blocks, r
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

func (c *Client) UseAsHTTPClient(
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
