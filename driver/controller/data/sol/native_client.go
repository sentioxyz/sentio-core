package sol

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/pkg/errors"

	solcore "sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
)

// nativeClient implements Client against a plain Solana JSON-RPC endpoint (no super node). It is used
// for sol chains that have no super node (anything but sol_mainnet) and for old processors
// (DriverVersion < 2). It reproduces the super-node sol_* capabilities over native primitives
// (getSlot / getBlock / getBlocks / getSignaturesForAddress), porting the pre-super-node driver
// (commit 87d8d1455). These calls are far more RPC-heavy than the super node — acceptable for the
// low-volume chains / legacy processors that need this path.
type nativeClient struct {
	endpoint            string
	firstBlockNumber    int64
	watchLatestInterval time.Duration
	getLatestTimeout    time.Duration

	resMgr *concurrency.ResourceManager
	stat   *data.CallStatistics

	cli *rpc.Client

	cachedHeaders *data.BlockCache[Block]

	savedLatestBlockNumber atomic.Uint64
	savedFirstBlockNumber  atomic.Uint64
}

func newNativeClient(
	endpoint string,
	maxConcurrency int,
	firstBlockNumber int64,
	watchLatestInterval time.Duration,
) (Client, error) {
	c := &nativeClient{
		endpoint:            endpoint,
		firstBlockNumber:    firstBlockNumber,
		watchLatestInterval: watchLatestInterval,
		getLatestTimeout:    30 * time.Second,
		resMgr:              concurrency.NewResourceManager(maxConcurrency),
		stat:                data.NewDefaultCallStatistics(),
		cli:                 rpc.New(endpoint),
	}
	c.cachedHeaders, _ = data.NewBlockCache[Block](100000)
	return c, nil
}

var slotSkippedErrorMatcher = regexp.MustCompile(`slot.*was skipped`)

func isSlotSkippedError(err error) bool {
	return err != nil && slotSkippedErrorMatcher.FindString(strings.ToLower(err.Error())) != ""
}

// parsedBlockResult is the subset of getBlock's jsonParsed/full response we need to assemble
// BlockTransactions (header + ordered transactions).
type parsedBlockResult struct {
	Blockhash         solana.Hash                 `json:"blockhash"`
	PreviousBlockhash solana.Hash                 `json:"previousBlockhash"`
	BlockTime         *solana.UnixTimeSeconds     `json:"blockTime"`
	Transactions      []parsedTransactionWithMeta `json:"transactions"`
}

type parsedTransactionWithMeta struct {
	Transaction *rpc.ParsedTransaction     `json:"transaction"`
	Meta        *rpc.ParsedTransactionMeta `json:"meta"`
}

// callContext applies the concurrency token + statistics around a native Solana RPC call, dispatched
// by an internal method name (kept sol_*-style for statistics continuity with the super-node client).
func (c *nativeClient) callContext(ctx context.Context, result any, priority uint64, method string, args ...any) error {
	startAt := time.Now()
	release, err := c.resMgr.Apply(ctx, int64(priority), 1, time.Minute, func(waited time.Duration) {
		_, logger := log.FromContext(ctx, "priority", priority, "args", utils.MustJSONMarshal(args))
		logger.Warnf("call method %s waited %s", method, waited.String())
	})
	if err != nil {
		return err // always context.Canceled
	}
	defer release()
	callStartAt := time.Now()
	switch method {
	case "sol_getLatestBlockNumber":
		r := result.(*uint64)
		*r, err = c.cli.GetSlot(ctx, rpc.CommitmentFinalized)
	case "sol_getBlock":
		opt := rpc.GetBlockOpts{TransactionDetails: rpc.TransactionDetailsSignatures}
		r := result.(*Block)
		r.Slot = args[0].(uint64)
		r.GetBlockResult, err = c.cli.GetBlockWithOpts(ctx, r.Slot, &opt)
		if err != nil && isSlotSkippedError(err) {
			r.GetBlockResult, err = nil, nil // skipped slot
		}
	case "sol_getBlockFull":
		// solana-go's GetBlockWithOpts rejects the jsonParsed encoding, so issue the raw getBlock
		// call to fetch every transaction's parsed detail (and the header) in one request.
		obj := rpc.M{
			"encoding":                       solana.EncodingJSONParsed,
			"transactionDetails":             rpc.TransactionDetailsFull,
			"maxSupportedTransactionVersion": uint64(0),
			"rewards":                        false,
		}
		r := result.(*parsedBlockResult)
		err = c.cli.RPCCallForInto(ctx, r, "getBlock", []any{args[0].(uint64), obj})
		if err != nil && isSlotSkippedError(err) {
			*r, err = parsedBlockResult{}, nil // skipped slot
		}
	case "sol_getBlocks":
		end := args[1].(uint64)
		r := result.(*rpc.BlocksResult)
		*r, err = c.cli.GetBlocks(ctx, args[0].(uint64), &end, rpc.CommitmentFinalized)
	case "sol_getSignaturesForAddress":
		limit := args[3].(int)
		opt := rpc.GetSignaturesForAddressOpts{
			Until:  args[1].(solana.Signature),
			Before: args[2].(solana.Signature),
			Limit:  &limit,
		}
		r := result.(*[]*rpc.TransactionSignature)
		*r, err = c.cli.GetSignaturesForAddressWithOpts(ctx, args[0].(solana.PublicKey), &opt)
	default:
		panic(errors.Errorf("unsupported method %q", method))
	}
	if err != nil {
		err = errors.Wrapf(err, "call method %s with args %s failed", method, utils.MustJSONMarshal(args))
	}
	c.stat.Called(method, args, err, startAt, callStartAt)
	return err
}

