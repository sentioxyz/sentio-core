package supernode

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio/chain/evm"
	"strings"
)

type TraceClickHouse struct {
	baseClickhouseService
}

func NewTraceClickHouseMiddleware(base baseClickhouseService) jsonrpc.Middleware {
	return jsonrpc.MakeServiceAsMiddleware("trace", &TraceClickHouse{
		baseClickhouseService: base,
	})
}

func (m *TraceClickHouse) filterTraceSQL(args *evm.TraceFilterArgs) string {
	var wheres []string
	wheres = append(wheres, fmt.Sprintf("block_number >= %d", *args.FromBlock))
	wheres = append(wheres, fmt.Sprintf("block_number <= %d", *args.ToBlock))
	if len(args.FromAddress) > 0 {
		addresses := utils.MapSliceNoError(args.FromAddress, func(addr common.Address) string {
			return strings.ToLower(addr.Hex())
		})
		wheres = append(wheres, fmt.Sprintf("lower(from_address) in ('%s')", strings.Join(addresses, "','")))
	}
	if len(args.ToAddress) > 0 {
		addresses := utils.MapSliceNoError(args.ToAddress, func(addr string) string {
			return strings.ToLower(addr)
		})
		wheres = append(wheres, fmt.Sprintf("lower(to_address) in ('%s')", strings.Join(addresses, "','")))
	}
	return strings.Join(wheres, " AND ")
}

func (m *TraceClickHouse) Filter(ctx context.Context, args *evm.TraceFilterArgs) ([]evm.ParityTrace, error) {
	if args.After != nil || args.Count != nil {
		return nil, fmt.Errorf("after and count is not supported")
	}
	if args.FromBlock == nil || args.ToBlock == nil {
		return nil, fmt.Errorf("fromBlock and toBlock are both required")
	}
	return m.store.QueryTraces(ctx, m.filterTraceSQL(args))
}

func (m *TraceClickHouse) FilterPacked(
	ctx context.Context,
	args *evm.TraceFilterArgs,
	needTransaction bool,
	needReceipt bool,
	needReceiptLogs bool,
) ([]*evm.PackedBlock, error) {
	if args.After != nil || args.Count != nil {
		return nil, fmt.Errorf("after and count is not supported")
	}
	if args.FromBlock == nil || args.ToBlock == nil {
		return nil, fmt.Errorf("fromBlock and toBlock are both required")
	}

	var headers []evm.ExtendedHeader
	var txs []evm.ExtendedTransaction
	var fullLogs []types.Log
	var traces []evm.ParityTrace
	var err error

	tracesWhere := m.filterTraceSQL(args)
	traces, err = m.store.QueryTraces(ctx, tracesWhere)
	if err != nil {
		return nil, err
	}
	if len(traces) == 0 {
		return nil, nil
	}

	blockNumbers := utils.BuildSet(utils.MapSliceNoError(traces, func(trace evm.ParityTrace) uint64 {
		return trace.BlockNumber
	}))
	where := fmt.Sprintf("block_number >= %d AND block_number <= %d", *args.FromBlock, *args.ToBlock)
	if len(blockNumbers) > 100 {
		where = fmt.Sprintf("%s AND block_number IN (%s)",
			where, m.store.QueryTracesBlockSQL(tracesWhere))
	} else {
		where = fmt.Sprintf("%s AND block_number IN (%s)",
			where, strings.Join(utils.MapSliceNoError(utils.GetMapKeys(blockNumbers), utils.UIntFormatter(10)), ","))
	}

	headers, err = m.store.QueryBlocks(ctx, where)
	if err != nil {
		return nil, err
	}
	if needTransaction {
		txs, err = m.store.QueryTxs(ctx, where)
		if err != nil {
			return nil, err
		}
		if !needReceipt {
			for i := range txs {
				txs[i].ExtendedReceipt = nil
			}
		} else if needReceiptLogs {
			fullLogs, err = m.store.QueryLogs(ctx, where)
			if err != nil {
				return nil, err
			}
		}
	}
	return buildPackedBlocks(headers, txs, nil, fullLogs, traces)
}
