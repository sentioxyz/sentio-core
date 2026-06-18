package sui

import (
	"context"
	"math"
	"strings"
	"time"

	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	lru "github.com/sentioxyz/golang-lru"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
)

type Client interface {
	GetLatest(ctx context.Context) (latest controller.BlockHeader, first uint64, err error)
	Subscribe(
		ctx context.Context,
		from controller.BlockHeader,
		callback func(latest controller.BlockHeader, broken error),
	)
	GetSimpleBlock(ctx context.Context, blockNumber uint64) (SimpleBlock, error)
	GetHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (controller.BlockHeader, error)

	GetObjectChanges(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.ObjectChangeFilter,
	) (map[uint64][]types.ObjectChangeExtend, error)
	GetTransactions(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.TransactionFilter,
		fetchConfig sui.TransactionFetchConfig,
	) (map[uint64][]types.TransactionResponseV1, error)

	// grpc-format counterparts (super node DriverVersion[2]); used by the suigrpc handler path.
	GetGrpcTransactions(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.TransactionFilter,
		fetchConfig sui.TransactionFetchConfig,
	) (map[uint64][]*sui.ExtendedGrpcTransaction, error)
	GetGrpcObjectChanges(
		ctx context.Context,
		fromBlock, toBlock uint64,
		filter sui.ObjectChangeFilter,
	) (map[uint64][]*sui.ExtendedGrpcChangedObject, error)
	GetGrpcObjects(
		ctx context.Context,
		reqs []*rpcv2.GetObjectRequest,
		concurrency, batchSize int,
	) ([]*rpcv2.GetObjectResult, error)

	TryMultiGetPastObjects(
		ctx context.Context,
		requests []types.SuiGetPastObjectRequest,
		options types.SuiObjectDataOptions,
	) ([]types.SuiPastObjectResponse, error)
	MultiGetTransactionBlocks(
		ctx context.Context,
		txDigests []string,
		options map[string]any,
	) ([]types.TransactionResponseV1, error)
	GetObjectStat(ctx context.Context, fromBlock, toBlock uint64, objectID string) (sui.ObjectStat, error)
	GetObjectsStat(ctx context.Context, fromBlock, toBlock uint64, objectIDList []string) ([]sui.ObjectStat, error)
	GetObjectVersionHistory(ctx context.Context, objectID string) ([]types.ObjectChangeExtend, error)

	GetPackageHistory(ctx context.Context, pkgID string) ([]string, error)
	GetObjectCreation(ctx context.Context, objectID string, start uint64) (uint64, bool, error)

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

	cachedSimpleBlock    *data.BlockCache[SimpleBlock]
	cachedPackageHistory *lru.Cache[string, []string]
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
	cli.cachedSimpleBlock, _ = data.NewBlockCache[SimpleBlock](100000)
	cli.cachedPackageHistory, _ = lru.New[string, []string](10000)
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
	if err = c.cli.CallContext(ctx, &result, method, args...); err != nil {
		return errors.Wrapf(err, "call method %s with args %s failed", method, utils.MustJSONMarshal(args))
	}
	c.stat.Called(method, args, err, startAt, callStartAt)
	return nil
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
		func(ctx context.Context, blockNumberGt uint64) (latest controller.BlockHeader, broken, err error) {
			var resp sui.GetLatestSimpleCheckpointResponse
			err = c.callContext(ctx, &resp, 0, "sui_getLatestSimpleCheckpoint", blockNumberGt)
			if err == nil {
				latest, broken = SimpleBlock(resp.Checkpoint), resp.CheckAPIVersion()
			}
			if broken != nil {
				broken = errors.Wrapf(controller.ErrInternalNeedUpgrade, broken.Error())
			}
			return
		},
		callback)
}

func (c *client) GetLatest(ctx context.Context) (latest controller.BlockHeader, first uint64, err error) {
	var resp sui.GetLatestSimpleCheckpointResponse
	if err = c.callContext(ctx, &resp, 0, "sui_getLatestSimpleCheckpoint", 0); err != nil {
		return nil, 0, err
	}
	if err = resp.CheckAPIVersion(); err != nil {
		return nil, 0, errors.Wrapf(controller.ErrInternalNeedUpgrade, err.Error())
	}
	latest = SimpleBlock(resp.Checkpoint)
	return latest, data.GetFirst(c.firstBlockNumber, latest.GetBlockNumber()), err
}

