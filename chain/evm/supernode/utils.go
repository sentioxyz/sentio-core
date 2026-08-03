package supernode

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

func buildPackedBlocks(
	headers []evm.ExtendedHeader,
	txs []evm.ExtendedTransaction,
	logs []types.Log,
	fullLogs []types.Log,
	traces []evm.ParityTrace,
) ([]*evm.PackedBlock, error) {
	blocks := make(map[uint64]*evm.PackedBlock)
	for i := range headers {
		blocks[headers[i].Number.Uint64()] = &evm.PackedBlock{BlockHeader: &headers[i]}
	}
	for _, lg := range logs {
		block, has := blocks[lg.BlockNumber]
		if !has {
			return nil, errors.Errorf("miss block header %d for log %d in txn %s", lg.BlockNumber, lg.Index, lg.TxHash)
		}
		block.Logs = append(block.Logs, lg)
	}
	fullLogsMap := make(map[common.Hash][]*types.Log)
	for i := range fullLogs {
		lg := &fullLogs[i]
		fullLogsMap[lg.TxHash] = append(fullLogsMap[lg.TxHash], lg)
	}
	for _, tx := range txs {
		block, has := blocks[tx.BlockNumber]
		if !has {
			return nil, errors.Errorf("miss block header %d for txn %s", tx.BlockNumber, tx.Hash.String())
		}
		block.RelevantTransactions = append(block.RelevantTransactions, tx.RPCTransaction)
		if r := tx.ExtendedReceipt; r != nil {
			r.SetLogs(fullLogsMap[tx.Hash])
			block.RelevantTransactionReceipts = append(block.RelevantTransactionReceipts, *r)
		}
	}
	for _, trace := range traces {
		block, has := blocks[trace.BlockNumber]
		if !has {
			return nil, errors.Errorf("miss block header %d for trace in txn %s", trace.BlockNumber, trace.TransactionHash)
		}
		block.Traces = append(block.Traces, trace)
	}
	return utils.GetMapValuesOrderByKey(blocks), nil
}

// ignoreStoreLimit adapts a store loader that has no record cap to queryWithCache's limit-aware
// collectFromStore signature.
func ignoreStoreLimit[ELEM any](
	f func(ctx context.Context, r rg.Range) ([]ELEM, error),
) func(ctx context.Context, r rg.Range, limit int) ([]ELEM, error) {
	return func(ctx context.Context, r rg.Range, _ int) ([]ELEM, error) {
		return f(ctx, r)
	}
}

// queryOptions holds the optional knobs of queryWithCache; see the with* constructors.
type queryOptions[ELEM any] struct {
	sizeOf                func(ELEM) int
	rejectUnsupportedTags bool
}

type queryOption[ELEM any] func(*queryOptions[ELEM])

// withoutTagFallthrough turns a block tag other than latest (pending, finalized, ...) into a
// plain error instead of falling through to the next middleware — the same style (an ordinary
// JSON-RPC error, deliberately not a typed -32602) as the blockHash rejection: both are misuse
// of a private API whose only real consumers never send these forms. The Ex methods use it:
// they must not be proxied upstream, and without it an exotic tag would bounce off an upstream
// node as method-not-found, which capability-probing callers would misread as "Ex unsupported".
func withoutTagFallthrough[ELEM any]() queryOption[ELEM] {
	return func(o *queryOptions[ELEM]) {
		o.rejectUnsupportedTags = true
	}
}

// withSizeOf overrides how many records one ELEM counts as for the record cap (default: 1 each).
// The Ex methods use it because their ELEM is a whole block whose record count is the number of
// items in it.
func withSizeOf[ELEM any](sizeOf func(ELEM) int) queryOption[ELEM] {
	return func(o *queryOptions[ELEM]) {
		o.sizeOf = sizeOf
	}
}

func unsupportedTag[ELEM any](opts queryOptions[ELEM], tag rpc.BlockNumber) error {
	if opts.rejectUnsupportedTags {
		return errors.Errorf("block tag %q is not supported, use latest or an explicit block number", tag.String())
	}
	return jsonrpc.CallNextMiddleware
}

