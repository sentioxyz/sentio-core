package adaptor_eventlogs

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/log"
	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
	clickhouselib "sentioxyz/sentio-core/driver/timeseries/clickhouse"
	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_eventlogs/cte"
	"sentioxyz/sentio-core/service/common/timeseries/lucene"
	"sentioxyz/sentio-core/service/common/timeseries/matrix"
	"sentioxyz/sentio-core/service/common/timeseries/util"
	processormodels "sentioxyz/sentio-core/service/processor/models"

	"github.com/blevesearch/bleve/search/query"
	"github.com/samber/lo"
)

const (
	EventNameColumn  = "event_name"
	AttributesColumn = "attributes"

	wideTable = "__wide_table__"
)

type LogAdaptor interface {
	FilterBy(filters ...string) LogAdaptor
	Limit(limit int) LogAdaptor
	Offset(offset int) LogAdaptor
	Order(order ...string) LogAdaptor
	PresetColumns() map[string]struct{}
	OriginalQuery() string
	BuildWideQuery() (string, string)
	BuildCountQuery(filters ...string) string
	GetValueBucket(field string, fieldType timeseries.FieldType, limit int, filters ...string) (map[string]uint64, error)
	GetValueMinMax(field string, fieldType timeseries.FieldType, filters ...string) (any, any, error)
	Scan(ctx context.Context, scan ScanFunc, sql string, args ...any) (matrix.Matrix, error)
}

type logAdaptor struct {
	Base

	timeRange    *timerange.TimeRange
	luceneSearch string
	luceneCond   string
	luceneAst    query.Query
	driver       lucene.Driver

	once         sync.Once
	presetColumn map[string]struct{}
	limit        int
	offset       int
	order        []string
	filters      []string
	cte          cte.CTEs
}

func NewLogAdaptor(ctx context.Context,
	store timeseries.Store,
	processor *processormodels.Processor,
	timeRange *timerange.TimeRange,
	luceneSearch string) (LogAdaptor, error) {
	ctx, logger := log.FromContext(ctx, "processor_id", processor.ID, "function", "LogAdaptor")
	wq := &logAdaptor{
		Base: Base{
			ctx:       ctx,
			logger:    logger,
			store:     store,
			processor: processor,
		},
		timeRange:    timeRange,
		luceneSearch: luceneSearch,
		luceneCond:   "(true)",
		presetColumn: make(map[string]struct{}),
	}

	wq.presetColumn[EventNameColumn] = struct{}{}
	metas := store.Meta().MetaByType(timeseries.MetaTypeEvent)
	for _, meta := range metas {
		for _, field := range meta.Fields {
			if field.IsBuiltIn() {
				wq.presetColumn[field.Name] = struct{}{}
			}
		}
	}

	logger = logger.With("preset_column", lo.Keys(wq.presetColumn))
	if luceneSearch != "" {
		logger = logger.With("lucene_search", luceneSearch)
		ast, err := lucene.Parse(luceneSearch)
		if err != nil {
			logger.Errorf("parse lucene query failed, query: %s, err: %v", luceneSearch, err)
			return nil, err
		}
		wq.luceneAst = ast
		wq.driver = lucene.NewClickhouse(AttributesColumn, wq.presetColumn, wq.store.Meta().MetaByType(timeseries.MetaTypeEvent))
		wq.luceneCond, err = wq.driver.Render(wq.luceneAst)
		if err != nil {
			logger.Errorf("render lucene query to sql failed, query: %s, err: %v", luceneSearch, err)
			return nil, err
		}
		wq.logger = logger.With("lucene_cond", wq.luceneCond)
		wq.logger.DebugEveryN(10, "lucene query to sql: %s", wq.luceneCond)
	}
	return wq, nil
}