func (c *nativeClient) getLatestBlockNumber(ctx context.Context) (uint64, error) {
	var latest uint64
	if err := c.callContext(ctx, &latest, 0, "sol_getLatestBlockNumber"); err != nil {
		return 0, err
	}
	c.savedLatestBlockNumber.Store(latest)
	c.savedFirstBlockNumber.CompareAndSwap(0, data.GetFirst(c.firstBlockNumber, latest))
	return latest, nil
}

func (c *nativeClient) fetchBlock(ctx context.Context, blockNumber uint64) (Block, error) {
	var blk Block
	if err := c.callContext(ctx, &blk, blockNumber, "sol_getBlock", blockNumber); err != nil {
		return Block{}, err
	}
	return blk, nil
}

func (c *nativeClient) getBlock(ctx context.Context, blockNumber uint64) (Block, error) {
	blk, err := c.fetchBlock(ctx, blockNumber)
	if err == nil {
		c.cachedHeaders.Add(blockNumber, blk)
	}
	return blk, err
}

func (c *nativeClient) getFullBlock(ctx context.Context, blockNumber uint64) (parsedBlockResult, error) {
	var blk parsedBlockResult
	if err := c.callContext(ctx, &blk, blockNumber, "sol_getBlockFull", blockNumber); err != nil {
		return parsedBlockResult{}, err
	}
	return blk, nil
}

// getBlocks returns the existing (non-skipped) slots in [start, end] in ascending order.
func (c *nativeClient) getBlocks(ctx context.Context, start, end uint64) ([]uint64, error) {
	var res rpc.BlocksResult
	if err := c.callContext(ctx, &res, end, "sol_getBlocks", start, end); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *nativeClient) GetLatest(ctx context.Context) (latest controller.BlockHeader, first uint64, err error) {
	latestBlockNumber, err := c.getLatestBlockNumber(ctx)
	if err != nil {
		return nil, 0, err
	}
	// The finalized slot is never skipped, but its block data may lag a moment; retry with exponential
	// backoff (cancelled with ctx).
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 200 * time.Millisecond
	var blk Block
	if err = backoff.Retry(func() error {
		blk, err = c.getBlock(ctx, latestBlockNumber)
		return err
	}, backoff.WithContext(backoff.WithMaxRetries(bo, 10), ctx)); err != nil {
		return nil, 0, err
	}
	return blk, c.savedFirstBlockNumber.Load(), nil
}

func (c *nativeClient) Subscribe(
	ctx context.Context,
	from controller.BlockHeader,
	callback func(latest controller.BlockHeader, broken error),
) {
	data.SubscribeUsingPolling(
		ctx,
		c.watchLatestInterval,
		c.getLatestTimeout,
		from,
		func(ctx context.Context) (controller.BlockHeader, error) {
			h, _, err := c.GetLatest(ctx)
			return h, err
		},
		callback)
}

func (c *nativeClient) GetHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (controller.BlockHeader, error) {
	return c.getBlock(ctx, blockNumber)
}

func (c *nativeClient) GetBlock(ctx context.Context, blockNumber uint64) (Block, error) {
	// Cache + singleflight: concurrent fetchers asking for the same block share one sol_getBlock.
	return c.cachedHeaders.GetOrFetch(blockNumber, func() (Block, error) {
		return c.fetchBlock(ctx, blockNumber)
	})
}

// timeGetter returns blockNumber's block time, walking back over skipped slots.
func (c *nativeClient) timeGetter(ctx context.Context, blockNumber uint64) (time.Time, error) {
	for n := blockNumber; ; n-- {
		h, err := c.GetBlock(ctx, n)
		if err != nil {
			return time.Time{}, err
		}
		if !h.Skipped() {
			return h.GetBlockTime(), nil
		}
		if n == 0 {
			return time.Time{}, nil // all slots through 0 skipped
		}
	}
}

