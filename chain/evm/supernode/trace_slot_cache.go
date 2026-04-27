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

	fromBlock := number.Number(*args.FromBlock)
	toBlock := number.Number(*args.ToBlock)
	interval := number.NewRange(fromBlock, toBlock)

	slots, uncachedRange, err := getSlotsByRangeFromCache(ctx, m.SlotCache, interval)
	if err != nil {
		return nil, err
	}

	slotToTraces := func(slot *evm.Slot) ([]evm.ParityTrace, error) {
		var traces []evm.ParityTrace
		for _, trace := range slot.Traces {
			if traceFilter(&trace, args) {
				traces = append(traces, trace)
			}
		}
		return traces, nil
	}

	var results []evm.ParityTrace
	for _, slot := range slots {
		slotTraces, err := slotToTraces(slot)
		if err != nil {
			return nil, err
		}
		results = append(results, slotTraces...)
	}

	if !uncachedRange.IsEmpty() {
		from := hexutil.Uint64(uncachedRange.L())
		to := hexutil.Uint64(uncachedRange.R())
		nextResults, err := ResultsFromNext[evm.ParityTrace](ctx, "trace_filter", &evm.TraceFilterArgs{
			FromBlock:   &from,
			ToBlock:     &to,
			FromAddress: args.FromAddress,
			ToAddress:   args.ToAddress,
			After:       args.After,
			Count:       args.Count,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, nextResults...)
	}

	return results, nil
}

func (m *TraceSlotCache) FilterPacked(ctx context.Context, args *evm.TraceFilterArgs,
	needTransaction, needReceipt, needReceiptLogs bool) ([]*evm.PackedBlock, error) {
	if m.networkOptions.DisableTrace {
		return nil, errors.New("trace is not enabled")
	}

	fromBlock := number.Number(*args.FromBlock)
	toBlock := number.Number(*args.ToBlock)
	interval := number.NewRange(fromBlock, toBlock)

	slots, uncachedRange, err := getSlotsByRangeFromCache(ctx, m.SlotCache, interval)
	if err != nil {
		return nil, err
	}
	slotToTracesPacked := func(slot *evm.Slot) ([]*evm.PackedBlock, error) {
		var traces []evm.ParityTrace
		for _, trace := range slot.Traces {
			if traceFilter(&trace, args) {
				traces = append(traces, trace)
			}
		}
		return []*evm.PackedBlock{evm.MakePackedBlock(slot, nil, traces, needTransaction, needReceipt, needReceiptLogs)}, nil
	}

	var results []*evm.PackedBlock
	for _, slot := range slots {
		slotTraces, err := slotToTracesPacked(slot)
		if err != nil {
			return nil, err
		}
		results = append(results, slotTraces...)
	}

	if !uncachedRange.IsEmpty() {
		from := hexutil.Uint64(uncachedRange.L())
		to := hexutil.Uint64(uncachedRange.R())
		nextResults, err := ResultsFromNext[*evm.PackedBlock](ctx, "trace_filterPacked", &evm.TraceFilterArgs{
			FromBlock:   &from,
			ToBlock:     &to,
			FromAddress: args.FromAddress,
			ToAddress:   args.ToAddress,
			After:       args.After,
			Count:       args.Count,
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
