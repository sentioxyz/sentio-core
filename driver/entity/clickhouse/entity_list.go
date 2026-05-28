package clickhouse

import (
	"context"
	"crypto/sha1"
	"fmt"
	"math/rand"
	"reflect"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/format"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/graph-gophers/graphql-go/types"
)

var conditionSymbol = map[persistent.EntityFilterOp]string{
	persistent.EntityFilterOpEq:      "=",
	persistent.EntityFilterOpNe:      "!=",
	persistent.EntityFilterOpGt:      ">",
	persistent.EntityFilterOpGe:      ">=",
	persistent.EntityFilterOpLt:      "<",
	persistent.EntityFilterOpLe:      "<=",
	persistent.EntityFilterOpIn:      "IN",
	persistent.EntityFilterOpNotIn:   "NOT IN",
	persistent.EntityFilterOpLike:    "LIKE",
	persistent.EntityFilterOpNotLike: "NOT LIKE",
}

const timeLayoutAllDigital = "20060102150405"
const tempTableFieldName = "s"

func (s *Store) buildTemporaryTable(ctx context.Context, filter persistent.EntityFilter) (string, func(), error) {
	start := time.Now()
	name := s.buildName("filter",
		fmt.Sprintf("%x_%s_%08x", sha1.Sum([]byte(filter.String())), start.Format(timeLayoutAllDigital), rand.Uint32()))
	_, logger := log.FromContext(ctx, "tmpTableName", name)
	table := chx.Table{
		Name:        name,
		IsTemporary: true,
		Config: chx.TableConfig{
			Engine: chx.NewMemoryEngine(),
		},
		Fields: []chx.Field{{Name: tempTableFieldName, Type: chx.FieldTypeString}},
	}
	if err := s.ctrl.Create(ctx, table); err != nil {
		logger.With("filter", filter.String(), "used", time.Since(start).String()).
			Errore(err, "created temporary table failed")
		return name, nil, err
	}
	logger.Debugw("created temporary table", "used", time.Since(start).String())
	const dropTempTableTimeout = time.Second * 30
	epilogue := func() {
		dropCtx, cancel := context.WithTimeout(context.Background(), dropTempTableTimeout)
		defer cancel()
		dropStart := time.Now()
		dropErr := s.ctrl.Drop(dropCtx, table)
		if dropErr != nil {
			logger.With("filter", filter.String(), "used", time.Since(dropStart).String()).
				Warne(dropErr, "drop temporary table failed")
		} else {
			logger.With("used", time.Since(dropStart).String()).
				Debug("drop temporary table succeed")
		}
	}
	start = time.Now()
	sql := fmt.Sprintf("INSERT INTO %s (%s)", s.ctrl.LogicName(table.Name), tempTableFieldName)
	getter := chx.NewGetter(filter.Value, func(v any) []any {
		return []any{v}
	})
	if err := s.ctrl.BatchInsert(ctx, sql, s.tableOpt.BatchInsertSizeLimit, getter); err != nil {
		logger.With("filter", filter.String(), "used", time.Since(start).String()).
			Errore(err, "insert to temporary table failed")
		return name, epilogue, err
	}
	logger.Debugw("insert to temporary table succeed", "used", time.Since(start).String())
	return name, epilogue, nil
}

func _isNil(v any) bool {
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Pointer, reflect.Slice:
		return val.IsNil()
	default:
		return false
	}
}