// queryWithCache resolves the requested range, checks the span cap, and serves it from the
// latest-slot cache + store. resultLimit (0 = unlimited) caps how many records the merged response
// may hold in TOTAL; single-block queries (including the blockHash / blockNumber forms) are exempt
// since they cannot be shrunk further. collectFromStore receives the scan limit to push down
// (chain.StoreQueryLimit of the effective cap; 0 when uncapped/exempt).
func queryWithCache[ELEM any](
	ctx context.Context,
	slotCache chain.LatestSlotCache[*evm.Slot],
	blockHash *common.Hash,
	blockNumber *rpc.BlockNumber,
	fromBlock *rpc.BlockNumber,
	toBlock *rpc.BlockNumber,
	maxQueryRangeSize uint64,
	resultLimit int,
	collectFromSlot func(st *evm.Slot) ([]ELEM, error),
	collectFromStore func(ctx context.Context, r rg.Range, limit int) ([]ELEM, error),
	cacheMissHashReturn error,
	options ...queryOption[ELEM],
) ([]ELEM, error) {
	var opts queryOptions[ELEM]
	for _, option := range options {
		option(&opts)
	}
	if blockHash != nil {
		st, err := slotCache.GetByHash(ctx, blockHash.String())
		if err != nil {
			if errors.Is(err, chain.ErrSlotNotFound) {
				return nil, cacheMissHashReturn
			}
			return nil, err
		}
		return collectFromSlot(st)
	}
	var sn, en uint64
	if blockNumber != nil {
		if *blockNumber >= 0 {
			sn, en = (uint64)(*blockNumber), (uint64)(*blockNumber)
		} else {
			if *blockNumber == rpc.LatestBlockNumber {
				r, err := slotCache.GetRange(ctx)
				if err != nil {
					return nil, err
				}
				sn, en = *r.End, *r.End
			} else {
				return nil, unsupportedTag(opts, *blockNumber)
			}
		}
	} else {
		if fromBlock == nil {
			fromBlock = utils.WrapPointer(rpc.LatestBlockNumber)
		}
		if toBlock == nil {
			toBlock = utils.WrapPointer(rpc.LatestBlockNumber)
		}
		// slotCache only holds the latest block, other tags fall through to the next handler
		// (or are rejected outright, see withoutTagFallthrough)
		if *fromBlock < 0 && *fromBlock != rpc.LatestBlockNumber {
			return nil, unsupportedTag(opts, *fromBlock)
		}
		if *toBlock < 0 && *toBlock != rpc.LatestBlockNumber {
			return nil, unsupportedTag(opts, *toBlock)
		}
		if *fromBlock >= 0 && *toBlock >= 0 {
			sn, en = (uint64)(*fromBlock), (uint64)(*toBlock)
		} else {
			r, err := slotCache.GetRange(ctx)
			if err != nil {
				return nil, err
			}
			if *fromBlock < 0 {
				sn = *r.End
			} else {
				sn = (uint64)(*fromBlock)
			}
			if *toBlock < 0 {
				en = *r.End
			} else {
				en = (uint64)(*toBlock)
			}
		}
		// A range reaching beyond this node's head is an error rather than a silent trim: a
		// well-behaved caller resolves its head against this same node, so an over-the-head
		// range means the caller talked to someone ahead of us (e.g. another replica) — and for
		// the Ex methods a trimmed answer would be indistinguishable from genuinely empty
		// blocks. The single-block eth_getBlockByNumber form keeps the trim (and thus the
		// JSON-RPC null-for-future-block semantics) via the blockNumber branch above.
		r, err := slotCache.GetRange(ctx)
		if err != nil {
			return nil, err
		}
		if en > *r.End {
			return nil, errors.Errorf("block %d is beyond the latest block %d of this node, retry later",
				en, *r.End)
		}
	}
	// The span cap applies to the REQUESTED range — a caller-visible contract — not to the
	// sub-range left over after the latest-slot cache serves its part: whether the cache covers
	// some blocks is an internal, dynamically changing detail a caller cannot reason about.
	if maxQueryRangeSize > 0 {
		if err := chain.CheckQuerySpan(sn, en, maxQueryRangeSize); err != nil {
			return nil, err
		}
	}
	limit := chain.RangeQueryLimit(sn, en, resultLimit)
	result, err := chain.QueryRangeWithCache[*evm.Slot, ELEM](
		ctx,
		rg.NewRange(sn, en),
		slotCache,
		collectFromSlot,
		func(ctx context.Context, r rg.Range) ([]ELEM, error) {
			return collectFromStore(ctx, r, chain.StoreQueryLimit(limit))
		},
	)
	return chain.CheckTooManyResultsBy(result, err, limit, opts.sizeOf)
}

