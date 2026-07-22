package chv3

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/objectx"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
)

type Controller struct {
	ctrl       chx.Controller
	rangeStore chain.RangeStore

	statistic
}

func NewController(ctrl chx.Controller, rangeStore chain.RangeStore) *Controller {
	c := &Controller{
		ctrl:       ctrl,
		rangeStore: rangeStore,
	}
	c.init()
	return c
}

func (c *Controller) checkRange(ctx context.Context, queryRange rg.Range) error {
	_, logger := log.FromContext(ctx)
	curRange, err := c.rangeStore.Get(ctx)
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

func (c *Controller) QueryCheckpointTime(ctx context.Context, checkpoint uint64) (sui.CheckpointTime, error) {
	if err := c.checkRange(ctx, rg.NewSingleRange(checkpoint)); err != nil {
		return sui.CheckpointTime{}, err
	}
	sql := fmt.Sprintf("SELECT COUNT(*), max(checkpoint_timestamp_ms), min(timestamp_ms), max(timestamp_ms) "+
		"FROM %s WHERE checkpoint = ?", c.ctrl.FullLogicName(tableNameTransactions))
	type Result struct {
		Count          uint64
		CheckpointTime uint64
		MinTxnTime     uint64
		MaxTxnTime     uint64
	}
	var r Result
	err := c.ctrl.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&r.Count, &r.CheckpointTime, &r.MinTxnTime, &r.MaxTxnTime)
	}, sql, checkpoint)
	if err != nil {
		return sui.CheckpointTime{}, err
	}
	if r.Count == 0 {
		return sui.CheckpointTime{}, errors.Errorf("no transaction in checkpoint %d", checkpoint)
	}
	return sui.CheckpointTime{
		CheckpointTime: r.CheckpointTime,
		MinTxnTime:     r.MinTxnTime,
		MaxTxnTime:     r.MaxTxnTime,
	}, nil
}

func (c *Controller) QuerySimpleCheckpoint(ctx context.Context, checkpoint uint64) (sui.SimpleCheckpoint, error) {
	if err := c.checkRange(ctx, rg.NewSingleRange(checkpoint)); err != nil {
		return sui.SimpleCheckpoint{}, err
	}
	sql := fmt.Sprintf("SELECT COUNT(*), max(checkpoint_digest), max(checkpoint_timestamp_ms) "+
		"FROM %s WHERE checkpoint = ?", c.ctrl.FullLogicName(tableNameTransactions))
	var count uint64
	var digest string
	var timestampMS uint64
	err := c.ctrl.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&count, &digest, &timestampMS)
	}, sql, checkpoint)
	if err != nil {
		return sui.SimpleCheckpoint{}, err
	}
	if count == 0 {
		return sui.SimpleCheckpoint{}, errors.Errorf("no transaction in checkpoint %d", checkpoint)
	}
	return sui.SimpleCheckpoint{
		Checkpoint:  checkpoint,
		Digest:      digest,
		TimestampMS: timestampMS,
	}, nil
}

