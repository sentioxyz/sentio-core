package sol

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/gagliardetto/solana-go"
	"github.com/pkg/errors"

	solcore "sentioxyz/sentio-core/chain/sol"
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
	// GetBlock returns the block header (no transactions). The result may be skipped.
	GetBlock(ctx context.Context, blockNumber uint64) (Block, error)

	// GetBlocksByInterval returns the first non-skipped block of each window in [from, to], for the
	// interval handler. The super node caps the result and errors when exceeded.
	//
	// Signatures: blocks within the ClickHouse range carry their transaction signatures; blocks
	// served from the BigQuery archival tier (slots below the ClickHouse range) carry headers only,
	// with NO signatures — a deliberate BigQuery cost optimization (see the archival store). The interval
	// handler only uses block headers (slot/hash/time), so this is fine; callers must not rely on
	// per-block signatures from this method.
	GetBlocksByInterval(
		ctx context.Context,
		from, to uint64,
		window solcore.IntervalWindow,
	) ([]Block, error)
	// FindTransactions returns, grouped by block, the full transactions in [from, to] invoking any
	// of the given programs, for the instruction handler. The super node caps the result and errors
	// when exceeded (except for a single-block range).
	FindTransactions(
		ctx context.Context,
		from, to uint64,
		programs []solana.PublicKey,
	) ([]solcore.BlockTransactions, error)
	GetContractStartBlock(ctx context.Context, address solana.PublicKey, start, latest uint64) (uint64, bool, error)
	// GetPreviousUnskippedBlock returns the nearest non-skipped block with slot < beforeSlot (pass
	// slot+1 to include the slot itself), used to learn the chain time around a possibly-skipped slot.
	GetPreviousUnskippedBlock(ctx context.Context, beforeSlot uint64) (solcore.PreviousUnskippedBlock, error)

	ResetCache(r controller.BlockRange)
	Snapshot() any
}

// client talks to the Solana super node over JSON-RPC. The super node answers the sol_* methods
// from its latest-slot cache and ClickHouse, so the driver no longer talks to a node directly.
type supernodeClient struct {
	endpoint            string
	watchLatestInterval time.Duration

	resMgr *concurrency.ResourceManager
	stat   *data.CallStatistics

	cli *ethrpc.Client

	cachedHeaders *data.BlockCache[Block]

	savedLatestBlockNumber atomic.Uint64
}

// NewClient selects the data client for a sol chain. It uses the native Solana RPC client when the
// processor is old (DriverVersion < 2) or the endpoint is not a super node (no sol_getLatestHeader),
// and the super-node client otherwise. Only sol_mainnet runs a super node (ClickHouse + BigQuery);
// other sol chains fall back to native RPC.
func NewClient(
	ctx context.Context,
	endpoint string,
	maxConcurrency int,
	firstBlockNumber int64,
	watchLatestInterval time.Duration,
	driverVersion int32,
) (Client, error) {
	_, logger := log.FromContext(ctx)
	if driverVersion < 2 {
		logger.Infof("sol: driver version %d < 2, using native Solana RPC at %s", driverVersion, endpoint)
		return newNativeClient(endpoint, maxConcurrency, firstBlockNumber, watchLatestInterval)
	}
	supported, err := endpointSupportsSuperNode(ctx, endpoint)
	if err != nil {
		// The probe kept hitting transient/HTTP/timeout errors and never got a definitive answer.
		// Surface the retryable NewClient error so the controller restarts the pod and tries again,
		// rather than failing permanently (NeverRetry) on what may be a temporary outage.
		return nil, err
	}
	if !supported {
		logger.Infof("sol: endpoint %s does not support sol_getLatestHeader, using native Solana RPC", endpoint)
		return newNativeClient(endpoint, maxConcurrency, firstBlockNumber, watchLatestInterval)
	}
	// firstBlockNumber is intentionally not passed: the super node is backed by ClickHouse + BigQuery,
	// so it serves the full available history and bounds the start range itself (range store +
	// BigQuery retention floor). The driver imposes no client-side first block.
	return newSupernodeClient(ctx, endpoint, maxConcurrency, watchLatestInterval)
}

func newSupernodeClient(
	ctx context.Context,
	endpoint string,
	maxConcurrency int,
	watchLatestInterval time.Duration,
) (Client, error) {
	cli := &supernodeClient{
		endpoint:            endpoint,
		watchLatestInterval: watchLatestInterval,
		resMgr:              concurrency.NewResourceManager(maxConcurrency),
		stat:                data.NewDefaultCallStatistics(),
	}
	var err error
	if cli.cli, err = ethrpc.DialOptions(ctx, endpoint, ethrpc.WithHTTPClient(https.DefaultClient)); err != nil {
		return nil, errors.Wrapf(err, "dial to %s failed", endpoint)
	}
	cli.cachedHeaders, _ = data.NewBlockCache[Block](100000)
	return cli, nil
}

// superNodeProbeCache memoizes endpointSupportsSuperNode by endpoint, so each endpoint is probed at
// most once across controllers.
var superNodeProbeCache sync.Map // endpoint string -> bool

