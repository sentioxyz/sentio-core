package sui

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"sentioxyz/sentio-core/chain/move"
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
	GetObjectStat(ctx context.Context, fromBlock, toBlock uint64, objectID string) (sui.ObjectStat, error)
	GetObjectsStat(ctx context.Context, fromBlock, toBlock uint64, objectIDList []string) ([]sui.ObjectStat, error)

	// GetPackageHistory resolves the full package upgrade history via json-rpc; the
	// suigrpc handler path uses GetGrpcPackageHistory, which walks the same history
	// purely over grpc data (no upstream full-node json-rpc).
	GetPackageHistory(ctx context.Context, pkgID string) ([]string, error)
	GetGrpcPackageHistory(ctx context.Context, pkgID string) ([]string, error)
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

// GetPackageHistory resolves the full package upgrade history over json-rpc using
// the shared UpgradeCap version-chain walk (point lookups via
// sui_getObject / sui_tryGetPastObject / sui_getTransactionBlock), instead of the
// previous sui_filterObjectChangesV2 range scan over ClickHouse.
func (c *client) GetPackageHistory(ctx context.Context, pkgID string) (history []string, err error) {
	var has bool
	if history, has = c.cachedPackageHistory.Get(pkgID); has {
		return history, nil
	}
	history, err = resolvePackageHistory(ctx, pkgID, &jsonrpcPackageHistoryLedger{c: c})
	if err == nil {
		c.cachedPackageHistory.Add(pkgID, history)
	}
	return history, err
}

// suiUpgradeCapType matches the UpgradeCap object change in a publish tx. It is a
// move.Type (not a raw string) because grpc reports the full-length address form
// (0x0000…0002::package::UpgradeCap) while json-rpc abbreviates it (0x2::…);
// move.Type comparison normalizes the address so both forms match.
var suiUpgradeCapType = mustBuildMoveType("0x2::package::UpgradeCap")

func mustBuildMoveType(s string) move.Type {
	t, err := move.BuildType(s)
	if err != nil {
		panic(err)
	}
	return t
}

// suiPackageObjectType is the object_type grpc reports for a published Move
// package. Different node versions encode a package publish either with this
// literal type or with an OUTPUT_OBJECT_STATE_PACKAGE_WRITE change, so the walk
// accepts both signals.
const suiPackageObjectType = "package"

// grpcPackageHistoryTxReadMask limits the grpc transaction fetch during package
// history walking to just the fields it needs: the digest plus the effects'
// changed objects (whose object id / type / input version drive the walk).
var grpcPackageHistoryTxReadMask = []string{"digest", "effects.changed_objects"}

// GetGrpcPackageHistory resolves the full package upgrade history using only grpc
// data (no upstream full-node json-rpc), via the shared UpgradeCap version-chain
// walk over grpc point lookups.
func (c *client) GetGrpcPackageHistory(ctx context.Context, pkgID string) (history []string, err error) {
	var has bool
	if history, has = c.cachedPackageHistory.Get(pkgID); has {
		return history, nil
	}
	history, err = resolvePackageHistory(ctx, pkgID, &grpcPackageHistoryLedger{c: c})
	if err == nil {
		c.cachedPackageHistory.Add(pkgID, history)
	}
	return history, err
}

// packageHistoryChange is the transport-neutral view of one object change that the
// package-history walk consults.
type packageHistoryChange struct {
	objectID     string
	isUpgradeCap bool    // the change is the package's 0x2::package::UpgradeCap
	isPublished  bool    // the change publishes a Move package (i.e. a package version)
	prevVersion  *uint64 // the object's version before this tx; nil when created here
}

// packageHistoryLedger provides the point lookups the package-history walk needs,
// backed by either grpc or json-rpc data.
type packageHistoryLedger interface {
	// objectPrevTx returns the digest of the tx that produced the object at the
	// given version (nil version = latest live version).
	objectPrevTx(ctx context.Context, objectID string, version *uint64) (string, error)
	// txChanges returns a transaction's object changes.
	txChanges(ctx context.Context, digest string) ([]packageHistoryChange, error)
}