func (c *Controller) QueryTransactions(
	ctx context.Context,
	query *sui.TransactionQuery,
) ([]types.TransactionResponseV1, error) {
	queryRange := rg.NewRange(query.FromSequenceNumber, query.ToSequenceNumber)
	if err := c.checkRange(ctx, queryRange); err != nil {
		return nil, err
	}

	// prepare sql
	var conditions []string
	var args []any

	// checkpoint range
	conditions = append(conditions, "checkpoint >= ?")
	conditions = append(conditions, "checkpoint <= ?")
	args = append(args, query.FromSequenceNumber, query.ToSequenceNumber)
	// Kind
	if query.Kind != "" {
		conditions = append(conditions, "kind = ?")
		args = append(args, query.Kind)
	}
	// MultiSigPublicKeyPrefix
	if len(query.MultiSigPublicKeyPrefix) > 0 {
		conditions = append(conditions, "length(tx_signature) = 1")
	}
	// Sender
	if query.Sender != nil {
		conditions = append(conditions, "sender = ?")
		args = append(args, query.Sender.String())
	}
	// EventFilter
	if query.EventFilter != nil {
		conditions = append(conditions, "notEmpty(events.tx_digest)")
		var pkgIDSet = make(map[string]bool)
		var modSet = make(map[string]bool)
		var senderSet = make(map[string]bool)
		var typeSet = make(map[string]bool)
		var rawTypeSet = make(map[string]bool)
		filters := []*sui.EventFilter{query.EventFilter}
		for i := 0; i < len(filters); i++ {
			f := filters[i]
			switch f.Op {
			case sui.EventFilterAnd, sui.EventFilterOr:
				filters = append(filters, f.Left, f.Right)
			default:
				if f.PackageID != nil {
					pkgIDSet[f.PackageID.String()] = true
				}
				if f.TransactionModule != "" {
					modSet[f.TransactionModule] = true
				}
				if f.Sender != "" {
					senderSet[f.Sender] = true
				}
				if f.Type != nil {
					if f.Type.Struct != nil {
						rawTypeSet[f.Type.Struct.Text(false)] = true
					} else {
						typeSet[f.Type.String()] = true
					}
				}
			}
		}
		if len(pkgIDSet) > 0 {
			conditions = append(conditions, "hasAny(events.package_id, ?)")
			args = append(args, utils.GetMapKeys(pkgIDSet))
		}
		if len(modSet) > 0 {
			conditions = append(conditions, "hasAny(events.transaction_module, ?)")
			args = append(args, utils.GetMapKeys(modSet))
		}
		if len(senderSet) > 0 {
			conditions = append(conditions, "hasAny(events.sender, ?)")
			args = append(args, utils.GetMapKeys(senderSet))
		}
		if len(typeSet) > 0 || len(rawTypeSet) > 0 {
			var parts []string
			if len(typeSet) > 0 {
				parts = append(parts, "hasAny(events.type, ?)")
				args = append(args, utils.GetMapKeys(typeSet))
			}
			if len(rawTypeSet) > 0 {
				parts = append(parts, "hasAny(events.raw_type, ?)")
				args = append(args, utils.GetMapKeys(rawTypeSet))
			}
			part := strings.Join(parts, " OR ")
			if len(parts) > 1 {
				part = fmt.Sprintf("(%s)", part)
			}
			conditions = append(conditions, part)
		}
	}
	// MoveCallFilter
	if query.MoveCallFilter != nil {
		conditions = append(conditions, "notEmpty(move_calls.package)")
		if query.MoveCallFilter.Package != nil {
			conditions = append(conditions, "has(move_calls.package, ?)")
			args = append(args, query.MoveCallFilter.Package.String())
		}
		if query.MoveCallFilter.Module != "" {
			conditions = append(conditions, "has(move_calls.module, ?)")
			args = append(args, query.MoveCallFilter.Module)
		}
		if query.MoveCallFilter.Function != "" {
			conditions = append(conditions, "has(move_calls.function, ?)")
			args = append(args, query.MoveCallFilter.Function)
		}
	}
	// BalanceChangeFilter
	if query.BalanceChange != nil && query.BalanceChange.AddressOwner != nil {
		conditions = append(conditions, "has(balance_changes.owner, ?)")
		args = append(args, objectOwnerString(types.ObjectOwner{
			ObjectOwnerInternal: &types.ObjectOwnerInternal{AddressOwner: query.BalanceChange.AddressOwner},
		}))
	}
	// IncludeFailed
	if !query.IncludeFailed {
		conditions = append(conditions, "status = ?")
		args = append(args, types.TransactionStatusSuccess)
	}
	// query data from clickhouse (no record limit: this legacy query keeps its original behavior)
	return c.queryTransactions(ctx, query.CheckAndTrim, 0, strings.Join(conditions, " AND "), args...)
}

func mergeCondition(parts []string, link string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, link) + ")"
}

