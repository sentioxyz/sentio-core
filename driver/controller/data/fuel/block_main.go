package fuel

import (
	"context"
	"fmt"
	"time"

	"sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

type DataRequirement struct {
	Tx       []TransactionRequirement
	Interval []data.IntervalRequirement
}

type BlockMainData struct {
	Txs       []fuel.WrappedTransaction
	Intervals []data.IntervalConfig
}

func (d BlockMainData) Size() int {
	return len(d.Txs) + len(d.Intervals)
}

func (d BlockMainData) IsEmpty() bool {
	return d.Size() == 0
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
		h, err := client.GetBlock(getCtx, blockNumber)
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
			return result, nil
		},
	)
}

func BuildBlockMainDataFetcher(
	namePrefix string,
	req DataRequirement,
	firstBlockNumber uint64,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client Client,
) controller.Fetcher[BlockMainData] {
	req.Tx = MergeTxRequirements(currentBlockNumber, req.Tx)
	req.Interval = data.MergeIntervalRequirements(req.Interval)
	var fetchers []controller.Fetcher[BlockMainData]
	for i, r := range req.Tx {
		fetchers = append(fetchers, BuildTxFetcher(
			namePrefix+fmt.Sprintf("TxFetcher#%d", i), r, currentBlockNumber, latest, client))
	}
	for i, r := range req.Interval {
		fetchers = append(fetchers, BuildIntervalFetcher(
			namePrefix+fmt.Sprintf("IntervalFetcher#%d", i), r, firstBlockNumber, currentBlockNumber, latest, client))
	}
	return fetcher.MergeIsomorphicFetchers(
		namePrefix+"MainDataFetcher",
		req,
		fetchers,
		func(bn uint64, from []BlockMainData) (data BlockMainData, has bool, _ error) {
			has = len(from) > 0
			// Txs will never be repeated, because a range will only have one fetcher with data.
			for _, box := range from {
				data.Txs = append(data.Txs, box.Txs...)
				data.Intervals = append(data.Intervals, box.Intervals...)
			}
			return
		})
}
