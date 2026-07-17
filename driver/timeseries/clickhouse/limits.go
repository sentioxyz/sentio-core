package clickhouse

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/format"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
)

var (
	defaultMetricNameLimit = envconf.LoadUInt64("SENTIO_METRIC_NAME_LIMIT",
		500, envconf.WithMin(1))
	defaultEventNameLimit = envconf.LoadUInt64("SENTIO_EVENT_NAME_LIMIT",
		500, envconf.WithMin(1))
	defaultSeriesLimitPerMetric = envconf.LoadUInt64("SENTIO_TIME_SERIES_LIMIT_PER_METRIC",
		100000, envconf.WithMin(1))
	defaultSeriesLimitTotal = envconf.LoadUInt64("SENTIO_TIME_SERIES_LIMIT",
		1000000, envconf.WithMin(1))
)

const (
	existingNamePreviewCount = 20
	topSeriesUsageCount      = 3
)

// checkMetaNameLimit rejects creating a new metric or event type once the processor already has
// too many distinct names. Existing names (already present in sm) are never rejected.
// Counters and gauges share the metric limit, event types have their own limit.
// The name limits are chain-independent: each name is one table shared by all chains.
func checkMetaNameLimit(sm storeMeta, meta timeseries.Meta, metricLimit, eventLimit int) error {
	var (
		names   []string
		limit   int
		baseErr error
		kind    string
		advice  string
	)
	switch meta.Type {
	case timeseries.MetaTypeCounter, timeseries.MetaTypeGauge:
		names = append(sm.MetaNames(timeseries.MetaTypeCounter), sm.MetaNames(timeseries.MetaTypeGauge)...)
		limit = metricLimit
		baseErr = timeseries.ErrTooManyMetrics
		kind = "metric"
		advice = "Avoid deriving metric names from unbounded values (use labels instead)."
	case timeseries.MetaTypeEvent:
		names = sm.MetaNames(timeseries.MetaTypeEvent)
		limit = eventLimit
		baseErr = timeseries.ErrTooManyEventTypes
		kind = "event type"
		advice = "Avoid deriving event names from unbounded values (use attributes instead)."
	default:
		return nil
	}
	if len(names) < limit {
		return nil
	}
	sort.Strings(names)
	preview := names
	if len(preview) > existingNamePreviewCount {
		preview = preview[:existingNamePreviewCount]
	}
	return errors.Wrapf(baseErr,
		"cannot create %s %q: the processor already has %d distinct %ss, over the limit %d. "+
			"%s First %d existing %ss:\n%s",
		kind, meta.Name, len(names), kind, limit, advice, len(preview), kind, strings.Join(preview, "\n"))
}

// collectNewSeriesIDs returns the distinct series IDs among rows that exists reports as unknown.
func collectNewSeriesIDs(
	rows []timeseries.Row,
	labelFields []timeseries.Field,
	exists func(seriesID string) bool,
) set.Set[string] {
	newIDs := set.New[string]()
	for _, row := range rows {
		id := buildSeriesID(row, labelFields)
		if exists(id) {
			continue
		}
		newIDs.Add(id)
	}
	return newIDs
}

func seriesCountKey(meta timeseries.Meta, chainID string) string {
	return meta.GetFullName() + "@" + chainID
}

// checkSeriesLimit verifies both the per-metric and the total series limits assuming newCount new
// series are about to be created for the given metric which already has existingCount series.
// Series are counted per chain (existingCount and newCount describe one chain of the metric);
// the total limit sums the per-chain counts of all tracked metrics.
// It also refreshes the known series count of the metric, so the counts of metrics that stopped
// receiving new series remain part of the total.
func (s *Store) checkSeriesLimit(meta timeseries.Meta, chainID string, existingCount, newCount int) error {
	count := existingCount + newCount
	if newCount == 0 {
		s.seriesCount.Put(seriesCountKey(meta, chainID), count)
		return nil
	}
	if count > s.option.SeriesLimitPerMetric {
		return errors.Wrapf(timeseries.ErrTooManySeries,
			"the number of time series of metric %s would reach %d, over the limit %d. "+
				"Avoid using unbounded values (such as block numbers, timestamps or hashes) as label values",
			meta.GetFullName(), count, s.option.SeriesLimitPerMetric)
	}
	s.seriesCount.Put(seriesCountKey(meta, chainID), count)
	total := 0
	s.seriesCount.Traverse(func(_ string, c int) {
		total += c
	})
	if total > s.option.SeriesLimitTotal {
		usage := s.seriesCount.Dump()
		keys := utils.GetOrderedMapKeys(usage)
		sort.Slice(keys, func(i, j int) bool {
			return usage[keys[i]] > usage[keys[j]]
		})
		if len(keys) > topSeriesUsageCount {
			keys = keys[:topSeriesUsageCount]
		}
		var top strings.Builder
		for _, key := range keys {
			top.WriteString(fmt.Sprintf("%s: %d\n", key, usage[key]))
		}
		return errors.Wrapf(timeseries.ErrTooManySeries,
			"the total number of time series would reach %d, over the limit %d. Top %d metric usage:\n%s",
			total, s.option.SeriesLimitTotal, len(keys), top.String())
	}
	return nil
}

