package sol

import (
	"context"
	"time"

	"github.com/gagliardetto/solana-go"

	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

// TransactionRequirement is one instruction handler's demand: the programs it indexes over a block
// range.
type TransactionRequirement struct {
	controller.BlockRange

	Programs []solana.PublicKey
}

func (r TransactionRequirement) Snapshot() any {
	programs := make([]string, len(r.Programs))
	for i, p := range r.Programs {
		programs[i] = p.String()
	}
	return map[string]any{
		"programs": programs,
		"range":    r.BlockRange.String(),
	}
}

// MergeTxRequirements cuts the requirements (from currentBlockNumber onward) into disjoint ranges,
// each carrying only the programs whose range covers it. This avoids fetching a program's
// transactions over sub-ranges it does not need (e.g. a program whose start is far ahead of the
// current progress is not queried over the gap before its start).
func MergeTxRequirements(current uint64, reqs []TransactionRequirement) (result []TransactionRequirement) {
	rs := controller.CutRangeSet(
		current,
		utils.MapSliceNoError(reqs, func(r TransactionRequirement) controller.BlockRange {
			return r.BlockRange
		}),
	)
	for _, r := range rs {
		seen := set.New[solana.PublicKey]()
		for _, req := range reqs {
			if req.BlockRange.Include(r) {
				seen.Add(req.Programs...)
			}
		}
		programs := seen.DumpValues()
		if len(programs) == 0 {
			continue
		}
		result = append(result, TransactionRequirement{BlockRange: r, Programs: programs})
	}
	return result
}

// BuildTxFetcher builds the fetcher for one disjoint range and its program set, fetching the full
// matching transactions per block via sol_findTransactions.
func BuildTxFetcher(
	name string,
	req TransactionRequirement,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client Client,
) controller.Fetcher[BlockMainData] {
	return fetcher.NewFetcher[BlockMainData](
		name,
		req,
		controller.BlockRange{
			StartBlock: max(currentBlockNumber, req.StartBlock),
			EndBlock:   req.EndBlock,
		},
		latest,
		// minQuerySize 1: the super node errors when a multi-block range exceeds its transaction cap,
		// so the fetcher must be able to shrink to a single block (where the cap no longer applies).
		1,
		10000,
		targetKeepBytes,
		targetQueryBytes,
		time.Second*15,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			// The super node caps the result and errors when exceeded (except for a single block);
			// that error propagates and the fetcher halves the range, so no client-side check here.
			blocks, err := client.FindTransactions(ctx, start, end, req.Programs)
			if err != nil {
				return nil, err
			}
			result := make(map[uint64]BlockMainData, len(blocks))
			for _, b := range blocks {
				result[b.Slot] = BlockMainData{
					Slot:              b.Slot,
					Blockhash:         b.Blockhash.String(),
					PreviousBlockhash: b.PreviousBlockhash.String(),
					BlockTime:         b.BlockTime,
					Transactions:      b.Transactions,
				}
			}
			return result, nil
		},
	)
}
