package chv4

import (
	"context"
	"fmt"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/objectx"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
)

type Storage struct {
	ctrl       chx.Controller
	rangeStore chain.RangeStore

	statistic
}

func NewStorage(ctrl chx.Controller, rangeStore chain.RangeStore) *Storage {
	s := &Storage{
		ctrl:       ctrl,
		rangeStore: rangeStore,
	}
	s.init()
	return s
}

func (s *Storage) checkRange(ctx context.Context, queryRange rg.Range) error {
	_, logger := log.FromContext(ctx)
	curRange, err := s.rangeStore.Get(ctx)
	if err != nil {
		logger.Errorfe(err, "get current range of clickhouse data source failed")
		return err
	}
	outRangeErrText := "out of range while query clickhouse"
	if queryRange.Start == 0 {
		if !curRange.Contains(*queryRange.End) {
			logger.Errorf("%s, query range is [,%d] but current is %s", outRangeErrText, *queryRange.End, curRange)
			return errors.Errorf("%s, query range is [,%d] but current is %s", outRangeErrText, *queryRange.End, curRange)
		}
	} else if !curRange.Include(queryRange) {
		logger.Errorf("%s, query range is %s but current is %s", outRangeErrText, queryRange, curRange)
		return errors.Errorf("%s, query range is %s but current is %s", outRangeErrText, queryRange, curRange)
	}
	return nil
}

func (s *Storage) QuerySimpleCheckpoint(ctx context.Context, checkpoint uint64) (sui.SimpleCheckpoint, error) {
	if err := s.checkRange(ctx, rg.NewSingleRange(checkpoint)); err != nil {
		return sui.SimpleCheckpoint{}, err
	}
	sql := fmt.Sprintf("SELECT checkpoint_digest, timestamp FROM %s WHERE checkpoint = ?",
		s.ctrl.FullLogicName(tableNameCheckpoints))
	sc := sui.SimpleCheckpoint{Checkpoint: checkpoint}
	var t time.Time
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&sc.Digest, &t)
	}, sql, checkpoint)
	if err != nil {
		return sui.SimpleCheckpoint{}, err
	}
	if sc.Digest == "" {
		return sui.SimpleCheckpoint{}, errors.Errorf("checkpoint %d not found", checkpoint)
	}
	sc.TimestampMS = uint64(t.UnixMilli())
	return sc, nil
}

// queryTransactions loads the matching transactions, scanning at most limit raw rows
// (0 = unlimited, pushed down as a SQL LIMIT to bound the ClickHouse-side resource use of one
// query). When the scan hits the limit it fails with chain.NewTooManyResultsError — the raw rows
// are counted before the Go-side converter, so the check is conservative, but a returned result is
// always complete. The super node passes its record cap + 1 (chain.StoreQueryLimit), so a query
// matching exactly the cap still succeeds.
func (s *Storage) queryTransactions(
	ctx context.Context,
	converter func(Transaction) (*sui.ExtendedGrpcTransaction, bool, error),
	limit int,
	where string,
	args ...any,
) (result []*sui.ExtendedGrpcTransaction, err error) {
	fieldFilter := objectx.HasTag("clickhouse").And(objectx.NoTag("required", "false"))
	columns := objectx.CollectTagValue(&Transaction{}, "clickhouse", fieldFilter)
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY checkpoint, tx_index",
		strings.Join(columns, ","),
		s.ctrl.FullLogicName(tableNameTransactions),
		where)
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	startAt := time.Now()
	var rawRows int
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var tx Transaction
		scanErr := rows.Scan(objectx.CollectFieldPointers(&tx, fieldFilter)...)
		if scanErr != nil {
			return scanErr
		}
		rawRows++
		r, need, convertErr := converter(tx)
		if convertErr != nil {
			return convertErr
		}
		if need {
			result = append(result, r)
		}
		return nil
	}, sql, args...)
	if err == nil && limit > 0 && rawRows >= limit {
		err = chain.NewTooManyResultsError()
	}
	s.recordQueryTx(ctx, time.Since(startAt), len(result))
	return result, err
}