// resolvePackageHistory resolves the full package upgrade history by walking the
// package's UpgradeCap version chain backwards: each upgrade tx publishes a new
// package version and mutates the cap, and the cap change carries the version it
// had before the tx, so following that version back hops to the previous upgrade
// tx until the cap's creation. It is transport-agnostic (see packageHistoryLedger),
// shared by GetPackageHistory (json-rpc) and GetGrpcPackageHistory (grpc), and
// replaces the old object-change range scan with millisecond point lookups.
func resolvePackageHistory(
	ctx context.Context,
	pkgID string,
	ledger packageHistoryLedger,
) (history []string, err error) {
	// step-1: the package object is immutable, so it has a single version; fetch it
	//         to learn the transaction that created it.
	createTx, err := ledger.objectPrevTx(ctx, pkgID, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "get package object %s failed", pkgID)
	}
	if createTx == "" {
		return nil, errors.Errorf("package object %s has no previous transaction", pkgID)
	}

	// step-2: the creating tx publishes the package together with its UpgradeCap;
	//         find the upgrade cap object id from that tx's object changes.
	createChanges, err := ledger.txChanges(ctx, createTx)
	if err != nil {
		return nil, errors.Wrapf(err, "get creation transaction %s for package %s failed", createTx, pkgID)
	}
	var upgradeCapID string
	for _, ch := range createChanges {
		if ch.isUpgradeCap {
			upgradeCapID = ch.objectID
			break
		}
	}
	if upgradeCapID == "" {
		return []string{pkgID}, nil
	}

	// step-3: walk the upgrade cap's version chain backwards. Start from the cap's
	//         latest version so the walk is independent of which version pkgID is;
	//         each upgrade tx publishes one package version, so collecting the
	//         published change in every walked tx yields the full package history.
	txDigest, err := ledger.objectPrevTx(ctx, upgradeCapID, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "get upgrade cap object %s for package %s failed", upgradeCapID, pkgID)
	}
	seenTx := set.New[string]()
	for txDigest != "" && !seenTx.Contains(txDigest) {
		seenTx.Add(txDigest)
		changes, terr := ledger.txChanges(ctx, txDigest)
		if terr != nil {
			return nil, errors.Wrapf(terr, "get upgrade history tx %s for package %s failed", txDigest, pkgID)
		}
		var capChange *packageHistoryChange
		for i := range changes {
			ch := &changes[i]
			if ch.isPublished {
				history = append(history, ch.objectID)
			}
			if ch.objectID == upgradeCapID {
				capChange = ch
			}
		}
		if capChange == nil {
			return nil, errors.Errorf("upgrade cap %s not found in its change tx %s", upgradeCapID, txDigest)
		}
		// the tx that created the cap has no prior version: the walk is done.
		if capChange.prevVersion == nil {
			break
		}
		// hop to the tx that produced the cap's previous version.
		if txDigest, err = ledger.objectPrevTx(ctx, upgradeCapID, capChange.prevVersion); err != nil {
			return nil, errors.Wrapf(err, "get upgrade cap %s at version %d failed", upgradeCapID, *capChange.prevVersion)
		}
	}

	if utils.IndexOf(history, pkgID) < 0 {
		history = append(history, pkgID)
	}
	return history, nil
}

// grpcPackageHistoryLedger backs the walk with grpc point lookups.
type grpcPackageHistoryLedger struct{ c *client }

func (l *grpcPackageHistoryLedger) objectPrevTx(ctx context.Context, objectID string, version *uint64) (string, error) {
	obj, err := l.c.getGrpcObject(ctx, objectID, version)
	if err != nil {
		return "", err
	}
	return obj.GetPreviousTransaction(), nil
}

func (l *grpcPackageHistoryLedger) txChanges(ctx context.Context, digest string) ([]packageHistoryChange, error) {
	tx, err := l.c.getGrpcTransactionByDigest(ctx, digest)
	if err != nil {
		return nil, err
	}
	cos := tx.GetEffects().GetChangedObjects()
	changes := make([]packageHistoryChange, 0, len(cos))
	for _, co := range cos {
		ch := packageHistoryChange{
			objectID:     co.GetObjectId(),
			isUpgradeCap: suiUpgradeCapType.IncludeTypeString(co.ObjectType),
			// some node versions encode a package publish as a "package"-typed
			// OBJECT_WRITE rather than a PACKAGE_WRITE change; accept either.
			isPublished: co.GetObjectType() == suiPackageObjectType || sui.GetChangeType(co) == types.ObjectChangeTypePublished,
		}
		if co.GetInputState() != rpcv2.ChangedObject_INPUT_OBJECT_STATE_DOES_NOT_EXIST {
			v := co.GetInputVersion()
			ch.prevVersion = &v
		}
		changes = append(changes, ch)
	}
	return changes, nil
}

// jsonrpcPackageHistoryLedger backs the walk with json-rpc point lookups
// (sui_getObject / sui_tryGetPastObject / sui_getTransactionBlock), avoiding the
// sui_filterObjectChangesV2 ClickHouse range scan.
type jsonrpcPackageHistoryLedger struct{ c *client }

func (l *jsonrpcPackageHistoryLedger) objectPrevTx(ctx context.Context, objectID string, version *uint64) (string, error) {
	return l.c.getObjectPrevTx(ctx, objectID, version)
}

func (l *jsonrpcPackageHistoryLedger) txChanges(ctx context.Context, digest string) ([]packageHistoryChange, error) {
	tx, err := l.c.getTransactionBlock(ctx, digest)
	if err != nil {
		return nil, err
	}
	changes := make([]packageHistoryChange, 0, len(tx.ObjectChanges))
	for i := range tx.ObjectChanges {
		co := &tx.ObjectChanges[i]
		objType := utils.EmptyStringIfNil(utils.NullOrToString(co.ObjectType))
		ch := packageHistoryChange{
			objectID:     co.GetObjectID(),
			isUpgradeCap: suiUpgradeCapType.IncludeTypeString(&objType),
			isPublished:  co.Type == types.ObjectChangeTypePublished,
		}
		if co.PreviousVersion != nil {
			v := co.PreviousVersion.Uint64()
			ch.prevVersion = &v
		}
		changes = append(changes, ch)
	}
	return changes, nil
}