func (c *client) fetchSimpleBlock(ctx context.Context, blockNumber uint64) (sc SimpleBlock, err error) {
	err = c.callContext(ctx, &sc, blockNumber, "sui_getSimpleCheckpoint", blockNumber)
	return
}

func (c *client) getSimpleBlockIgnoreCache(ctx context.Context, blockNumber uint64) (SimpleBlock, error) {
	sc, err := c.fetchSimpleBlock(ctx, blockNumber)
	if err == nil {
		c.cachedSimpleBlock.Add(blockNumber, sc)
	}
	return sc, err
}

func (c *client) GetHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (controller.BlockHeader, error) {
	return c.getSimpleBlockIgnoreCache(ctx, blockNumber)
}

func (c *client) GetSimpleBlock(ctx context.Context, blockNumber uint64) (SimpleBlock, error) {
	// The cache + singleflight (in BlockCache) collapses the concurrent prefetch requests for the same
	// checkpoint — made by the object-change / txn / interval fetchers — into a single RPC.
	return c.cachedSimpleBlock.GetOrFetch(blockNumber, func() (SimpleBlock, error) {
		return c.fetchSimpleBlock(ctx, blockNumber)
	})
}

func (c *client) GetObjectChanges(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.ObjectChangeFilter,
) (map[uint64][]types.ObjectChangeExtend, error) {
	var result []types.ObjectChangeExtend
	err := c.callContext(ctx, &result, 0, "sui_filterObjectChangesV2", fromBlock, toBlock, filter)
	return utils.Group(result, func(oc types.ObjectChangeExtend) uint64 {
		return oc.Checkpoint.Uint64()
	}), err
}

func (c *client) GetTransactions(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.TransactionFilter,
	fetchConfig sui.TransactionFetchConfig,
) (map[uint64][]types.TransactionResponseV1, error) {
	var result []types.TransactionResponseV1
	err := c.callContext(ctx, &result, 0, "sui_getTransactionsV2", fromBlock, toBlock, filter, fetchConfig)
	return utils.Group(result, func(oc types.TransactionResponseV1) uint64 {
		return oc.Checkpoint.Uint64()
	}), err
}

func (c *client) GetGrpcTransactions(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.TransactionFilter,
	fetchConfig sui.TransactionFetchConfig,
) (map[uint64][]*sui.ExtendedGrpcTransaction, error) {
	var result []*sui.ExtendedGrpcTransaction
	err := c.callContext(ctx, &result, 0, "sui_getGrpcTransactions", fromBlock, toBlock, filter, fetchConfig)
	return utils.Group(result, func(tx *sui.ExtendedGrpcTransaction) uint64 {
		return tx.Checkpoint
	}), err
}

func (c *client) GetGrpcObjectChanges(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.ObjectChangeFilter,
) (map[uint64][]*sui.ExtendedGrpcChangedObject, error) {
	var result []*sui.ExtendedGrpcChangedObject
	err := c.callContext(ctx, &result, 0, "sui_filterGrpcChangedObjects", fromBlock, toBlock, filter)
	return utils.Group(result, func(oc *sui.ExtendedGrpcChangedObject) uint64 {
		return oc.Checkpoint
	}), err
}

// GetGrpcObjects fetches objects by id+version in grpc format. The super node batches/parallelizes
// server-side per the concurrency/batchSize args, so no client-side paging is needed.
func (c *client) GetGrpcObjects(
	ctx context.Context,
	reqs []*rpcv2.GetObjectRequest,
	concurrency, batchSize int,
) ([]*rpcv2.GetObjectResult, error) {
	// GetObjectResult carries a protobuf oneof, which the JSON-RPC transport's
	// encoding/json can't round-trip; decode into the protojson-backed wrapper
	// (sui.GrpcObjectResult) and unwrap back to the raw proto for callers.
	var wrapped []*sui.GrpcObjectResult
	if err := c.callContext(ctx, &wrapped, 0, "sui_getGrpcObjects", reqs, concurrency, batchSize); err != nil {
		return nil, err
	}
	return sui.UnwrapGrpcObjectResults(wrapped), nil
}

const QueryObjectsPageSize = 50 // Max number of objects to request in a single call

