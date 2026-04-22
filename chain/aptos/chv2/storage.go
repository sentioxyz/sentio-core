package chv2

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
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
	tables     []chx.FullName
	ctrl       chx.Controller
	rangeStore chain.RangeStore

	cachedTxVersionLock      sync.Mutex
	cachedTxVersionRange     rg.Range
	cachedTxVersionRangeTime time.Time

	statistic
}

func NewStore(
	connCtrl chx.Controller,
	tableNamePrefix string,
	rangeStore chain.RangeStore,
) *Store {
	s := &Store{
		tables:     tableNames(connCtrl.GetDatabase(), tableNamePrefix),
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
		s.tables[BlockTableIdx].InSQL())
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

const getTransactionsMaxReturn = 500

func (s *Store) queryTransactions(
	ctx context.Context,
	includeEvent bool,
	includeChanges bool,
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
	sql := fmt.Sprintf("SELECT `%s` FROM %s WHERE %s ORDER BY transaction_version LIMIT %d",
		strings.Join(selectFields, "`,`"),
		s.tables[TransactionTableIdx].InSQL(),
		where,
		getTransactionsMaxReturn)
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var tx Transaction
		if scanErr := rows.Scan(objectx.CollectFieldPointers(&tx, filter)...); scanErr != nil {
			return scanErr
		}
		var raw api.CommittedTransaction
		if raw, err = tx.toRawTransaction(); err != nil {
			return err
		}
		result = append(result, aptos.NewTransaction(&raw))
		return nil
	}, sql, args...)
	if err != nil {
		return nil, err
	}
	s.recordQueryTx(ctx, time.Since(startAt), len(result))
	if len(result) >= getTransactionsMaxReturn {
		return nil, errors.Errorf("transactions in query result exceeds the limit %d, please decrease the version range",
			getTransactionsMaxReturn)
	}
	return result, nil
}

func (s *Store) Functions(ctx context.Context, req aptos.GetFunctionsArgs) ([]*aptos.Transaction, error) {
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
		whereArgs = append(whereArgs, strings.ToLower(req.Sender))
	}
	if !req.IncludeMultiSigFunc {
		wheres = append(wheres, "payload_type != ?")
		whereArgs = append(whereArgs, api.TransactionPayloadVariantMultisig)
	}
	if !req.IncludeFailedTransaction {
		wheres = append(wheres, "success")
	}
	// actually query clickhouse
	txs, err := s.queryTransactions(ctx, req.IncludeAllEvents, req.IncludeChanges, strings.Join(wheres, " AND "), whereArgs...)
	if err != nil {
		return nil, err
	}
	// post filter and adjust
	var result []*aptos.Transaction
	txFilter := req.TxnFilter()
	changesFilter := req.ChangeFilter()
	for i := range txs {
		if !txFilter(&txs[i]) {
			continue
		}
		txs[i].Changes = utils.FilterArr(txs[i].Changes, changesFilter)
		result = append(result, &txs[i])
	}
	return result, nil
}

func (s *Store) FullEvents(ctx context.Context, req aptos.GetEventsArgs) ([]*aptos.Transaction, error) {
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
	// actually query clickhouse
	txs, err := s.queryTransactions(ctx, true, req.IncludeChanges, strings.Join(wheres, " AND "), whereArgs...)
	if err != nil {
		return nil, err
	}
	// post filter and adjust
	var result []*aptos.Transaction
	eventsFilter := req.EventFilter()
	changesFilter := req.ChangeFilter()
	for i := range txs {
		events := utils.FilterArr(txs[i].Events, eventsFilter)
		if len(events) == 0 {
			continue
		}
		if !req.IncludeAllEvents {
			txs[i].Events = events
		}
		txs[i].Changes = utils.FilterArr(txs[i].Changes, changesFilter)
		result = append(result, &txs[i])
	}
	return result, nil
}