func (l *logAdaptor) timeRangeCondString() string {
	if l.timeRange == nil {
		return "1"
	}
	var conditions []string
	if l.timeRange.RangeMode == timerange.LeftOpenRange || l.timeRange.RangeMode == timerange.BothOpenRange {
		conditions = append(conditions,
			timeseries.SystemTimestamp+">"+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')",
				l.timeRange.Start.UTC().Format("2006-01-02 15:04:05")))
	} else {
		conditions = append(conditions,
			timeseries.SystemTimestamp+">="+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')",
				l.timeRange.Start.UTC().Format("2006-01-02 15:04:05")))
	}
	if l.timeRange.RangeMode == timerange.RightOpenRange || l.timeRange.RangeMode == timerange.BothOpenRange {
		conditions = append(conditions,
			timeseries.SystemTimestamp+"<"+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')",
				l.timeRange.End.UTC().Format("2006-01-02 15:04:05")))
	} else {
		conditions = append(conditions,
			timeseries.SystemTimestamp+"<="+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')",
				l.timeRange.End.UTC().Format("2006-01-02 15:04:05")))
	}
	return strings.Join(conditions, " AND ")
}

func (l *logAdaptor) buildMetaQuery(meta timeseries.Meta) string {
	const (
		tpl = "SELECT " +
			"{preset}, " +
			"'{event_name}' AS " + EventNameColumn + ", " +
			"{attributes_field} AS " + AttributesColumn + " " +
			"FROM `{table}` " +
			"WHERE {time_range}"
	)
	var (
		attributes []string
		preset     []string
	)
	for _, field := range meta.Fields {
		if field.IsBuiltIn() {
			continue
		}
		attributes = append(attributes,
			"'"+field.Name+"'", timeseries.EscapeEventlogFieldName(field.Name)+"::Dynamic")
	}
	lo.ForEach(utils.GetOrderedMapKeys(l.presetColumn), func(column string, _ int) {
		preset = append(preset, timeseries.EscapeEventlogFieldName(column))
	})
	return builder.FormatSQLTemplate(tpl, map[string]any{
		"preset":           strings.Join(preset, ", "),
		"event_name":       meta.Name,
		"attributes_field": "map(" + strings.Join(attributes, ", ") + ")::JSON",
		"table":            l.store.MetaTable(meta),
		"time_range":       l.timeRangeCondString(),
	})
}

func (l *logAdaptor) buildQuery() {
	l.once.Do(func() {
		var queries []string
		for _, meta := range l.store.Meta().MetaByType(timeseries.MetaTypeEvent) {
			queries = append(queries, l.buildMetaQuery(meta))
		}
		q := strings.Join(queries, " UNION ALL ")
		l.logger = l.logger.With("query", q)
		l.cte = append(l.cte, cte.CTE{
			Alias: wideTable,
			Query: q,
		})
		l.logger.DebugEveryN(10, "build log query finish")
	})
}

func (l *logAdaptor) OriginalQuery() string {
	l.buildQuery()
	const (
		tpl = "{with} SELECT * FROM {wide_table} {where}"
	)
	var (
		withParameter = l.cte.String()
		where         = lo.If(l.luceneCond != "", " WHERE "+l.luceneCond).Else("")
		sql           = builder.FormatSQLTemplate(tpl, map[string]any{
			"with":       withParameter,
			"wide_table": wideTable,
			"where":      where,
		})
	)
	l.logger.DebugEveryN(10, "original query: %s", sql)
	return sql
}