// superNodeProbeMaxAttempts bounds how many times endpointSupportsSuperNode retries a transient
// (HTTP/timeout/dial) probe failure before giving up with a retryable NewClient error.
const superNodeProbeMaxAttempts = 20

// endpointSupportsSuperNode probes whether the endpoint implements the super-node sol_getLatestHeader
// method (gt=0 returns immediately when supported), returning:
//
//   - (true, nil)  — the endpoint answers sol_getLatestHeader, so it is a super node.
//   - (false, nil) — the endpoint returns -32601 / "method not found", so it is a plain Solana RPC
//     endpoint and the caller should fall back to native RPC.
//   - (false, err) — the probe kept failing with transient errors (HTTP/timeout/dial) across all
//     attempts; err is a *data.NewClientRetryableError so the caller can restart the pod and retry,
//     instead of incorrectly assuming either kind of endpoint.
//
// Transient failures are retried with exponential backoff up to superNodeProbeMaxAttempts times, each
// logged. Only definitive answers (super node / not a super node) are cached per endpoint.
func endpointSupportsSuperNode(ctx context.Context, endpoint string) (bool, error) {
	if v, ok := superNodeProbeCache.Load(endpoint); ok {
		return v.(bool), nil
	}
	_, logger := log.FromContext(ctx)

	var supported bool
	attempt := 0
	operation := func() error {
		attempt++
		probe, err := ethrpc.DialOptions(ctx, endpoint, ethrpc.WithHTTPClient(https.DefaultClient))
		if err != nil {
			logger.Warnf("sol: probe dial to %s failed (attempt %d/%d): %v; will retry",
				endpoint, attempt, superNodeProbeMaxAttempts, err)
			return err
		}
		defer probe.Close()
		probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		var blk Block
		err = probe.CallContext(probeCtx, &blk, "sol_getLatestHeader", uint64(0))
		if err == nil {
			supported = true
			return nil
		}
		// A -32601 / "method not found" is a definitive negative: the endpoint is a plain Solana RPC
		// node. Stop retrying by returning nil with supported left false.
		var rpcErr ethrpc.Error
		if errors.As(err, &rpcErr) && rpcErr.ErrorCode() == -32601 {
			supported = false
			return nil
		}
		if msg := strings.ToLower(err.Error()); strings.Contains(msg, "method not found") || strings.Contains(msg, "does not exist") {
			supported = false
			return nil
		}
		// Anything else (HTTP error, timeout, connection reset, ...) is transient: retry.
		logger.Warnf("sol: probe sol_getLatestHeader at %s failed (attempt %d/%d): %v; will retry",
			endpoint, attempt, superNodeProbeMaxAttempts, err)
		return err
	}

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 200 * time.Millisecond
	bo.MaxInterval = 10 * time.Second
	// WithMaxRetries(n) allows n retries after the initial attempt → superNodeProbeMaxAttempts total.
	retry := backoff.WithContext(backoff.WithMaxRetries(bo, superNodeProbeMaxAttempts-1), ctx)
	if err := backoff.Retry(operation, retry); err != nil {
		return false, data.NewClientRetryable(
			fmt.Sprintf("sol: probing super-node support at %s kept failing after %d attempts", endpoint, superNodeProbeMaxAttempts),
			err,
		)
	}
	superNodeProbeCache.Store(endpoint, supported)
	return supported, nil
}

func (c *supernodeClient) callContext(ctx context.Context, result any, priority uint64, method string, args ...any) error {
	startAt := time.Now()
	release, err := c.resMgr.Apply(ctx, int64(priority), 1, time.Minute, func(waited time.Duration) {
		_, logger := log.FromContext(ctx, "priority", priority, "args", utils.MustJSONMarshal(args))
		logger.Warnf("call method %s waited %s", method, waited.String())
	})
	if err != nil {
		return err // always be context.Canceled
	}
	defer release()
	callStartAt := time.Now()
	err = c.cli.CallContext(ctx, &result, method, args...)
	if err != nil {
		err = errors.Wrapf(err, "call method %s with args %s failed", method, utils.MustJSONMarshal(args))
	}
	c.stat.Called(method, args, err, startAt, callStartAt)
	return err
}

func (c *supernodeClient) fetchBlock(ctx context.Context, blockNumber uint64) (Block, error) {
	var blk Block
	if err := c.callContext(ctx, &blk, blockNumber, "sol_getBlock", blockNumber); err != nil {
		return Block{}, err
	}
	return blk, nil
}

func (c *supernodeClient) getBlock(ctx context.Context, blockNumber uint64) (Block, error) {
	blk, err := c.fetchBlock(ctx, blockNumber)
	if err == nil {
		c.cachedHeaders.Add(blockNumber, blk)
	}
	return blk, err
}

