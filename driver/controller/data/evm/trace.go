package evm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

type Trace struct {
	Raw json.RawMessage

	BlockNumber      uint64
	BlockHash        string
	TransactionHash  string
	TransactionIndex int32

	Error     string
	Address   string
	Signature string
}

func (t *Trace) UnmarshalJSON(raw []byte) error {
	var payload *struct {
		Action struct {
			Input string `json:"input"`
			To    string `json:"to"`
		} `json:"action"`
		BlockHash           string `json:"blockHash"`
		BlockNumber         uint64 `json:"blockNumber"`
		TransactionPosition int32  `json:"transactionPosition"`
		TransactionHash     string `json:"transactionHash"`
		Error               string `json:"error"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	if payload == nil {
		t = nil
		return nil
	}
	t.Raw = raw
	t.BlockNumber = payload.BlockNumber
	t.BlockHash = payload.BlockHash
	t.TransactionHash = payload.TransactionHash
	t.TransactionIndex = payload.TransactionPosition
	t.Error = payload.Error
	t.Address = payload.Action.To
	if len(payload.Action.Input) >= 10 {
		t.Signature = payload.Action.Input[:10]
	}
	return nil
}

// TraceFilter has 2 parts, there are linked by AND
type TraceFilter struct {
	Signature []string
	Address   []string
}

func (f TraceFilter) Check(trace Trace) bool {
	if len(trace.Error) > 0 {
		return false
	}
	if len(f.Address) > 0 && utils.IndexOf(f.Address, strings.ToLower(trace.Address)) < 0 {
		return false
	}
	if len(f.Signature) > 0 && utils.IndexOf(f.Signature, trace.Signature) < 0 {
		return false
	}
	return true
}

func (t TraceFilter) String() string {
	return fmt.Sprintf("Sig:[%s],Addr:%s", utils.ArrSummary(t.Signature, 10), utils.ArrSummary(t.Address, 10))
}

// Merge traces match t always match r, traces match a also always match r. Traces(r) >= Traces(t) + Traces(a)
func (t TraceFilter) Merge(a TraceFilter) (r TraceFilter) {
	if len(t.Signature) > 0 && len(a.Signature) > 0 {
		r.Signature = set.SmartNew[string](t.Signature, a.Signature).DumpValues()
	}
	if len(t.Address) > 0 && len(a.Address) > 0 {
		r.Address = set.SmartNew[string](t.Address, a.Address).DumpValues()
	}
	return r
}

func MergeTraceFilters(filters ...TraceFilter) TraceFilter {
	if len(filters) == 0 {
		panic("filters is empty")
	}
	signatures := set.New[string]()
	for _, filter := range filters {
		if len(filter.Signature) == 0 {
			signatures = set.New[string]()
			break
		}
		signatures.Add(filter.Signature...)
	}
	addresses := set.New[string]()
	for _, filter := range filters {
		if len(filter.Address) == 0 {
			addresses = set.New[string]()
			break
		}
		addresses.Add(filter.Address...)
	}
	return TraceFilter{
		Signature: signatures.DumpValues(),
		Address:   addresses.DumpValues(),
	}
}

type TraceRequirement struct {
	controller.BlockRange
	TraceFilter
}

func (r TraceRequirement) String() string {
	return fmt.Sprintf("TraceRequirement[%s]%s", r.TraceFilter.String(), r.BlockRange.String())
}

func (r TraceRequirement) Snapshot() any {
	return map[string]any{
		"filter": r.TraceFilter,
		"range":  r.BlockRange.String(),
	}
}

// MergeTraceRequirement it can be guaranteed that all the item ranges of the result must be disjoint,
// and each range has at most one filter
func MergeTraceRequirement(current uint64, reqs []TraceRequirement) (result []TraceRequirement) {
	rs := controller.CutRangeSet(current, utils.MapSliceNoError(reqs, func(r TraceRequirement) controller.BlockRange {
		return r.BlockRange
	}))
	for _, r := range rs {
		var filters []TraceFilter
		for _, req := range reqs {
			if req.BlockRange.Include(r) {
				filters = append(filters, req.TraceFilter)
			}
		}
		if len(filters) == 0 {
			continue
		}
		result = append(result, TraceRequirement{
			TraceFilter: MergeTraceFilters(filters...),
			BlockRange:  r,
		})
	}
	return result
}

func BuildTraceFetcher(
	name string,
	req TraceRequirement,
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
		1,
		100,
		10000,
		2000, // the target is that each query got no more than 2000 traces
		time.Second*10,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			allTraces, err := client.GetTraces(ctx, start, end, req.TraceFilter.Address)
			if err != nil {
				return nil, err
			}
			allTraces = utils.FilterArr(allTraces, req.TraceFilter.Check)
			blockTraces := utils.Group(allTraces, func(trace Trace) uint64 {
				return trace.BlockNumber
			})
			result := make(map[uint64]BlockMainData)
			for bn, traces := range blockTraces {
				result[bn] = BlockMainData{Traces: traces}
			}
			return result, nil
		},
	)
}