func (s *Store) buildCondition(ctx context.Context, entity Entity, filter persistent.EntityFilter) (
	condition string,
	param []any,
	epilogue func(),
	err error,
) {
	invalidErr := func(format string, args ...any) error {
		return fmt.Errorf("%w %s: %v", persistent.ErrInvalidListFilter, filter.String(), fmt.Errorf(format, args...))
	}

	var extra string
	symbol := conditionSymbol[filter.Op]
	field := entity.GetFieldByName(filter.Field.Name)
	if field.IsReverseForeignKeyField() {
		err = invalidErr("filter field cannot use reverse foreign key field")
		return
	}
	fieldTypeChain := schema.BreakType(filter.Field.Type)
	var slot string
	switch filter.Op {
	case persistent.EntityFilterOpEq, persistent.EntityFilterOpNe:
		if len(filter.Value) != 1 {
			err = invalidErr("number of filter value is %d not 1", len(filter.Value))
			return
		}
		raw := filter.Value[0]
		if _isNil(raw) {
			condition = field.NullCondition(filter.Op == persistent.EntityFilterOpEq)
			return
		}
		param = field.FieldValuesForSetMain(raw)
		slot = field.FieldSlotsForSet()[0]
	case
		persistent.EntityFilterOpGt,
		persistent.EntityFilterOpGe,
		persistent.EntityFilterOpLt,
		persistent.EntityFilterOpLe:
		if len(filter.Value) != 1 {
			err = invalidErr("number of filter value is %d not 1", len(filter.Value))
			return
		}
		if fieldTypeChain.CountListLayer() > 0 {
			err = invalidErr("array cannot use this operation")
			return
		}
		raw := filter.Value[0]
		if _isNil(raw) {
			// compare with null always return false
			condition = "false"
			return
		}
		param = field.FieldValuesForSetMain(raw)
		slot = field.FieldSlotsForSet()[0]
		// AND field not null
		// to make sure exclude null value
		extra = "AND " + field.NullCondition(false)
	case persistent.EntityFilterOpIn, persistent.EntityFilterOpNotIn:
		if fieldTypeChain.CountListLayer() > 0 {
			err = invalidErr("array cannot use this operation")
			return
		}
		if len(filter.Value) == 0 {
			// condition is in empty set means false, not in empty set means true
			condition = strconv.FormatBool(filter.Op != persistent.EntityFilterOpIn)
			return
		}
		if filter.Field.Name == schema.EntityPrimaryFieldName && uint(len(filter.Value)) > s.tableOpt.HugeIDSetSize {
			var tempTable string
			tempTable, epilogue, err = s.buildTemporaryTable(ctx, filter)
			if err != nil {
				return
			}
			slot = fmt.Sprintf("(SELECT %s FROM %s)", tempTableFieldName, s.ctrl.LogicName(tempTable))
		} else {
			var hasNil bool
			var setSize int
			for _, v := range filter.Value {
				if _isNil(v) {
					hasNil = true
				} else {
					setSize++
					param = append(param, field.FieldValuesForSetMain(v)...)
				}
			}
			if setSize == 0 {
				// hasNil must be true
				condition = field.NullCondition(filter.Op == persistent.EntityFilterOpIn)
				return
			}
			// hasNormal is true
			slot = "(" + utils.Dup(field.FieldSlotsForSet()[0], ",", setSize) + ")"
			if hasNil {
				if filter.Op == persistent.EntityFilterOpIn {
					extra = "OR " + field.NullCondition(true)
				} else {
					extra = "AND " + field.NullCondition(false)
				}
			} else {
				if filter.Op == persistent.EntityFilterOpNotIn {
					extra = "OR " + field.NullCondition(true)
				}
			}
		}
	case persistent.EntityFilterOpLike, persistent.EntityFilterOpNotLike:
		if len(filter.Value) != 1 {
			err = invalidErr("number of filter value is %d not 1", len(filter.Value))
			return
		}
		if fieldTypeChain.CountListLayer() > 0 {
			err = invalidErr("array cannot use this operation")
			return
		}
		scalarType, is := fieldTypeChain.InnerType().(*types.ScalarTypeDefinition)
		if !is || (scalarType.Name != "String" && scalarType.Name != "ID") {
			err = invalidErr("%s cannot use this operation", filter.Field.Type.String())
			return
		}
		if _isNil(filter.Value[0]) {
			// like null and not like null always return false
			condition = "false"
			return
		}
		param = field.FieldValuesForSetMain(filter.Value[0])
		slot = field.FieldSlotsForSet()[0]
	case persistent.EntityFilterOpHasAll, persistent.EntityFilterOpHasAny:
		if fieldTypeChain.CountListLayer() != 1 {
			err = invalidErr("only one-dimension array can use this operation")
			return
		}
		function := utils.Select(filter.Op == persistent.EntityFilterOpHasAll, "hasAll", "hasAny")
		if field.IsForeignKeyField() {
			// db type will be Array(String) or Array(Nullable(String))
			condition = fmt.Sprintf("%s(%s,?)", function, quote(field.FieldMainName()))
			param = append(param, filter.Value)
			return
		} else {
			// db type will be String, and the value will be an JSONArray
			simple := SimpleField{BaseField: NewBaseField(entity.Def, filter.Field)}
			condition = fmt.Sprintf("%s(JSONExtract(%s,'%s'),?)",
				function, quote(field.FieldMainName()), simple.fieldDBType())
			param = append(param, filter.Value)
			return
		}
	default:
		err = invalidErr("invalid operation")
		return
	}
	condition = fmt.Sprintf("%s %s %s", quote(field.FieldMainName()), symbol, slot)
	if extra != "" {
		condition = fmt.Sprintf("(%s %s)", condition, extra)
	}
	return
}

