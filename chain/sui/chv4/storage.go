package chv4

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/objectx"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
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

func (s *Storage) QueryCheckpointTime(ctx context.Context, checkpoint uint64) (sui.CheckpointTime, error) {
	if err := s.checkRange(ctx, rg.NewSingleRange(checkpoint)); err != nil {
		return sui.CheckpointTime{}, err
	}
	sql := fmt.Sprintf("SELECT timestamp FROM %s WHERE checkpoint = ?", s.ctrl.FullLogicName(tableNameCheckpoints))
	var t time.Time
	var has bool
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		has = true
		return rows.Scan(&t)
	}, sql, checkpoint)
	if err != nil {
		return sui.CheckpointTime{}, err
	}
	if !has {
		return sui.CheckpointTime{}, errors.Errorf("checkpoint %d not found", checkpoint)
	}
	ts := uint64(t.UnixMilli())
	return sui.CheckpointTime{
		CheckpointTime: ts,
		MinTxnTime:     ts,
		MaxTxnTime:     ts,
	}, nil
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

func (s *Storage) queryTransactions(
	ctx context.Context,
	postHandler func(*types.TransactionResponseV1) bool,
	where string,
	args ...any,
) (result []types.TransactionResponseV1, err error) {
	fieldFilter := objectx.HasTag("clickhouse").And(objectx.NoTag("required", "false"))
	columns := objectx.CollectTagValue(&Transaction{}, "clickhouse", fieldFilter)
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s",
		strings.Join(columns, ","),
		s.ctrl.FullLogicName(tableNameTransactions),
		where)
	startAt := time.Now()
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var tx Transaction
		scanErr := rows.Scan(objectx.CollectFieldPointers(&tx, fieldFilter)...)
		if scanErr != nil {
			return scanErr
		}
		etx, parseErr := tx.ToExecutedTransaction()
		if parseErr != nil {
			return parseErr
		}
		res, buildErr := sui.BuildTransactionResponseV1(tx.Checkpoint, tx.Timestamp, int(tx.TxIndex), etx, false, nil)
		if buildErr != nil {
			return buildErr
		}
		if postHandler(&res) {
			result = append(result, res)
		}
		return nil
	}, sql, args...)
	s.recordQueryTx(ctx, time.Since(startAt), len(result))
	return result, err
}

func (s *Storage) QueryTransactions(ctx context.Context, query *sui.TransactionQuery) ([]types.TransactionResponseV1, error) {
	queryRange := rg.NewRange(query.FromSequenceNumber, query.ToSequenceNumber)
	if err := s.checkRange(ctx, queryRange); err != nil {
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
		conditions = append(conditions, "length(signatures) = 1")
	}
	// Sender
	if query.Sender != nil {
		conditions = append(conditions, "sender = ?")
		args = append(args, query.Sender.String())
	}
	// EventFilter
	if query.EventFilter != nil {
		conditions = append(conditions, "notEmpty(events_type)")
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
			conditions = append(conditions, "hasAny(events_package_id, ?)")
			args = append(args, utils.GetMapKeys(pkgIDSet))
		}
		if len(modSet) > 0 {
			conditions = append(conditions, "hasAny(events_module, ?)")
			args = append(args, utils.GetMapKeys(modSet))
		}
		if len(senderSet) > 0 {
			conditions = append(conditions, "hasAny(events_sender, ?)")
			args = append(args, utils.GetMapKeys(senderSet))
		}
		if len(typeSet) > 0 || len(rawTypeSet) > 0 {
			var parts []string
			if len(typeSet) > 0 {
				parts = append(parts, "hasAny(events_type, ?)")
				args = append(args, utils.GetMapKeys(typeSet))
			}
			if len(rawTypeSet) > 0 {
				parts = append(parts, "hasAny(events_main_type, ?)")
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
		conditions = append(conditions, "notEmpty(move_calls_package)")
		if query.MoveCallFilter.Package != nil {
			conditions = append(conditions, "has(move_calls_package, ?)")
			args = append(args, query.MoveCallFilter.Package.String())
		}
		if query.MoveCallFilter.Module != "" {
			conditions = append(conditions, "has(move_calls_module, ?)")
			args = append(args, query.MoveCallFilter.Module)
		}
		if query.MoveCallFilter.Function != "" {
			conditions = append(conditions, "has(move_calls_function, ?)")
			args = append(args, query.MoveCallFilter.Function)
		}
	}
	// BalanceChangeFilter
	if query.BalanceChange != nil && query.BalanceChange.AddressOwner != nil {
		conditions = append(conditions, "has(balance_changes_address, ?)")
		args = append(args, query.BalanceChange.AddressOwner.String())
	}
	// IncludeFailed
	if !query.IncludeFailed {
		conditions = append(conditions, "success")
	}
	// query data from clickhouse
	return s.queryTransactions(ctx, query.CheckAndTrim, strings.Join(conditions, " AND "), args...)
}

func mergeCondition(parts []string, link string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, link) + ")"
}