func (l *logAdaptor) GetValueBucket(field string, fieldType timeseries.FieldType, limit int, filters ...string) (map[string]uint64, error) {
	l.buildQuery()
	const (
		tpl = "{with} " +
			"SELECT " +
			"category AS category, " +
			"sum(cnt) AS count " +
			"FROM {stats_table} GROUP BY category ORDER BY CASE WHEN category = 'others' THEN 0 ELSE count END DESC, count DESC;"
	)
	var conds []string
	conds = append(conds, l.filters...)
	conds = append(conds, filters...)
	if l.luceneCond != "" {
		conds = append(conds, l.luceneCond)
	}
	var (
		_, preset = l.presetColumn[field]
		where     = lo.If(len(conds) > 0, " WHERE "+strings.Join(conds, " AND ")).Else("")
		column    = lo.If(preset, clickhouselib.DbTypeCasting(timeseries.EscapeEventlogFieldName(field), fieldType)).
				Else(clickhouselib.DbTypeCasting(timeseries.EscapeEventlogFieldName(AttributesColumn+"."+field), fieldType))
		topKTable  = "top_k"
		statsTable = "stats"
	)
	l.cte = append(l.cte, cte.CTE{
		Alias: topKTable,
		Query: "SELECT arrayJoin(topK(" +
			strconv.FormatInt(int64(limit), 10) + ")" +
			"(" + column + ")) AS top_value FROM " + wideTable + where,
	})
	l.cte = append(l.cte, cte.CTE{
		Alias: statsTable,
		Query: "SELECT " + column + " as column_name, count() as cnt, " +
			"if(column_name IN (SELECT top_value FROM " + topKTable + "), column_name, 'others') as category FROM " + wideTable + where + " GROUP BY column_name",
	})
	var (
		withParameter = l.cte.String()
		sql           = builder.FormatSQLTemplate(tpl, map[string]any{
			"with":        withParameter,
			"stats_table": statsTable,
		})
	)
	l.logger.Infof("get value bucket query: %s", sql)
	rows, err := l.store.Client().Query(ckhmanager.ContextMergeSettings(l.ctx, map[string]any{
		"allow_simdjson": 0,
	}), sql)
	if err != nil {
		l.logger.Warnf("get value bucket query failed, sql: %s, err: %v", sql, err)
		return nil, err
	}

	defer func() {
		_ = rows.Close()
	}()

	var (
		buckets     = make(map[string]uint64)
		columnTypes = rows.ColumnTypes()
	)
	for rows.Next() {
		var (
			vars = make([]any, len(columnTypes))
		)
		for i := range columnTypes {
			vars[i] = reflect.New(columnTypes[i].ScanType()).Interface()
		}
		if err := rows.Scan(vars...); err != nil {
			l.logger.Warnf("scan rows failed, err: %v", err)
			return nil, err
		}
		cnt, _ := utils.Any2Float(vars[1])
		buckets[utils.Any2String(vars[0])] = uint64(cnt)
	}
	if err := rows.Err(); err != nil {
		l.logger.Warnf("scan rows failed, err: %v", err)
		return nil, err
	}
	return buckets, nil
}

func (l *logAdaptor) GetValueMinMax(field string, fieldType timeseries.FieldType, filters ...string) (any, any, error) {
	l.buildQuery()
	const (
		tpl = "{with} SELECT min({field}) AS min, max({field}) AS max FROM {wide_table} {where}"
	)
	var conds []string
	conds = append(conds, l.filters...)
	conds = append(conds, filters...)
	if l.luceneCond != "" {
		conds = append(conds, l.luceneCond)
	}
	var (
		withParameter = l.cte.String()
		where         = lo.If(len(conds) > 0, " WHERE "+strings.Join(conds, " AND ")).Else("")
		_, preset     = l.presetColumn[field]
		sql           = builder.FormatSQLTemplate(tpl, map[string]any{
			"with":       withParameter,
			"wide_table": wideTable,
			"where":      where,
			"field": lo.If(preset, clickhouselib.DbTypeCasting(timeseries.EscapeEventlogFieldName(field), fieldType)).
				Else(clickhouselib.DbTypeCasting(timeseries.EscapeEventlogFieldName(AttributesColumn+"."+field), fieldType)),
		})
	)
	l.logger.Infof("get value min max query: %s", sql)
	rows, err := l.store.Client().Query(ckhmanager.ContextMergeSettings(l.ctx, map[string]any{
		"allow_simdjson": 0,
	}), sql)
	if err != nil {
		l.logger.Warnf("get value min max query failed, sql: %s, err: %v", sql, err)
		return nil, nil, err
	}

	defer func() {
		_ = rows.Close()
	}()
	var (
		columnTypes        = rows.ColumnTypes()
		minValue, maxValue any
	)
	if rows.Next() {
		var (
			vars = make([]any, len(columnTypes))
		)
		for i := range columnTypes {
			vars[i] = reflect.New(columnTypes[i].ScanType()).Interface()
		}
		if err := rows.Scan(vars...); err != nil {
			l.logger.Warnf("scan rows failed, err: %v", err)
			return nil, nil, err
		}
		minValue = vars[0]
		maxValue = vars[1]
	}
	if err := rows.Err(); err != nil {
		l.logger.Warnf("scan rows failed, err: %v", err)
		return nil, nil, err
	}
	return minValue, maxValue, nil
}

