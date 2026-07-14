package chv2

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	aptosSdk "github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/objectx"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"sync"
	"time"
)

type Store struct {
	ctrl       chx.Controller
	rangeStore chain.RangeStore

	cachedTxVersionLock      sync.Mutex
	cachedTxVersionRange     rg.Range
	cachedTxVersionRangeTime time.Time

	statistic
}

func NewStore(
	connCtrl chx.Controller,
	rangeStore chain.RangeStore,
) *Store {
	s := &Store{
		ctrl:       connCtrl,
		rangeStore: rangeStore,
	}
	s.statistic.init()
	return s
}

const txVersionRangeCacheDur = time.Second * 10

func (s *Store) getTxVersionRange(ctx context.Context) (rg.Range, error) {
	s.cachedTxVersionLock.Lock()
	defer s.cachedTxVersionLock.Unlock()
	if time.Since(s.cachedTxVersionRangeTime) < txVersionRangeCacheDur {
		return s.cachedTxVersionRange, nil
	}
	blockRange, err := s.rangeStore.Get(ctx)
	if err != nil {
		return rg.EmptyRange, err
	}
	sql := fmt.Sprintf("SELECT block_height, first_version, last_version FROM %s WHERE block_height IN (?, ?)",
		s.ctrl.FullLogicName(tableNameBlocks))
	var blocks []Block
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var blk Block
		if scanErr := rows.Scan(&blk.BlockHeight, &blk.FirstVersion, &blk.LastVersion); scanErr != nil {
			return scanErr
		}
		blocks = append(blocks, blk)
		return nil
	}, sql, blockRange.Start, *blockRange.End)
	if err != nil {
		return rg.EmptyRange, err
	}
	if len(blocks) != 2 {
		return rg.EmptyRange, errors.Errorf("miss block %d or %d", blockRange.Start, *blockRange.End)
	}
	txVersionRange := rg.NewRange(
		min(blocks[0].FirstVersion, blocks[1].FirstVersion),
		max(blocks[0].LastVersion, blocks[1].LastVersion),
	)
	s.cachedTxVersionRange, s.cachedTxVersionRangeTime = txVersionRange, time.Now()
	return txVersionRange, nil
}

func (s *Store) checkInRange(ctx context.Context, txVersions ...uint64) error {
	if len(txVersions) == 0 {
		return nil
	}
	curRange, err := s.getTxVersionRange(ctx)
	if err != nil {
		return errors.Wrapf(err, "get tx version range failed")
	}
	for _, txVersion := range txVersions {
		if !curRange.Contains(txVersion) {
			return errors.Errorf("out of range while query clickhouse, %d not in %s", txVersion, curRange)
		}
	}
	return nil
}

// queryTransactions loads the matching transactions, scanning at most limit raw rows
// (0 = unlimited, pushed down as a SQL LIMIT to bound the ClickHouse-side resource use of one
// query). When the scan hits the limit it fails with chain.NewTooManyResultsError — the raw rows
// are counted before the Go-side post filter (nil = keep all; it may mutate the transaction, e.g.
// to trim its events/changes), so the check is conservative, but a returned result is always
// complete. The super node passes its record cap + 1 (chain.StoreQueryLimit), so a query matching
// exactly the cap still succeeds.
func (s *Store) queryTransactions(
	ctx context.Context,
	includeEvent bool,
	includeChanges bool,
	postFilter func(*aptos.Transaction) bool,
	limit int,
	where string,
	args ...any,
) (result []aptos.Transaction, err error) {
	filter := objectx.HasTag("clickhouse").And(objectx.AnyHasTagEqualTo("required", "true"))
	if !includeEvent {
		filter = filter.And(objectx.TagNotEqualTo("clickhouse", "events"))
	}
	if !includeChanges {
		filter = filter.And(objectx.TagNotEqualTo("clickhouse", "changes"))
	}
	selectFields := objectx.CollectTagValue(Transaction{}, "clickhouse", filter)
	startAt := time.Now()
	sql := fmt.Sprintf("SELECT `%s` FROM %s WHERE %s ORDER BY transaction_version",
		strings.Join(selectFields, "`,`"),
		s.ctrl.FullLogicName(tableNameTransactions),
		where)
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	var rawRows int
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var tx Transaction
		if scanErr := rows.Scan(objectx.CollectFieldPointers(&tx, filter)...); scanErr != nil {
			return scanErr
		}
		rawRows++
		var raw api.CommittedTransaction
		if raw, err = tx.toRawTransaction(); err != nil {
			return err
		}
		res := aptos.NewTransaction(&raw)
		if postFilter == nil || postFilter(&res) {
			result = append(result, res)
		}
		return nil
	}, sql, args...)
	if err != nil {
		return nil, err
	}
	s.recordQueryTx(ctx, time.Since(startAt), len(result))
	if limit > 0 && rawRows >= limit {
		return nil, chain.NewTooManyResultsError()
	}
	return result, nil
}

