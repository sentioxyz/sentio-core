package ch

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/objectx"
	"strings"
	"time"
)

type Store struct {
	ctrl chx.Controller

	statistic
}

func NewStore(connCtrl chx.Controller) *Store {
	s := &Store{ctrl: connCtrl}
	s.init()
	return s
}

func (s *Store) QueryTransactions(
	ctx context.Context,
	startBlock uint64,
	endBlock uint64,
	filters []fuel.TransactionFilter,
	limit int,
) ([]fuel.WrappedTransaction, error) {
	where := "block_height >= ? AND block_height <= ?"
	whereArgs := []any{startBlock, endBlock}
	if len(filters) == 1 {
		if filters[0].ExcludeFailed {
			where = where + " AND status = 'SuccessStatus'"
		}
		if filters[0].CallFilter != nil && filters[0].CallFilter.ContractID != "" {
			where = where + " AND has(call_contracts, ?)"
			whereArgs = append(whereArgs, filters[0].CallFilter.ContractID)
		}
		if filters[0].CallFilter != nil && filters[0].CallFilter.Function != nil {
			where = where + " AND has(call_functions, ?)"
			whereArgs = append(whereArgs, *(filters[0].CallFilter.Function))
		}
		if filters[0].TransferFilter != nil && filters[0].TransferFilter.AssetID != "" {
			where = where + " AND has(assets, ?)"
			whereArgs = append(whereArgs, filters[0].TransferFilter.AssetID)
		}
		if filters[0].TransferFilter != nil && filters[0].TransferFilter.From != "" {
			where = where + " AND has(asset_input_owners, ?)"
			whereArgs = append(whereArgs, filters[0].TransferFilter.From)
		}
		if filters[0].TransferFilter != nil && filters[0].TransferFilter.To != "" {
			where = where + " AND has(asset_output_owners, ?)"
			whereArgs = append(whereArgs, filters[0].TransferFilter.To)
		}
		if filters[0].LogFilter != nil && filters[0].LogRa != nil {
			where = where + " AND has(log_ra_set, ?)"
			whereArgs = append(whereArgs, *(filters[0].LogRa))
		}
		if filters[0].LogFilter != nil && filters[0].LogRb != nil {
			where = where + " AND has(log_rb_set, ?)"
			whereArgs = append(whereArgs, *(filters[0].LogRb))
		}
		if filters[0].LogFilter != nil && filters[0].LogRc != nil {
			where = where + " AND has(log_rc_set, ?)"
			whereArgs = append(whereArgs, *(filters[0].LogRc))
		}
		if filters[0].LogFilter != nil && filters[0].LogRd != nil {
			where = where + " AND has(log_rd_set, ?)"
			whereArgs = append(whereArgs, *(filters[0].LogRd))
		}
	} else if len(filters) > 1 {
		var callContractIDList []string
		var functionList []uint64
		var assetIDList []string
		var fromList []string
		var toList []string
		var logRaSet []uint64
		var logRbSet []uint64
		var logRcSet []uint64
		var logRdSet []uint64
		for _, filter := range filters {
			if filter.CallFilter != nil && filter.CallFilter.ContractID != "" {
				callContractIDList = append(callContractIDList, filter.CallFilter.ContractID)
			}
			if filter.CallFilter != nil && filter.CallFilter.Function != nil {
				functionList = append(functionList, *(filter.CallFilter.Function))
			}
			if filter.TransferFilter != nil && filter.TransferFilter.AssetID != "" {
				assetIDList = append(assetIDList, filter.TransferFilter.AssetID)
			}
			if filter.TransferFilter != nil && filter.TransferFilter.From != "" {
				fromList = append(fromList, filter.TransferFilter.From)
			}
			if filter.TransferFilter != nil && filter.TransferFilter.To != "" {
				toList = append(toList, filter.TransferFilter.To)
			}
			if filter.LogFilter != nil && filter.LogRa != nil {
				logRaSet = append(logRaSet, *filter.LogRa)
			}
			if filter.LogFilter != nil && filter.LogRb != nil {
				logRbSet = append(logRbSet, *filter.LogRb)
			}
			if filter.LogFilter != nil && filter.LogRc != nil {
				logRcSet = append(logRcSet, *filter.LogRc)
			}
			if filter.LogFilter != nil && filter.LogRd != nil {
				logRdSet = append(logRdSet, *filter.LogRd)
			}
		}
		if len(callContractIDList) > 0 {
			where = where + " AND hasAny(call_contracts, ?)"
			whereArgs = append(whereArgs, callContractIDList)
		}
		if len(functionList) > 0 {
			where = where + " AND hasAny(call_functions, ?)"
			whereArgs = append(whereArgs, functionList)
		}
		if len(assetIDList) > 0 {
			where = where + " AND hasAny(assets, ?)"
			whereArgs = append(whereArgs, assetIDList)
		}
		if len(fromList) > 0 {
			where = where + " AND hasAny(asset_input_owners, ?)"
			whereArgs = append(whereArgs, fromList)
		}
		if len(toList) > 0 {
			where = where + " AND hasAny(asset_output_owners, ?)"
			whereArgs = append(whereArgs, toList)
		}
		if len(logRaSet) > 0 {
			where = where + " AND hasAny(log_ra_set, ?)"
			whereArgs = append(whereArgs, logRaSet)
		}
		if len(logRbSet) > 0 {
			where = where + " AND hasAny(log_rb_set, ?)"
			whereArgs = append(whereArgs, logRbSet)
		}
		if len(logRcSet) > 0 {
			where = where + " AND hasAny(log_rc_set, ?)"
			whereArgs = append(whereArgs, logRcSet)
		}
		if len(logRdSet) > 0 {
			where = where + " AND hasAny(log_rd_set, ?)"
			whereArgs = append(whereArgs, logRdSet)
		}
	}
	startAt := time.Now()
	result, err := s.queryTransactions(ctx, func(tx fuel.WrappedTransaction) bool {
		return fuel.CheckTransaction(tx, filters)
	}, limit, where, whereArgs)
	if err != nil {
		return nil, err
	}
	s.recordQueryTx(ctx, time.Since(startAt), len(result))
	return result, nil
}

