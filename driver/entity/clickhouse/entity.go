package clickhouse

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/format"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
	"sentioxyz/sentio-core/driver/entity/schema/exp"
)

func (s *Store) GetEntity(
	ctx context.Context,
	entityType *schema.Entity,
	chain string,
	id string,
) (box *persistent.EntityBox, err error) {
	if entityType.IsCache() {
		return nil, nil
	}

	// prepare and build sql
	start := time.Now()
	kit := s.NewEntity(entityType)
	var sql string
	if s.UseVersionedCollapsingTable(entityType) {
		sql = fmt.Sprintf("SELECT %s FROM %s WHERE %s = ? AND %s = ? AND %s > 0 ORDER BY %s DESC LIMIT 1",
			joinWithQuote(kit.FieldNamesForGet(), ","),
			s.fullName(s.VersionedTableName(entityType)),
			quote(schema.EntityPrimaryFieldName),
			quote(genBlockChainFieldName),
			quote(signFieldName),
			quote(genBlockNumberFieldName))
	} else {
		sql = fmt.Sprintf("SELECT %s FROM %s WHERE %s = ? AND %s = ?",
			joinWithQuote(kit.FieldNamesForGet(), ","),
			s.fullName(s.TableName(entityType)),
			quote(schema.EntityPrimaryFieldName),
			quote(genBlockChainFieldName))
		if !entityType.IsImmutable() {
			sql = sql + fmt.Sprintf(" ORDER BY %s DESC LIMIT 1", quote(genBlockNumberFieldName))
		}
	}

	// execute query and get the response
	err = s.ctrl.Query(SelectCtx(ctx), func(rows driver.Rows) error {
		box_, scanErr := kit.ScanOne(rows)
		box = box_.Get()
		return scanErr
	}, sql, id, chain)
	_, logger := log.FromContext(ctx,
		"processorID", s.processorID,
		"entity", entityType.Name,
		"id", id,
		"chain", chain,
		"sql", sql,
		"used", time.Since(start).String())
	logger = logger.With("used", time.Since(start).String())
	if err != nil {
		logger.Errore(err, "get failed")
		return box, err
	}
	logger.Debug("get succeed")
	if box != nil {
		box.Entity = entityType.Name
	}
	return box, nil
}

func (s *Store) queryExistEntity(
	ctx context.Context,
	entityType *schema.Entity,
	chain string,
	ids ...string,
) (exists []string, err error) {
	_, logger := log.FromContext(ctx, "entity", entityType.Name, "chain", chain, "ids", utils.ArrSummary(ids))
	var sql string
	var args []any
	switch len(ids) {
	case 0:
		return
	case 1:
		sql = format.Format("SELECT distinct %primaryField#s "+
			"FROM %tableName#s "+
			"WHERE %primaryField#s = ? AND %gbc#s = ?",
			map[string]any{
				"primaryField": quote(schema.EntityPrimaryFieldName),
				"tableName":    s.fullName(s.TableName(entityType)),
				"gbc":          quote(genBlockChainFieldName),
			})
		args = []any{ids[0], chain}
	default:
		sql = format.Format("SELECT distinct %primaryField#s "+
			"FROM %tableName#s "+
			"WHERE %primaryField#s IN ? AND %gbc#s = ?",
			map[string]any{
				"primaryField": quote(schema.EntityPrimaryFieldName),
				"tableName":    s.fullName(s.TableName(entityType)),
				"gbc":          quote(genBlockChainFieldName),
			})
		args = []any{ids, chain}
	}
	logger.With("sql", sql)
	err = s.ctrl.Query(SelectCtx(ctx), func(rows driver.Rows) error {
		var ex string
		if scanErr := rows.Scan(&ex); scanErr != nil {
			return scanErr
		}
		exists = append(exists, ex)
		return nil
	}, sql, args...)
	if err != nil {
		logger.Errore(err, "query exist entity count failed")
		return nil, fmt.Errorf("query exist entity count failed: %w", err)
	}
	logger.Debugw("query exist entity succeed", "exists", utils.ArrSummary(exists))
	return
}

