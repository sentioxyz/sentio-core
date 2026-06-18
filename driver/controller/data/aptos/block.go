package aptos

import (
	"context"
	"fmt"
	"time"

	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

type BlockMainData struct {
	SimpleTxn *aptos.MinimalistTransaction
	Txn       *aptos.Transaction
	Changes   []Change
	Intervals []data.IntervalConfig
}

type DataRequirement struct {
	Interval []data.IntervalRequirement
	Changes  []ChangeRequirement
	Txn      []TransactionRequirement
}

func (d BlockMainData) IsEmpty() bool {
	return d.Size() == 0
}

func (d BlockMainData) Size() int {
	return len(d.Changes) + len(d.Intervals) + utils.Select(d.Txn == nil, 0, 10)
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
		h, err := client.GetMinimalistTransaction(getCtx, blockNumber)
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
	req.Changes = MergeChangeRequirements(currentBlockNumber, req.Changes)
	req.Txn = MergeTxnRequirements(currentBlockNumber, req.Txn)
	req.Interval = data.MergeIntervalRequirements(req.Interval)
	var fetchers []controller.Fetcher[BlockMainData]
	for i, r := range req.Changes {
		fetchers = append(fetchers, BuildChangeFetcher(
			namePrefix+fmt.Sprintf("ChangeFetcher#%d", i), r, currentBlockNumber, latest, client))
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
			// Changes and Txn never be repeated, because a range will only have one fetcher with data.
			for _, box := range from {
				if box.Txn != nil {
					data.Txn = box.Txn
				}
				if box.SimpleTxn != nil {
					data.SimpleTxn = box.SimpleTxn
				}
				data.Changes = append(data.Changes, box.Changes...)
				data.Intervals = append(data.Intervals, box.Intervals...)
			}
			return
		})
}