func (s *Store) ResourceChanges(ctx context.Context, req aptos.ResourceChangeArgs) ([]*aptos.Transaction, error) {
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
				return nil, fmt.Errorf("resourceChangesMoveTypePrefix is required when address is *")
			}
		} else {
			wheres = append(wheres, "hasAny(change_addresses, ?)")
			whereArgs = append(whereArgs, utils.MapSliceNoError(req.Addresses, strings.ToLower))
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
		return nil, fmt.Errorf("addresses is required")
	}
	// actually query clickhouse
	txs, err := s.queryTransactions(ctx, false, true, strings.Join(wheres, " AND "), whereArgs...)
	if err != nil {
		return nil, err
	}
	// post filter and adjust
	var result []*aptos.Transaction
	changesFilter := req.ChangeFilter()
	for i := range txs {
		txs[i].Changes = utils.FilterArr(txs[i].Changes, changesFilter)
		if len(txs[i].Changes) > 0 {
			result = append(result, &txs[i])
		}
	}
	return result, nil
}

func (s *Store) GetTransactionByVersion(ctx context.Context, txVersion uint64) (*aptos.Transaction, error) {
	if err := s.checkInRange(ctx, txVersion); err != nil {
		return nil, err
	}
	txs, err := s.queryTransactions(ctx, true, true, "transaction_version = ?", txVersion)
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
		"FROM %s WHERE transaction_version >= ? AND address = ?", s.tables[ChangeTableIdx].InSQL())
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
		s.tables[ChangeTableIdx].InSQL())
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
		"FROM %s WHERE transaction_version = ?", s.tables[TransactionTableIdx].InSQL())
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
			whereArgs = append(whereArgs, *ff.Sender)
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
			whereArgs = append(whereArgs, *ef.GuiAccountAddress)
		}
		filters = append(filters, strings.Join(parts, " AND "))
	}
	if len(filters) > 0 {
		where = where + " AND (" + strings.Join(filters, " OR ") + ")"
	}
	// actually query clickhouse
	txs, err := s.queryTransactions(
		ctx,
		req.FetchConfig.NeedAllEvents || len(req.Filter.EventFilters) > 0,
		len(req.FetchConfig.ChangeResourceTypes) > 0,
		where,
		whereArgs...)
	if err != nil {
		return nil, err
	}
	// post filter and adjust
	var result []aptos.Transaction
	for _, tx := range txs {
		if !req.Filter.Check(tx) {
			continue
		}
		tx = req.FetchConfig.PruneTransaction(tx, req.Filter.EventFilters)
		result = append(result, tx)
	}
	return result, nil
}

func (s *Store) QueryResourceChanges(
	ctx context.Context,
	req aptos.GetResourceChangesRequest,
) (results []aptos.MinimalistTransactionWithChanges, err error) {
	if err = s.checkInRange(ctx, req.FromVersion, req.ToVersion); err != nil {
		return nil, err
	}
	where := "transaction_version >= ? AND transaction_version <= ?"
	args := []any{req.FromVersion, req.ToVersion}
	if !req.Filter.Address.Empty() {
		where += " AND hasAny(change_addresses, ?)"
		args = append(args, utils.MapSliceNoError(req.Filter.Address.DumpValues(), strings.ToLower))
	}
	if len(req.Filter.ResourceTypes) > 0 {
		where += " AND hasAny(resource_raw_type, ?)"
		args = append(args, utils.MapSliceNoError(req.Filter.ResourceTypes, move.Type.Main))
	}
	sql := fmt.Sprintf("SELECT transaction_version, transaction_hash, timestamp, changes "+
		"FROM %s WHERE %s ORDER BY transaction_version",
		s.tables[TransactionTableIdx].InSQL(), where)
	startAt := time.Now()
	var count int
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var tx aptos.MinimalistTransactionWithChanges
		var ts time.Time
		var rawChanges []string
		if scanErr := rows.Scan(&tx.Version, &tx.Hash, &ts, &rawChanges); scanErr != nil {
			return scanErr
		}
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
	return results, nil
}