func fetchIDSet(entities []persistent.EntityBox) map[string]bool {
	ids := make(map[string]bool)
	for _, entity := range entities {
		ids[entity.ID] = true
	}
	return ids
}

func (s *Store) setEntities(
	ctx context.Context,
	entityType *schema.Entity,
	chain string,
	entities []persistent.EntityBox,
) (created int, err error) {
	if entityType.IsCache() {
		return 0, nil
	}

	batchSize := s.tableOpt.BatchInsertSizeLimit
	if batchSize <= 0 {
		batchSize = DefaultCreateTableOption.BatchInsertSizeLimit
	}
	useVersionedCollapsingTable := s.UseVersionedCollapsingTable(entityType)
	ctx, logger := log.FromContext(ctx,
		"processorID", s.processorID,
		"entity", entityType.Name,
		"chain", chain,
		"count", len(entities),
		"batchSize", batchSize,
		"useVersionedCollapsingTable", useVersionedCollapsingTable)

	kit := s.NewEntity(entityType)

	// trim entities
	dict := make(map[string][]persistent.EntityBox)
	for _, entity := range entities {
		dict[entity.ID] = append(dict[entity.ID], entity)
	}
	for _, history := range dict {
		sort.Slice(history, func(i, j int) bool {
			return history[i].GenBlockNumber < history[j].GenBlockNumber
		})
	}

	var batchIndex int
	queue := make([]persistent.EntityBox, 0, batchSize)
	doneIds := make(map[string]bool)
	preBoxes := make(map[string]*EntityBox)
	zeroDataBox := persistent.EntityBox{Data: make(map[string]any)}
	zeroDataBox.FillLostFields(make(map[string]any), entityType)
	tableName := utils.Select(useVersionedCollapsingTable, s.VersionedTableName(entityType), s.TableName(entityType))
	fields := joinWithQuote(kit.FieldNamesForSet(), ",")
	slots := "(" + strings.Join(kit.FieldSlotsForSet(), ",") + ")"

	reportUpdateImmutable := func(ids map[string]bool, logger *log.SentioLogger) error {
		var sample persistent.EntityBox
		for _, box := range queue {
			if ids[box.ID] {
				sample = box
			}
		}
		summary := fmt.Sprintf("with id %s sample box %s", utils.ArrSummary(utils.GetOrderedMapKeys(ids)), sample.String())
		logger.Errorf("set immutable entities %s", summary)
		return errors.Wrapf(persistent.ErrUpdateImmutable,
			"set %s entities in chain %s %s", entityType.Name, chain, summary)
	}

	// insert data in queue and update created
	flush := func() error {
		if len(queue) == 0 {
			return nil
		}
		start := time.Now()
		batchLogger := logger.With("batchIndex", batchIndex)
		batchIndex++
		ids := fetchIDSet(queue)
		newIds := utils.SetSub(ids, doneIds)
		// confirm the created quantity and immutable check
		if len(newIds) != len(ids) && entityType.IsImmutable() {
			return reportUpdateImmutable(utils.SetSub(ids, newIds), batchLogger)
		}
		var insertSetting map[string]any
		if useVersionedCollapsingTable {
			insertSetting = enableVersionedCollapsingInsertSettings()
			// select pre-values and update preBoxes
			exists, queryErr := s.listEntities(ctx, entityType, chain, []persistent.EntityFilter{{
				Field: entityType.GetPrimaryKeyField(),
				Op:    persistent.EntityFilterOpIn,
				Value: utils.ToAnyArray(utils.GetOrderedMapKeys(newIds)),
			}}, false, math.MaxInt)
			if queryErr != nil {
				batchLogger.Errorfe(queryErr, "list pre-values for set entities failed")
				return errors.Wrapf(queryErr, "set %s entities in chain %s failed: list pre-values failed",
					entityType.Name, chain)
			}
			for _, box := range exists {
				preBoxes[box.ID] = box
			}
			created += len(newIds) - len(exists)
		} else {
			exists, queryErr := s.queryExistEntity(ctx, entityType, chain, utils.GetOrderedMapKeys(newIds)...)
			if queryErr != nil {
				batchLogger.Errorfe(queryErr, "query exists for set entities failed")
				return errors.Wrapf(queryErr, "set %s entities in chain %s failed: query exists failed",
					entityType.Name, chain)
			}
			if entityType.IsImmutable() && len(exists) > 0 {
				return reportUpdateImmutable(utils.BuildSet(exists), batchLogger)
			}
			created += len(newIds) - len(exists)
		}
		// actually insert rows
		var slotValues []any
		var rows int
		for _, box := range queue {
			cur := EntityBox{EntityBox: box, Sign: 1, Version: 1}
			if pre, has := preBoxes[box.ID]; useVersionedCollapsingTable && has {
				// insert the opposite row for pre-value, and set the new version for current row
				cur.Version = pre.Version + 1
				pre.Sign = -1
				pre.GenBlockNumber = cur.GenBlockNumber
				pre.GenBlockTime = cur.GenBlockTime
				pre.GenBlockHash = cur.GenBlockHash
				slotValues, rows = append(slotValues, kit.FieldValuesForSet(*pre, zeroDataBox.Data)...), rows+1
			}
			slotValues, rows = append(slotValues, kit.FieldValuesForSet(cur, zeroDataBox.Data)...), rows+1
			if useVersionedCollapsingTable {
				preBoxes[box.ID] = &cur
			}
		}
		sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", s.fullName(tableName), fields, utils.Dup(slots, ",", rows))
		uniqToken := strconv.FormatUint(rand.Uint64(), 16)
		insertErr := s.ctrl.Exec(chx.InsertCtx(ctx, uniqToken, insertSetting), sql, slotValues...)
		batchLogger = batchLogger.With("uniqToken", uniqToken, "rows", rows, "used", time.Since(start).String())
		if insertErr != nil {
			batchLogger.Errore(insertErr, "batch insert failed")
			return insertErr
		}
		batchLogger.Debug("batch insert succeed")
		// reset queue and update doneIds
		queue = queue[:0]
		utils.MergeMap(doneIds, newIds)
		return nil
	}

	// build batch and flush
	for _, history := range dict {
		for _, box := range history {
			queue = append(queue, box)
			if len(queue) >= batchSize {
				if err = flush(); err != nil {
					return
				}
			}
		}
	}
	if len(queue) > 0 {
		if err = flush(); err != nil {
			return
		}
	}
	return
}