func (c *client) TryMultiGetPastObjects(
	ctx context.Context,
	requests []types.SuiGetPastObjectRequest,
	options types.SuiObjectDataOptions,
) ([]types.SuiPastObjectResponse, error) {
	var result []types.SuiPastObjectResponse
	for len(requests) > 0 {
		query := requests
		if len(requests) > QueryObjectsPageSize {
			query = requests[:QueryObjectsPageSize]
			requests = requests[QueryObjectsPageSize:]
		} else {
			requests = requests[:0]
		}
		var pageResult []types.SuiPastObjectResponse
		err := c.callContext(ctx, &pageResult, 0, "sui_tryMultiGetPastObjects", query, options)
		if err != nil {
			return nil, err
		}
		if len(pageResult) != len(query) {
			return nil, errors.Errorf("call sui_tryMultiGetPastObjects failed: unexpected number of results: %d", len(pageResult))
		}
		result = append(result, pageResult...)
	}
	return result, nil
}

const QueryTxPageSize = 50 // Max number of txs to request in a single call

func (c *client) MultiGetTransactionBlocks(
	ctx context.Context,
	txDigests []string,
	options map[string]any,
) ([]types.TransactionResponseV1, error) {
	txList := make([]types.TransactionResponseV1, 0, len(txDigests))
	for len(txDigests) > 0 {
		query := txDigests
		if len(txDigests) > QueryTxPageSize {
			query = txDigests[:QueryTxPageSize]
			txDigests = txDigests[QueryTxPageSize:]
		} else {
			txDigests = txDigests[:0]
		}
		var result []types.TransactionResponseV1
		err := c.callContext(ctx, &result, 0, "sui_multiGetTransactionBlocks", query, options)
		if err != nil {
			return nil, err
		}
		txList = append(txList, result...)
	}
	return txList, nil
}

func (c *client) GetObjectStat(ctx context.Context, fromBlock, toBlock uint64, objectID string) (sui.ObjectStat, error) {
	var result sui.ObjectStat
	err := c.callContext(ctx, &result, fromBlock, "sui_getObjectStat", fromBlock, toBlock, objectID)
	return result, err
}

func (c *client) getObjectsStat(
	ctx context.Context,
	fromBlock, toBlock uint64,
	objectIDList []string,
) ([]sui.ObjectStat, error) {
	var dict map[string]sui.ObjectStat
	err := c.callContext(ctx, &dict, fromBlock, "sui_getObjectsStat", fromBlock, toBlock, objectIDList)
	result := make([]sui.ObjectStat, len(objectIDList))
	for i, id := range objectIDList {
		result[i] = dict[id]
	}
	return result, err
}

func (c *client) GetObjectsStat(
	ctx context.Context,
	fromBlock, toBlock uint64,
	objectIDList []string,
) ([]sui.ObjectStat, error) {
	const maxPageSize = 200
	const maxConcurrency = 20
	return concurrency.TraverseByPage(ctx, maxConcurrency, maxPageSize, objectIDList,
		func(ctx context.Context, page concurrency.Page, ids []string) ([]sui.ObjectStat, error) {
			return c.getObjectsStat(ctx, fromBlock, toBlock, ids)
		},
	)
}

func (c *client) GetObjectVersionHistory(ctx context.Context, objectID string) ([]types.ObjectChangeExtend, error) {
	stat, err := c.GetObjectStat(ctx, 0, math.MaxUint64, objectID)
	if err != nil {
		return nil, err
	}
	if stat.Count == 0 {
		return nil, nil
	}
	filter := sui.ObjectChangeFilter{
		ObjectIDIn: set.New(objectID),
	}
	var changes map[uint64][]types.ObjectChangeExtend
	if changes, err = c.GetObjectChanges(ctx, stat.MinCheckpoint, stat.MaxCheckpoint, filter); err != nil {
		return nil, err
	}
	return utils.MergeArr(utils.GetMapValuesOrderByKey(changes)...), nil
}