func windowToConfig(w solcore.IntervalWindow) data.IntervalConfig {
	if w.IsBlockWindow() {
		return data.IntervalConfig{BlockInterval: &data.BlockInterval{Backfill: w.BlockWindow, Watching: w.BlockWindow}}
	}
	return data.IntervalConfig{TimeInterval: &data.TimeInterval{Backfill: w.TimeWindow, Watching: w.TimeWindow}}
}

func (c *nativeClient) GetBlocksByInterval(
	ctx context.Context,
	from, to uint64,
	window solcore.IntervalWindow,
) ([]Block, error) {
	latest, _, err := c.GetLatest(ctx)
	if err != nil {
		return nil, err
	}
	req := data.IntervalRequirement{
		BlockRange:     controller.BlockRange{StartBlock: from, EndBlock: &to},
		IntervalConfig: windowToConfig(window),
	}
	// backfill==watching window ⇒ QueryInterval needs no end-time lookup; timeGetter only matters for
	// time windows.
	bns, err := data.QueryInterval(ctx, from, to, c.savedFirstBlockNumber.Load(), latest, req, c.timeGetter)
	if err != nil {
		return nil, errors.Wrapf(err, "query interval blocks in [%d,%d] failed", from, to)
	}
	blocks := make([]Block, 0, len(bns))
	for _, bn := range bns {
		blk, err := c.GetBlock(ctx, bn)
		if err != nil {
			return nil, err
		}
		if blk.Skipped() {
			continue
		}
		blocks = append(blocks, blk)
	}
	return blocks, nil
}

const findSignaturesPageSize = 1000

// findProgramSignatures returns the signatures of transactions in [fromBlock, toBlock] referencing
// address, using getSignaturesForAddress paginated by the surrounding non-skipped blocks' border
// signatures (ported from the pre-super-node driver). Result is in descending slot order.
func (c *nativeClient) findProgramSignatures(
	ctx context.Context,
	fromBlock, toBlock uint64,
	address solana.PublicKey,
) (result []*rpc.TransactionSignature, err error) {
	latest := c.savedLatestBlockNumber.Load()
	first := c.savedFirstBlockNumber.Load()
	var fromTxSig, toTxSig solana.Signature

	// fromTxSig = last signature of the nearest non-skipped block below fromBlock (the `until` bound).
	for n := fromBlock - 1; fromBlock > 0 && n >= first; n-- {
		blk, gerr := c.GetBlock(ctx, n)
		if gerr != nil {
			return nil, gerr
		}
		if !blk.Skipped() && len(blk.Signatures) > 0 {
			fromTxSig = blk.Signatures[len(blk.Signatures)-1]
			break
		}
		if n == 0 {
			break
		}
	}
	// toTxSig = first signature of the nearest non-skipped block above toBlock (the `before` bound).
	for n := toBlock + 1; n <= latest; n++ {
		blk, gerr := c.GetBlock(ctx, n)
		if gerr != nil {
			return nil, gerr
		}
		if !blk.Skipped() && len(blk.Signatures) > 0 {
			toTxSig = blk.Signatures[0]
			break
		}
	}

	limit := findSignaturesPageSize
	page, err := c.getSignaturesForAddress(ctx, address, fromTxSig, toTxSig, limit)
	if err != nil {
		return nil, errors.Wrapf(err, "get signatures for %s in [%d,%d] failed", address, fromBlock, toBlock)
	}
	// page is DESC. fromTxSig/toTxSig may be empty (open borders), so clip to the block range.
	var finished bool
	for _, sig := range page {
		if sig.Slot > toBlock {
			continue
		}
		if sig.Slot < fromBlock {
			finished = true
			continue
		}
		result = append(result, sig)
	}
	if len(page) >= limit && !finished {
		return nil, errors.Errorf("too many signatures for %s in [%d,%d] (page limit %d)", address, fromBlock, toBlock, limit)
	}
	return result, nil
}

func (c *nativeClient) getSignaturesForAddress(
	ctx context.Context,
	address solana.PublicKey,
	until, before solana.Signature,
	limit int,
) ([]*rpc.TransactionSignature, error) {
	var sigs []*rpc.TransactionSignature
	if err := c.callContext(ctx, &sigs, 0, "sol_getSignaturesForAddress", address, until, before, limit); err != nil {
		return nil, err
	}
	return sigs, nil
}

