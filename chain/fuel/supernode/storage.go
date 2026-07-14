package supernode

import (
	"context"
	"sentioxyz/sentio-core/chain/fuel"
)

type Storage interface {
	// QueryTransactions scans at most limit raw rows (0 = unlimited; a SQL LIMIT bounding the
	// ClickHouse-side resource use of one query) and fails with chain.NewTooManyResultsError when
	// the scan hits it, so a returned result is always complete. The super node passes its record
	// cap + 1 (chain.StoreQueryLimit), so a query matching exactly the cap still succeeds.
	QueryTransactions(
		ctx context.Context,
		startBlock uint64,
		endBlock uint64,
		filters []fuel.TransactionFilter,
		limit int,
	) (result []fuel.WrappedTransaction, err error)
	QueryContractCreateTransaction(ctx context.Context, contractID string) (result *fuel.WrappedTransaction, err error)
}