func (s *Store) Functions(
	ctx context.Context,
	req aptos.GetFunctionsArgs,
	limit int,
) ([]*aptos.Transaction, error) {
	if err := s.checkInRange(ctx, req.FromVersion, req.ToVersion); err != nil {
		return nil, err
	}
	// build where conditions
	wheres := []string{"transaction_version >= ?", "transaction_version <= ?"}
	whereArgs := []any{req.FromVersion, req.ToVersion}
	if req.Function != "" && req.Function != "*" {
		if len(strings.Split(req.Function, "::")) == 3 {
			wheres = append(wheres, "entry_function = ?")
			whereArgs = append(whereArgs, req.Function)
		} else {
			wheres = append(wheres, "entry_function LIKE ?")
			whereArgs = append(whereArgs, req.Function+"%")
		}
	}
	// match all actually means ignore typed arguments
	if !req.MatchAll && len(req.TypedArguments) > 0 {
		wheres = append(wheres, "hasAll(entry_function_type_arguments, ?)")
		whereArgs = append(whereArgs, req.TypedArguments)
	}
	if req.Sender != "" {
		wheres = append(wheres, "sender = ?")
		whereArgs = append(whereArgs, aptos.NormalizeAccountAddress(req.Sender))
	}
	if !req.IncludeMultiSigFunc {
		wheres = append(wheres, "payload_type != ?")
		whereArgs = append(whereArgs, api.TransactionPayloadVariantMultisig)
	}
	if !req.IncludeFailedTransaction {
		wheres = append(wheres, "success")
	}
	// actually query clickhouse, post-filtering and adjusting each transaction in the scan
	txFilter := req.TxnFilter()
	changesFilter := req.ChangeFilter()
	txs, err := s.queryTransactions(ctx, req.IncludeAllEvents, req.IncludeChanges,
		func(tx *aptos.Transaction) bool {
			if !txFilter(tx) {
				return false
			}
			tx.Changes = utils.FilterArr(tx.Changes, changesFilter)
			return true
		}, limit, strings.Join(wheres, " AND "), whereArgs...)
	if err != nil {
		return nil, err
	}
	return utils.WrapPointerForArray(txs), nil
}