func (c *Controller) QueryTransactionsV2(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.TransactionFilter,
	fetchConfig sui.TransactionFetchConfig,
	limit int,
) ([]types.TransactionResponseV1, error) {
	if err := c.checkRange(ctx, rg.NewRange(fromBlock, toBlock)); err != nil {
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
	// filter.FailedIsOK
	if !filter.FailedIsOK {
		conditions = append(conditions, "status = ?")
		args = append(args, types.TransactionStatusSuccess)
	}
	// filter.EventFilters
	if len(filter.EventFilters) > 0 {
		var parts []string
		// typePattern
		for _, ff := range filter.EventFilters {
			for _, typ := range ff.TypePattern {
				if typ.MainHasAny() {
					return nil, errors.Errorf("invalid event type %s", typ.String())
				}
			}
		}
		rawTypes := utils.MapSliceNoError(filter.EventFilters, func(ff sui.EventFilterV2) []string {
			return utils.MapSliceNoError(ff.TypePattern, move.Type.Main)
		})
		hasEmpty := utils.HasAny(rawTypes, func(tps []string) bool {
			return len(tps) == 0
		})
		if !hasEmpty && len(rawTypes) > 0 {
			parts = append(parts, "hasAny(events.raw_type, ?)")
			args = append(args, utils.MergeArr(rawTypes...))
		}
		// sender
		senderSet := make(map[string]struct{})
		for _, ff := range filter.EventFilters {
			if ff.Sender != nil {
				senderSet[*ff.Sender] = struct{}{}
			}
		}
		if len(senderSet) > 0 {
			parts = append(parts, "hasAny(events.sender, ?)")
			args = append(args, utils.GetMapKeys(senderSet))
		}
		// build event filter condition
		filters = append(filters, mergeCondition(parts, " AND "))
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
				parts = append(parts, "has(move_calls.package, ?)")
				args = append(args, *ff.CommandFilter.CallPackage)
			}
			if ff.CommandFilter.CallModule != nil {
				parts = append(parts, "has(move_calls.module, ?)")
				args = append(args, *ff.CommandFilter.CallModule)
			}
			if ff.CommandFilter.CallFunction != nil {
				parts = append(parts, "has(move_calls.function, ?)")
				args = append(args, *ff.CommandFilter.CallFunction)
			}
		}
		if ff.MultiSigPublicKeyPrefix != nil {
			parts = append(parts, "length(tx_signature) = 1")
		}
		if ff.Sender != nil {
			parts = append(parts, "sender = ?")
			args = append(args, *ff.Sender)
		}
		if ff.Receiver != nil {
			parts = append(parts, "has(balance_changes.owner, ?)")
			args = append(args, *ff.Receiver)
		}
		if !ff.FailedIsOK {
			parts = append(parts, "status = ?")
			args = append(args, types.TransactionStatusSuccess)
		}
		filters = append(filters, mergeCondition(parts, " AND "))
	}
	// append filter part
	if len(filters) > 0 {
		conditions = append(conditions, mergeCondition(filters, " OR "))
	}

	// query data from clickhouse
	return c.queryTransactions(ctx, func(tx *types.TransactionResponseV1) bool {
		if !filter.Check(*tx) {
			return false
		}
		*tx = fetchConfig.PruneTransaction(*tx, filter.EventFilters)
		return true
	}, limit, strings.Join(conditions, " AND "), args...)
}

// queryTransactions loads the matching transactions, scanning at most limit raw rows
// (0 = unlimited, pushed down as a SQL LIMIT to bound the ClickHouse-side resource use of one
// query). When the scan hits the limit it fails with chain.NewTooManyResultsError — the raw rows
// are counted before the Go-side post handler, so the check is conservative, but a returned result
// is always complete. The super node passes its record cap + 1 (chain.StoreQueryLimit), so a query
// matching exactly the cap still succeeds.
func (c *Controller) queryTransactions(
	ctx context.Context,
	postHandler func(*types.TransactionResponseV1) bool,
	limit int,
	where string,
	args ...any,
) (result []types.TransactionResponseV1, err error) {
	fieldFilter := objectx.HasTag("clickhouse").And(objectx.AnyHasTagEqualTo("required", "true"))
	columns := objectx.CollectTagValue(&CHUTransaction{}, "clickhouse", fieldFilter)
	// ORDER BY checkpoint, transaction_position so a checkpoint's transactions are
	// returned in on-chain order. Without it the result follows the table sort key
	// (checkpoint, ..., digest) = digest order within a checkpoint, which is not the
	// on-chain order (chv4 likewise orders by checkpoint, tx_index).
	sql := fmt.Sprintf("SELECT `%s` FROM %s WHERE %s ORDER BY checkpoint, transaction_position",
		strings.Join(columns, "`,`"),
		c.ctrl.FullLogicName(tableNameTransactions),
		where)
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	startAt := time.Now()
	var rawRows int
	err = c.ctrl.Query(ctx, func(rows driver.Rows) error {
		var tx CHUTransaction
		scanErr := rows.Scan(objectx.CollectFieldPointers(&tx, fieldFilter)...)
		if scanErr != nil {
			return scanErr
		}
		rawRows++
		res := tx.BuildTransactionResponseV1()
		if postHandler(&res) {
			result = append(result, res)
		}
		return nil
	}, sql, args...)
	if err == nil && limit > 0 && rawRows >= limit {
		err = chain.NewTooManyResultsError()
	}
	c.recordQueryTx(ctx, time.Since(startAt), len(result))
	return result, err
}