func (c *client) GetPackageHistory(ctx context.Context, pkgID string) (history []string, err error) {
	var has bool
	if history, has = c.cachedPackageHistory.Get(pkgID); has {
		return history, nil
	}
	defer func() {
		if err == nil {
			c.cachedPackageHistory.Add(pkgID, history)
		}
	}()
	// step-1: package object is immutable, so only have one version, just use sui_getObject to fetch it
	type getObjectResponse struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
		Data struct {
			PreviousTransaction string `json:"previousTransaction"`
		} `json:"data"`
	}
	var getObjectResp getObjectResponse
	getObjectOpt := types.SuiObjectDataOptions{ShowPreviousTransaction: true}
	if err = c.callContext(ctx, &getObjectResp, 0, "sui_getObject", pkgID, getObjectOpt); err != nil {
		return nil, errors.Wrapf(err, "get package object %s failed", pkgID)
	} else if getObjectResp.Error.Code != "" {
		return nil, errors.Errorf("get package object %s failed: %s", pkgID, getObjectResp.Error.Code)
	}
	// step-2: the tx getObjectResp.Data.PreviousTransaction is the creating tx of the package,
	//         which contains the creating or update record of the upgrade cap object,
	//         use sui_getTransactionBlock to fetch the object changes in this tx
	var pkgCreatingTxDigest = types.StrToDigestMust(getObjectResp.Data.PreviousTransaction)
	var pkgCreateTx *types.TransactionResponseV1
	var getTxOpt = map[string]any{"showObjectChanges": true}
	if err = c.callContext(ctx, &pkgCreateTx, 0, "sui_getTransactionBlock", pkgCreatingTxDigest, getTxOpt); err != nil {
		err = errors.Wrapf(err, "get creation trransaction %s for package %s failed", pkgCreatingTxDigest.String(), pkgID)
		return nil, err
	}
	const upgradeCapObjectType = "0x2::package::UpgradeCap"
	var upgradeCapID string
	for _, objectChange := range pkgCreateTx.ObjectChanges {
		if utils.EmptyStringIfNil(utils.NullOrToString(objectChange.ObjectType)) == upgradeCapObjectType {
			upgradeCapID = objectChange.GetObjectID()
			break
		}
	}
	if upgradeCapID == "" {
		return []string{pkgID}, nil
	}
	// step-3: now have the upgrade cap object id, we should find the change history of it,
	//         which contains all upgraded package creating record.
	//         use super node to get all change records of the upgrade cap object,
	//         which contains all package creating tx digest.
	var upgradeCapHistory []types.ObjectChangeExtend
	upgradeCapHistory, err = c.GetObjectVersionHistory(ctx, upgradeCapID)
	if err != nil {
		err = errors.Wrapf(err, "get the first tx of the upgrade cap object %s for package %s failed", upgradeCapID, pkgID)
		return nil, err
	}
	if len(upgradeCapHistory) == 0 {
		err = errors.Errorf("the history of the upgrade cap object %s for package %s is not found", upgradeCapID, pkgID)
		return nil, err
	}
	historyTxDigest := utils.MapSliceNoError(upgradeCapHistory, func(oc types.ObjectChangeExtend) string {
		return oc.TxDigest.String()
	})
	// step-4: now get the object changes of all history tx, all the package id is in it
	var historyTxList []types.TransactionResponseV1
	historyTxList, err = c.MultiGetTransactionBlocks(ctx, historyTxDigest, getTxOpt)
	if err != nil {
		err = errors.Wrapf(err, "get history tx of the upgrade cap object %s for package %s failed", upgradeCapID, pkgID)
		return nil, err
	}
	for _, tx := range historyTxList {
		for _, change := range tx.ObjectChanges {
			if change.Type == types.ObjectChangeTypePublished {
				history = append(history, change.GetObjectID())
			}
		}
	}
	if utils.IndexOf(history, pkgID) < 0 {
		history = append(history, pkgID)
	}
	return history, nil
}

func (c *client) GetObjectCreation(ctx context.Context, objectID string, start uint64) (uint64, bool, error) {
	var creation *sui.ObjectCreation
	err := c.callContext(ctx, &creation, start, "sui_getObjectCreation", objectID)
	if err != nil {
		return 0, false, err
	}
	if creation == nil {
		return 0, false, nil
	}
	return max(creation.Checkpoint, start), true, nil
}

func (c *client) ResetCache(r controller.BlockRange) {
	for _, bn := range c.cachedSimpleBlock.Keys() {
		if r.Contains(bn) {
			c.cachedSimpleBlock.Remove(bn)
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
			"cachedSimpleBlock": c.cachedSimpleBlock.Snapshot(10, controller.GetBlockFullText[SimpleBlock]),
			"cachedPackageHistory": utils.CacheSnapshot(c.cachedPackageHistory, 100, func(history []string) string {
				return strings.Join(history, ",")
			}),
		},
	}
}