func (s *Store) SetEntities(
	ctx context.Context,
	entityType *schema.Entity,
	boxes []persistent.EntityBox,
) (created int, err error) {
	dict := make(map[string][]persistent.EntityBox)
	for _, box := range boxes {
		dict[box.GenBlockChain] = append(dict[box.GenBlockChain], box)
	}
	for chain, list := range dict {
		var chainCreated int
		if chainCreated, err = s.setEntities(ctx, entityType, chain, list); err != nil {
			return
		}
		created += chainCreated
	}
	return
}

type clickhouseFuncAliasController struct {
	exp.EmptyAliasController
}

func (c clickhouseFuncAliasController) GetOpName(org string) string {
	switch strings.ToLower(org) {
	case "max":
		return "greatest"
	case "min":
		return "least"
	default:
		return org
	}
}

var chFuncAliasCtl = clickhouseFuncAliasController{}

func (s *Store) GrowthAggregation(ctx context.Context, chain string, curBlockTime time.Time) error {
	_, logger := log.FromContext(ctx, "chain", chain, "curBlockTime", curBlockTime.String())
	for _, agg := range s.sch.ListAggregations() {
		var dimFieldNames []string // dim fields without id and timestamp field
		for _, f := range agg.DimFields {
			if f.Name != schema.EntityPrimaryFieldName && f.Name != schema.EntityTimestampFieldName {
				dimFieldNames = append(dimFieldNames, quote(BaseField{Def: f}.FieldMainName()))
			}
		}
		var aggFieldNames []string
		var aggFields []string
		for _, f := range agg.AggFields {
			aggFieldNames = append(aggFieldNames, quote(BaseField{Def: f.FieldDefinition}.FieldMainName()))
			switch fn := f.GetAggFunc(); fn {
			case "sum", "min", "max":
				aggFields = append(aggFields, fmt.Sprintf("%s(%s)", fn, f.GetAggExp().Text(chFuncAliasCtl)))
			case "count":
				aggFields = append(aggFields, "count(*)")
			case "first":
				aggFields = append(aggFields, fmt.Sprintf("first_value(%s)", f.GetAggExp().Text(chFuncAliasCtl)))
			case "last":
				aggFields = append(aggFields, fmt.Sprintf("last_value(%s)", f.GetAggExp().Text(chFuncAliasCtl)))
			default:
				return fmt.Errorf("unknown agg fn %q for %s.%s", fn, agg.GetName(), f.Name)
			}
		}
		for _, interval := range agg.GetIntervals() {
			uniqToken := strconv.FormatUint(rand.Uint64(), 16)
			var twField string         // time window of the timestamp, type is equal to the timestamp field
			var curBlockTimeWin string // time window of current block time, type is equal to the timestamp field
			if s.feaOpt.TimestampUseDateTime64 {
				// type of timestamp field is DateTime64(6)
				twField = fmt.Sprintf("toStartOfInterval(%s,%s)", quote(schema.EntityTimestampFieldName), interval.PGInterval())
				curBlockTimeWin = fmt.Sprintf("toStartOfInterval(toDateTime64('%s',6),%s)",
					curBlockTime.UTC().Format(time.DateTime), interval.PGInterval())
			} else {
				// type of timestamp field is Int64 and value is micro second timestamp
				twField = fmt.Sprintf("toInt64(toUnixTimestamp(toStartOfInterval(toDateTime64(%s/1000000,6),%s))*1000000)",
					quote(schema.EntityTimestampFieldName), interval.PGInterval())
				curBlockTimeWin = fmt.Sprintf("toInt64(toUnixTimestamp(toStartOfInterval(toDateTime64('%s',6),%s))*1000000)",
					curBlockTime.UTC().Format(time.DateTime), interval.PGInterval())
			}
			sql := format.Format("INSERT INTO %aggTableName#s ("+
				" %primaryField#s,"+
				" %timestampField#s,"+
				" %dimFieldNames#s,"+
				" %aggFieldNames#s,"+
				" %gbn#s,"+
				" %gbt#s,"+
				" %gbh#s,"+
				" %gbc#s,"+
				" %deleted#s,"+
				" %insertTime#s,"+
				" %intervalField#s"+
				") "+
				"SELECT "+
				" max(%primaryField#s),"+
				" %twField#s as __timeWin__,"+
				" %dimFieldNames#s,"+
				" %aggFields#s,"+
				" max(%gbn#s),"+
				" max(%gbt#s),"+
				" '',"+
				" %gbc#s,"+
				" false,"+
				" NOW(),"+
				" '%intervalText#s' "+
				"FROM %srcTableName#s "+
				"WHERE"+
				" __timeWin__ > ("+
				" SELECT MAX(%timestampField#s)"+
				" FROM %aggTableName#s"+
				" WHERE %intervalField#s = '%intervalText#s' AND %gbc#s = '%chain#s'"+
				" ) AND"+
				" %timestampField#s < %curBlockTimeWin#s AND"+
				" %gbc#s = '%chain#s' "+
				"GROUP BY __timeWin__, %dimFieldNames#s, %gbc#s",
				map[string]any{
					"aggTableName":    s.fullName(s.TableName(agg)),
					"srcTableName":    s.fullName(s.TableName(s.sch.GetEntity(agg.GetSource()))),
					"primaryField":    quote(schema.EntityPrimaryFieldName),
					"timestampField":  quote(schema.EntityTimestampFieldName),
					"intervalField":   quote(aggIntervalFieldName),
					"twField":         twField,
					"interval":        interval.PGInterval(),
					"dimFieldNames":   strings.Join(dimFieldNames, ", "),
					"aggFieldNames":   strings.Join(aggFieldNames, ", "),
					"aggFields":       strings.Join(aggFields, ", "),
					"gbn":             quote(genBlockNumberFieldName),
					"gbt":             quote(genBlockTimeFieldName),
					"gbh":             quote(genBlockHashFieldName),
					"gbc":             quote(genBlockChainFieldName),
					"deleted":         quote(deletedFieldName),
					"insertTime":      quote(timestampFieldName),
					"curBlockTimeWin": curBlockTimeWin,
					"chain":           chain,
					"intervalText":    interval.String(),
				})
			start := time.Now()
			err := s.ctrl.Exec(chx.InsertSelectCtx(chx.InsertCtx(ctx, uniqToken)), sql)
			exeLogger := logger.With(
				"agg", agg.Name,
				"interval", interval.String(),
				"uniqToken", uniqToken,
				"sql", sql,
				"used", time.Since(start).String())
			if err != nil {
				exeLogger.Errorfe(err, "growth aggregation failed")
				return err
			} else {
				exeLogger.Debugf("growth aggregation succeeded")
			}
		}
	}
	return nil
}

