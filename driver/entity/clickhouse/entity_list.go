package clickhouse

import (
	"context"
	"crypto/sha1"
	"fmt"
	"math/rand"
	"reflect"
	"sentioxyz/sentio-core/common/format"
	"sentioxyz/sentio-core/common/log"
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

func (s *Store) buildTemporaryTable(ctx context.Context, filter persistent.EntityFilter) (string, func(), error) {
	_, logger := log.FromContext(ctx)
	start := time.Now()
	name := s.buildTableOrViewName("filter",
		fmt.Sprintf("%x_%s_%08x", sha1.Sum([]byte(filter.String())), start.Format(timeLayoutAllDigital), rand.Uint32()))
	err := s.ctrl.Exec(ctx, fmt.Sprintf("CREATE TEMPORARY TABLE %s (s String) ENGINE = Memory", name))
	if err != nil {
		logger.With("filter", filter.String(), "used", time.Since(start).String()).
			Errorfe(err, "created temporary table %s failed", name)
		return name, nil, err
	}
	logger.With("used", time.Since(start).String()).Debugf("created temporary table %s", name)
	const dropTempTableTimeout = time.Second * 30
	epilogue := func() {
		dropCtx, cancel := context.WithTimeout(context.Background(), dropTempTableTimeout)
		defer cancel()
		dropStart := time.Now()
		dropErr := s.ctrl.Exec(dropCtx, fmt.Sprintf("DROP TABLE %s", name))
		if dropErr != nil {
			logger.With("filter", filter.String(), "used", time.Since(dropStart).String()).
				Warnfe(dropErr, "drop temporary table %s failed", name)
		} else {
			logger.With("used", time.Since(dropStart).String()).
				Debugf("drop temporary table %s succeed", name)
		}
	}
	for si := 0; si < len(filter.Value); si += s.tableOpt.BatchInsertSizeLimit {
		n := min(s.tableOpt.BatchInsertSizeLimit, len(filter.Value)-si)
		start = time.Now()
		sql := fmt.Sprintf("INSERT INTO %s (s) VALUES %s", name, utils.Dup("(?)", ",", n))
		err = s.ctrl.Exec(ctx, sql, filter.Value[si:si+n]...)
		if err != nil {
			logger.With("filter", filter.String(), "used", time.Since(start).String()).
				Errorfe(err, "insert %d:%d/%d rows to temporary table %s failed", si, si+n, len(filter.Value), name)
			return name, epilogue, err
		}
		logger.With("used", time.Since(start).String()).
			Debugf("insert %d:%d/%d rows to temporary table %s succeed", si, si+n, len(filter.Value), name)
	}
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
			slot = fmt.Sprintf("(SELECT s FROM %s)", tempTable)
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

func SplitFilters(filters []persistent.EntityFilter) (primaryKeyFilters, otherFilters []persistent.EntityFilter) {
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
) (result []*EntityBox, err error) {
	if entityType.IsCache() {
		return nil, nil
	}

	start := time.Now()
	kit := s.NewEntity(entityType)
	var sql string
	var sqlArgs []any
	if s.UseVersionedCollapsingTable(entityType) {
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
		selects := utils.FilterArr(kit.FieldNamesForGet(), func(fn string) bool {
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
			joinWithQuote(kit.FieldNamesForGet(), ","),
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
		lastFields := utils.Prepend(utils.FilterArr(kit.FieldNamesForGet(), func(fn string) bool {
			return fn != schema.EntityPrimaryFieldName &&
				fn != genBlockChainFieldName &&
				fn != genBlockNumberFieldName &&
				fn != deletedFieldName
		}), genBlockNumberFieldName, deletedFieldName)
		lastAs := make([]string, len(lastFields))
		for i, fieldName := range lastFields {
			lastAs[i] = fmt.Sprintf("__last__.%d AS %s", i+1, quote(fieldName))
		}
		primaryKeyFilters, otherFilters := SplitFilters(filters)
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
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		box, scanErr := kit.ScanOne(rows)
		if scanErr != nil {
			return scanErr
		}
		box.Entity = entityType.Name
		result = append(result, box)
		return nil
	}, sql, sqlArgs...)
	_, logger := log.FromContext(ctx,
		"processorID", s.processorID,
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

func (s *Store) ListEntities(
	ctx context.Context,
	entityType *schema.Entity,
	chain string,
	filters []persistent.EntityFilter,
	limit int,
) ([]*persistent.EntityBox, error) {
	result, err := s.listEntities(ctx, entityType, chain, filters, true, limit)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return utils.MapSliceNoError(result, func(box *EntityBox) *persistent.EntityBox {
		return box.Get()
	}), nil
}

func (s *Store) CountEntity(ctx context.Context, entityType *schema.Entity, chain string) (count uint64, err error) {
	start := time.Now()
	var sql string
	if entityType.IsImmutable() {
		sql = fmt.Sprintf("SELECT COUNT(DISTINCT %s) FROM %s WHERE %s = ?",
			quote(schema.EntityPrimaryFieldName),
			s.TableName(entityType),
			quote(genBlockChainFieldName))
	} else {
		// SELECT COUNT(*)
		// FROM (
		//   SELECT id, MAX((__genBlockNumber__, __deleted__)) as __last__
		//   FROM entity
		//   WHERE __genBlockChain__ = ?
		//   GROUP by id
		// )
		// WHERE NOT __last__.2
		sql = format.Format("SELECT COUNT(*) "+
			"FROM ( "+
			"  SELECT %pk#s, MAX((%gbn#s,%ded#s)) AS __last__ "+
			"  FROM %ft#s "+
			"  WHERE %gbc#s = ?"+
			"  GROUP BY %pk#s"+
			") "+
			"WHERE NOT __last__.2",
			map[string]any{
				"pk":  quote(schema.EntityPrimaryFieldName),
				"gbn": quote(genBlockNumberFieldName),
				"gbc": quote(genBlockChainFieldName),
				"ded": quote(deletedFieldName),
				"ft":  s.fullName(s.TableName(entityType)),
			})
	}
	count, err = s.ctrl.QueryCount(SelectCtx(ctx), sql, chain)
	_, logger := log.FromContext(ctx,
		"processorID", s.processorID,
		"entity", entityType.Name,
		"chain", chain,
		"sql", sql,
		"used", time.Since(start).String())
	if err != nil {
		logger.Errore(err, "count failed")
	} else {
		logger.Debugw("count succeed", "count", count)
	}
	return
}

func (s *Store) GetAllID(ctx context.Context, entityType *schema.Entity, chain string) (ids []string, err error) {
	start := time.Now()
	var sql string
	if entityType.IsImmutable() {
		sql = fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE %s = ?",
			quote(schema.EntityPrimaryFieldName),
			s.TableName(entityType),
			quote(genBlockChainFieldName))
	} else {
		// SELECT id
		// FROM (
		//   SELECT id, MAX((__genBlockNumber__, __deleted__)) as __last__
		//   FROM entity
		//   WHERE __genBlockChain__ = ?
		//   GROUP by id
		// )
		// WHERE NOT __last__.2
		sql = format.Format("SELECT %pk#s "+
			"FROM ( "+
			"  SELECT %pk#s, MAX((%gbn#s,%ded#s)) AS __last__ "+
			"  FROM %ft#s "+
			"  WHERE %gbc#s = ?"+
			"  GROUP BY %pk#s"+
			") "+
			"WHERE NOT __last__.2",
			map[string]any{
				"pk":  quote(schema.EntityPrimaryFieldName),
				"gbn": quote(genBlockNumberFieldName),
				"gbc": quote(genBlockChainFieldName),
				"ded": quote(deletedFieldName),
				"ft":  s.fullName(s.TableName(entityType)),
			})
	}
	err = s.ctrl.Query(SelectCtx(ctx), func(rows driver.Rows) error {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return scanErr
		}
		ids = append(ids, id)
		return nil
	}, sql, chain)
	_, logger := log.FromContext(ctx,
		"processorID", s.processorID,
		"entity", entityType.Name,
		"chain", chain,
		"sql", sql,
		"used", time.Since(start).String())
	if err != nil {
		logger.Errore(err, "list ids failed")
	} else {
		logger.Debugw("list ids succeed", "count", len(ids))
	}
	return
}

func (s *Store) GetMaxID(ctx context.Context, entityType *schema.Entity, chain string) (int64, error) {
	if !entityType.IsTimeSeries() {
		return 0, fmt.Errorf("%q is not timeseries entity", entityType.Name)
	}
	start := time.Now()
	sql := fmt.Sprintf("SELECT max(%s) FROM %s WHERE %s = ?",
		quote(schema.EntityPrimaryFieldName),
		s.TableName(entityType),
		quote(genBlockChainFieldName))
	var maxID int64
	err := s.ctrl.Query(SelectCtx(ctx), func(rows driver.Rows) error {
		return rows.Scan(&maxID)
	}, sql, chain)
	_, logger := log.FromContext(ctx,
		"processorID", s.processorID,
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