// ensureSeriesCountsLoaded backfills the series counts of metrics that already exist in the
// storage. seriesCount is otherwise only refreshed by checkSeriesLimit on insert, so after a
// restart a metric that no longer receives data would silently drop out of the total series
// count and the total limit could be exceeded without being detected. Runs once per chain.
// Metrics in skipNames are inserted into by the current commit, their counts are refreshed by
// checkSeriesLimit anyway, so their (potentially expensive) count queries are skipped here.
// Must be called with s.metaLock held.
func (s *Store) ensureSeriesCountsLoaded(ctx context.Context, chainID string, skipNames set.Set[string]) error {
	if s.seriesCountLoadedChains.Contains(chainID) {
		return nil
	}
	_, logger := log.FromContext(ctx, "chainID", chainID)
	for _, item := range s.meta {
		meta := item.meta
		if meta.Type != timeseries.MetaTypeCounter && meta.Type != timeseries.MetaTypeGauge {
			continue
		}
		if meta.Aggregation != nil {
			continue
		}
		if skipNames.Contains(meta.GetFullName()) {
			continue
		}
		key := seriesCountKey(meta, chainID)
		if _, has := s.seriesCount.Get(key); has {
			continue
		}
		labelFields := meta.GetFieldsByRole(timeseries.FieldRoleSeriesLabel)
		if len(labelFields) == 0 {
			// a metric without labels has at most one series, it is only counted when it inserts
			continue
		}
		count, err := s.countSeries(ctx, meta, chainID, labelFields)
		if err != nil {
			return err
		}
		if count > s.option.SeriesLimitPerMetric {
			// the metric exceeded the limit before it was introduced, the limit is not
			// enforced for it, so its count is also left out of the total
			logger.Warnf("metric %s already has more than %d series, it will not be counted",
				meta.GetFullName(), s.option.SeriesLimitPerMetric)
			continue
		}
		s.seriesCount.Put(key, count)
	}
	s.seriesCountLoadedChains.Add(chainID)
	return nil
}

// countSeries counts the distinct series of a metric for one chain, capped at the per-metric
// limit plus one so that over-limit metrics do not require an unbounded scan result.
func (s *Store) countSeries(
	ctx context.Context,
	meta timeseries.Meta,
	chainID string,
	labelFields []timeseries.Field,
) (int, error) {
	sql := format.Format("SELECT count() FROM ("+
		"SELECT DISTINCT %labelFieldNames#s "+
		"FROM %tableName#s "+
		"WHERE %chainIDFieldName#s = '%chainID#s' "+
		"LIMIT %limit#d)",
		map[string]any{
			"labelFieldNames": strings.Join(utils.MapSliceNoError(labelFields, func(f timeseries.Field) string {
				return quote(f.Name)
			}), ", "),
			"tableName":        s.ctrl.FullLogicName(meta.GetTableName()),
			"chainIDFieldName": quote(meta.GetChainIDField().Name),
			"chainID":          chainID,
			"limit":            s.option.SeriesLimitPerMetric + 1,
		})
	var count uint64
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&count)
	}, sql)
	if err != nil {
		return 0, errors.Wrapf(err, "count series for %q failed", meta.GetFullName())
	}
	return int(count), nil
}

// loadGaugeSeriesIDs returns the cached series ID set of a gauge metric, loading the existing
// label combinations from the storage on the first use (per chain). If the metric already has
// more series than the limit, the set is not tracked and the limit is not enforced for it, so
// that metrics which exceeded the limit before it was introduced keep working.
func (s *Store) loadGaugeSeriesIDs(
	ctx context.Context,
	meta timeseries.Meta,
	chainID string,
	labelFields []timeseries.Field,
) (*gaugeSeriesIDCache, error) {
	key := seriesCountKey(meta, chainID)
	cache, has := s.cachedGaugeSeriesIDs.Get(key)
	if has && timeseries.SameFields(cache.labelFields, labelFields) {
		return cache, nil
	}
	// has new fields, the previously cached data needs to be discarded
	// because the calculation result of the series ID may be different
	_, logger := log.FromContext(ctx, "chainID", chainID, "meta", meta.GetFullName())
	sql := format.Format("SELECT %labelFieldNames#s "+
		"FROM %tableName#s "+
		"WHERE %chainIDFieldName#s = '%chainID#s' "+
		"GROUP BY %labelFieldNames#s "+
		"LIMIT %limit#d",
		map[string]any{
			"labelFieldNames": strings.Join(utils.MapSliceNoError(labelFields, func(f timeseries.Field) string {
				return quote(f.Name)
			}), ", "),
			"tableName":        s.ctrl.FullLogicName(meta.GetTableName()),
			"chainIDFieldName": quote(meta.GetChainIDField().Name),
			"chainID":          chainID,
			"limit":            s.option.SeriesLimitPerMetric + 1,
		})
	ids := set.New[string]()
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		row, err := scanRow(rows, labelFields)
		if err != nil {
			return err
		}
		ids.Add(buildSeriesID(row, labelFields))
		return nil
	}, sql)
	if err != nil {
		logger.Errore(err, "query existing series of gauge failed")
		return nil, errors.Wrapf(err, "query existing series for %q failed", meta.GetFullName())
	}
	cache = &gaugeSeriesIDCache{
		labelFields:     labelFields,
		ids:             ids,
		overLimitOnLoad: ids.Size() > s.option.SeriesLimitPerMetric,
	}
	if cache.overLimitOnLoad {
		logger.Warnf("gauge already has more than %d series, the series limit will not be enforced for it",
			s.option.SeriesLimitPerMetric)
		cache.ids = nil
	} else {
		logger.Infow("got existing series of gauge", "series", ids.Size())
	}
	s.cachedGaugeSeriesIDs.Put(key, cache)
	return cache, nil
}
