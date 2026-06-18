package aptos

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

type Change struct {
	Raw string

	*aptos.WriteSetChange
}

func (e *Change) UnmarshalJSON(raw []byte) error {
	if err := json.Unmarshal(raw, &e.WriteSetChange); err != nil {
		return err
	}
	e.Raw = string(raw)
	return nil
}

type MinimalistTransactionWithChanges struct {
	aptos.MinimalistTransaction `json:",inline"`

	Changes []Change `json:"changes"`
}

type ChangeRequirement struct {
	controller.BlockRange
	aptos.ChangeFilter
}

func (r ChangeRequirement) String() string {
	return fmt.Sprintf("ChangeRequirement[%s]%s", r.ChangeFilter.String(), r.BlockRange.String())
}

func (r ChangeRequirement) Snapshot() any {
	return map[string]any{
		"filter": r.ChangeFilter,
		"range":  r.BlockRange.String(),
	}
}

// MergeChangeRequirements it can be guaranteed that all the item ranges of the result must be disjoint,
// and each range has at most one filter
func MergeChangeRequirements(current uint64, reqs []ChangeRequirement) (result []ChangeRequirement) {
	rs := controller.CutRangeSet(current, utils.MapSliceNoError(reqs, func(r ChangeRequirement) controller.BlockRange {
		return r.BlockRange
	}))
	for _, r := range rs {
		var filters []aptos.ChangeFilter
		for _, req := range reqs {
			if req.BlockRange.Include(r) {
				filters = append(filters, req.ChangeFilter)
			}
		}
		if len(filters) == 0 {
			continue
		}
		result = append(result, ChangeRequirement{
			ChangeFilter: aptos.MergeChangeFilters(filters...),
			BlockRange:   r,
		})
	}
	return result
}

func BuildChangeFetcher(
	name string,
	req ChangeRequirement,
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
		1000,
		1000000,
		100000,
		10000,
		time.Minute,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			txs, err := client.GetChanges(ctx, start, end, req.ChangeFilter)
			if err != nil {
				return nil, err
			}
			result := make(map[uint64]BlockMainData, len(txs))
			for _, tx := range txs {
				result[tx.Version] = BlockMainData{SimpleTxn: &tx.MinimalistTransaction, Changes: tx.Changes}
			}
			return result, nil
		},
	)
}