func (s *Store) FullEvents(
	ctx context.Context,
	req aptos.GetEventsArgs,
	limit int,
) ([]*aptos.Transaction, error) {
	if err := s.checkInRange(ctx, req.FromVersion, req.ToVersion); err != nil {
		return nil, err
	}
	// build where conditions
	wheres := []string{"transaction_version >= ?", "transaction_version <= ?"}
	whereArgs := []any{req.FromVersion, req.ToVersion}
	var eventType string
	if len(req.Address) > 0 && len(req.Type) > 0 {
		eventType = fmt.Sprintf("%s::%s", req.Address, req.Type)
		wheres = append(wheres, "has(event_raw_type, ?)")
		whereArgs = append(whereArgs, move.RemoveTypeArgs(eventType))
	}
	if !req.IncludeFailedTransaction {
		wheres = append(wheres, "success")
	}
	if len(req.AccountAddress) > 0 {
		wheres = append(wheres, "arrayExists(x -> JSONExtractString(x, 'guid', 'account_address') = ?, events)")
		whereArgs = append(whereArgs, req.AccountAddress)
	}
	// actually query clickhouse, post-filtering and adjusting each transaction in the scan
	eventsFilter := req.EventFilter()
	changesFilter := req.ChangeFilter()
	txs, err := s.queryTransactions(ctx, true, req.IncludeChanges,
		func(tx *aptos.Transaction) bool {
			events := utils.FilterArr(tx.Events, eventsFilter)
			if len(events) == 0 {
				return false
			}
			if !req.IncludeAllEvents {
				tx.Events = events
			}
			tx.Changes = utils.FilterArr(tx.Changes, changesFilter)
			return true
		}, limit, strings.Join(wheres, " AND "), whereArgs...)
	if err != nil {
		return nil, err
	}
	return utils.WrapPointerForArray(txs), nil
}

func (s *Store) ResourceChanges(
	ctx context.Context,
	req aptos.ResourceChangeArgs,
	limit int,
) ([]*aptos.Transaction, error) {
	if err := s.checkInRange(ctx, req.FromVersion, req.ToVersion); err != nil {
		return nil, err
	}
	// build where conditions
	wheres := []string{"transaction_version >= ?", "transaction_version <= ?"}
	whereArgs := []any{req.FromVersion, req.ToVersion}
	if len(req.Addresses) > 0 {
		if strings.TrimSpace(req.Addresses[0]) == "*" {
			// * = bind all address
			if len(req.ResourceChangesMoveTypePrefix) == 0 {
				return nil, errors.Errorf("resourceChangesMoveTypePrefix is required when address is *")
			}
		} else {
			wheres = append(wheres, "hasAny(change_addresses, ?)")
			whereArgs = append(whereArgs, utils.MapSliceNoError(req.Addresses, aptos.NormalizeAccountAddress))
		}
		if len(req.ResourceChangesMoveTypePrefix) > 0 {
			if strings.Contains(req.ResourceChangesMoveTypePrefix, "<") {
				wheres = append(wheres, "has(resource_type, ?)")
				whereArgs = append(whereArgs, req.ResourceChangesMoveTypePrefix)
			} else {
				// resource_raw_type has striped generic type already, so here we use equal
				wheres = append(wheres, "has(resource_raw_type, ?)")
				whereArgs = append(whereArgs, move.RemoveTypeArgs(req.ResourceChangesMoveTypePrefix))
			}
		}
	} else {
		return nil, errors.Errorf("addresses is required")
	}
	// actually query clickhouse, post-filtering and adjusting each transaction in the scan
	changesFilter := req.ChangeFilter()
	txs, err := s.queryTransactions(ctx, false, true,
		func(tx *aptos.Transaction) bool {
			tx.Changes = utils.FilterArr(tx.Changes, changesFilter)
			return len(tx.Changes) > 0
		}, limit, strings.Join(wheres, " AND "), whereArgs...)
	if err != nil {
		return nil, err
	}
	return utils.WrapPointerForArray(txs), nil
}

func (s *Store) GetTransactionByVersion(ctx context.Context, txVersion uint64) (*aptos.Transaction, error) {
	if err := s.checkInRange(ctx, txVersion); err != nil {
		return nil, err
	}
	txs, err := s.queryTransactions(ctx, true, true, nil, 0, "transaction_version = ?", txVersion)
	if err != nil {
		return nil, err
	}
	if len(txs) == 0 {
		return nil, nil
	}
	return &txs[0], nil
}

