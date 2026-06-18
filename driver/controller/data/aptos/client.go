package aptos

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"

	aptossdk "github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
)

type Client interface {
	GetLatest(ctx context.Context) (latest controller.BlockHeader, first uint64, err error)
	Subscribe(
		ctx context.Context,
		from controller.BlockHeader,
		callback func(latest controller.BlockHeader, broken error),
	)
	GetTransaction(ctx context.Context, txnVersion uint64) (aptos.Transaction, error)
	GetMinimalistTransaction(ctx context.Context, txnVersion uint64) (MinimalistTransaction, error)
	GetHeaderIgnoreCache(ctx context.Context, txnVersion uint64) (controller.BlockHeader, error)

	GetChanges(
		ctx context.Context,
		startTxnVersion, endTxnVersion uint64,
		filter aptos.ChangeFilter,
	) ([]MinimalistTransactionWithChanges, error)
	GetTransactions(
		ctx context.Context,
		startTxnVersion, endTxnVersion uint64,
		filter aptos.TransactionFilter,
		fetchConfig aptos.TransactionFetchConfig,
	) ([]aptos.Transaction, error)
	// GetAccountResources key of requirement is Address,
	// value is the set of ResourceType, empty set means need all Resource of the Account
	GetAccountResources(
		ctx context.Context,
		txnVersion uint64,
		requirement map[string][]string, // requirement[<account>] is null means need all resource of <account>
	) ([]AccountResource, error)

	GetAddressStartBlock(ctx context.Context, address string, start, latest uint64) (uint64, bool, error)

	ResetCache(r controller.BlockRange)
	Snapshot() any
}

type client struct {
	endpoint            string
	firstTxnVersion     int64
	watchLatestInterval time.Duration

	resMgr *concurrency.ResourceManager
	stat   *data.CallStatistics

	rpcCli *rpc.Client
	rawCli *aptossdk.NodeClient

	cachedMinimalistTxn *data.BlockCache[MinimalistTransaction]
}

func NewClient(
	ctx context.Context,
	endpoint string,
	maxConcurrency int,
	firstTxnVersion int64,
	watchLatestInterval time.Duration,
) (Client, error) {
	cli := &client{
		endpoint:            endpoint,
		firstTxnVersion:     firstTxnVersion,
		watchLatestInterval: watchLatestInterval,
		resMgr:              concurrency.NewResourceManager(maxConcurrency),
		stat:                data.NewDefaultCallStatistics(),
	}
	var err error
	if cli.rpcCli, err = rpc.DialOptions(ctx, endpoint, rpc.WithHTTPClient(https.DefaultClient)); err != nil {
		return nil, errors.Wrapf(err, "dial to %s failed", endpoint)
	}
	cli.rawCli, err = aptossdk.NewNodeClientWithHttpClient(
		fmt.Sprintf("%s/v1", strings.TrimRight(endpoint, "/")),
		0,
		https.NewClient(https.WithTimeout(time.Minute)))
	if err != nil {
		return nil, errors.Wrapf(err, "build aptos node client with endpoint %s failed", endpoint)
	}
	cli.cachedMinimalistTxn, _ = data.NewBlockCache[MinimalistTransaction](100000)
	return cli, nil
}

