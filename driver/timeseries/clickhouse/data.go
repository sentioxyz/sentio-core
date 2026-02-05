package clickhouse

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/format"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/timer"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func (s *Store) AppendData(ctx context.Context, data []timeseries.Dataset, chainID string, curTime time.Time) error {
	s.meta.Lock()
	defer s.meta.Unlock()
	_, logger := log.FromContext(ctx,
		"processorID", s.processorID,
		"chainID", chainID,
		"curTime", curTime.String(),
		"datasets", timeseries.GetDatasetsSummary(data))
	logger.Infof("ready to append data")
	tm := timer.NewTimer()
	allTm := tm.Start("ALL")

	// merge data
	mergeTm := tm.Start("M")
	var err error
	if data, err = timeseries.MergeDatasets(data); err != nil {
		return err
	}
	mergeTm.End()

	// sync meta
	syncTm := tm.Start("S")
	err = s.syncMetas(ctx, data)
	if err != nil {
		return err
	}
	syncTm.End()

	// insert data
	insertTm := tm.Start("I")
	g, gctx := errgroup.WithContext(ctx)
	for _, ds_ := range data {
		ds := ds_
		if len(ds.Rows) == 0 {
			continue
		}
		if ds.Meta.Aggregation != nil {
			continue
		}
		ds.Meta, _ = utils.GetFromK2Map(s.meta.Metas, ds.Meta.Type, ds.Meta.Name)
		g.Go(func() error {
			return s.insertData(ctx, ds, chainID)
		})
	}
	if err = g.Wait(); err != nil {
		return err
	}
	insertTm.End()

	// aggregation growth
	growthTm := tm.Start("G")
	g, gctx = errgroup.WithContext(ctx)
	for _, metas := range s.meta.Metas {
		for _, meta_ := range metas {
			if meta_.Aggregation == nil {
				continue
			}
			meta := meta_
			g.Go(func() error {
				return s.aggregationGrowth(gctx, meta, chainID, curTime)
			})
		}
	}
	if err = g.Wait(); err != nil {
		return err
	}
	growthTm.End()

	// print report
	allTm.End()
	logger.Infow("append data succeed", "used", tm.ReportDistribution("ALL", "M,S,I,G"))
	return nil
}