func (s *Store) QueryContractCreateTransaction(
	ctx context.Context,
	contractID string,
) (*fuel.WrappedTransaction, error) {
	startAt := time.Now()
	result, err := s.queryTransactions(ctx, nil, 1, "is_create AND has(created_contracts, ?)", []any{contractID})
	if err != nil {
		return nil, err
	}
	s.recordQueryContractStart(ctx, time.Since(startAt))
	if len(result) == 0 {
		return nil, nil
	}
	return &result[0], nil
}

// queryTransactions returns at most limit (0 = unlimited) matching transactions; limit counts the
// records that survive postFilter (nil = keep all), so a truncated result is always a prefix of
// the full result. It never checks the limit itself — the super node passes its record cap + 1 and
// detects an over-cap query from the record count (chain.StoreQueryLimit /
// chain.CheckTooManyResults). Without a postFilter the limit is pushed down as a SQL LIMIT.
func (s *Store) queryTransactions(
	ctx context.Context,
	postFilter func(fuel.WrappedTransaction) bool,
	limit int,
	where string,
	args []any,
) (result []fuel.WrappedTransaction, err error) {
	fieldFilter := objectx.HasTag("clickhouse").And(objectx.AnyHasTagEqualTo("required", "true"))
	columns := objectx.CollectTagValue(&ClickhouseTransaction{}, "clickhouse", fieldFilter)
	sql := fmt.Sprintf("SELECT `%s` FROM %s WHERE %s ORDER BY block_height, transaction_index",
		strings.Join(columns, "`,`"),
		s.ctrl.FullLogicName(tableNameTransactions),
		where)
	if limit > 0 && postFilter == nil {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var tx ClickhouseTransaction
		if scanErr := rows.Scan(objectx.CollectFieldPointers(&tx, fieldFilter)...); scanErr != nil {
			return scanErr
		}
		res, parseErr := tx.toWrappedTransaction()
		if parseErr != nil {
			return errors.Wrapf(parseErr, "parse from %d/%s clickhouse transaction failed", tx.BlockHeight, tx.TransactionID)
		}
		if (postFilter == nil || postFilter(res)) && (limit <= 0 || len(result) < limit) {
			result = append(result, res)
		}
		return nil
	}, sql, args...)
	return result, err
}