func mergeCondition(parts []string, link string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, link) + ")"
}

func (s *Storage) QueryTransactions(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.TransactionFilter,
	fetchConfig sui.TransactionFetchConfig,
	limit int,
) ([]*sui.ExtendedGrpcTransaction, error) {
	if err := s.checkRange(ctx, rg.NewRange(fromBlock, toBlock)); err != nil {
		return nil, err
	}

	// prepare sql
	var conditions []string
	var args []any
	var filters []string

	// checkpoint range
	conditions = append(conditions, "checkpoint >= ?", "checkpoint <= ?")
	args = append(args, fromBlock, toBlock)
	// filter.FailedIsOK
	if !filter.FailedIsOK {
		conditions = append(conditions, "success")
	}
	// filter.EventFilters
	if len(filter.EventFilters) > 0 {
		var parts []string
		// typePattern; a filter with an empty TypePattern, or with a pattern whose main
		// part is a full wildcard, matches events of any type
		hasEmpty := utils.HasAny(filter.EventFilters, func(ff sui.EventFilterV2) bool {
			return len(ff.TypePattern) == 0 || utils.HasAny(ff.TypePattern, move.Type.MainIsAny)
		})
		if !hasEmpty {
			var exactTypes, likePatterns []string
			for _, ff := range filter.EventFilters {
				for _, typ := range ff.TypePattern {
					if typ.MainHasAny() {
						likePatterns = append(likePatterns, strings.ReplaceAll(typ.Main(), "*", "%"))
					} else {
						exactTypes = append(exactTypes, typ.Main())
					}
				}
			}
			var typeConds []string
			if len(exactTypes) > 0 {
				typeConds = append(typeConds, "hasAny(events_main_type, ?)")
				args = append(args, exactTypes)
			}
			for _, pattern := range likePatterns {
				typeConds = append(typeConds, "arrayExists(x -> x LIKE ?, events_main_type)")
				args = append(args, pattern)
			}
			if len(typeConds) > 0 {
				parts = append(parts, mergeCondition(typeConds, " OR "))
			}
		}

		// sender
		senderSet := set.New[string]()
		for _, ff := range filter.EventFilters {
			if ff.Sender != nil {
				senderSet.Add(*ff.Sender)
			}
		}
		if !senderSet.Empty() {
			parts = append(parts, "hasAny(events_sender, ?)")
			args = append(args, senderSet.DumpValues())
		}

		// build event filter condition
		if len(parts) > 0 {
			filters = append(filters, mergeCondition(parts, " AND "))
		}
	}
	// filter.FunctionFilters
	for _, ff := range filter.FunctionFilters {
		var parts []string
		if ff.Kind != nil {
			parts = append(parts, "kind = ?")
			args = append(args, *ff.Kind)
		}
		if !ff.CommandFilter.IsEmpty() {
			if ff.CommandFilter.CallPackage != nil {
				parts = append(parts, "has(move_calls_package, ?)")
				args = append(args, *ff.CommandFilter.CallPackage)
			}
			if ff.CommandFilter.CallModule != nil {
				parts = append(parts, "has(move_calls_module, ?)")
				args = append(args, *ff.CommandFilter.CallModule)
			}
			if ff.CommandFilter.CallFunction != nil {
				parts = append(parts, "has(move_calls_function, ?)")
				args = append(args, *ff.CommandFilter.CallFunction)
			}
		}
		if ff.MultiSigPublicKeyPrefix != nil {
			parts = append(parts, "length(signatures) = 1")
		}
		if ff.Sender != nil {
			parts = append(parts, "sender = ?")
			args = append(args, *ff.Sender)
		}
		if ff.Receiver != nil {
			parts = append(parts, "has(balance_changes_address, ?)")
			args = append(args, *ff.Receiver)
		}
		if !ff.FailedIsOK {
			parts = append(parts, "success")
		}
		filters = append(filters, mergeCondition(parts, " AND "))
	}
	// append filter part
	if len(filters) > 0 {
		conditions = append(conditions, mergeCondition(filters, " OR "))
	}
	// query data from clickhouse
	return s.queryTransactions(ctx, func(tx Transaction) (*sui.ExtendedGrpcTransaction, bool, error) {
		etx, parseErr := tx.ToExecutedTransaction()
		if parseErr != nil {
			return nil, false, parseErr
		}
		if !filter.CheckGrpcTx(etx.ExecutedTransaction) {
			return nil, false, nil
		}
		return fetchConfig.PruneGrpcTransaction(etx, filter.EventFilters), true, nil
	}, limit, strings.Join(conditions, " AND "), args...)
}

