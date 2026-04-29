package supernode

import (
	"context"
	"errors"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio/chain/chain"
	"sentioxyz/sentio/chain/evm"
	"sentioxyz/sentio/common/number"
)

type TraceSlotCache struct {
	SlotCache      chain.LatestSlotCache[*evm.Slot]
	networkOptions *evm.NetworkOptions
}

func NewTraceSlotCacheMiddleware(
	slotCache chain.LatestSlotCache[*evm.Slot],
	networkOptions *evm.NetworkOptions,
) jsonrpc.Middleware {
	m := TraceSlotCache{
		SlotCache:      slotCache,
		networkOptions: networkOptions,
	}
	return jsonrpc.MakeServiceAsMiddleware("trace", &m)
}

func (m *TraceSlotCache) Filter(ctx context.Context, args *evm.TraceFilterArgs) ([]evm.ParityTrace, error) {
	if m.networkOptions.DisableTrace {
		return nil, errors.New("trace is not enabled")
	}

	slots, uncachedRange, err := getSlotsFromCache(ctx, m.SlotCache, nil, args.FromBlock, args.ToBlock)
	if err != nil {
		return nil, err
	}

	var results []evm.ParityTrace
	for _, slot := range slots {
		for _, trace := range slot.Traces {
			if traceFilter(&trace, args) {
				results = append(results, trace)
			}
		}
	}

	if !uncachedRange.IsEmpty() {
		from := hexutil.Uint64(uncachedRange.Start)
		to := hexutil.Uint64(*uncachedRange.End)
		nextResults, err := ResultsFromNext[evm.ParityTrace](ctx, "trace_filter", &evm.TraceFilterArgs{
			FromBlock:   &from,
			ToBlock:     &to,
			FromAddress: args.FromAddress,
			ToAddress:   args.ToAddress,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, nextResults...)
	}

	return results, nil
}

func (m *TraceSlotCache) FilterPacked(
	ctx context.Context,
	args *evm.TraceFilterArgs,
	needTransaction, needReceipt, needReceiptLogs bool,
) ([]*evm.PackedBlock, error) {
	if m.networkOptions.DisableTrace {
		return nil, errors.New("trace is not enabled")
	}

	slots, uncachedRange, err := getSlotsFromCache(ctx, m.SlotCache, nil, args.FromBlock, args.ToBlock)
	if err != nil {
		return nil, err
	}

	var results []*evm.PackedBlock
	for _, slot := range slots {
		var traces []evm.ParityTrace
		for _, trace := range slot.Traces {
			if traceFilter(&trace, args) {
				traces = append(traces, trace)
			}
		}
		if len(traces) == 0 {
			continue
		}
		results = append(results, evm.MakePackedBlock(slot, nil, traces, needTransaction, needReceipt, needReceiptLogs))
	}

	if !uncachedRange.IsEmpty() {
		from := hexutil.Uint64(uncachedRange.Start)
		to := hexutil.Uint64(*uncachedRange.End)
		nextResults, err := ResultsFromNext[*evm.PackedBlock](ctx, "trace_filterPacked", &evm.TraceFilterArgs{
			FromBlock:   &from,
			ToBlock:     &to,
			FromAddress: args.FromAddress,
			ToAddress:   args.ToAddress,
		}, needTransaction, needReceipt, needReceiptLogs)
		if err != nil {
			return nil, err
		}
		results = append(results, nextResults...)
	}

	return results, nil
}

func traceFilter(trace *evm.ParityTrace, args *evm.TraceFilterArgs) bool {
	if len(args.FromAddress) > 0 {
		found := false
		for _, address := range args.FromAddress {
			if trace.Action.From != nil && *trace.Action.From == address {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(args.ToAddress) > 0 {
		found := false
		for _, address := range args.ToAddress {
			if trace.Action.To == address {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