func mergeFunc(fn []func()) func() {
	return func() {
		for _, f := range fn {
			if f != nil {
				f()
			}
		}
	}
}

func (s *Store) buildConditions(
	ctx context.Context,
	entity Entity,
	filters []persistent.EntityFilter,
	conditionPrefix string,
) (
	condition string,
	params []any,
	epilogue func(),
	err error,
) {
	if len(filters) == 0 {
		return "", nil, func() {}, nil
	}
	var conditions []string
	var epilogues []func()
	for _, filter := range filters {
		cond, param, epi, buildErr := s.buildCondition(ctx, entity, filter)
		if buildErr != nil {
			return "", nil, mergeFunc(epilogues), buildErr
		}
		conditions = append(conditions, cond)
		params = append(params, param...)
		epilogues = append(epilogues, epi)
	}
	return conditionPrefix + strings.Join(conditions, " AND "), params, mergeFunc(epilogues), nil
}

func splitFilters(filters []persistent.EntityFilter) (primaryKeyFilters, otherFilters []persistent.EntityFilter) {
	for _, filter := range filters {
		if filter.Field.Name == schema.EntityPrimaryFieldName {
			primaryKeyFilters = append(primaryKeyFilters, filter)
		} else {
			otherFilters = append(otherFilters, filter)
		}
	}
	return
}

func (s *Store) listEntities(
	ctx context.Context,
	entityType *schema.Entity,
	chain string,
	filters []persistent.EntityFilter,
	excludeDeleted bool,
	limit int,
) ([]*entityRow, error) {
	const maxRetry = 10
	const retryInterval = time.Second
	// List entity may use temporary table.
	// If the session is interrupted, the temporary table will be automatically released,
	// and will got a 'Table xxx does not exist' error. In this case, we should retry.
	for retry := maxRetry; ; retry-- {
		thisCtx, _ := log.FromContext(ctx, "retry", retry)
		result, err := s._listEntities(thisCtx, entityType, chain, filters, excludeDeleted, limit)
		if err == nil {
			return result, nil
		}
		if retry == 0 || !strings.Contains(err.Error(), "does not exist") {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryInterval):
		}
	}
}