func (l *logAdaptor) PresetColumns() map[string]struct{} {
	return l.presetColumn
}

func (l *logAdaptor) Limit(limit int) LogAdaptor {
	l.limit = limit
	l.logger = l.logger.With("limit", limit)
	return l
}

func (l *logAdaptor) Offset(offset int) LogAdaptor {
	l.offset = offset
	l.logger = l.logger.With("offset", offset)
	return l
}

func (l *logAdaptor) Order(order ...string) LogAdaptor {
	l.order = append(l.order, order...)
	l.logger = l.logger.With("order", l.order)
	return l
}

func (l *logAdaptor) FilterBy(filters ...string) LogAdaptor {
	l.filters = filters
	l.logger = l.logger.With("filters", filters)
	return l
}

func (l *logAdaptor) BuildWideQuery() (string, string) {
	l.buildQuery()
	const (
		tpl      = "{with} SELECT * FROM {wide_table} {where} {order} {limit} {offset}"
		countTpl = "{with} SELECT count() FROM {wide_table} {where}"
	)
	var conds []string
	conds = append(conds, l.filters...)
	if l.luceneCond != "" {
		conds = append(conds, l.luceneCond)
	}
	var (
		withParameter = l.cte.String()
		where         = lo.If(len(conds) > 0, " WHERE "+strings.Join(conds, " AND ")).Else("")
		order         = lo.If(len(l.order) > 0, " ORDER BY "+strings.Join(l.order, ", ")).Else("")
		limit         = lo.If(l.limit > 0, " LIMIT "+strconv.FormatInt(int64(l.limit), 10)).Else("")
		offset        = lo.If(l.offset > 0, " OFFSET "+strconv.FormatInt(int64(l.offset), 10)).Else("")
		sql           = builder.FormatSQLTemplate(tpl, map[string]any{
			"with":       withParameter,
			"wide_table": wideTable,
			"where":      where,
			"order":      order,
			"limit":      limit,
			"offset":     offset,
		})
		countSql = builder.FormatSQLTemplate(countTpl, map[string]any{
			"with":       withParameter,
			"wide_table": wideTable,
			"where":      where,
		})
	)
	l.logger.DebugEveryN(10, "build log query: %s", sql)
	return sql, countSql
}

func (l *logAdaptor) ClickhouseHistogramTime(timeField string) string {
	return util.HistogramFunction(l.timeRange.Step, timeField, l.timeRange.Timezone.String())
}

func (l *logAdaptor) BuildCountQuery(filters ...string) string {
	l.buildQuery()
	const (
		countTpl = "{with} SELECT {time_field} AS " + matrix.TimeFieldName +
			", count() AS " + matrix.AggFieldName +
			" FROM {wide_table} {where} GROUP BY " + matrix.TimeFieldName
	)
	var conds []string
	conds = append(conds, l.filters...)
	conds = append(conds, filters...)
	if l.luceneCond != "" {
		conds = append(conds, l.luceneCond)
	}
	var (
		withParameter = l.cte.String()
		where         = lo.If(len(conds) > 0, " WHERE "+strings.Join(conds, " AND ")).Else("")
		sql           = builder.FormatSQLTemplate(countTpl, map[string]any{
			"with":       withParameter,
			"wide_table": wideTable,
			"where":      where,
			"time_field": l.ClickhouseHistogramTime(timeseries.SystemTimestamp),
		})
	)
	l.logger.DebugEveryN(10, "build log count query: %s", sql)
	return sql
}