func (c *nativeClient) FindTransactions(
	ctx context.Context,
	from, to uint64,
	programs []solana.PublicKey,
) ([]solcore.BlockTransactions, error) {
	// Collect the matched transaction signatures per slot, across all programs.
	matched := make(map[uint64]map[solana.Signature]struct{})
	for _, p := range programs {
		sigs, err := c.findProgramSignatures(ctx, from, to, p)
		if err != nil {
			return nil, err
		}
		for _, s := range sigs {
			if matched[s.Slot] == nil {
				matched[s.Slot] = make(map[solana.Signature]struct{})
			}
			matched[s.Slot][s.Signature] = struct{}{}
		}
	}
	if len(matched) == 0 {
		return nil, nil
	}
	slots := make([]uint64, 0, len(matched))
	for s := range matched {
		slots = append(slots, s)
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })

	out := make([]solcore.BlockTransactions, 0, len(slots))
	for _, slot := range slots {
		full, err := c.getFullBlock(ctx, slot)
		if err != nil {
			return nil, err
		}
		want := matched[slot]
		var wts []solcore.WrappedTransaction
		for i, tx := range full.Transactions {
			if tx.Transaction == nil || len(tx.Transaction.Signatures) == 0 {
				continue
			}
			sig := tx.Transaction.Signatures[0]
			if _, ok := want[sig]; !ok {
				continue
			}
			wts = append(wts, solcore.WrappedTransaction{
				TransactionIndex: uint32(i),
				Signature:        sig,
				// Native getBlock(jsonParsed) does not surface the version cheaply; default legacy
				// (same as the BigQuery store). Only the version label is affected.
				Version:     rpc.LegacyTransactionVersion,
				Transaction: tx.Transaction,
				Meta:        tx.Meta,
			})
		}
		if len(wts) == 0 {
			continue
		}
		out = append(out, solcore.BlockTransactions{
			Slot:              slot,
			Blockhash:         full.Blockhash,
			PreviousBlockhash: full.PreviousBlockhash,
			BlockTime:         full.BlockTime,
			Transactions:      wts,
		})
	}
	return out, nil
}

func (c *nativeClient) GetContractStartBlock(
	ctx context.Context,
	address solana.PublicKey,
	start, latest uint64,
) (uint64, bool, error) {
	return data.BinarySearchContractStart(ctx, start, latest, func(ctx context.Context, bn uint64) (bool, error) {
		// Does address appear at or after slot bn? Page back from the top border until a signature at
		// slot <= bn is found (present) or signatures run out (absent).
		var toTxSig solana.Signature
		for n := bn + 1; n <= latest; n++ {
			blk, err := c.GetBlock(ctx, n)
			if err != nil {
				return false, err
			}
			if !blk.Skipped() && len(blk.Signatures) > 0 {
				toTxSig = blk.Signatures[0]
				break
			}
		}
		for {
			sigs, err := c.getSignaturesForAddress(ctx, address, solana.Signature{}, toTxSig, 1)
			if err != nil {
				return false, err
			}
			if len(sigs) == 0 {
				return false, nil
			}
			if sigs[0].Slot <= bn {
				return true, nil
			}
			toTxSig = sigs[0].Signature
		}
	})
}

func (c *nativeClient) GetPreviousUnskippedBlock(
	ctx context.Context,
	beforeSlot uint64,
) (solcore.PreviousUnskippedBlock, error) {
	if beforeSlot == 0 {
		return solcore.PreviousUnskippedBlock{}, nil
	}
	first := c.savedFirstBlockNumber.Load()
	const chunk = 1000
	hi := beforeSlot - 1
	for {
		lo := first
		if hi >= chunk && hi-chunk+1 > first {
			lo = hi - chunk + 1
		}
		slots, err := c.getBlocks(ctx, lo, hi)
		if err != nil {
			return solcore.PreviousUnskippedBlock{}, err
		}
		if len(slots) > 0 {
			s := slots[len(slots)-1] // nearest below beforeSlot
			blk, err := c.GetBlock(ctx, s)
			if err != nil {
				return solcore.PreviousUnskippedBlock{}, err
			}
			res := solcore.PreviousUnskippedBlock{Slot: s, Found: true}
			if !blk.Skipped() {
				res.BlockTime = blk.BlockTime
			}
			return res, nil
		}
		if lo <= first {
			return solcore.PreviousUnskippedBlock{}, nil // none in [first, beforeSlot)
		}
		hi = lo - 1
	}
}

func (c *nativeClient) ResetCache(r controller.BlockRange) {
	for _, bn := range c.cachedHeaders.Keys() {
		if r.Contains(bn) {
			c.cachedHeaders.Remove(bn)
		}
	}
}

func (c *nativeClient) Snapshot() any {
	return map[string]any{
		"mode": "native",
		"config": map[string]any{
			"endpoint":            c.endpoint,
			"firstBlockNumber":    c.firstBlockNumber,
			"watchLatestInterval": c.watchLatestInterval.String(),
			"getLatestTimeout":    c.getLatestTimeout.String(),
		},
		"savedFirstBlockNumber":  c.savedFirstBlockNumber.Load(),
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