func (s *Store) _listEntities(
	ctx context.Context,
	entityType *schema.Entity,
	chain string,
	filters []persistent.EntityFilter,
	excludeDeleted bool,
	limit int,
) (result []*entityRow, err error) {
	if entityType.IsCache() {
		return nil, nil
	}

	start := time.Now()
	kit := s.NewEntity(entityType)
	var sql string
	var sqlArgs []any
	if s.useVersionedCollapsingTable(entityType) {
		// The select field name needs to be converted, otherwise the aggregated field will be referenced
		// in the where clause, it will cause error
		// --------------------
		// SELECT
		//   __genBlockChain__, id, __version__,
		//   __any___genBlockNumber__ AS __genBlockNumber__
		//   __any_propA AS prepA
		// FROM (
		//   SELECT
		//     __genBlockChain__, id, __version__,
		//     any_respect_nulls(__genBlockNumber__) as __any___genBlockNumber__,
		//     any_respect_nulls(propA) as __any_propA
		//   FROM versionedEntity
		//   WHERE __genBlockChain__ = ? AND NOT __deleted__ AND propA > ?
		//   GROUP BY __genBlockChain__, id, __version__
		//   HAVING SUM(__sign__) > 0
		//   ORDER BY id
		//   LIMIT ?
		// )
		selects := utils.FilterArr(kit.fieldNamesForGet(), func(fn string) bool {
			return fn != schema.EntityPrimaryFieldName && fn != genBlockChainFieldName && fn != versionFieldName
		})
		innerSelects := make([]string, len(selects))
		outerSelects := make([]string, len(selects))
		for i, field := range selects {
			innerSelects[i] = fmt.Sprintf("any_respect_nulls(%s) AS __any_%s", field, field)
			outerSelects[i] = fmt.Sprintf("__any_%s AS %s", field, field)
		}
		filterConditions, params, epilogue, err := s.buildConditions(ctx, kit, filters, "AND ")
		defer epilogue()
		if err != nil {
			return nil, err
		}
		var excludeDeletedConditions string
		if excludeDeleted {
			excludeDeletedConditions = fmt.Sprintf("AND NOT %s", quote(deletedFieldName))
		}
		sql = format.Format("SELECT %gbc#s, %pk#s, %version#s, %outerSelects#s "+
			"FROM ("+
			"SELECT %gbc#s, %pk#s, %version#s, %innerSelects#s "+
			"FROM %table#s "+
			"WHERE %gbc#s = ? %edc#s %otc#s "+
			"GROUP BY %gbc#s, %pk#s, %version#s "+
			"HAVING SUM(%sign#s) > 0 "+
			"ORDER BY %pk#s "+
			"LIMIT ?"+
			")",
			map[string]any{
				"innerSelects": strings.Join(innerSelects, ","),
				"outerSelects": strings.Join(outerSelects, ","),
				"pk":           quote(schema.EntityPrimaryFieldName),
				"gbc":          quote(genBlockChainFieldName),
				"version":      quote(versionFieldName),
				"sign":         quote(signFieldName),
				"table":        s.fullName(s.VersionedTableName(entityType)),
				"edc":          excludeDeletedConditions,
				"otc":          filterConditions,
			})
		sqlArgs = []any{chain}
		sqlArgs = append(sqlArgs, params...)
		sqlArgs = append(sqlArgs, limit)
	} else if entityType.IsImmutable() {
		// SELECT id, propA
		// FROM entity
		// WHERE __genBlockChain__ = ? AND NOT __deleted__ AND propA > ?
		// ORDER BY id
		// LIMIT ?
		filterConditions, params, epilogue, err := s.buildConditions(ctx, kit, filters, "AND ")
		defer epilogue()
		if err != nil {
			return nil, err
		}
		var excludeDeletedConditions string
		if excludeDeleted {
			excludeDeletedConditions = fmt.Sprintf(" AND NOT %s", quote(deletedFieldName))
		}
		sql = fmt.Sprintf("SELECT %s FROM %s WHERE %s = ? %s %s ORDER BY %s LIMIT ?",
			joinWithQuote(kit.fieldNamesForGet(), ","),
			s.fullName(s.TableName(entityType)),
			quote(genBlockChainFieldName),
			excludeDeletedConditions,
			filterConditions,
			quote(schema.EntityPrimaryFieldName))
		sqlArgs = []any{chain}
		sqlArgs = append(sqlArgs, params...)
		sqlArgs = append(sqlArgs, limit)
	} else {
		// SELECT id, __last__.1 AS __genBlockNumber__, __last__.2 AS __deleted__, __last__.3 AS propA
		// FROM (
		//   SELECT id, MAX((__genBlockNumber__, __deleted__, propA)) as __last__
		//   FROM entity
		//   WHERE __genBlockChain__ = ? AND id NOT IN ?
		//   GROUP by id
		// )
		// WHERE NOT __last__.2 AND propA > ?
		// ORDER BY id
		// LIMIT ?
		lastFields := utils.Prepend(utils.FilterArr(kit.fieldNamesForGet(), func(fn string) bool {
			return fn != schema.EntityPrimaryFieldName &&
				fn != genBlockChainFieldName &&
				fn != genBlockNumberFieldName &&
				fn != deletedFieldName
		}), genBlockNumberFieldName, deletedFieldName)
		lastAs := make([]string, len(lastFields))
		for i, fieldName := range lastFields {
			lastAs[i] = fmt.Sprintf("__last__.%d AS %s", i+1, quote(fieldName))
		}
		primaryKeyFilters, otherFilters := splitFilters(filters)
		primaryKeyConditions, primaryKeyParams, primaryEpi, err := s.buildConditions(ctx, kit, primaryKeyFilters, "AND ")
		defer primaryEpi()
		if err != nil {
			return nil, err
		}
		otherConditions, otherParams, otherEpi, err := s.buildConditions(ctx, kit, otherFilters, "AND ")
		defer otherEpi()
		if err != nil {
			return nil, err
		}
		var excludeDeletedConditions = "true"
		if excludeDeleted {
			excludeDeletedConditions = "NOT __last__.2"
		}
		sql = format.Format("SELECT %pk#s, %gbc#s, %lastAs#s "+
			"FROM ( "+
			"  SELECT %pk#s, %gbc#s, MAX((%last#s)) AS __last__ "+
			"  FROM %ft#s "+
			"  WHERE %gbc#s = ? %pkc#s"+
			"  GROUP BY %pk#s, %gbc#s"+
			") "+
			"WHERE %edc#s %otc#s "+
			"ORDER BY %pk#s "+
			"LIMIT ?",
			map[string]any{
				"pk":     quote(schema.EntityPrimaryFieldName),
				"gbn":    quote(genBlockNumberFieldName),
				"gbc":    quote(genBlockChainFieldName),
				"ft":     s.fullName(s.TableName(entityType)),
				"pkc":    primaryKeyConditions,
				"edc":    excludeDeletedConditions,
				"otc":    otherConditions,
				"last":   joinWithQuote(lastFields, ","),
				"lastAs": strings.Join(lastAs, ","),
			})
		sqlArgs = []any{chain}
		sqlArgs = append(sqlArgs, primaryKeyParams...)
		sqlArgs = append(sqlArgs, otherParams...)
		sqlArgs = append(sqlArgs, limit)
	}
	// execute query and get the response
	// may be used temporary table, so here do not use SelectCtx(ctx) instead of ctx
	err = s.ctrl.Query(SelectCtx(ctx), func(rows driver.Rows) error {
		row, scanErr := kit.scanOne(rows)
		if scanErr != nil {
			return scanErr
		}
		row.Entity = entityType.Name
		result = append(result, &row)
		return nil
	}, sql, sqlArgs...)
	_, logger := log.FromContext(ctx,
		"entity", entityType.Name,
		"chain", chain,
		"excludeDeleted", excludeDeleted,
		"limit", limit,
		"sql", sql,
		"sqlArgs", sqlArgs,
		"used", time.Since(start).String())
	if err != nil {
		logger.Errore(err, "list failed")
	} else {
		logger.Debugw("list succeed", "count", len(result))
	}
	return
}