func (c *Controller) QueryObjectChanges(
	ctx context.Context,
	query *sui.ObjectChangeQuery,
) ([]types.ObjectChangeExtend, error) {
	queryRange := rg.NewRange(query.FromSequenceNumber, query.ToSequenceNumber)
	if err := c.checkRange(ctx, queryRange); err != nil {
		return nil, err
	}

	// prepare sql
	var conditions []string
	var args []any

	// checkpoint range
	conditions = append(conditions, "checkpoint >= ?")
	conditions = append(conditions, "checkpoint <= ?")
	args = append(args, query.FromSequenceNumber, query.ToSequenceNumber)

	// ownerType condition
	if query.OwnerType != "" {
		conditions = append(conditions, "owner_type = ?")
		args = append(args, query.OwnerType)
	}

	// objectID and ownerID and objectType condition
	var objectConditions []string
	if len(query.OwnerIDIn) > 0 {
		objectConditions = append(objectConditions, "owner_id IN ?")
		args = append(args, query.OwnerIDIn)
	}
	if len(query.ObjectIDIn) > 0 {
		objectConditions = append(objectConditions, "object_id IN ?")
		args = append(args, query.ObjectIDIn)
	}
	if len(query.ObjectTypeIn) > 0 {
		var argExactTypes []string
		var argRawTypes []string
		var argLikeTypes []string
		for _, typ := range query.ObjectTypeIn {
			if typ.Vector != nil {
				if !types.ContainsAnyType(*typ.Vector) {
					argExactTypes = append(argExactTypes, typ.String())
				} else {
					argLikeTypes = append(argLikeTypes, typ.Text(true, func(tag types.TypeTag) (string, bool) {
						return "%", tag.Any
					}))
				}
			} else if typ.Struct != nil {
				if len(typ.Struct.TypeArgs) == 0 {
					argRawTypes = append(argRawTypes, typ.String())
				} else if !types.ContainsAnyType(typ.Struct.TypeArgs...) {
					argExactTypes = append(argExactTypes, typ.String())
				} else {
					argLikeTypes = append(argLikeTypes, typ.Text(true, func(tag types.TypeTag) (string, bool) {
						return "%", tag.Any
					}))
				}
			} else {
				argExactTypes = append(argExactTypes, typ.String())
			}
		}
		if len(argExactTypes) > 0 {
			objectConditions = append(objectConditions, "object_type IN ?")
			args = append(args, argExactTypes)
		}
		if len(argRawTypes) > 0 {
			objectConditions = append(objectConditions, "object_raw_type IN ?")
			args = append(args, argRawTypes)
		}
		for _, like := range argLikeTypes {
			objectConditions = append(objectConditions, "object_type LIKE ?")
			args = append(args, like)
		}
	}
	if len(objectConditions) > 0 {
		condi := strings.Join(objectConditions, " OR ")
		if len(objectConditions) > 1 {
			condi = fmt.Sprintf("(%s)", condi)
		}
		conditions = append(conditions, condi)
	}

	// build sql
	var where string
	if query.OnlyLastVersion {
		sql := fmt.Sprintf("SELECT distinct object_id FROM %s WHERE %s",
			c.ctrl.FullLogicName(tableNameObjectChanges),
			strings.Join(conditions, " AND "))
		sql = fmt.Sprintf("SELECT object_id,max(object_version),max(checkpoint) FROM %s "+
			"WHERE object_id IN (%s) AND checkpoint >= %d AND checkpoint <= %d GROUP BY object_id",
			c.ctrl.FullLogicName(tableNameObjectPositions),
			sql,
			query.FromSequenceNumber,
			query.ToSequenceNumber)
		where = fmt.Sprintf("(object_id,object_version,checkpoint) IN (%s)", sql)
	} else {
		where = strings.Join(conditions, " AND ")
	}

	// no record limit: this legacy query keeps its original behavior
	return c.queryObjectChanges(ctx, query.Check, 0, where, args...)
}

