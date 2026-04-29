package supernode

import (
	"context"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"sentioxyz/sentio-core/chain/evm"
	"time"
)

type Storage interface {
	QueryBlocks(ctx context.Context, where string, args ...any) ([]evm.ExtendedHeader, error)
	QueryBlockTxHashes(ctx context.Context, blockNumber uint64) ([]string, error)
	QueryTxs(ctx context.Context, where string, args ...any) ([]evm.ExtendedTransaction, error)
	QueryLogs(ctx context.Context, where string, args ...any) ([]types.Log, error)
	QueryLogsBlockSQL(where string) string
	QueryTraces(ctx context.Context, where string, args ...any) ([]evm.ParityTrace, error)
	QueryTracesBlockSQL(where string) string

	// QuerySimpleTrace used to query traces by address and some other conditions,
	// each transaction only return the first trace match the condition.
	// The result order by block_number DESC, transaction_index DESC
	QuerySimpleTrace(ctx context.Context, where string, limit int) ([]evm.SimpleTrace, error)

	// QueryEstimateBlockNumberAtDate Find the smallest block with timestamp >= targetTimestampMs (lessEqual is false) or
	// the biggest block with timestamp <= targetTimestampMs (lessEqual is true) in the interval [startBlock,endBlock].
	// If there is no block match the condition, null will be returned.
	QueryEstimateBlockNumberAtDate(
		ctx context.Context,
		targetTime time.Time,
		startBlock uint64,
		endBlock uint64,
		lessEqual bool,
	) (*rpc.BlockNumber, error)
}
