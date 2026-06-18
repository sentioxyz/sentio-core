package sui

import (
	"context"
	"fmt"
	"time"

	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

type SimpleBlock sui.SimpleCheckpoint

func (b SimpleBlock) GetBlockNumber() uint64 {
	return b.Checkpoint
}

func (b SimpleBlock) GetBlockParentHash() string {
	return ""
}

func (b SimpleBlock) GetBlockHash() string {
	return b.Digest
}

func (b SimpleBlock) GetBlockTime() time.Time {
	return time.UnixMilli(int64(b.TimestampMS))
}

func BuildIntervalFetcher(
	name string,
	req data.IntervalRequirement,
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client Client,
) controller.Fetcher[BlockMainData] {
	timeGetter := func(ctx context.Context, blockNumber uint64) (time.Time, error) {
		getCtx, cancel := context.WithTimeout(ctx, time.Second*3)
		defer cancel()
		h, err := client.GetSimpleBlock(getCtx, blockNumber)
		if err != nil {
			return time.Time{}, err
		}
		return h.GetBlockTime(), nil
	}
	return fetcher.NewFetcher[BlockMainData](
		name,
		req,
		controller.BlockRange{
			StartBlock: max(currentBlockNumber, req.StartBlock),
			EndBlock:   req.EndBlock,
		},
		latest,
		10000,
		10000,
		10000,
		1000,
		time.Minute,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			bns, err := data.QueryInterval(ctx, start, end, firstBlockNumber, latest, req, timeGetter)
			if err != nil {
				return nil, err
			}
			result := make(map[uint64]BlockMainData)
			for _, bn := range bns {
				result[bn] = BlockMainData{
					Intervals: []data.IntervalConfig{req.IntervalConfig},
				}
			}
			// timeGetter above already warmed GetSimpleBlock for these blocks, so this mostly hits cache.
			if err := attachSimpleBlocks(ctx, client, result); err != nil {
				return nil, err
			}
			return result, nil
		},
	)
}

type BlockMainData struct {
	Txs           []types.TransactionResponseV1
	ObjectChanges []types.ObjectChangeExtend
	Intervals     []data.IntervalConfig
	// SimpleBlock is the checkpoint header, prefetched concurrently by the data fetchers alongside the
	// block's data. The strictly block-ordered transfer step (handler.go) needs the header for every
	// non-empty block; fetching it here (off the serial path) keeps sui_getSimpleCheckpoint — which is
	// order-independent — from becoming the throughput bottleneck. nil means it wasn't prefetched, in
	// which case the transfer step falls back to a (serial) Client.GetSimpleBlock.
	SimpleBlock *SimpleBlock
}

func (b BlockMainData) IsEmpty() bool {
	return b.Size() == 0
}

// Size intentionally ignores SimpleBlock: it is header metadata that only ever rides along with real
// data, and must not on its own make an otherwise-empty block look non-empty.
func (b BlockMainData) Size() int {
	return len(b.Intervals) + len(b.ObjectChanges) + len(b.Txs)*10
}

// attachSimpleBlocks fetches the checkpoint header for every block that already carries data and stores
// it on the BlockMainData, so the downstream block-ordered transfer can avoid a serial RPC. It runs in
// the (concurrent, read-ahead) fetcher goroutines, and GetSimpleBlock is cached + singleflighted, so
// overlapping fetchers requesting the same block don't produce duplicate sui_getSimpleCheckpoint calls.
func attachSimpleBlocks(ctx context.Context, client Client, result map[uint64]BlockMainData) error {
	if len(result) == 0 {
		return nil
	}
	// Snapshot the keys first: the worker goroutines start immediately on Go(), so ranging the map here
	// while they write back into it would be a concurrent map iteration + write (a runtime panic). Each
	// worker writes its own headers[i] (distinct index, no lock needed); the map is mutated only after
	// Wait(), back on this goroutine.
	bns := make([]uint64, 0, len(result))
	for bn := range result {
		bns = append(bns, bn)
	}
	headers := make([]SimpleBlock, len(bns))
	// No concurrency limit here: GetSimpleBlock goes through the client's resource manager, which
	// already bounds in-flight RPCs.
	g, gctx := errgroup.WithContext(ctx)
	for i, bn := range bns {
		i, bn := i, bn // nogo's loopclosure analyzer predates Go 1.22 per-iteration loop vars
		g.Go(func() error {
			sb, err := client.GetSimpleBlock(gctx, bn)
			if err != nil {
				return err
			}
			headers[i] = sb
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	for i, bn := range bns {
		d := result[bn]
		d.SimpleBlock = &headers[i]
		result[bn] = d
	}
	return nil
}

type DataRequirement struct {
	Interval      []data.IntervalRequirement
	ObjectChanges []ObjectChangeRequirement
	Txn           []TransactionRequirement
}

func BuildBlockMainDataFetcher(
	namePrefix string,
	req DataRequirement,
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client Client,
) controller.Fetcher[BlockMainData] {
	req.ObjectChanges = MergeObjectChangeRequirements(currentBlockNumber, req.ObjectChanges)
	req.Txn = MergeTxnRequirements(currentBlockNumber, req.Txn)
	req.Interval = data.MergeIntervalRequirements(req.Interval)
	var fetchers []controller.Fetcher[BlockMainData]
	for i, r := range req.ObjectChanges {
		fetchers = append(fetchers, BuildObjectChangeFetcher(
			namePrefix+fmt.Sprintf("ObjectChangeFetcher#%d", i), r, currentBlockNumber, latest, client))
	}
	for i, r := range req.Txn {
		fetchers = append(fetchers, BuildTxnFetcher(
			namePrefix+fmt.Sprintf("TxnFetcher#%d", i), r, currentBlockNumber, latest, client))
	}
	for i, r := range req.Interval {
		fetchers = append(fetchers, BuildIntervalFetcher(
			namePrefix+fmt.Sprintf("IntervalFetcher#%d", i), r, firstBlockNumber, currentBlockNumber, latest, client))
	}
	return fetcher.MergeIsomorphicFetchers(
		namePrefix+"MainDataFetcher",
		req,
		fetchers,
		func(_ uint64, from []BlockMainData) (data BlockMainData, has bool, _ error) {
			has = len(from) > 0
			// ObjectChanges and Txn never be repeated, because a range will only have one fetcher with data.
			for _, box := range from {
				data.Txs = append(data.Txs, box.Txs...)
				data.ObjectChanges = append(data.ObjectChanges, box.ObjectChanges...)
				data.Intervals = append(data.Intervals, box.Intervals...)
				if data.SimpleBlock == nil && box.SimpleBlock != nil {
					data.SimpleBlock = box.SimpleBlock
				}
			}
			return
		})
}