func (s *Store) GetChangeStat(ctx context.Context, minTxVersion uint64, address string) (aptos.ChangeStat, error) {
	sql := fmt.Sprintf("SELECT "+
		"min(transaction_version), "+
		"max(transaction_version), "+
		"min(block_height), "+
		"max(block_height), "+
		"count(*) "+
		"FROM %s WHERE transaction_version >= ? AND address = ?", s.ctrl.FullLogicName(tableNameChanges))
	var cs aptos.ChangeStat
	startAt := time.Now()
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&cs.MinTxVersion, &cs.MaxTxVersion, &cs.MinBlockHeight, &cs.MaxBlockHeight, &cs.Count)
	}, sql, minTxVersion, address)
	s.recordQueryChangeStat(ctx, time.Since(startAt))
	return cs, err
}

func (s *Store) GetFirstChange(
	ctx context.Context,
	address string,
	maxTxVersion uint64,
) (version, blockHeight uint64, has bool, err error) {
	if err = s.checkInRange(ctx, maxTxVersion); err != nil {
		return
	}
	sql := fmt.Sprintf("SELECT transaction_version, block_height "+
		"FROM %s WHERE transaction_version <= ? AND address = ? ORDER BY transaction_version LIMIT 1",
		s.ctrl.FullLogicName(tableNameChanges))
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		has = true
		return rows.Scan(&version, &blockHeight)
	}, sql, maxTxVersion, address)
	return
}

func (s *Store) QueryMinimalistTransaction(
	ctx context.Context,
	txVersion uint64,
) (*aptos.MinimalistTransaction, error) {
	if err := s.checkInRange(ctx, txVersion); err != nil {
		return nil, err
	}
	sql := fmt.Sprintf("SELECT transaction_hash, timestamp "+
		"FROM %s WHERE transaction_version = ?", s.ctrl.FullLogicName(tableNameTransactions))
	var txs []aptos.MinimalistTransaction
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var tx aptos.MinimalistTransaction
		var ts time.Time
		if scanErr := rows.Scan(&tx.Hash, &ts); scanErr != nil {
			return scanErr
		}
		tx.Version = txVersion
		tx.TimestampMS = ts.UnixMicro()
		txs = append(txs, tx)
		return nil
	}, sql, txVersion)
	if err != nil {
		return nil, err
	}
	if len(txs) == 0 {
		return nil, nil
	}
	return &txs[0], nil
}

func (s *Store) QueryTransactions(
	ctx context.Context,
	req aptos.GetTransactionsRequest,
	limit int,
) ([]aptos.Transaction, error) {
	if err := s.checkInRange(ctx, req.FromVersion, req.ToVersion); err != nil {
		return nil, err
	}
	// build where conditions
	where := "transaction_version >= ? AND transaction_version <= ?"
	whereArgs := []any{req.FromVersion, req.ToVersion}
	if !req.Filter.FailedIsOK {
		where += " AND success"
	}
	if !req.Filter.MultiSigTxnIsOK {
		where += " AND payload_type != ?"
		whereArgs = append(whereArgs, string(api.TransactionPayloadVariantMultisig))
	}
	if len(req.Filter.EventFilters) > 0 {
		where += " AND event_count > 0"
	}
	var filters []string
	for _, ff := range req.Filter.FunctionFilters {
		if ff.IsEmpty() {
			filters = nil
			break
		}
		var parts []string
		if !ff.FunctionPattern.IsAny() {
			if ff.FunctionPattern.HasAny() {
				parts = append(parts, "entry_function LIKE ?")
				whereArgs = append(whereArgs, strings.ReplaceAll(ff.FunctionPattern.String(), "*", "%"))
			} else {
				parts = append(parts, "entry_function = ?")
				whereArgs = append(whereArgs, ff.FunctionPattern.String())
			}
		}
		if ff.CheckTypeArguments {
			parts = append(parts, "entry_function_type_arguments = ?")
			whereArgs = append(whereArgs, ff.TypedArguments)
		}
		if ff.Sender != nil {
			parts = append(parts, "sender = ?")
			whereArgs = append(whereArgs, ff.Sender.String())
		}
		filters = append(filters, strings.Join(parts, " AND "))
	}
	for _, ef := range req.Filter.EventFilters {
		if ef.IsEmpty() {
			return nil, errors.Errorf("has empty event filter")
		}
		var parts []string
		if !ef.Type.IsAny() {
			if main := ef.Type.Main(); ef.Type.MainHasAny() {
				parts = append(parts, "arrayExists(x -> x LIKE ?, event_raw_type)")
				whereArgs = append(whereArgs, strings.ReplaceAll(main, "*", "%"))
			} else {
				parts = append(parts, "has(event_raw_type, ?)")
				whereArgs = append(whereArgs, main)
			}
		}
		if ef.GuiAccountAddress != nil {
			parts = append(parts, "arrayExists(x -> JSONExtractString(x, 'guid', 'account_address') = ?, events)")
			whereArgs = append(whereArgs, ef.GuiAccountAddress.String())
		}
		filters = append(filters, strings.Join(parts, " AND "))
	}
	if len(filters) > 0 {
		where = where + " AND (" + strings.Join(filters, " OR ") + ")"
	}
	// actually query clickhouse, post-filtering and pruning each transaction in the scan
	return s.queryTransactions(
		ctx,
		req.FetchConfig.NeedAllEvents || len(req.Filter.EventFilters) > 0,
		len(req.FetchConfig.ChangeResourceTypes) > 0,
		func(tx *aptos.Transaction) bool {
			if !req.Filter.Check(*tx) {
				return false
			}
			*tx = req.FetchConfig.PruneTransaction(*tx, req.Filter.EventFilters)
			return true
		},
		limit,
		where,
		whereArgs...)
}