func (c *client) callContext(ctx context.Context, result any, priority uint64, method string, args ...any) (err error) {
	startAt := time.Now()
	// waiting concurrency control token
	var release func()
	release, err = c.resMgr.Apply(ctx, int64(priority), 1, time.Minute, func(waited time.Duration) {
		_, logger := log.FromContext(ctx, "priority", priority, "args", utils.MustJSONMarshal(args))
		logger.Warnf("call method %s waited %s", method, waited.String())
	})
	waitEndAt := time.Now()
	defer func() {
		c.stat.Called(method, args, err, startAt, waitEndAt)
	}()
	if err != nil {
		return err // always be context.Canceled or context.DeadlineExceeded
	}
	defer func() {
		if err != nil {
			err = errors.Wrapf(err, "call method %s with args %s failed", method, utils.MustJSONMarshal(args))
		}
	}()
	defer release()
	// actually call
	switch method {
	case "raw_getAccountResourcesAll":
		txnVersion := args[0].(uint64)
		address := args[1].(string)
		addr := aptossdk.AccountAddress(common.HexToHash(address))
		var resources []aptossdk.AccountResourceInfo
		resources, err = c.rawCli.AccountResourcesByPages(addr, txnVersion, 0)
		if err != nil {
			return err
		}
		r := result.(*[]AccountResource)
		(*r), err = utils.MapSlice(resources, func(res aptossdk.AccountResourceInfo) (AccountResource, error) {
			raw, marshalErr := json.Marshal(res)
			if marshalErr != nil {
				return AccountResource{}, marshalErr
			}
			return AccountResource{
				Raw:     string(raw),
				Address: address,
				Type:    res.Type,
			}, nil
		})
		return err
	case "raw_getAccountResource":
		txnVersion := args[0].(uint64)
		address := args[1].(string)
		resourceType := args[2].(string)
		addr := aptossdk.AccountAddress(common.HexToHash(address))
		var res map[string]any
		res, err = c.rawCli.AccountResource(addr, resourceType, txnVersion)
		if err != nil {
			return err
		}
		r := result.(*AccountResource)
		if raw, marshalErr := json.Marshal(res); marshalErr != nil {
			return marshalErr
		} else {
			r.Raw = string(raw)
			r.Address = address
			r.Type = resourceType
		}
		return nil
	case "raw_getTxByVersion":
		txnVersion := args[0].(uint64)
		var tx *api.CommittedTransaction
		tx, err = c.rawCli.TransactionByVersion(txnVersion)
		if err != nil {
			return err
		}
		// will return error if not found, so here tx will always be non-null
		r := result.(*api.CommittedTransaction)
		*r = *tx
		return nil
	default:
		return c.rpcCli.CallContext(ctx, &result, method, args...)
	}
}

func (c *client) GetLatest(ctx context.Context) (controller.BlockHeader, uint64, error) {
	var resp aptos.GetLatestMinimalistTransactionResponse
	err := c.callContext(ctx, &resp, 0, "aptosV2_getLatestMinimalistTransaction", 0)
	if err != nil {
		return nil, 0, err
	}
	if err = resp.CheckAPIVersion(); err != nil {
		return nil, 0, errors.Wrapf(controller.ErrInternalNeedUpgrade, err.Error())
	}
	latest := MinimalistTransaction(resp.Transaction)
	return latest, data.GetFirst(c.firstTxnVersion, latest.Version), err
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
		func(ctx context.Context, txVersionGt uint64) (latest controller.BlockHeader, broken, err error) {
			var resp aptos.GetLatestMinimalistTransactionResponse
			err = c.callContext(ctx, &resp, 0, "aptosV2_getLatestMinimalistTransaction", txVersionGt)
			if err == nil {
				latest, broken = MinimalistTransaction(resp.Transaction), resp.CheckAPIVersion()
			}
			if broken != nil {
				broken = errors.Wrapf(controller.ErrInternalNeedUpgrade, broken.Error())
			}
			return
		},
		callback)
}

func (c *client) GetTransaction(ctx context.Context, txnVersion uint64) (aptos.Transaction, error) {
	var raw api.CommittedTransaction
	if err := c.callContext(ctx, &raw, txnVersion, "raw_getTxByVersion", txnVersion); err != nil {
		return aptos.Transaction{}, err
	}
	return aptos.NewTransaction(&raw), nil
}

func (c *client) GetMinimalistTransaction(ctx context.Context, txnVersion uint64) (MinimalistTransaction, error) {
	// Cache + singleflight: concurrent fetchers asking for the same version share one RPC.
	return c.cachedMinimalistTxn.GetOrFetch(txnVersion, func() (MinimalistTransaction, error) {
		return c.fetchMinimalistTransaction(ctx, txnVersion)
	})
}