func (s *Store) reorgInTable(ctx context.Context, blockNumber int64, chain string, table string) (uint64, error) {
	sql := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s > ? AND %s = ?",
		s.fullName(table),
		quote(genBlockNumberFieldName),
		quote(genBlockChainFieldName))
	exists, err := s.ctrl.QueryCount(SelectCtx(ctx), sql, blockNumber, chain)
	if err != nil {
		return 0, fmt.Errorf("query rows in %s to delete failed: %w", table, err)
	}
	if exists == 0 {
		return 0, nil
	}
	sql = fmt.Sprintf("DELETE FROM %s WHERE %s > ? AND %s = ?",
		s.fullName(table),
		quote(genBlockNumberFieldName),
		quote(genBlockChainFieldName))
	if enableClickhouseLightDelete {
		ctx = chx.LightDeleteCtx(ctx)
	}
	if err = s.ctrl.Exec(ctx, sql, blockNumber, chain); err != nil {
		return 0, fmt.Errorf("delete rows in %s failed: %w", table, err)
	}
	return exists, nil
}

func (s *Store) reorgInVersionedLatestTable(
	ctx context.Context,
	blockNumber int64,
	chain string,
	table string,
	rawTable string,
) (deleted uint64, rebuilt bool, err error) {
	// First try to delete rows gen after blockNumber
	deleted, err = s.reorgInTable(ctx, blockNumber, chain, table)
	if err != nil {
		return
	}

	// Check data integrity.
	// Because change history in table may have collapsed, there may be missing items after deletion.
	//
	// data in raw table may be:
	//  bn:  1  2  |  3
	// ============|====
	// id0:  + -+  | -+
	// id1:  + -+  | -+
	// id2:  + -+  | -+
	// id3:  + -+  | -+
	//
	// data in table may be missing some of the pairs like:
	//  bn:  1  2  |  3
	// ============|====
	// id0:  + -+  | -+
	// id1:     +  | -+
	// id2:  + -   |  +
	// id3:        |  +
	//
	// if delete rows at the right of '|', 'id2' and 'id3' will lost.

	sql := fmt.Sprintf("SELECT COUNT(distinct %s) FROM %s WHERE %s = ? AND %s <= ?",
		schema.EntityPrimaryFieldName, s.fullName(rawTable), genBlockChainFieldName, genBlockNumberFieldName)
	var expCount, realCount uint64
	expCount, err = s.ctrl.QueryCount(SelectCtx(ctx), sql, chain, blockNumber)
	if err != nil {
		err = fmt.Errorf("count distinct id in %s failed: %w", rawTable, err)
		return
	}
	sql = format.Format("SELECT COUNT(*) FROM ("+
		"SELECT %pk#s "+
		"FROM %table#s "+
		"WHERE %gbc#s = ? AND %gbn#s <= ? "+
		"GROUP BY %pk#s, %version#s "+
		"HAVING SUM(%sign#s) > 0"+
		")",
		map[string]any{
			"pk":      schema.EntityPrimaryFieldName,
			"table":   s.fullName(table),
			"gbc":     genBlockChainFieldName,
			"gbn":     genBlockNumberFieldName,
			"version": versionFieldName,
			"sign":    signFieldName,
		})
	realCount, err = s.ctrl.QueryCount(SelectCtx(ctx), sql, chain, blockNumber)
	if err != nil {
		err = fmt.Errorf("count distinct id in %s failed: %w", table, err)
		return
	}

	if expCount == realCount {
		// no missing items
		return
	}

	// Now the missing parts need to be filled in.
	// To confirm what needs to be filled, we need to scan all data in the two tables and comparing,
	// it will be very heavy. Considering that the probability is relatively low, we just rebuild them all.
	rebuilt = true
	// always partition by chain, so use drop partition delete all data in the chain
	sql = fmt.Sprintf("ALTER TABLE %s DROP PARTITION ?", s.ctrl.FullNameWithOnCluster(chx.FullName{
		Database: s.database,
		Name:     table,
	}))
	if err = s.ctrl.Exec(ctx, sql, chain); err != nil {
		err = fmt.Errorf("delete all in %s failed: %w", table, err)
		return
	}
	sql = fmt.Sprintf("INSERT INTO %s SELECT * FROM %s WHERE %s = ? AND %s <= ?",
		s.fullName(table), s.fullName(rawTable), genBlockChainFieldName, genBlockNumberFieldName)
	if err = s.ctrl.Exec(chx.InsertSelectCtx(ctx), sql, chain, blockNumber); err != nil {
		err = fmt.Errorf("rebuild all in %s from %s failed: %w", table, rawTable, err)
		return
	}
	return
}