func (s *Store) QueryResourceChanges(
	ctx context.Context,
	req aptos.GetResourceChangesRequest,
	limit int,
) (results []aptos.MinimalistTransactionWithChanges, err error) {
	if err = s.checkInRange(ctx, req.FromVersion, req.ToVersion); err != nil {
		return nil, err
	}
	where := "transaction_version >= ? AND transaction_version <= ?"
	args := []any{req.FromVersion, req.ToVersion}
	if !req.Filter.Address.Empty() {
		where += " AND hasAny(change_addresses, ?)"
		addresses := utils.MapSliceNoError(req.Filter.Address.DumpValues(), func(addr aptosSdk.AccountAddress) string {
			return addr.String()
		})
		args = append(args, addresses)
	}
	if len(req.Filter.ResourceTypes) > 0 {
		where += " AND hasAny(resource_raw_type, ?)"
		args = append(args, utils.MapSliceNoError(req.Filter.ResourceTypes, move.Type.Main))
	}
	sql := fmt.Sprintf("SELECT transaction_version, transaction_hash, timestamp, changes "+
		"FROM %s WHERE %s ORDER BY transaction_version",
		s.ctrl.FullLogicName(tableNameTransactions), where)
	// like queryTransactions, the SQL LIMIT bounds the ClickHouse-side resource use of one query;
	// hitting it fails with chain.NewTooManyResultsError below
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	startAt := time.Now()
	var rawRows, count int
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var tx aptos.MinimalistTransactionWithChanges
		var ts time.Time
		var rawChanges []string
		if scanErr := rows.Scan(&tx.Version, &tx.Hash, &ts, &rawChanges); scanErr != nil {
			return scanErr
		}
		rawRows++
		tx.TimestampMS = ts.UnixMicro()
		for i, rawChange := range rawChanges {
			var change aptos.WriteSetChange
			parseErr := json.Unmarshal([]byte(rawChange), &change)
			if parseErr != nil {
				return errors.Wrapf(parseErr, "parse #%d change data in txn %d failed", i, tx.Version)
			}
			// post filter and adjust
			if req.Filter.Check(&change) {
				tx.Changes = append(tx.Changes, change)
			}
		}
		count += len(tx.Changes)
		if len(tx.Changes) > 0 {
			results = append(results, tx)
		}
		return nil
	}, sql, args...)
	s.recordQueryChanges(ctx, time.Since(startAt), count)
	if err != nil {
		return nil, err
	}
	if limit > 0 && rawRows >= limit {
		return nil, chain.NewTooManyResultsError()
	}
	return results, nil
}