func (s *Store) countEntity(
	ctx context.Context,
	entityType *schema.Entity,
	chain string,
	excludeDeleted bool,
) (count uint64, err error) {
	queryCtx, logger := log.FromContext(ctx, "entity", entityType.Name, "chain", chain, "excludeDeleted", excludeDeleted)
	// Determine deletion state once globally so bucket queries don't re-check per bucket.
	hasDeletions := false
	if excludeDeleted && !entityType.IsImmutable() {
		deletionSQL := fmt.Sprintf("SELECT COUNT(*) FROM (SELECT %s FROM %s WHERE %s = ? AND %s LIMIT 1)",
			quote(schema.EntityPrimaryFieldName),
			s.fullName(s.TableName(entityType)),
			quote(genBlockChainFieldName),
			quote(deletedFieldName),
		)
		startAt := time.Now()
		var deletionCount uint64
		deletionCount, err = s.ctrl.QueryCount(SelectCtx(queryCtx), deletionSQL, chain)
		if err != nil {
			logger.With("sql", deletionSQL, "used", time.Since(startAt).String()).Errore(err, "count deleted failed")
			return 0, err
		}
		hasDeletions = deletionCount > 0
	}
	count, err = s._countEntity(queryCtx, entityType, chain, excludeDeleted, hasDeletions, "")
	if err == nil {
		return count, nil
	}
	if !isQueryMemoryLimitExceededError(err) {
		return 0, err
	}
	// memory limit exceeded, too many entities, should bucket query
	const (
		minBuckets = 10
		maxBuckets = 1000
		multi      = 10
	)
	for buckets := minBuckets; err != nil && buckets <= maxBuckets; buckets *= multi {
		logger.Warnfe(err, "result too large, will count in %d bucket", buckets)
		count, err = 0, nil
		for bi := 0; bi < buckets; bi++ {
			condition := fmt.Sprintf(" AND cityHash64(%s) %% %d = %d", quote(schema.EntityPrimaryFieldName), buckets, bi)
			bucketIndex := fmt.Sprintf("%d/%d", bi, buckets)
			queryCtx, _ = log.FromContext(ctx, "entity", entityType.Name, "chain", chain, "excludeDeleted", excludeDeleted, "bucket", bucketIndex)
			bucketCount, bucketErr := s._countEntity(queryCtx, entityType, chain, excludeDeleted, hasDeletions, condition)
			if bucketErr != nil {
				if !isQueryMemoryLimitExceededError(bucketErr) {
					return 0, bucketErr
				}
				err = bucketErr
				break
			}
			count += bucketCount
		}
	}
	return count, err
}