func (c *Controller) QueryObjectChangesV2(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.ObjectChangeFilter,
	limit int,
) ([]types.ObjectChangeExtend, error) {
	if err := c.checkRange(ctx, rg.NewRange(fromBlock, toBlock)); err != nil {
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
			filters = append(filters, "(owner_id IN ? AND owner_type IN ?)")
			args = append(args, filter.OwnerFilter.OwnerID, filter.OwnerFilter.OwnerType)
		}
	}
	// typePattern filter
	for _, typ := range filter.TypePattern {
		if typ.HasArgs() && !typ.HasAny() {
			filters = append(filters, "object_type = ?")
			args = append(args, typ.String())
		} else if !typ.MainHasAny() {
			filters = append(filters, "object_raw_type = ?")
			args = append(args, typ.Main())
		} else {
			filters = append(filters, "object_raw_type LIKE ?")
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

	return c.queryObjectChanges(ctx, filter.Checker(), limit, strings.Join(conditions, " AND "), args...)
}

// queryObjectChanges applies limit like queryTransactions (a SQL LIMIT on the raw rows scanned;
// hitting it fails with chain.NewTooManyResultsError).
func (c *Controller) queryObjectChanges(
	ctx context.Context,
	postFilter func(types.ObjectChangeExtend) bool,
	limit int,
	where string,
	args ...any,
) ([]types.ObjectChangeExtend, error) {
	fieldFilter := objectx.HasTag("clickhouse").And(objectx.AnyHasTagEqualTo("required", "true"))
	columns := objectx.CollectTagValue(&CHUObjectChange{}, "clickhouse", fieldFilter)
	sql := fmt.Sprintf("SELECT `%s` FROM %s WHERE %s ORDER BY checkpoint",
		strings.Join(columns, "`,`"),
		c.ctrl.FullLogicName(tableNameObjectChanges),
		where)
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	startAt := time.Now()
	var rawRows int
	var result []types.ObjectChangeExtend
	err := c.ctrl.Query(ctx, func(rows driver.Rows) error {
		var oc CHUObjectChange
		if scanErr := rows.Scan(objectx.CollectFieldPointers(&oc, fieldFilter)...); scanErr != nil {
			return scanErr
		}
		rawRows++
		// post filter
		res := oc.BuildObjectChangeExtend()
		if postFilter(res) {
			result = append(result, res)
		}
		return nil
	}, sql, args...)
	if err == nil && limit > 0 && rawRows >= limit {
		err = chain.NewTooManyResultsError()
	}
	c.recordQueryObj(ctx, time.Since(startAt), len(result))
	return result, err
}

// QueryLastObjectChange returns the object's newest recorded change with
// object_version <= maxVersion (no bound when maxVersion is 0) and
// checkpoint <= maxCheckpoint, or nil when nothing is recorded. The position
// lookup runs on the object-id-partitioned object_positions table, so the cost is
// independent of how many checkpoints the object's history spans; the change row
// is then read at that single checkpoint.
func (c *Controller) QueryLastObjectChange(
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
		c.ctrl.FullLogicName(tableNameObjectPositions), condition)
	var version, checkpoint uint64
	var found bool
	if err := c.ctrl.Query(ctx, func(rows driver.Rows) error {
		found = true
		return rows.Scan(&version, &checkpoint)
	}, sql, args...); err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	changes, err := c.queryObjectChanges(ctx, func(types.ObjectChangeExtend) bool { return true }, 0,
		"checkpoint = ? AND object_id = ? AND object_version = ?", checkpoint, objectID, version)
	if err != nil {
		return nil, err
	}
	if len(changes) == 0 {
		return nil, errors.Errorf("object change of %s@%d at checkpoint %d not found", objectID, version, checkpoint)
	}
	oc := &changes[0]
	record := &sui.ObjectChangeRecord{
		Checkpoint:    checkpoint,
		TxDigest:      oc.TxDigest.String(),
		Type:          string(oc.Type),
		ObjectVersion: version,
	}
	if oc.PreviousVersion != nil {
		v := oc.PreviousVersion.Uint64()
		record.PreviousVersion = &v
	}
	return record, nil
}

func (c *Controller) QueryObjectsStat(
	ctx context.Context,
	startCheckpoint uint64,
	endCheckpoint uint64,
	objectIDList []string,
) (map[string]sui.ObjectStat, error) {
	queryRange := rg.NewRange(startCheckpoint, endCheckpoint)
	if err := c.checkRange(ctx, queryRange); err != nil {
		return nil, err
	}

	result := make(map[string]sui.ObjectStat)
	sql := fmt.Sprintf("SELECT object_id, "+
		"count(distinct object_version), "+
		"min(object_version), "+
		"max(object_version), "+
		"min(checkpoint), "+
		"max(checkpoint) "+
		"FROM %s "+
		"WHERE checkpoint >= ? AND checkpoint <= ? AND object_id IN ? "+
		"GROUP BY object_id",
		c.ctrl.FullLogicName(tableNameObjectPositions))
	err := c.ctrl.Query(ctx, func(rows driver.Rows) error {
		var r sui.ObjectStat
		var objectID string
		err := rows.Scan(&objectID, &r.Count, &r.MinObjectVersion, &r.MaxObjectVersion, &r.MinCheckpoint, &r.MaxCheckpoint)
		if err != nil {
			return err
		}
		result[objectID] = r
		return nil
	}, sql, startCheckpoint, endCheckpoint, objectIDList)
	return result, err
}
