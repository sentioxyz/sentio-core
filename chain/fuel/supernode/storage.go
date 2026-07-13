package supernode

import (
	"context"
	"sentioxyz/sentio-core/chain/fuel"
)

type Storage interface {
	// QueryTransactions returns at most limit (0 = unlimited) matching transactions, counted after
	// post-filtering so a truncated result is always a prefix of the full result. It performs no
	// limit check itself: the super node passes its record cap + 1 (chain.StoreQueryLimit) and
	// detects an over-cap query from the record count.
	QueryTransactions(
		ctx context.Context,
		startBlock uint64,
		endBlock uint64,
		filters []fuel.TransactionFilter,
		limit int,
	) (result []fuel.WrappedTransaction, err error)
	QueryContractCreateTransaction(ctx context.Context, contractID string) (result *fuel.WrappedTransaction, err error)
}