// getObjectPrevTx returns the digest of the tx that produced the object at the
// given version: latest via sui_getObject, a historical version via
// sui_tryGetPastObject.
func (c *client) getObjectPrevTx(ctx context.Context, objectID string, version *uint64) (string, error) {
	opt := types.SuiObjectDataOptions{ShowPreviousTransaction: true, ShowType: true}
	if version == nil {
		var resp struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
			Data struct {
				PreviousTransaction string `json:"previousTransaction"`
			} `json:"data"`
		}
		if err := c.callContext(ctx, &resp, 0, "sui_getObject", objectID, opt); err != nil {
			return "", errors.Wrapf(err, "get object %s failed", objectID)
		}
		if resp.Error.Code != "" {
			return "", errors.Errorf("get object %s failed: %s", objectID, resp.Error.Code)
		}
		return resp.Data.PreviousTransaction, nil
	}
	var resp types.SuiPastObjectResponse
	if err := c.callContext(ctx, &resp, 0, "sui_tryGetPastObject", objectID, *version, opt); err != nil {
		return "", errors.Wrapf(err, "get past object %s@%d failed", objectID, *version)
	}
	if resp.Status != types.SuiPastObjectStatusVersionFound {
		return "", errors.Errorf("get past object %s@%d failed: %s", objectID, *version, resp.Status)
	}
	var detail struct {
		PreviousTransaction string `json:"previousTransaction"`
	}
	if err := json.Unmarshal(resp.Details, &detail); err != nil {
		return "", errors.Wrapf(err, "decode past object %s@%d", objectID, *version)
	}
	return detail.PreviousTransaction, nil
}

// getTransactionBlock fetches a transaction with its object changes over json-rpc.
func (c *client) getTransactionBlock(ctx context.Context, digest string) (*types.TransactionResponseV1, error) {
	var tx *types.TransactionResponseV1
	opt := map[string]any{"showObjectChanges": true}
	if err := c.callContext(ctx, &tx, 0, "sui_getTransactionBlock", digest, opt); err != nil {
		return nil, errors.Wrapf(err, "get transaction %s failed", digest)
	}
	if tx == nil {
		return nil, errors.Errorf("transaction %s not found", digest)
	}
	return tx, nil
}

// getGrpcObject fetches a single object (latest version when version is nil) over
// grpc and returns its proto, erroring on a not-found / error result.
func (c *client) getGrpcObject(ctx context.Context, objectID string, version *uint64) (*rpcv2.Object, error) {
	req := &rpcv2.GetObjectRequest{ObjectId: &objectID, Version: version}
	results, err := c.GetGrpcObjects(ctx, []*rpcv2.GetObjectRequest{req}, 1, 1)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 || results[0] == nil {
		return nil, errors.Errorf("object %s not found", objectID)
	}
	if st := results[0].GetError(); st != nil {
		return nil, errors.Errorf("get object %s failed: %s", objectID, st.GetMessage())
	}
	obj := results[0].GetObject()
	if obj == nil {
		return nil, errors.Errorf("object %s not found", objectID)
	}
	return obj, nil
}

// getGrpcTransactionByDigest fetches a single transaction over grpc.
func (c *client) getGrpcTransactionByDigest(ctx context.Context, digest string) (*rpcv2.ExecutedTransaction, error) {
	txs, err := c.getGrpcTransactionsByDigest(ctx, []string{digest})
	if err != nil {
		return nil, err
	}
	return txs[0], nil
}

// getGrpcTransactionsByDigest fetches transactions by digest over grpc, unwrapping
// the JSON-RPC-safe result and surfacing per-transaction errors.
func (c *client) getGrpcTransactionsByDigest(ctx context.Context, digests []string) ([]*rpcv2.ExecutedTransaction, error) {
	const concurrency, batchSize = 10, 50
	var wrapped []*sui.GrpcTransactionResult
	if err := c.callContext(ctx, &wrapped, 0, "sui_getGrpcTransactionsByDigest",
		digests, grpcPackageHistoryTxReadMask, concurrency, batchSize); err != nil {
		return nil, err
	}
	results := sui.UnwrapGrpcTransactionResults(wrapped)
	if len(results) != len(digests) {
		return nil, errors.Errorf("get %d transactions but got %d", len(digests), len(results))
	}
	txs := make([]*rpcv2.ExecutedTransaction, len(results))
	for i, r := range results {
		if r == nil {
			return nil, errors.Errorf("get transaction %s failed: empty result", digests[i])
		}
		if st := r.GetError(); st != nil {
			return nil, errors.Errorf("get transaction %s failed: %s", digests[i], st.GetMessage())
		}
		tx := r.GetTransaction()
		if tx == nil {
			return nil, errors.Errorf("get transaction %s failed: no transaction in result", digests[i])
		}
		txs[i] = tx
	}
	return txs, nil
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
