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
) ([]ELEM, error) {
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
				return nil, jsonrpc.CallNextMiddleware
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
		if *fromBlock < 0 && *fromBlock != rpc.LatestBlockNumber {
			return nil, jsonrpc.CallNextMiddleware
		}
		if *toBlock < 0 && *toBlock != rpc.LatestBlockNumber {
			return nil, jsonrpc.CallNextMiddleware
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
	return chain.CheckTooManyResults(result, err, limit)
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

// exResultLimit is RangeQueryLimit for the Ex methods: since their ELEM is a block rather than a
// record, the record cap cannot ride on queryWithCache's ELEM count and is applied on the
// reassembled items instead — this computes it from the unresolved request (a single-block
// request, by hash or by an equal from/to pair, is exempt because it cannot be shrunk further).
func exResultLimit(blockHash *common.Hash, fromBlock, toBlock *rpc.BlockNumber, limit int) int {
	if blockHash != nil {
		return 0
	}
	norm := func(p *rpc.BlockNumber) rpc.BlockNumber {
		if p == nil {
			return rpc.LatestBlockNumber
		}
		return *p
	}
	if norm(fromBlock) == norm(toBlock) {
		return 0
	}
	return limit
}

// assembleEx flattens queryWithCache's per-block elems into the Ex response sections, applying
// the record cap and validating the link chain (elems arrive ascending: store part first, then
// the cache tail, so blocks with identity form one contiguous run).
func assembleEx[ITEM any](elems []exBlock[ITEM], limit int) (evm.BlockHashLinkPart, []ITEM, error) {
	var links []evm.BlockLink
	items := make([]ITEM, 0)
	for _, e := range elems {
		if e.link != nil {
			links = append(links, *e.link)
		}
		items = append(items, e.items...)
	}
	if _, err := chain.CheckTooManyResults(items, nil, limit); err != nil {
		return evm.BlockHashLinkPart{}, nil, err
	}
	part, err := evm.NewBlockHashLinkPart(links)
	return part, items, err
}

// exFinalizeErr maps queryWithCache's internal fallthrough signals to hard errors: the Ex
// methods must not be proxied upstream (real nodes do not implement them), so an exotic block
// tag or a by-hash cache miss is the caller's cue to retry or use the plain method instead.
func exFinalizeErr(err error, method string) error {
	if errors.Is(err, jsonrpc.CallNextMiddleware) {
		return errors.Errorf("the request cannot be served by %s, retry later or use the plain method instead", method)
	}
	return err
}
