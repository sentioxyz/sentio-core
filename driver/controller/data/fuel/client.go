package fuel

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"github.com/sentioxyz/fuel-go/types"

	"sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
)

type Client interface {
	GetLatest(ctx context.Context) (latest controller.BlockHeader, first uint64, err error)
	Subscribe(
		ctx context.Context,
		from controller.BlockHeader,
		callback func(latest controller.BlockHeader, broken error),
	)
	GetHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (controller.BlockHeader, error)

	GetBlock(ctx context.Context, blockNumber uint64) (Block, error)
	GetTransactions(ctx context.Context, param fuel.GetTransactionsParam) ([]fuel.WrappedTransaction, error)
	GetContractCreateBlockHeight(ctx context.Context, contractID string, startBlock uint64) (uint64, bool, error)

	ResetCache(r controller.BlockRange)
	Snapshot() any
}

type client struct {
	endpoint            string
	firstBlockNumber    int64
	watchLatestInterval time.Duration

	resMgr *concurrency.ResourceManager
	stat   *data.CallStatistics

	cli *rpc.Client

	cachedHeaders *data.BlockCache[Block]
}

func NewClient(
	ctx context.Context,
	endpoint string,
	maxConcurrency int,
	firstBlockNumber int64,
	watchLatestInterval time.Duration,
) (c Client, err error) {
	cli := &client{
		endpoint:            endpoint,
		firstBlockNumber:    firstBlockNumber,
		watchLatestInterval: watchLatestInterval,
		resMgr:              concurrency.NewResourceManager(maxConcurrency),
		stat:                data.NewDefaultCallStatistics(),
	}
	if cli.cli, err = rpc.DialOptions(ctx, endpoint, rpc.WithHTTPClient(https.DefaultClient)); err != nil {
		return nil, errors.Wrapf(err, "dial to %s failed", endpoint)
	}
	cli.cachedHeaders, _ = data.NewBlockCache[Block](100000)
	return cli, nil
}

func (c *client) callContext(ctx context.Context, result any, priority uint64, method string, args ...any) error {
	startAt := time.Now()
	// waiting concurrency control token
	release, err := c.resMgr.Apply(ctx, int64(priority), 1, time.Minute, func(waited time.Duration) {
		_, logger := log.FromContext(ctx, "priority", priority, "args", utils.MustJSONMarshal(args))
		logger.Warnf("call method %s waited %s", method, waited.String())
	})
	if err != nil {
		return err // always be context.Canceled
	}
	defer release()
	// actually call
	callStartAt := time.Now()
	err = c.cli.CallContext(ctx, &result, method, args...)
	if err != nil {
		err = errors.Wrapf(err, "call method %s with args %s failed", method, utils.MustJSONMarshal(args))
	}
	c.stat.Called(method, args, err, startAt, callStartAt)
	return err
}

func (c *client) GetLatest(ctx context.Context) (latest controller.BlockHeader, first uint64, err error) {
	var resp fuel.GetLatestBlockResponse
	if err = c.callContext(ctx, &resp, 0, "fuel_getLatestHeader", 0); err != nil {
		return nil, 0, err
	}
	if err = resp.CheckAPIVersion(); err != nil {
		return nil, 0, errors.Wrapf(controller.ErrInternalNeedUpgrade, err.Error())
	}
	latest = Block{Header: resp.Header}
	return latest, data.GetFirst(c.firstBlockNumber, latest.GetBlockNumber()), err
}

func (c *client) Subscribe(
	ctx context.Context,
	from controller.BlockHeader,
	callback func(latest controller.BlockHeader, broken error),
) {
	data.SubscribeUsingWaiting(
		ctx,
		c.watchLatestInterval,
		from,
		func(ctx context.Context, blockHeightGt uint64) (latest controller.BlockHeader, broken, err error) {
			var resp fuel.GetLatestBlockResponse
			err = c.callContext(ctx, &resp, 0, "fuel_getLatestHeader", blockHeightGt)
			if err == nil {
				latest, broken = Block{Header: resp.Header}, resp.CheckAPIVersion()
			}
			if broken != nil {
				broken = errors.Wrapf(controller.ErrInternalNeedUpgrade, broken.Error())
			}
			return
		},
		callback)
}

func (c *client) fetchBlock(ctx context.Context, blockNumber uint64) (Block, error) {
	var header types.Header
	if err := c.callContext(ctx, &header, blockNumber, "fuel_getBlockHeader", blockNumber); err != nil {
		return Block{}, err
	}
	return Block{Header: header}, nil
}

func (c *client) getHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (Block, error) {
	block, err := c.fetchBlock(ctx, blockNumber)
	if err == nil {
		c.cachedHeaders.Add(blockNumber, block)
	}
	return block, err
}

func (c *client) GetHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (controller.BlockHeader, error) {
	return c.getHeaderIgnoreCache(ctx, blockNumber)
}

func (c *client) GetBlock(ctx context.Context, blockNumber uint64) (Block, error) {
	// Cache + singleflight: concurrent fetchers asking for the same block share one fuel_getBlockHeader.
	return c.cachedHeaders.GetOrFetch(blockNumber, func() (Block, error) {
		return c.fetchBlock(ctx, blockNumber)
	})
}

func (c *client) GetTransactions(ctx context.Context, param fuel.GetTransactionsParam) ([]fuel.WrappedTransaction, error) {
	var txs []fuel.WrappedTransaction
	err := c.callContext(ctx, &txs, param.StartHeight, "fuel_getTransactions", param)
	return txs, err
}

func (c *client) GetContractCreateBlockHeight(
	ctx context.Context,
	contractID string,
	startBlock uint64,
) (blockNumber uint64, has bool, err error) {
	var tx *fuel.WrappedTransaction
	if err = c.callContext(ctx, &tx, 0, "fuel_getContractCreateTransaction", contractID); err != nil {
		return 0, false, err
	}
	if tx == nil {
		return 0, false, nil
	}
	return max(tx.BlockHeight, startBlock), true, nil
}

func (c *client) ResetCache(r controller.BlockRange) {
	for _, bn := range c.cachedHeaders.Keys() {
		if r.Contains(bn) {
			c.cachedHeaders.Remove(bn)
		}
	}
}

func (c *client) Snapshot() any {
	return map[string]any{
		"config": map[string]any{
			"endpoint":            c.endpoint,
			"firstBlockNumber":    c.firstBlockNumber,
			"watchLatestInterval": c.watchLatestInterval.String(),
		},
		"resourceManager": c.resMgr.Snapshot(),
		"statistics":      c.stat.Snapshot(),
		"cache": map[string]any{
			"cachedHeaders": c.cachedHeaders.Snapshot(10, controller.GetBlockFullText[Block]),
		},
	}
}