// getLatestHeader long-polls the super node for the latest non-skipped block header with slot > gt.
// The super node blocks until such a block exists, so the driver never has to poll, and the
// returned header is guaranteed non-skipped (BlockTime set).
func (c *supernodeClient) getLatestHeader(ctx context.Context, gt uint64) (Block, error) {
	var blk Block
	if err := c.callContext(ctx, &blk, 0, "sol_getLatestHeader", gt); err != nil {
		return Block{}, err
	}
	c.savedLatestBlockNumber.Store(blk.Slot)
	return blk, nil
}

// GetLatest returns the latest non-skipped header. first is 0: the super node serves the full
// available history (ClickHouse + BigQuery) and bounds the start range itself, so the driver imposes
// no client-side first block (the processor's own start / GetContractStartBlock decide it).
func (c *supernodeClient) GetLatest(ctx context.Context) (latest controller.BlockHeader, first uint64, err error) {
	blk, err := c.getLatestHeader(ctx, 0)
	if err != nil {
		return nil, 0, err
	}
	return blk, 0, nil
}

func (c *supernodeClient) Subscribe(
	ctx context.Context,
	from controller.BlockHeader,
	callback func(latest controller.BlockHeader, broken error),
) {
	data.SubscribeUsingWaiting(
		ctx,
		c.watchLatestInterval,
		from,
		func(ctx context.Context, blockNumberGt uint64) (latest controller.BlockHeader, broken, err error) {
			blk, err := c.getLatestHeader(ctx, blockNumberGt)
			if err != nil {
				return nil, nil, err
			}
			return blk, nil, nil
		},
		callback)
}

func (c *supernodeClient) GetHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (controller.BlockHeader, error) {
	return c.getBlock(ctx, blockNumber)
}

func (c *supernodeClient) GetBlock(ctx context.Context, blockNumber uint64) (Block, error) {
	// Cache + singleflight: concurrent fetchers asking for the same block share one sol_getBlock.
	return c.cachedHeaders.GetOrFetch(blockNumber, func() (Block, error) {
		return c.fetchBlock(ctx, blockNumber)
	})
}

func (c *supernodeClient) GetBlocksByInterval(
	ctx context.Context,
	from, to uint64,
	window solcore.IntervalWindow,
) ([]Block, error) {
	var blocks []Block
	param := solcore.GetBlocksByIntervalParam{From: from, To: to, Window: window}
	if err := c.callContext(ctx, &blocks, to, "sol_getBlocksByInterval", param); err != nil {
		return nil, errors.Wrapf(err, "get interval blocks in [%d,%d] failed", from, to)
	}
	return blocks, nil
}

func (c *supernodeClient) FindTransactions(
	ctx context.Context,
	from, to uint64,
	programs []solana.PublicKey,
) ([]solcore.BlockTransactions, error) {
	var result []solcore.BlockTransactions
	param := solcore.FindTransactionsParam{From: from, To: to, ProgramIDs: programs}
	if err := c.callContext(ctx, &result, to, "sol_findTransactions", param); err != nil {
		return nil, errors.Wrapf(err, "find transactions in [%d,%d] for %d programs failed", from, to, len(programs))
	}
	return result, nil
}

// GetContractStartBlock asks the super node for the contract's earliest appearance block, then maps
// it to [start, latest]: an appearance before start clamps to start; an appearance after latest (or
// no appearance) is treated as not yet in range.
func (c *supernodeClient) GetContractStartBlock(
	ctx context.Context,
	address solana.PublicKey,
	start, latest uint64,
) (uint64, bool, error) {
	var result solcore.GetContractStartBlockResult
	if err := c.callContext(ctx, &result, 0, "sol_getContractStartBlock", address); err != nil {
		return 0, false, err
	}
	if !result.Found || result.Slot > latest {
		return 0, false, nil
	}
	if result.Slot < start {
		return start, true, nil
	}
	return result.Slot, true, nil
}

func (c *supernodeClient) GetPreviousUnskippedBlock(
	ctx context.Context,
	beforeSlot uint64,
) (solcore.PreviousUnskippedBlock, error) {
	var result solcore.PreviousUnskippedBlock
	if err := c.callContext(ctx, &result, beforeSlot, "sol_getPreviousUnskippedBlock", beforeSlot); err != nil {
		return solcore.PreviousUnskippedBlock{}, err
	}
	return result, nil
}

func (c *supernodeClient) ResetCache(r controller.BlockRange) {
	for _, bn := range c.cachedHeaders.Keys() {
		if r.Contains(bn) {
			c.cachedHeaders.Remove(bn)
		}
	}
}

func (c *supernodeClient) Snapshot() any {
	return map[string]any{
		"config": map[string]any{
			"endpoint":            c.endpoint,
			"watchLatestInterval": c.watchLatestInterval.String(),
		},
		"savedLatestBlockNumber": c.savedLatestBlockNumber.Load(),
		"resourceManager":        c.resMgr.Snapshot(),
		"statistics":             c.stat.Snapshot(),
		"cache": map[string]any{
			"cachedHeaders": c.cachedHeaders.Snapshot(10, func(block Block) string {
				if block.Skipped() {
					return fmt.Sprintf("%d/<skipped>", block.Slot)
				}
				return controller.GetBlockFullText(block)
			}),
		},
	}
}
