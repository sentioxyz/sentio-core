package clickhouse

import (
	"context"
	"fmt"
	"math"
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
	"github.com/pkg/errors"
)

func (s *Store) AppendData(ctx context.Context, data []timeseries.Dataset, chainID string, curTime time.Time) error {
	s.metaLock.Lock()
	defer s.metaLock.Unlock()
	_, logger := log.FromContext(ctx,
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
		ds.Meta, _, _ = s.meta.find(ds.Meta.Type, ds.Meta.Name)
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
	for _, meta_ := range s.meta.listMetaWithAgg() {
		meta := meta_
		g.Go(func() error {
			return s.aggregationGrowth(gctx, meta, chainID, curTime)
		})
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
	_, logger := log.FromContext(ctx, "chainID", chainID, "meta", ds.Meta.GetFullName(), "rows", len(ds.Rows))
	logger.Debug("will insert rows")
	fieldNames := utils.GetOrderedMapKeys(ds.Meta.Fields)
	chainIDField := ds.Meta.GetChainIDField()
	var modifier func(row timeseries.Row) timeseries.Row
	if ds.Type == timeseries.MetaTypeCounter {
		labelFields := ds.Meta.GetFieldsByRole(timeseries.FieldRoleSeriesLabel)
		valueFields := ds.Meta.GetFieldsByRole(timeseries.FieldRoleSeriesValue)
		var seriesLast map[string]timeseries.Row
		cache, has := s.cachedCounterSeriesLatest.Get(ds.Name)
		if has && timeseries.SameFields(cache.labelFields, labelFields) && timeseries.SameFields(cache.valueFields, valueFields) {
			logger.Debug("use cached last value of each series")
			seriesLast = cache.seriesLast
		} else {
			// has new fields, the previously cached data needs to be discarded
			// because the calculation result of the series ID may be different
			sql := format.Format("SELECT %labelFieldNames#s, %lastValueFields#s "+
				"FROM %tableName#s "+
				"WHERE %chainIDFieldName#s = '%chainID#s' "+
				"GROUP BY %labelFieldNames#s",
				map[string]any{
					"labelFieldNames": strings.Join(utils.MapSliceNoError(labelFields, func(f timeseries.Field) string {
						return quote(f.Name)
					}), ", "),
					"lastValueFields": strings.Join(utils.MapSliceNoError(valueFields, func(f timeseries.Field) string {
						return fmt.Sprintf("argMax(%s, %s)", quote(f.Name), quote(ds.Meta.GetTimestampField().Name))
					}), ", "),
					"tableName":        s.ctrl.FullLogicName(ds.Meta.GetTableName()),
					"chainIDFieldName": quote(chainIDField.Name),
					"chainID":          chainID,
				})
			seriesLast = make(map[string]timeseries.Row)
			startAt := time.Now()
			scanFields := append(labelFields, valueFields...)
			err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
				row, err := scanRow(rows, scanFields)
				if err != nil {
					return err
				}
				seriesLast[buildSeriesID(row, labelFields)] = row
				return nil
			}, sql)
			if err != nil {
				logger.With("used", time.Since(startAt).String()).Errore(err, "query last value of each series failed")
				return fmt.Errorf("query last value of each series for %q failed: %v", ds.Meta.GetFullName(), err)
			}
			logger.Infow("got last value of all series",
				"used", time.Since(startAt).String(),
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
	sql := fmt.Sprintf("INSERT INTO %s (%s)",
		s.ctrl.FullLogicName(ds.Meta.GetTableName()),
		strings.Join(utils.MapSliceNoError(fieldNames, quote), ","),
	)
	startAt := time.Now()
	getter := chx.NewGetter(ds.Rows, func(row timeseries.Row) []any {
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
		columns := make([]any, len(fieldNames))
		for i, fn := range fieldNames {
			columns[i] = row[fn]
		}
		return columns
	})
	err := s.ctrl.BatchInsert(ctx, sql, s.option.BatchInsertSizeLimit, getter)
	logger = logger.With("used", time.Since(startAt).String())
	if err != nil {
		logger.Errorfe(err, "insert data failed")
		return errors.Wrapf(err, "batch insert for %q failed", ds.Meta.GetFullName())
	}
	logger.Infow("insert data succeed")
	return nil
}

var aggFunctionMapping = map[string]string{
	"count": "count(%expr#s)",
	"sum":   "sum(%expr#s)",
	"avg":   "avg(%expr#s)",
	"max":   "max(%expr#s)",
	"min":   "min(%expr#s)",
	"last":  "argMax(%expr#s,%ts#s)",
	"first": "argMin(%expr#s,%ts#s)",
}

func (s *Store) aggregationGrowth(ctx context.Context, meta timeseries.Meta, chainID string, curTime time.Time) error {
	_, logger := log.FromContext(ctx,
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
	srcMeta, _, has := s.meta.find(meta.Type, meta.Aggregation.Source)
	if !has {
		return fmt.Errorf("%w: source for %q is %q, but not found",
			timeseries.ErrInvalidMeta, meta.GetFullName(), meta.Aggregation.Source)
	}
	aggFieldNames := utils.GetOrderedMapKeys(meta.Aggregation.Fields)
	aggFields := utils.MapSliceNoError(aggFieldNames, func(fn string) string {
		agg := meta.Aggregation.Fields[fn]
		return format.Format(aggFunctionMapping[agg.Function], map[string]any{
			"expr": agg.Expression,
			"ts":   quote(srcMeta.GetTimestampField().Name),
		})
	})

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
			"FROM %srcTable#s "+
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
				"aggTableName":           s.ctrl.FullLogicName(meta.GetTableName()),
				"chainIDFieldName":       quote(meta.GetChainIDField().Name),
				"timestampFieldName":     quote(meta.GetTimestampField().Name),
				"slotNumberFieldName":    quote(meta.GetSlotNumberField().Name),
				"intervalFieldName":      quote(meta.GetAggIntervalField().Name),
				"srcTable":               s.ctrl.FullLogicName(srcMeta.GetTableName()),
				"srcChainIDFieldName":    quote(srcMeta.GetChainIDField().Name),
				"srcTimestampFieldName":  quote(srcMeta.GetTimestampField().Name),
				"srcSlotNumberFieldName": quote(srcMeta.GetSlotNumberField().Name),
				"intervalText":           interval.String(),
				"curTime":                curTime.UTC().Format(time.DateTime),
				"interval":               interval.PGInterval(),
				"chainID":                chainID,
				"dimFieldNames":          strings.Join(utils.MapSliceNoError(dimFieldNames, quote), ","),
				"aggFieldNames":          strings.Join(utils.MapSliceNoError(aggFieldNames, quote), ","),
				"aggFields":              strings.Join(aggFields, ","),
			})
		start := time.Now()
		uniqToken := strconv.FormatUint(rand.Uint64(), 16)
		err := s.ctrl.Exec(chx.InsertSelectCtx(chx.InsertCtx(ctx, uniqToken)), sql)
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
	s.metaLock.Lock()
	defer s.metaLock.Unlock()
	_, logger := log.FromContext(ctx, "chainID", chainID, "slotNumberGt", slotNumberGt)
	logger.Debug("will delete")
	for _, item := range s.meta {
		startAt := time.Now()
		var deleted uint64 = math.MaxUint64
		var err error
		if slotNumberGt < 0 {
			err = s.ctrl.AlterTable(ctx, item.meta.GetTableName(), "DROP PARTITION ?", chainID)
		} else {
			where := fmt.Sprintf("%s = '%s' AND %s > %d",
				quote(item.meta.GetChainIDField().Name),
				chainID,
				quote(item.meta.GetSlotNumberField().Name),
				slotNumberGt,
			)
			deleted, err = s.ctrl.Delete(ctx, item.meta.GetTableName(), where, true)
		}
		metaLogger := logger.With("meta", item.meta.GetFullName(), "used", time.Since(startAt).String())
		if err != nil {
			metaLogger.Errore(err, "delete failed")
			return errors.Wrapf(err, "delete for %s with chainID %s and slotNumber greater than %d failed",
				item.meta.GetFullName(), chainID, slotNumberGt)
		}
		if deleted == math.MaxUint64 {
			metaLogger.Infof("deleted the whole partition")
		} else {
			metaLogger.Infof("deleted %d rows", deleted)
		}
	}
	logger.Info("deleted")
	return nil
}