func (s *Storage) QueryTransactionsV2(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.TransactionFilter,
	fetchConfig sui.TransactionFetchConfig,
) ([]types.TransactionResponseV1, error) {
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
			parts = append(parts, "hasAny(events_main_type, ?)")
			args = append(args, utils.MergeArr(rawTypes...))
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
	return s.queryTransactions(ctx, func(tx *types.TransactionResponseV1) bool {
		if !filter.Check(*tx) {
			return false
		}
		*tx = fetchConfig.PruneTransaction(*tx, filter.EventFilters)
		return true
	}, strings.Join(conditions, " AND "), args...)
}

func ownerKindToType(kind rpcv2.Owner_OwnerKind) string {
	switch kind {
	case rpcv2.Owner_OWNER_KIND_UNKNOWN:
		return ""
	case rpcv2.Owner_ADDRESS:
		return types.OwnerTypeAddress
	case rpcv2.Owner_OBJECT:
		return types.OwnerTypeObject
	case rpcv2.Owner_SHARED:
		return types.OwnerTypeShared
	case rpcv2.Owner_IMMUTABLE:
		return types.OwnerTypeSpecial
	case rpcv2.Owner_CONSENSUS_ADDRESS:
		return types.OwnerTypeConsensusAddress
	default:
		panic(errors.Errorf("unexpected owner kind %s", kind))
	}
}

func ownerTypeToKind(typ string) rpcv2.Owner_OwnerKind {
	switch typ {
	case types.OwnerTypeSpecial:
		return rpcv2.Owner_IMMUTABLE
	case types.OwnerTypeObject:
		return rpcv2.Owner_OBJECT
	case types.OwnerTypeAddress:
		return rpcv2.Owner_ADDRESS
	case types.OwnerTypeShared:
		return rpcv2.Owner_SHARED
	case types.OwnerTypeConsensusAddress:
		return rpcv2.Owner_CONSENSUS_ADDRESS
	default:
		panic(errors.Errorf("unexpected owner type %q", typ))
	}
}

func (s *Storage) queryObjectChanges(
	ctx context.Context,
	postFilter func(types.ObjectChangeExtend) bool,
	where string,
	args ...any,
) ([]types.ObjectChangeExtend, error) {
	// json and package field are big but useless for convert to types.ObjectChangeExtend
	fieldFilter := objectx.HasTag("clickhouse").
		And(objectx.NoTag("clickhouse", "json")).
		And(objectx.NoTag("clickhouse", "package"))
	columns := objectx.CollectTagValue(&Object{}, "clickhouse", fieldFilter)
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY checkpoint",
		strings.Join(columns, ","),
		s.ctrl.FullLogicName(tableNameObjects),
		where)
	startAt := time.Now()
	var result []types.ObjectChangeExtend
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var oc Object
		if scanErr := rows.Scan(objectx.CollectFieldPointers(&oc, fieldFilter)...); scanErr != nil {
			return scanErr
		}
		res, convertErr := oc.ToObjectChangeExtend()
		if convertErr != nil {
			return convertErr
		}
		// post filter
		if postFilter(res) {
			result = append(result, res)
		}
		return nil
	}, sql, args...)
	s.recordQueryObj(ctx, time.Since(startAt), len(result))
	return result, err
}

func (s *Storage) QueryObjectChanges(ctx context.Context, query *sui.ObjectChangeQuery) ([]types.ObjectChangeExtend, error) {
	queryRange := rg.NewRange(query.FromSequenceNumber, query.ToSequenceNumber)
	if err := s.checkRange(ctx, queryRange); err != nil {
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
		conditions = append(conditions, "(owner_kind = ? OR pre_owner_kind = ?)")
		queryOwnerKind := ownerTypeToKind(query.OwnerType).String()
		args = append(args, queryOwnerKind, queryOwnerKind)
	}

	// objectID and ownerID and objectType condition
	var objectConditions []string
	if len(query.OwnerIDIn) > 0 {
		objectConditions = append(objectConditions, "(owner_address IN ? OR pre_owner_address IN ?)")
		args = append(args, query.OwnerIDIn, query.OwnerIDIn)
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
			objectConditions = append(objectConditions, "object_main_type IN ?")
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
		sql := fmt.Sprintf("SELECT object_id,max(object_version),max(checkpoint) "+
			"FROM %s WHERE %s GROUP BY object_id",
			s.ctrl.FullLogicName(tableNameObjects),
			strings.Join(conditions, " AND "))
		where = fmt.Sprintf("(object_id,object_version,checkpoint) IN (%s)", sql)
	} else {
		where = strings.Join(conditions, " AND ")
	}

	return s.queryObjectChanges(ctx, query.Check, where, args...)
}

func (s *Storage) QueryObjectChangesV2(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter sui.ObjectChangeFilter,
) ([]types.ObjectChangeExtend, error) {
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
			ownerKinds := utils.MapSliceNoError(filter.OwnerFilter.OwnerType, func(ot string) string {
				return ownerTypeToKind(ot).String()
			})
			args = append(args, filter.OwnerFilter.OwnerID, filter.OwnerFilter.OwnerID, ownerKinds, ownerKinds)
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

	return s.queryObjectChanges(ctx, filter.Checker(), strings.Join(conditions, " AND "), args...)
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