func (s *Store) _countEntity(
	ctx context.Context,
	entityType *schema.Entity,
	chain string,
	excludeDeleted bool,
	hasDeletions bool,
	extraCondition string,
) (count uint64, err error) {
	var sql string
	if entityType.IsImmutable() {
		sql = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?"+extraCondition,
			s.fullName(s.TableName(entityType)),
			quote(genBlockChainFieldName),
		)
	} else if !excludeDeleted || !hasDeletions {
		// No delete operation, or do not exclude deleted items, can use simple query.
		// Most of the time there is no deletion behavior, and this is enough.
		// Note: for versioned collapsing entities, tableName refers to the deduplicated view
		// (sign=-1 rows excluded), so COUNT(DISTINCT id) correctly reflects live entity count.
		sql = fmt.Sprintf("SELECT COUNT(DISTINCT %s) FROM %s WHERE %s = ?"+extraCondition,
			quote(schema.EntityPrimaryFieldName),
			s.fullName(s.TableName(entityType)),
			quote(genBlockChainFieldName),
		)
	} else if s.useVersionedCollapsingTable(entityType) {
		// SELECT COUNT(*)
		// FROM (
		//  SELECT id
		//  FROM versionedLatestEntity
		//  WHERE __genBlockChain__ = ? AND NOT __deleted__
		//  GROUP BY id, __version__
		//  HAVING sum(__sign__) > 0
		// )
		sql = format.Format("SELECT COUNT(*) "+
			"FROM ("+
			" SELECT %pk#s"+
			" FROM %ft#s"+
			" WHERE %gbc#s = ? AND NOT %ded#s"+extraCondition+
			" GROUP BY %pk#s, %ver#s"+
			" HAVING SUM(%sign#s) > 0"+
			")",
			map[string]any{
				"pk":   quote(schema.EntityPrimaryFieldName),
				"gbc":  quote(genBlockChainFieldName),
				"ded":  quote(deletedFieldName),
				"ver":  quote(versionFieldName),
				"sign": quote(signFieldName),
				"ft":   s.fullName(s.VersionedLatestTableName(entityType)),
			})
	} else {
		// this query is slower but much less memory requirement
		// -------------
		// SELECT COUNT(*)
		// FROM (
		//   SELECT id
		//   FROM entity
		//   WHERE __genBlockChain__ = ?
		//   GROUP BY id
		//   HAVING NOT argMax(__deleted__,__genBlockNumber__)
		// )
		sql = format.Format("SELECT COUNT(*) "+
			"FROM ("+
			" SELECT %pk#s"+
			" FROM %ft#s"+
			" WHERE %gbc#s = ?"+extraCondition+
			" GROUP BY %pk#s"+
			" HAVING NOT argMax(%ded#s,%gbn#s)"+
			")",
			map[string]any{
				"pk":  quote(schema.EntityPrimaryFieldName),
				"gbn": quote(genBlockNumberFieldName),
				"gbc": quote(genBlockChainFieldName),
				"ded": quote(deletedFieldName),
				"ft":  s.fullName(s.TableName(entityType)),
			})
	}
	return s.ctrl.QueryCount(SelectCtx(ctx), sql, chain)
}

