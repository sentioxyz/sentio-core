package supernode

import (
	"context"
	"sentioxyz/sentio-core/chain/fuel"
)

type Storage interface {
	// QueryTransactions returns chain.NewTooManyResultsError when the query matches more than
	// limit transactions (limit <= 0 means unlimited), so an over-dense range aborts early
	// instead of materializing an unbounded result.
	QueryTransactions(
		ctx context.Context,
		startBlock uint64,
		endBlock uint64,
		filters []fuel.TransactionFilter,
		limit int,
	) (result []fuel.WrappedTransaction, err error)
	QueryContractCreateTransaction(ctx context.Context, contractID string) (result *fuel.WrappedTransaction, err error)
}
