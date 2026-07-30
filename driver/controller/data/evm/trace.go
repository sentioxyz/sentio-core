package evm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/errgroup"
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
		50000, // maxReadyBlockCount: with Ex every covered block buffers a header-only entry
		2000, // the target is that each query got no more than 2000 traces
		time.Second*10,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			// trace_filterEx is always the first choice: its hash link carries every covered
			// block's identity.
			exResp, err := client.GetTracesEx(ctx, start, end, req.TraceFilter.Address)
			if err == nil {
				return buildTraceEntriesFromEx(req, exResp, end)
			}
			if !errors.Is(err, errMethodNotSupported) {
				return nil, err
			}
			// The endpoint has no Ex support. In the watching range, query block by block and
			// verify each trace's block hash against the header fetched first (trace_filter has
			// no by-hash form, so unlike logs the verification is by comparison, and an empty
			// result stays unverifiable).
			if end == latest.GetBlockNumber() {
				return fetchTracesAtTip(ctx, client, req, start, end)
			}
			// plain fallback, see the log fetcher for the trade-off
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

// fetchTracesAtTip serves the watching range for endpoints without Ex support, mirroring
// fetchLogsAtTip: every block is fetched individually and concurrently — first its header, then
// its traces by number, and every returned trace must carry that header's block hash, otherwise
// the endpoint answered from a different fork and the plain retryable error keeps the fetcher
// retrying until the views converge. Every returned entry carries the header, so even trace-less
// tip blocks flow into the header list and the checkpoint chain (an empty result itself stays
// unverifiable: trace_filter has no by-hash form).
func fetchTracesAtTip(
	ctx context.Context,
	client Client,
	req TraceRequirement,
	start, end uint64,
) (map[uint64]BlockMainData, error) {
	entries := make([]BlockMainData, end-start+1)
	g, gctx := errgroup.WithContext(ctx)
	for bn := start; bn <= end; bn++ {
		g.Go(func() error {
			h, err := client.GetHeader(gctx, bn)
			if err != nil {
				return err
			}
			traces, err := client.GetTraces(gctx, bn, bn, req.TraceFilter.Address)
			if err != nil {
				return err
			}
			for _, trace := range traces {
				if trace.BlockHash != h.BlockHash {
					return errors.Errorf(
						"trace of tx %s in block %d has block hash %s but the header is %s, "+
							"the endpoint answered from a different fork, will retry",
						trace.TransactionHash, bn, trace.BlockHash, h.BlockHash)
				}
			}
			entries[bn-start] = BlockMainData{
				Traces: utils.FilterArr(traces, req.TraceFilter.Check),
				Header: &h,
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	result := make(map[uint64]BlockMainData, len(entries))
	for i := range entries {
		result[start+uint64(i)] = entries[i]
	}
	return result, nil
}

// buildTraceEntriesFromEx converts a trace_filterEx response into per-block entries, mirroring
// buildLogEntriesFromEx: covered blocks (with or without traces) carry their identity, covered
// traces get the stripped blockHash backfilled into both the struct and the Raw JSON handed to
// the processor.
func buildTraceEntriesFromEx(
	req TraceRequirement,
	resp GetTracesExResponse,
	end uint64,
) (map[uint64]BlockMainData, error) {
	links, err := verifiedLinks(resp.BlockHashLinkPart)
	if err != nil {
		return nil, err
	}
	if err = checkLinksReachEnd(links, end); err != nil {
		return nil, err
	}
	result := emptyEntriesFromLinks(links)
	dict := linkIndex(links)
	for _, trace := range resp.Traces {
		if link, covered := dict[trace.BlockNumber]; covered {
			if err = setTraceBlockHash(&trace, link.Hash.Hex()); err != nil {
				return nil, err
			}
		}
		if !req.TraceFilter.Check(trace) {
			continue
		}
		entry := result[trace.BlockNumber]
		entry.Traces = append(entry.Traces, trace)
		result[trace.BlockNumber] = entry
	}
	return result, nil
}