// queryObjectChanges applies limit like queryTransactions (a SQL LIMIT on the raw rows scanned;
// hitting it fails with chain.NewTooManyResultsError).
func (s *Storage) queryObjectChanges(
	ctx context.Context,
	postFilter func(*sui.ExtendedGrpcChangedObject) bool,
	limit int,
	where string,
	args ...any,
) ([]*sui.ExtendedGrpcChangedObject, error) {
	fieldFilter := objectx.HasTag("clickhouse").
		And(objectx.NoTag("clickhouse", "json")).
		And(objectx.NoTag("clickhouse", "package"))
	columns := objectx.CollectTagValue(&Object{}, "clickhouse", fieldFilter)
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY checkpoint, tx_index",
		strings.Join(columns, ","),
		s.ctrl.FullLogicName(tableNameObjects),
		where)
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	startAt := time.Now()
	var rawRows int
	var result []*sui.ExtendedGrpcChangedObject
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var oc Object
		if scanErr := rows.Scan(objectx.CollectFieldPointers(&oc, fieldFilter)...); scanErr != nil {
			return scanErr
		}
		rawRows++
		res := oc.ToChangedObject()
		// post filter
		if postFilter(res) {
			result = append(result, res)
		}
		return nil
	}, sql, args...)
	if err == nil && limit > 0 && rawRows >= limit {
		err = chain.NewTooManyResultsError()
	}
	s.recordQueryObj(ctx, time.Since(startAt), len(result))
	return result, err
}

func (s *Storage) QueryObjectChanges(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.ObjectChangeFilter,
	limit int,
) ([]*sui.ExtendedGrpcChangedObject, error) {
	if err := s.checkRange(ctx, rg.NewRange(fromBlock, toBlock)); err != nil {
		return nil, err
	}

	// prepare sql
	var conditions []string
	var args []any
	var filters []string

	// checkpoint range
	conditions = append(conditions, "checkpoint >= ?")
	conditions = append(conditions, "checkpoint <= ?")
	args = append(args, fromBlock, toBlock)

	// ownerFilter
	if filter.OwnerFilter != nil && len(filter.OwnerFilter.OwnerID) > 0 {
		filters = append(filters, "object_id IN ?")
		args = append(args, filter.OwnerFilter.OwnerID)
		if len(filter.OwnerFilter.OwnerType) > 0 {
			filters = append(filters,
				"((owner_address IN ? OR pre_owner_address IN ?) AND (owner_kind IN ? OR pre_owner_kind IN ?))")
			args = append(args,
				filter.OwnerFilter.OwnerID,
				filter.OwnerFilter.OwnerID,
				filter.OwnerFilter.OwnerType,
				filter.OwnerFilter.OwnerType,
			)
		}
	}
	// typePattern filter
	for _, typ := range filter.TypePattern {
		if typ.HasArgs() && !typ.HasAny() {
			filters = append(filters, "object_type = ?")
			args = append(args, typ.String())
		} else if !typ.MainHasAny() {
			filters = append(filters, "object_main_type = ?")
			args = append(args, typ.Main())
		} else {
			filters = append(filters, "object_main_type LIKE ?")
			args = append(args, strings.ReplaceAll(typ.Main(), "*", "%"))
		}
	}
	// objectIDIn filter
	if filter.ObjectIDIn != nil && !filter.ObjectIDIn.Empty() {
		filters = append(filters, "object_id IN ?")
		args = append(args, filter.ObjectIDIn.DumpValues())
	}
	if len(filters) > 0 {
		conditions = append(conditions, mergeCondition(filters, " OR "))
	}
	checker := filter.CheckerGrpc()
	return s.queryObjectChanges(ctx, func(co *sui.ExtendedGrpcChangedObject) bool {
		return checker(co.ChangedObject)
	}, limit, strings.Join(conditions, " AND "), args...)
}