func isQueryMemoryLimitExceededError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Query memory limit exceeded")
}

func (s *Store) getAllID(ctx context.Context, entityType *schema.Entity, chain string) (ids set.Set[string], err error) {
	queryCtx, logger := log.FromContext(ctx, "entity", entityType.Name, "chain", chain)
	ids, err = s._getAllID(queryCtx, entityType, chain, "")
	if err == nil {
		return ids, nil
	}
	if !isQueryMemoryLimitExceededError(err) {
		return nil, err
	}
	// memory limit exceeded, too many ids, should bucket query
	const (
		minBuckets = 10
		maxBuckets = 1000
		multi      = 10
	)
	for buckets := minBuckets; err != nil && buckets <= maxBuckets; buckets *= multi {
		logger.Warnfe(err, "result too large, will query in %d bucket", buckets)
		ids, err = set.New[string](), nil
		for bi := 0; bi < buckets; bi++ {
			condition := fmt.Sprintf(" AND cityHash64(%s) %% %d = %d", quote(schema.EntityPrimaryFieldName), buckets, bi)
			bucketIndex := fmt.Sprintf("%d/%d", bi, buckets)
			queryCtx, _ = log.FromContext(ctx, "entity", entityType.Name, "chain", chain, "bucket", bucketIndex)
			bucketResult, bucketErr := s._getAllID(queryCtx, entityType, chain, condition)
			if bucketErr != nil {
				if !isQueryMemoryLimitExceededError(bucketErr) {
					return nil, bucketErr
				} else {
					// still memory limit exceeded, retry with bigger buckets
					err = bucketErr
					break
				}
			}
			bucketResult.Traverse(func(id string) {
				ids.Add(id)
			})
		}
	}
	return ids, err
}