func (c *client) fetchMinimalistTransaction(ctx context.Context, txnVersion uint64) (MinimalistTransaction, error) {
	var txn *MinimalistTransaction
	if err := c.callContext(ctx, &txn, txnVersion, "aptosV2_getMinimalistTransaction", txnVersion); err != nil {
		return MinimalistTransaction{}, err
	}
	if txn == nil {
		return MinimalistTransaction{}, errors.Errorf("transaction %d not found", txnVersion)
	}
	return *txn, nil
}

func (c *client) getMinimalistTransaction(ctx context.Context, txnVersion uint64) (MinimalistTransaction, error) {
	txn, err := c.fetchMinimalistTransaction(ctx, txnVersion)
	if err == nil {
		c.cachedMinimalistTxn.Add(txnVersion, txn)
	}
	return txn, err
}

func (c *client) GetHeaderIgnoreCache(ctx context.Context, txnVersion uint64) (controller.BlockHeader, error) {
	return c.getMinimalistTransaction(ctx, txnVersion)
}

func (c *client) GetChanges(
	ctx context.Context,
	startTxnVersion, endTxnVersion uint64,
	filter aptos.ChangeFilter,
) (result []MinimalistTransactionWithChanges, err error) {
	args := aptos.GetResourceChangesRequest{
		FromVersion: startTxnVersion,
		ToVersion:   endTxnVersion,
		Filter:      filter,
	}
	err = c.callContext(ctx, &result, startTxnVersion, "aptosV2_getResourceChanges", args)
	return
}

func (c *client) GetTransactions(
	ctx context.Context,
	startTxnVersion, endTxnVersion uint64,
	filter aptos.TransactionFilter,
	fetchConfig aptos.TransactionFetchConfig,
) (result []aptos.Transaction, err error) {
	args := aptos.GetTransactionsRequest{
		FromVersion: startTxnVersion,
		ToVersion:   endTxnVersion,
		Filter:      filter,
		FetchConfig: fetchConfig,
	}
	err = c.callContext(ctx, &result, startTxnVersion, "aptosV2_getTransactions", args)
	return
}

func (c *client) GetAccountResources(
	ctx context.Context,
	txnVersion uint64,
	requirement map[string][]string,
) ([]AccountResource, error) {
	if int64(txnVersion) < c.firstTxnVersion || len(requirement) == 0 {
		return nil, nil
	}
	var result utils.SafeSlice[AccountResource]
	g, gctx := errgroup.WithContext(ctx)
	for address_, types := range requirement {
		address := address_
		if types == nil { // need all resources
			g.Go(func() error {
				var resources []AccountResource
				err := c.callContext(gctx, &resources, txnVersion, "raw_getAccountResourcesAll", txnVersion, address)
				if err == nil {
					result.Append(resources...)
				}
				return err
			})
		} else {
			for _, typ_ := range set.New(types...).DumpValues() {
				typ := typ_
				g.Go(func() error {
					var resource AccountResource
					err := c.callContext(gctx, &resource, txnVersion, "raw_getAccountResource", txnVersion, address, typ)
					if err == nil {
						result.Append(resource)
					}
					return err
				})
			}
		}
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return result.Dump(), nil
}

func (c *client) GetAddressStartBlock(ctx context.Context, address string, start, latest uint64) (uint64, bool, error) {
	var result *uint64
	err := c.callContext(ctx, &result, start, "aptosV2_getAddressStartTxVersion", address, latest)
	if err != nil {
		return 0, false, err
	}
	if result == nil {
		return 0, false, nil
	}
	return max(*result, start), true, nil
}

func (c *client) ResetCache(r controller.BlockRange) {
	for _, bn := range c.cachedMinimalistTxn.Keys() {
		if r.Contains(bn) {
			c.cachedMinimalistTxn.Remove(bn)
		}
	}
}

func (c *client) Snapshot() any {
	return map[string]any{
		"config": map[string]any{
			"endpoint":            c.endpoint,
			"firstTxnVersion":     c.firstTxnVersion,
			"watchLatestInterval": c.watchLatestInterval.String(),
		},
		"resourceManager":     c.resMgr.Snapshot(),
		"statistics":          c.stat.Snapshot(),
		"cachedMinimalistTxn": c.cachedMinimalistTxn.Snapshot(10, controller.GetBlockFullText[MinimalistTransaction]),
	}
}