// exBlock pairs one block's chain identity with the items (logs or traces) a query matched in
// it. The Ex methods run queryWithCache with this ELEM so the cache/store splicing is shared
// with the plain methods; the caller reassembles the per-block elems into the Ex response. A nil
// link marks items from a source that cannot vouch for block identities (upstream proxy).
type exBlock[ITEM any] struct {
	link  *evm.BlockLink
	items []ITEM
}

func exBlockFromSlot[ITEM any](st *evm.Slot, items []ITEM) []exBlock[ITEM] {
	link := evm.SlotBlockLink(st)
	return []exBlock[ITEM]{{link: &link, items: items}}
}

// checkStoreLinks validates the store's block identities for a queried sub-range: exactly one
// identity per block, no gaps. Plain duplicate rows are collapsed by the store query itself (the
// blocks table has no deduplication), so anything left over here means the range is either
// incompletely synced or holds rows that disagree about a block — both retryable, never
// something to serve.
func checkStoreLinks(links []evm.BlockLink, r rg.Range) error {
	seen := make(map[uint64]evm.BlockLink, len(links))
	for _, link := range links {
		if prev, dup := seen[link.Number]; dup {
			return errors.Errorf(
				"the store holds conflicting identities for block %d (%s and %s), retry later",
				link.Number, prev.Hash, link.Hash)
		}
		seen[link.Number] = link
	}
	for bn := r.Start; bn <= *r.End; bn++ {
		if _, has := seen[bn]; !has {
			return errors.Errorf("the store is missing block %d of range %s, retry later", bn, r)
		}
	}
	return nil
}

// exBlocksFromStore distributes store items into one exBlock per covered block; links must cover
// the queried sub-range completely (one per block, ascending), which the caller verifies.
func exBlocksFromStore[ITEM any](
	links []evm.BlockLink,
	items []ITEM,
	blockNumberOf func(ITEM) uint64,
) []exBlock[ITEM] {
	group := utils.Group(items, blockNumberOf)
	elems := make([]exBlock[ITEM], len(links))
	for i := range links {
		elems[i] = exBlock[ITEM]{link: &links[i], items: group[links[i].Number]}
	}
	return elems
}

// exBlockSize is the sizeOf of the Ex methods: an exBlock counts as many records as it holds
// items, so queryWithCache's record cap applies to logs/traces, not blocks.
func exBlockSize[ITEM any](e exBlock[ITEM]) int {
	return len(e.items)
}

// assembleEx flattens queryWithCache's per-block elems into the Ex response sections, validating
// the link chain (elems arrive ascending: store part first, then the cache tail, so blocks with
// identity form one contiguous run). The record cap was already applied by queryWithCache.
func assembleEx[ITEM any](elems []exBlock[ITEM]) (evm.BlockHashLinkPart, []ITEM, error) {
	var links []evm.BlockLink
	items := make([]ITEM, 0)
	for _, e := range elems {
		if e.link != nil {
			links = append(links, *e.link)
		}
		items = append(items, e.items...)
	}
	part, err := evm.NewBlockHashLinkPart(links)
	return part, items, err
}