func (s *Store) insertData(ctx context.Context, ds timeseries.Dataset, chainID string) error {
	_, logger := log.FromContext(ctx,
		"processorID", s.processorID,
		"chainID", chainID,
		"meta", ds.Meta.GetFullName(),
		"rows", len(ds.Rows))
	logger.Debug("will insert rows")
	fieldNames := utils.GetOrderedMapKeys(ds.Meta.Fields)
	chainIDField := ds.Meta.GetChainIDField()
	var modifier func(row timeseries.Row) timeseries.Row
	if ds.Type == timeseries.MetaTypeCounter {
		logger.Debug("will query last value of each series")
		labelFields := ds.Meta.GetFieldsByRole(timeseries.FieldRoleSeriesLabel)
		valueFields := ds.Meta.GetFieldsByRole(timeseries.FieldRoleSeriesValue)
		var seriesLast map[string]timeseries.Row
		cache, has := s.cachedCounterSeriesLatest.Get(ds.Name)
		if has && timeseries.SameFields(cache.labelFields, labelFields) && timeseries.SameFields(cache.valueFields, valueFields) {
			seriesLast = cache.seriesLast
		} else {
			// has new fields, the previously cached data needs to be discarded
			// because the calculation result of the series ID may be different
			sql := format.Format("SELECT %labelFieldNames#s, %lastValueFields#s "+
				"FROM (SELECT * FROM %tableName#s WHERE %chainIDFieldName#s = '%chainID#s' ORDER BY %timestampFieldName#s) "+
				"GROUP BY %labelFieldNames#s",
				map[string]any{
					"labelFieldNames": strings.Join(utils.MapSliceNoError(labelFields, s.buildFieldName), ", "),
					"lastValueFields": strings.Join(utils.MapSliceNoError(valueFields, func(f timeseries.Field) string {
						return fmt.Sprintf("last_value(%s)", s.buildFieldName(f))
					}), ", "),
					"tableName":          s.buildTableName(ds.Meta),
					"timestampFieldName": s.buildFieldName(ds.Meta.GetTimestampField()),
					"chainIDFieldName":   s.buildFieldName(chainIDField),
					"chainID":            chainID,
				})
			seriesLast = make(map[string]timeseries.Row)
			startTime := time.Now()
			err := queryAndScan(ctx, s.client, func(rows driver.Rows) error {
				for rows.Next() {
					if row, err := scanRow(rows, append(labelFields, valueFields...)); err != nil {
						return err
					} else {
						seriesLast[buildSeriesID(row, labelFields)] = row
					}
				}
				return nil
			}, sql)
			if err != nil {
				logger.With("used", time.Since(startTime).String()).Errore(err, "query last value of each series failed")
				return fmt.Errorf("query last value of each series for %q failed: %v", ds.Meta.GetFullName(), err)
			}
			logger.Infow("got last value of all series",
				"used", time.Since(startTime).String(),
				"series", buildSeriesSummary(seriesLast, 10))
			s.cachedCounterSeriesLatest.Put(ds.Name, &counterSeriesLatestCache{
				labelFields: labelFields,
				valueFields: valueFields,
				seriesLast:  seriesLast,
			})
		}
		modifier = func(row timeseries.Row) timeseries.Row {
			seriesID := buildSeriesID(row, labelFields)
			lastRow, has := seriesLast[seriesID]
			if !has {
				logger.Infow("new series", "seriesID", seriesID, "row", row)
				seriesLast[seriesID] = row
				return row
			}
			after := addValues(row, lastRow, valueFields)
			seriesLast[seriesID] = after
			logger.Debugw("cumulative", "before", row, "after", after)
			return after
		}
	}
	sql := fmt.Sprintf("INSERT INTO %s (%s)", s.buildTableName(ds.Meta), strings.Join(fieldNames, ","))
	startTime := time.Now()
	for bi, cursor := 0, 0; cursor < len(ds.Rows); bi++ {
		next := min(len(ds.Rows), cursor+s.option.BatchInsertSizeLimit)
		batchStartTime := time.Now()
		uniqToken := strconv.FormatUint(rand.Uint64(), 16)
		pageLogger := logger.With("batch", fmt.Sprintf("%d-%d/%d", cursor, next, len(ds.Rows)), "uniqToken", uniqToken)
		batch, err := s.client.PrepareBatch(chx.InsertCtx(ctx, uniqToken), sql)
		if err != nil {
			pageLogger.Errore(err, "prepare batch failed")
			return fmt.Errorf("prepare batch for %q failed: %w", ds.Meta.GetFullName(), err)
		}
		for _, row := range ds.Rows[cursor:next] {
			// check chainID
			if rowChainID, is := row[chainIDField.Name].(string); !is {
				panic(fmt.Errorf("row chainID for %q is %T not a string", ds.Meta.GetFullName(), row[chainIDField.Name]))
			} else if rowChainID != chainID {
				panic(fmt.Errorf("chainID for %q is %q, not %q", ds.Meta.GetFullName(), rowChainID, chainID))
			}
			// build columns
			if modifier != nil {
				row = modifier(row)
			}
			var columns []any
			for _, fn := range fieldNames {
				columns = append(columns, row[fn])
			}
			// append
			if err = batch.Append(columns...); err != nil {
				pageLogger.With("fields", utils.GetMapValuesOrderByKey(ds.Meta.Fields), "row", row, "columns", columns).
					Errore(err, "batch append failed")
				return fmt.Errorf("batch append for %q failed: %w", ds.Meta.GetFullName(), err)
			}
		}
		if err = batch.Send(); err != nil {
			pageLogger.Errore(err, "batch send failed")
			return fmt.Errorf("batch send for %q failed: %w", ds.Meta.GetFullName(), err)
		}
		pageLogger.With("used", time.Since(batchStartTime).String()).Debug("batch send succeed")
		cursor = next
	}
	logger.Infow("insert data succeed", "used", time.Since(startTime).String())
	return nil
}

var aggFunctionMapping = map[string]string{
	"count": "count",
	"sum":   "sum",
	"avg":   "avg",
	"max":   "max",
	"min":   "min",
	"last":  "last_value",
	"first": "first_value",
}