// Query memory limit exceeded
func (s *Store) _getAllID(
	ctx context.Context,
	entityType *schema.Entity,
	chain string,
	extraCondition string,
) (ids set.Set[string], err error) {
	var sql string
	if entityType.IsImmutable() {
		sql = fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?"+extraCondition,
			quote(schema.EntityPrimaryFieldName),
			s.fullName(s.TableName(entityType)),
			quote(genBlockChainFieldName),
		)
	} else {
		sql = fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ? AND %s"+extraCondition,
			s.fullName(s.TableName(entityType)),
			quote(genBlockChainFieldName),
			quote(deletedFieldName),
		)
		count, countErr := s.ctrl.QueryCount(SelectCtx(ctx), sql, chain)
		if countErr != nil {
			return nil, countErr
		}
		if count == 0 {
			// No delete operation, can use simple query.
			// Most of the time there is no deletion behavior, and this is enough.
			// Note: for versioned collapsing entities, tableName refers to the deduplicated view
			// (sign=-1 rows excluded), so DISTINCT id correctly returns only live entity IDs.
			sql = fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE %s = ?"+extraCondition,
				quote(schema.EntityPrimaryFieldName),
				s.fullName(s.TableName(entityType)),
				quote(genBlockChainFieldName),
			)
		} else if s.useVersionedCollapsingTable(entityType) {
			// SELECT id
			// FROM versionedLatestEntity
			// WHERE __genBlockChain__ = ? AND NOT __deleted__
			// GROUP BY id, __version__
			// HAVING sum(__sign__) > 0
			sql = format.Format("SELECT %pk#s "+
				"FROM %ft#s "+
				"WHERE %gbc#s = ? AND NOT %ded#s"+extraCondition+" "+
				"GROUP BY %pk#s, %ver#s "+
				"HAVING SUM(%sign#s) > 0",
				map[string]any{
					"pk":   quote(schema.EntityPrimaryFieldName),
					"gbc":  quote(genBlockChainFieldName),
					"ded":  quote(deletedFieldName),
					"ver":  quote(versionFieldName),
					"sign": quote(signFieldName),
					"ft":   s.fullName(s.VersionedLatestTableName(entityType)),
				})
		} else {
			// SELECT id
			// FROM entity
			// WHERE __genBlockChain__ = ?
			// GROUP BY id
			// HAVING NOT argMax(__deleted__,__genBlockNumber__)
			sql = format.Format("SELECT %pk#s "+
				"FROM %ft#s "+
				"WHERE %gbc#s = ?"+extraCondition+" "+
				"GROUP BY %pk#s "+
				"HAVING NOT argMax(%ded#s,%gbn#s)",
				map[string]any{
					"pk":  quote(schema.EntityPrimaryFieldName),
					"gbn": quote(genBlockNumberFieldName),
					"gbc": quote(genBlockChainFieldName),
					"ded": quote(deletedFieldName),
					"ft":  s.fullName(s.TableName(entityType)),
				})
		}
	}
	ids = set.New[string]()
	err = s.ctrl.Query(SelectCtx(ctx), func(rows driver.Rows) error {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return scanErr
		}
		ids.Add(id)
		return nil
	}, sql, chain)
	return ids, err
}

func (s *Store) getMaxID(ctx context.Context, entityType *schema.Entity, chain string) (int64, error) {
	if !entityType.IsTimeSeries() {
		return 0, fmt.Errorf("%q is not timeseries entity", entityType.Name)
	}
	start := time.Now()
	sql := fmt.Sprintf("SELECT max(%s) FROM %s WHERE %s = ?",
		quote(schema.EntityPrimaryFieldName),
		s.fullName(s.TableName(entityType)),
		quote(genBlockChainFieldName))
	var maxID int64
	err := s.ctrl.Query(SelectCtx(ctx), func(rows driver.Rows) error {
		return rows.Scan(&maxID)
	}, sql, chain)
	_, logger := log.FromContext(ctx,
		"entity", entityType.Name,
		"chain", chain,
		"sql", sql,
		"used", time.Since(start).String())
	if err != nil {
		logger.Errore(err, "get max id failed")
	} else {
		logger.Debugw("get max id succeed", "maxID", maxID)
	}
	return maxID, err
}