// QueryLastObjectChange returns the object's newest recorded change with
// object_version <= maxVersion (no bound when maxVersion is 0) and
// checkpoint <= maxCheckpoint, or nil when nothing is recorded. The objects table
// carries the full grpc-derived lifecycle (including wrapped / unwrapped /
// deleted rows). Like the chv3 variant this runs in two steps so both are cheap:
// the version + checkpoint come from the object-id-keyed idv projection (which
// only covers object_id / object_version / checkpoint / change_type), then the
// full row is read with a checkpoint condition so the base-table lookup prunes to
// a single partition.
func (s *Storage) QueryLastObjectChange(
	ctx context.Context,
	objectID string,
	maxVersion uint64,
	maxCheckpoint uint64,
) (*sui.ObjectChangeRecord, error) {
	condition, args := "object_id = ? AND checkpoint <= ?", []any{objectID, maxCheckpoint}
	if maxVersion > 0 {
		condition += " AND object_version <= ?"
		args = append(args, maxVersion)
	}
	sql := fmt.Sprintf("SELECT object_version, checkpoint FROM %s WHERE %s ORDER BY object_version DESC LIMIT 1",
		s.ctrl.FullLogicName(tableNameObjects), condition)
	var version, checkpoint uint64
	var found bool
	if err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		found = true
		return rows.Scan(&version, &checkpoint)
	}, sql, args...); err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	sql = fmt.Sprintf("SELECT tx_digest, change_type, pre_object_version FROM %s "+
		"WHERE checkpoint = ? AND object_id = ? AND object_version = ? LIMIT 1",
		s.ctrl.FullLogicName(tableNameObjects))
	record := &sui.ObjectChangeRecord{Checkpoint: checkpoint, ObjectVersion: version}
	var preVersion uint64
	found = false
	if err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		found = true
		return rows.Scan(&record.TxDigest, &record.Type, &preVersion)
	}, sql, checkpoint, objectID, version); err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.Errorf("object change of %s@%d at checkpoint %d not found", objectID, version, checkpoint)
	}
	// pre_object_version is 0 for created / unwrapped rows (no previous version).
	if preVersion > 0 {
		record.PreviousVersion = &preVersion
	}
	return record, nil
}

func (s *Storage) QueryObjectsStat(
	ctx context.Context,
	fromBlock, toBlock uint64,
	objectIDList []string,
) (map[string]sui.ObjectStat, error) {
	if err := s.checkRange(ctx, rg.NewRange(fromBlock, toBlock)); err != nil {
		return nil, err
	}
	// Because the projection may contain duplicate data, the result of `count(*)` may be biased. The accurate result
	// should be obtained using `count(distinct object_version)`.
	// However, since we only need to check if this value is greater than 0 later, `count(*)` is sufficient, it requires
	// much less memory and is much faster.
	sql := fmt.Sprintf("SELECT object_id, "+
		"count(*), "+
		"min(object_version), "+
		"max(object_version), "+
		"min(checkpoint), "+
		"max(checkpoint) "+
		"FROM %s "+
		"WHERE checkpoint >= ? AND checkpoint <= ? AND object_id IN ? "+
		"GROUP BY object_id",
		s.ctrl.FullLogicName(tableNameObjects))
	result := make(map[string]sui.ObjectStat)
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var r sui.ObjectStat
		var objectID string
		err := rows.Scan(&objectID, &r.Count, &r.MinObjectVersion, &r.MaxObjectVersion, &r.MinCheckpoint, &r.MaxCheckpoint)
		if err != nil {
			return err
		}
		result[objectID] = r
		return nil
	}, sql, fromBlock, toBlock, objectIDList)
	return result, err
}