func (s *Store) aggregationGrowth(ctx context.Context, meta timeseries.Meta, chainID string, curTime time.Time) error {
	_, logger := log.FromContext(ctx,
		"processorID", s.processorID,
		"chainID", chainID,
		"meta", meta.GetFullName(),
		"source", meta.Aggregation.Source,
		"curTime", curTime.String())
	logger.Debugf("will growth aggregation")
	dimFieldNames := utils.MapSliceNoError(
		meta.GetFieldsByRole(timeseries.FieldRoleSeriesLabel),
		func(f timeseries.Field) string {
			return f.Name
		})
	aggFieldNames := utils.GetOrderedMapKeys(meta.Aggregation.Fields)
	aggFields := utils.MapSliceNoError(aggFieldNames, func(fn string) string {
		agg := meta.Aggregation.Fields[fn]
		return fmt.Sprintf("%s(%s)", aggFunctionMapping[agg.Function], agg.Expression)
	})

	srcMeta, has := utils.GetFromK2Map(s.meta.Metas, meta.Type, meta.Aggregation.Source)
	if !has {
		return fmt.Errorf("%w: source for %q is %q, but not found",
			timeseries.ErrInvalidMeta, meta.GetFullName(), meta.Aggregation.Source)
	}

	for _, interval := range meta.Aggregation.Intervals {
		// time window of the <target> for <interval> is:
		//   toStartOfInterval(<target> - INTERVAL '1 ns', <interval>) + <interval>
		// using the right point of the time window as the sample time
		sql := format.Format("INSERT INTO %aggTableName#s ("+
			" %chainIDFieldName#s,"+
			" %timestampFieldName#s,"+
			" %slotNumberFieldName#s,"+
			" %intervalFieldName#s,"+
			" %dimFieldNames#s,"+
			" %aggFieldNames#s"+
			") "+
			"SELECT"+
			" %srcChainIDFieldName#s,"+
			" toStartOfInterval(%srcTimestampFieldName#s-%const1ns#s,%interval#s)+%interval#s as __timeWin__,"+
			" max(%srcSlotNumberFieldName#s),"+
			" '%intervalText#s',"+
			" %dimFieldNames#s,"+
			" %aggFields#s "+
			"FROM %srcTableName#s "+
			"WHERE"+
			" __timeWin__ > ("+
			" SELECT MAX(%timestampFieldName#s)"+
			" FROM %aggTableName#s"+
			" WHERE %intervalFieldName#s = '%intervalText#s' AND %chainIDFieldName#s = '%chainID#s'"+
			" ) AND"+
			" __timeWin__ < toStartOfInterval(toDateTime64('%curTime#s',6)-%const1ns#s,%interval#s)+%interval#s AND"+
			" %srcChainIDFieldName#s = '%chainID#s' "+
			"GROUP BY %srcChainIDFieldName#s, %dimFieldNames#s, __timeWin__",
			map[string]any{
				"const1ns":               "INTERVAL '1 ns'",
				"aggTableName":           s.buildTableName(meta),
				"chainIDFieldName":       meta.GetChainIDField().Name,
				"timestampFieldName":     meta.GetTimestampField().Name,
				"slotNumberFieldName":    meta.GetSlotNumberField().Name,
				"intervalFieldName":      meta.GetAggIntervalField().Name,
				"srcTableName":           s.buildTableName(srcMeta),
				"srcChainIDFieldName":    srcMeta.GetChainIDField().Name,
				"srcTimestampFieldName":  srcMeta.GetTimestampField().Name,
				"srcSlotNumberFieldName": srcMeta.GetSlotNumberField().Name,
				"intervalText":           interval.String(),
				"curTime":                curTime.UTC().Format(time.DateTime),
				"interval":               interval.PGInterval(),
				"chainID":                chainID,
				"dimFieldNames":          strings.Join(dimFieldNames, ","),
				"aggFieldNames":          strings.Join(aggFieldNames, ","),
				"aggFields":              strings.Join(aggFields, ","),
			})
		start := time.Now()
		uniqToken := strconv.FormatUint(rand.Uint64(), 16)
		err := s.client.Exec(chx.InsertSelectCtx(chx.InsertCtx(ctx, uniqToken)), sql)
		exeLogger := logger.With(
			"interval", interval.String(),
			"sql", sql,
			"uniqToken", uniqToken,
			"used", time.Since(start).String())
		if err != nil {
			exeLogger.Errorfe(err, "growth aggregation failed")
			return fmt.Errorf("growth aggregation for %q with interval %s chainID %s and current time %s failed: %w",
				meta.GetFullName(), interval.String(), chainID, curTime, err)
		} else {
			exeLogger.Info("growth aggregation succeeded")
		}
	}
	return nil
}

func (s *Store) DeleteData(ctx context.Context, chainID string, slotNumberGt int64) error {
	s.meta.Lock()
	defer s.meta.Unlock()

	_, logger := log.FromContext(ctx, "processorID", s.processorID, "chainID", chainID, "slotNumberGt", slotNumberGt)
	logger.Debug("will delete")

	for _, metas := range s.meta.Metas {
		for _, meta := range metas {
			tableName := s.buildTableName(meta)
			chainIDField := meta.GetChainIDField()
			slotNumberField := meta.GetSlotNumberField()

			// count rows to delete
			startTime := time.Now()
			sql := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ? AND %s > ?",
				tableName, chainIDField.Name, slotNumberField.Name)
			count, err := queryCount(ctx, s.client, sql, chainID, slotNumberGt)
			metaLogger := logger.With("meta", meta.GetFullName(), "used", time.Since(startTime), "sql", sql)
			if err != nil {
				metaLogger.Errore(err, "count for deleting failed")
				return fmt.Errorf("count for deleting for %s with chainID %s and slotNumber greater than %d failed: %w",
					meta.GetFullName(), chainID, slotNumberGt, err)
			}
			if count == 0 {
				metaLogger.Debug("not need to delete")
				continue
			}
			metaLogger.Infof("need to delete %d rows", count)

			// execute delete
			// because always partition by ChainID field, so here can use `IN PARTITION` segment
			startTime = time.Now()
			sql = fmt.Sprintf("DELETE FROM %s %s IN PARTITION '%s' WHERE %s > ?",
				tableName, s.sqlOnClusterPart(), chainID, slotNumberField.Name)
			err = s.client.Exec(chx.LightDeleteCtx(ctx), sql, slotNumberGt)
			metaLogger = logger.With("meta", meta.GetFullName(), "used", time.Since(startTime), "sql", sql)
			if err != nil {
				metaLogger.Errore(err, "delete failed")
				return fmt.Errorf("delete for %s with chainID %s and slotNumber greater than %d failed: %w",
					meta.GetFullName(), chainID, slotNumberGt, err)
			}
			metaLogger.Infof("deleted %d rows", count)
		}
	}
	logger.Info("deleted")
	return nil
}
