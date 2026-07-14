package aptos

import (
	"context"
	"fmt"
	"time"

	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

type MinimalistTransaction aptos.MinimalistTransaction

func (t MinimalistTransaction) GetBlockNumber() uint64 {
	return t.Version
}

func (t MinimalistTransaction) GetBlockHash() string {
	return t.Hash
}

func (t MinimalistTransaction) GetBlockParentHash() string {
	return ""
}

func (t MinimalistTransaction) GetBlockTime() time.Time {
	return time.UnixMicro(t.TimestampMS)
}

type Transaction aptos.Transaction

func (t *Transaction) GetBlockNumber() uint64 {
	return t.Version()
}

func (t *Transaction) GetBlockHash() string {
	return t.Hash()
}

func (t *Transaction) GetBlockParentHash() string {
	return ""
}

func (t *Transaction) GetBlockTime() time.Time {
	if tm := (*aptos.Transaction)(t).Time(); tm != nil {
		return *tm
	}
	return time.Time{}
}

type TransactionRequirement struct {
	controller.BlockRange

	Filter      aptos.TransactionFilter
	FetchConfig aptos.TransactionFetchConfig
}

func (r TransactionRequirement) String() string {
	return fmt.Sprintf("TransactionRequirement[Filter:[%s],FetchConfig:[%s]]%s",
		r.Filter.String(), r.FetchConfig.String(), r.BlockRange.String())
}

func (r TransactionRequirement) Snapshot() any {
	return map[string]any{
		"filter":      r.Filter,
		"fetchConfig": r.FetchConfig,
		"range":       r.BlockRange.String(),
	}
}

func MergeTxnRequirements(current uint64, reqs []TransactionRequirement) (result []TransactionRequirement) {
	rs := controller.CutRangeSet(
		current,
		utils.MapSliceNoError(reqs, func(r TransactionRequirement) controller.BlockRange {
			return r.BlockRange
		}),
	)
	for _, r := range rs {
		rr := TransactionRequirement{BlockRange: r}
		first := true
		for _, req := range reqs {
			if !req.BlockRange.Include(r) {
				continue
			}
			if first {
				rr.Filter = req.Filter
				rr.FetchConfig = req.FetchConfig
				first = false
			} else {
				rr.Filter = rr.Filter.Merge(req.Filter)
				rr.FetchConfig = rr.FetchConfig.Merge(req.FetchConfig)
			}
		}
		if first {
			continue
		}
		result = append(result, rr)
	}
	return result

}

func BuildTxnFetcher(
	name string,
	req TransactionRequirement,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client Client,
) controller.Fetcher[BlockMainData] {
	return fetcher.NewFetcher(
		name,
		req,
		controller.BlockRange{
			StartBlock: max(currentBlockNumber, req.StartBlock),
			EndBlock:   req.EndBlock,
		},
		latest,
		// minQuerySize 1: the super node errors when a multi-block range exceeds its record cap,
		// so the fetcher must be able to shrink to a single block (where the cap no longer applies).
		1,
		100000,
		10000, // size of transaction is 10, so will cache 1000 transactions
		5000,  // the target is that each query got no more than 500 transactions
		time.Second*10,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			txs, err := client.GetTransactions(ctx, start, end, req.Filter, req.FetchConfig)
			if err != nil {
				return nil, err
			}
			result := make(map[uint64]BlockMainData)
			for i := range txs {
				result[txs[i].Version()] = BlockMainData{Txn: &txs[i]}
			}
			return result, nil
		},
	)
}