func (s *Store) Reorg(ctx context.Context, blockNumber int64, chain string) error {
	_, logger := log.FromContext(ctx)
	for _, entityType := range s.sch.ListEntities(false) {
		failMsg := fmt.Sprintf("delete %q entities created after block %d in chain %q failed",
			entityType.Name, blockNumber, chain)
		tableName := s.TableName(entityType)
		if s.UseVersionedCollapsingTable(entityType) {
			tableName = s.VersionedTableName(entityType)
		}
		start := time.Now()
		deleted, err := s.reorgInTable(ctx, blockNumber, chain, tableName)
		entityLogger := logger.With(
			"processorID", s.processorID,
			"entity", entityType.Name,
			"blockNumber", blockNumber,
			"chain", chain,
			"used", time.Since(start).String())
		if err != nil {
			entityLogger.Errorfe(err, "delete failed")
			return fmt.Errorf("%s: %w", failMsg, err)
		}
		entityLogger.Infof("deleted %d rows in table %s", deleted, tableName)

		if s.UseVersionedCollapsingTable(entityType) {
			// delete in versionedLatestEntity table
			latestTableName := s.VersionedLatestTableName(entityType)
			var rebuilt bool
			deleted, rebuilt, err = s.reorgInVersionedLatestTable(ctx, blockNumber, chain, latestTableName, tableName)
			if err != nil {
				entityLogger.Errorfe(err, "delete entities failed")
				return fmt.Errorf("%s: %w", failMsg, err)
			}
			if rebuilt {
				entityLogger.Infof("rebuilt table %s", latestTableName)
			} else {
				entityLogger.Infof("deleted %d rows in table %s", deleted, latestTableName)
			}
		}
	}
	for _, agg := range s.sch.ListAggregations() {
		failMsg := fmt.Sprintf("delete %q aggregation created after block %d in chain %q failed",
			agg.Name, blockNumber, chain)
		tableName := s.TableName(agg)
		start := time.Now()
		deleted, err := s.reorgInTable(ctx, blockNumber, chain, tableName)
		entityLogger := logger.With(
			"processorID", s.processorID,
			"aggregation", agg.Name,
			"blockNumber", blockNumber,
			"chain", chain,
			"used", time.Since(start).String())
		if err != nil {
			entityLogger.Errorfe(err, "delete failed")
			return fmt.Errorf("%s: %w", failMsg, err)
		}
		entityLogger.Infof("deleted %d rows in table %s", deleted, tableName)
	}
	return nil
}
