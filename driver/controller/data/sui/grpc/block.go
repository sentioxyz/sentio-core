// Package suigrpc is the grpc-data twin of data/sui: same fetch/requirement
// machinery, but the block data carries grpc-format transactions / object
// changes (sentio-core ExtendedGrpc*), fetched from the super node's
// sui_getGrpcTransactions / sui_filterGrpcChangedObjects methods. The
// format-agnostic pieces (Client, SimpleBlock, the *Requirement types and their
// Merge helpers, DataRequirement) are reused from data/sui.
package grpc

import (
	"context"
	"fmt"
	"time"

	chainsui "sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	suidata "sentioxyz/sentio-core/driver/controller/data/sui"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

// BlockMainData is the grpc-format counterpart of suidata.BlockMainData.
type BlockMainData struct {
	Txs           []*chainsui.ExtendedGrpcTransaction
	ObjectChanges []*chainsui.ExtendedGrpcChangedObject
	Intervals     []data.IntervalConfig
	// SimpleBlock is the checkpoint header, prefetched concurrently alongside the block's data so the
	// strictly block-ordered transfer step doesn't need a serial RPC. nil means not prefetched.
	SimpleBlock *suidata.SimpleBlock
}

func (b BlockMainData) IsEmpty() bool {
	return b.Size() == 0
}

// Size intentionally ignores SimpleBlock (header metadata only rides along with real data).
func (b BlockMainData) Size() int {
	return len(b.Intervals) + len(b.ObjectChanges) + len(b.Txs)*10
}

// attachSimpleBlocks fetches the checkpoint header for every block and stores it on the BlockMainData.
// Only the interval fetcher needs it: those blocks carry no tx / object-change data to derive the header
// from (the txn and object-change fetchers read the header off the grpc data via GetSimpleCheckpoint).
// GetSimpleBlock is cached + singleflighted.
func attachSimpleBlocks(ctx context.Context, client suidata.Client, result map[uint64]BlockMainData) error {
	if len(result) == 0 {
		return nil
	}
	bns := make([]uint64, 0, len(result))
	for bn := range result {
		bns = append(bns, bn)
	}
	headers := make([]suidata.SimpleBlock, len(bns))
	g, gctx := errgroup.WithContext(ctx)
	for i, bn := range bns {
		i, bn := i, bn
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

func BuildIntervalFetcher(
	name string,
	req data.IntervalRequirement,
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client suidata.Client,
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
				result[bn] = BlockMainData{Intervals: []data.IntervalConfig{req.IntervalConfig}}
			}
			if err := attachSimpleBlocks(ctx, client, result); err != nil {
				return nil, err
			}
			return result, nil
		},
	)
}

// BuildBlockMainDataFetcher mirrors suidata.BuildBlockMainDataFetcher but builds grpc fetchers. It
// reuses suidata.DataRequirement and the Merge* helpers (all format-agnostic).
func BuildBlockMainDataFetcher(
	namePrefix string,
	req suidata.DataRequirement,
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client suidata.Client,
) controller.Fetcher[BlockMainData] {
	req.ObjectChanges = suidata.MergeObjectChangeRequirements(currentBlockNumber, req.ObjectChanges)
	req.Txn = suidata.MergeTxnRequirements(currentBlockNumber, req.Txn)
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
		func(_ uint64, from []BlockMainData) (out BlockMainData, has bool, _ error) {
			has = len(from) > 0
			for _, box := range from {
				out.Txs = append(out.Txs, box.Txs...)
				out.ObjectChanges = append(out.ObjectChanges, box.ObjectChanges...)
				out.Intervals = append(out.Intervals, box.Intervals...)
				if out.SimpleBlock == nil && box.SimpleBlock != nil {
					out.SimpleBlock = box.SimpleBlock
				}
			}
			return
		})
}
