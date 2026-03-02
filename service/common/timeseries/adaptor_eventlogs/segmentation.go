package adaptor_eventlogs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/protojson"
	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/driver/timeseries/clickhouse"
	"sentioxyz/sentio-core/service/analytic/clients/rewriter"
	commonprotos "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_eventlogs/cte"
	"sentioxyz/sentio-core/service/common/timeseries/matrix"
	processormodels "sentioxyz/sentio-core/service/processor/models"
	protosrewriter "sentioxyz/sentio-core/service/rewriter/protos"

	clickhouselib "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jinzhu/copier"
	"github.com/samber/lo"
)

type ScanFunc func(ctx context.Context, sql string, args ...any) (clickhouselib.Rows, error)

type SegmentationAdaptor interface {
	WithResource(resources ...string) SegmentationAdaptor
	WithTimeRange(timeRange *timerange.TimeRange) SegmentationAdaptor
	WithSelector(selector *commonprotos.SegmentationQuery_SelectorExpr) SegmentationAdaptor
	Distinct() SegmentationAdaptor
	Breakdown(breakdown ...string) SegmentationAdaptor
	Order(order ...string) SegmentationAdaptor
	AggregateBy(op *commonprotos.SegmentationQuery_Aggregation) SegmentationAdaptor
	FetchBy(fields ...string) SegmentationAdaptor
	Limit(limit int) SegmentationAdaptor
	Offset(offset int) SegmentationAdaptor

	Build() string
	Error() error
	Scan(ctx context.Context, scan ScanFunc, sql string, args ...any) (matrix.Matrix, error)
}

const (
	mainTable            = "_main_"
	beforeTimeRangeTable = "_before_"
	preAggTable          = "_pre_agg_"
)

type columnProperty struct {
	nested    bool
	fieldType timeseries.FieldType
}

type segmentationAdaptor struct {
	Base
	option QueryOption

	resources          []string
	columns            map[string]struct{}
	resourceColumns    map[string]map[string]columnProperty
	scanColumns        []string
	resourceConditions map[string]string
	breakdown          Breakdown
	aggregator         Aggregator
	distinct           bool
	orders             []string
	limit              int
	offset             int
	timeRange          *timerange.TimeRange
	cte                []cte.CTE
	cumulative         bool
}

var (
	presetColumns = map[string]timeseries.FieldType{
		timeseries.SystemFieldPrefix + "block_number":      timeseries.FieldTypeInt,
		timeseries.SystemFieldPrefix + "block_hash":        timeseries.FieldTypeString,
		timeseries.SystemFieldPrefix + "transaction_hash":  timeseries.FieldTypeString,
		timeseries.SystemFieldPrefix + "transaction_index": timeseries.FieldTypeInt,
		timeseries.SystemFieldPrefix + "log_index":         timeseries.FieldTypeInt,
		timeseries.SystemFieldPrefix + "chain":             timeseries.FieldTypeString,
		timeseries.SystemTimestamp:                         timeseries.FieldTypeTime,
		timeseries.SystemUserID:                            timeseries.FieldTypeString,
	}
)

type FormatMode string

const (
	FormatModeNone        FormatMode = ""
	FormatModeViaRewriter FormatMode = "rewriter"
)

type QueryOption struct {
	FormatMode           FormatMode
	Rewriter             rewriter.Client
	CumulativePreCheck   bool
	CumulativeLabelLimit int64
	Conn                 clickhouselib.Conn
}

func mergeQueryOptions(options ...QueryOption) QueryOption {
	var merged QueryOption
	for _, option := range options {
		if option.FormatMode != "" {
			merged.FormatMode = option.FormatMode
		}
		if option.Rewriter != nil {
			merged.Rewriter = option.Rewriter
		}
		if option.CumulativePreCheck {
			merged.CumulativePreCheck = true
		}
		if option.CumulativeLabelLimit > merged.CumulativeLabelLimit {
			merged.CumulativeLabelLimit = option.CumulativeLabelLimit
		}
		if option.Conn != nil {
			merged.Conn = option.Conn
		}
	}
	return merged
}

func NewSegmentationAdaptor(ctx context.Context,
	store timeseries.Store, processor *processormodels.Processor, option ...QueryOption) SegmentationAdaptor {
	ctx, logger := log.FromContext(ctx, "processor_id", processor.ID, "function", "SegmentationAdaptor")
	q := &segmentationAdaptor{
		Base: Base{
			ctx:       ctx,
			logger:    logger,
			store:     store,
			meta:      store.Meta().MetaByType(timeseries.MetaTypeEvent),
			processor: processor,
		},
		columns:            make(map[string]struct{}),
		resourceColumns:    make(map[string]map[string]columnProperty),
		resourceConditions: make(map[string]string),
		breakdown:          Breakdown{},
		orders:             make([]string, 0),
		option:             mergeQueryOptions(option...),
	}
	_ = copier.CopyWithOption(&q.columns, presetColumns, copier.Option{
		DeepCopy: true,
		Converters: []copier.TypeConverter{
			{
				SrcType: timeseries.FieldType(""),
				DstType: struct{}{},
				Fn: func(src interface{}) (interface{}, error) {
					return struct{}{}, nil
				},
			},
		},
	})
	return q
}

func (s *segmentationAdaptor) columnToResources(field string) map[string]columnProperty {
	var resources = make(map[string]columnProperty)
	for _, resource := range s.resources {
		meta := s.meta[resource]
		fieldType, ok := meta.GetFieldType(field)
		if ok {
			_, directly := s.resourceColumns[resource][field]
			resources[resource] = columnProperty{
				nested:    !directly,
				fieldType: fieldType,
			}
		}
	}
	var fieldTypes timeseries.FieldTypes
	for _, c := range resources {
		fieldTypes = append(fieldTypes, c.fieldType)
	}
	if !fieldTypes.Compatible() {
		s.logger.Errorf("field %s is not compatible", field)
		s.errors = append(s.errors, fmt.Errorf("field %s is not compatible", field))
		return make(map[string]columnProperty)
	}
	gcd := fieldTypes.SimplyGCD()
	for r := range resources {
		resources[r] = columnProperty{
			nested:    resources[r].nested,
			fieldType: gcd,
		}
	}
	return resources
}

func (s *segmentationAdaptor) touchField(field string) string {
	field = timeseries.UnescapeFieldName(field)
	if resources := s.columnToResources(field); len(resources) > 0 {
		s.columns[field] = struct{}{}
		for resource, nested := range resources {
			s.resourceColumns[resource][field] = nested
		}
	} else {
		s.logger.Errorf("field %s not found in resources", field)
		s.errors = append(s.errors, fmt.Errorf("field %s not found in resources", field))
	}
	return field
}

func (s *segmentationAdaptor) WithTimeRange(timeRange *timerange.TimeRange) SegmentationAdaptor {
	if timeRange == nil {
		panic("timeRange must not be nil")
	}
	s.logger = s.logger.With("time_range", *timeRange)
	s.timeRange = timeRange
	if s.timeRange.Timezone == nil {
		s.timeRange.Timezone = time.UTC
	}
	return s
}

func (s *segmentationAdaptor) WithResource(resources ...string) SegmentationAdaptor {
	s.logger = s.logger.With("resources", resources)
	if len(resources) == 0 {
		s.resources = lo.Keys(s.meta)
	} else {
		for _, resource := range resources {
			if _, ok := s.meta[resource]; ok {
				s.resources = append(s.resources, resource)
			} else {
				s.logger.Errorf("resource %s not found", resource)
				s.errors = append(s.errors, fmt.Errorf("resource %s not found", resource))
			}
		}
	}
	for _, resource := range s.resources {
		resourceColumns := make(map[string]columnProperty)
		_ = copier.CopyWithOption(&resourceColumns, presetColumns, copier.Option{
			DeepCopy: true,
			Converters: []copier.TypeConverter{
				{
					SrcType: timeseries.FieldType(""),
					DstType: columnProperty{},
					Fn: func(src interface{}) (interface{}, error) {
						return columnProperty{
							nested:    false,
							fieldType: src.(timeseries.FieldType),
						}, nil
					},
				},
			},
		})
		s.resourceColumns[resource] = resourceColumns
	}
	return s
}

func (s *segmentationAdaptor) WithSelector(selector *commonprotos.SegmentationQuery_SelectorExpr) SegmentationAdaptor {
	selectorJSON, _ := protojson.Marshal(selector)
	s.logger = s.logger.With("selector", string(selectorJSON))

	if len(s.resources) == 0 {
		panic("must called after WithResource()")
	}

	for _, resource := range s.resources {
		selector := NewSelectorExpression(s.ctx, selector, s.meta[resource])
		s.resourceConditions[resource] = selector.String()
		if err := selector.Error(); err != nil {
			s.logger.Errorf("selector %s for resource %s failed: %v", selector, resource, err)
			s.errors = append(s.errors, err)
		}
	}
	return s
}

func (s *segmentationAdaptor) Distinct() SegmentationAdaptor {
	s.logger = s.logger.With("distinct", true)
	s.distinct = true
	return s
}

func (s *segmentationAdaptor) Breakdown(breakdown ...string) SegmentationAdaptor {
	s.logger = s.logger.With("breakdown", breakdown)
	if len(s.resources) == 0 {
		panic("must called after WithResource()")
	}
	if s.aggregator != nil {
		panic("must called before AggregateBy()")
	}

	for _, field := range breakdown {
		s.breakdown = append(s.breakdown, s.touchField(field))
	}
	return s
}

func (s *segmentationAdaptor) Order(order ...string) SegmentationAdaptor {
	s.logger = s.logger.With("order", order)
	s.orders = append(s.orders, order...)
	return s
}

func (s *segmentationAdaptor) Limit(limit int) SegmentationAdaptor {
	s.logger = s.logger.With("limit", limit)
	s.limit = limit
	return s
}

func (s *segmentationAdaptor) Offset(offset int) SegmentationAdaptor {
	s.logger = s.logger.With("offset", offset)
	s.offset = offset
	return s
}

func (s *segmentationAdaptor) AggregateBy(op *commonprotos.SegmentationQuery_Aggregation) SegmentationAdaptor {
	aggJSON, _ := protojson.Marshal(op)
	s.logger = s.logger.With("aggregate", string(aggJSON))

	if op.GetAggregateProperties() != nil {
		s.touchField(op.GetAggregateProperties().GetPropertyName())
	}

	var err error
	s.aggregator, err = NewAggregator(s.ctx, s.logger, op, s.timeRange, s.breakdown, s.option)
	if err != nil {
		s.logger.Errorf("new aggregator failed: %v", err)
		s.errors = append(s.errors, err)
	} else {
		s.cumulative = s.aggregator.Cumulative()
	}
	return s
}

func (s *segmentationAdaptor) FetchBy(fields ...string) SegmentationAdaptor {
	s.logger = s.logger.With("fetch", fields)
	s.scanColumns = append(s.scanColumns, fields...)
	s.columns = lo.SliceToMap(s.scanColumns, func(i string) (string, struct{}) {
		return i, struct{}{}
	})
	for _, field := range fields {
		s.touchField(field)
	}
	return s
}

func (s *segmentationAdaptor) timeRangeCondString() string {
	if s.timeRange == nil {
		return "1"
	}
	var conditions []string
	if s.timeRange.RangeMode == timerange.LeftOpenRange || s.timeRange.RangeMode == timerange.BothOpenRange {
		conditions = append(conditions,
			timeseries.SystemTimestamp+">"+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", s.timeRange.Start.UTC().Format("2006-01-02 15:04:05")))
	} else {
		conditions = append(conditions,
			timeseries.SystemTimestamp+">="+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", s.timeRange.Start.UTC().Format("2006-01-02 15:04:05")))
	}
	if s.timeRange.RangeMode == timerange.RightOpenRange || s.timeRange.RangeMode == timerange.BothOpenRange {
		conditions = append(conditions,
			timeseries.SystemTimestamp+"<"+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", s.timeRange.End.UTC().Format("2006-01-02 15:04:05")))
	} else {
		conditions = append(conditions,
			timeseries.SystemTimestamp+"<="+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", s.timeRange.End.UTC().Format("2006-01-02 15:04:05")))
	}
	return strings.Join(conditions, " AND ")
}

func (s *segmentationAdaptor) earlierTimeRangeCondString() string {
	if s.timeRange == nil {
		return "1"
	}
	if s.timeRange.RangeMode == timerange.LeftOpenRange || s.timeRange.RangeMode == timerange.BothOpenRange {
		return timeseries.SystemTimestamp + "<=" + fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", s.timeRange.Start.UTC().Format("2006-01-02 15:04:05"))
	} else {
		return timeseries.SystemTimestamp + "<" + fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", s.timeRange.Start.UTC().Format("2006-01-02 15:04:05"))
	}
}

func (s *segmentationAdaptor) resourceTable(resource string, nullAsDefault, earlier bool) string {
	var (
		fields   []string
		timeCond = lo.If(s.aggregator != nil && s.aggregator.TimePostcondition(), "1").
				Else(lo.If(earlier, s.earlierTimeRangeCondString()).Else(s.timeRangeCondString()))
		resourceCond = lo.If(s.resourceConditions[resource] == "", "1").
				Else(s.resourceConditions[resource])
		conds = []string{
			timeCond,
			resourceCond,
		}
		condsStr = strings.Join(conds, " AND ")
		table    = s.store.MetaTable(s.meta[resource])
	)
	for _, column := range utils.GetOrderedMapKeys(s.columns) {
		if property, ok := s.resourceColumns[resource][column]; !ok {
			switch nullAsDefault {
			case true:
				fields = append(fields, "NULL AS `"+column+"`")
			default:
				// do nothing
			}
		} else {
			switch nullAsDefault {
			case true:
				if property.nested {
					fields = append(fields, clickhouse.DbNullableTypeCasting(timeseries.EscapeEventlogFieldName(column), property.fieldType)+" AS `"+column+"`")
				} else {
					fields = append(fields, "toNullable("+timeseries.EscapeEventlogFieldName(column)+") AS `"+column+"`")
				}
			case false:
				if property.nested {
					fields = append(fields, clickhouse.DbTypeCasting(timeseries.EscapeEventlogFieldName(column), property.fieldType)+" AS `"+column+"`")
				} else {
					fields = append(fields, timeseries.EscapeEventlogFieldName(column)+" AS `"+column+"`")
				}
			}
		}
	}
	resourceTable := "SELECT " + strings.Join(fields, ",") + " FROM `" + table + "` WHERE " + condsStr
	s.logger.Debugf("resource[%s] table: %s", resource, resourceTable)
	return resourceTable
}

func (s *segmentationAdaptor) dataQueries(earlier bool) []string {
	var nullAsDefault bool
	if len(s.resources) > 1 {
		nullAsDefault = true
	}
	var tables []string
	for _, resource := range s.resources {
		tables = append(tables, s.resourceTable(resource, nullAsDefault, earlier))
	}
	return tables
}

func (s *segmentationAdaptor) dataTable(earlier bool) string {
	if len(s.resources) == 0 {
		panic("must called after WithResource()")
	}

	tables := s.dataQueries(earlier)
	if len(tables) > 1 {
		return "SELECT * FROM (" + strings.Join(tables, " UNION ALL ") + ")"
	} else {
		return tables[0]
	}
}

func (s *segmentationAdaptor) buildAggregation() string {
	const (
		aggTpl = `{union} 
SELECT {distinct} 
	{time_field} AS {time_alias},
	{agg_field} AS {agg_alias} {label}
FROM {main_table} 
	{join}
{breakdown} {order} {limit} {offset}`
		finalTpl = `{cte} SELECT * FROM ({agg_tpl}) AS _agg_table_ {post_where}`
	)

	var (
		ctes                        cte.CTEs
		joinType, joinTable, joinOn = s.aggregator.Join()
		joinParameters              = lo.If(joinType != "", joinType+" JOIN "+joinTable+" ON "+joinOn).Else("")
		timeCondStr                 = lo.If(s.aggregator.TimePostcondition(), s.timeRangeCondString()).Else("1")
	)
	ctes = append(ctes, cte.CTE{
		Alias: mainTable,
		Query: s.dataTable(false),
	})
	if joinType != "" {
		ctes = append(ctes, cte.CTE{
			Alias: beforeTimeRangeTable,
			Query: s.dataTable(true),
		})
	}
	ctes = append(ctes, s.aggregator.CTE()...)
	aggSql := builder.FormatSQLTemplate(aggTpl, map[string]any{
		"union":      lo.If(len(s.aggregator.Union()) > 0, strings.Join(s.aggregator.Union(), " UNION ALL ")+" UNION ALL ").Else(""),
		"distinct":   lo.If(s.aggregator.Distinct() || s.distinct, " DISTINCT ").Else(""),
		"time_field": s.aggregator.TimeField(),
		"time_alias": matrix.TimeFieldName,
		"agg_field":  s.aggregator.AggField(),
		"agg_alias":  matrix.AggFieldName,
		"label":      s.aggregator.Label().String(true),
		"main_table": s.aggregator.Table(),
		"join":       joinParameters,
		"breakdown":  lo.If(s.aggregator.Breakdown().String(false) != "", " GROUP BY "+s.aggregator.Breakdown().String(false)).Else(""),
		"order":      lo.If(len(s.orders) > 0, " ORDER BY "+strings.Join(s.orders, ",")).Else(""),
		"limit":      lo.If(s.limit > 0, fmt.Sprintf(" LIMIT %d ", s.limit)).Else(""),
		"offset":     lo.If(s.offset > 0, fmt.Sprintf(" OFFSET %d ", s.offset)).Else(""),
	})
	return builder.FormatSQLTemplate(finalTpl, map[string]any{
		"cte":        ctes.String(),
		"agg_tpl":    aggSql,
		"post_where": "WHERE " + timeCondStr,
	})
}

func (s *segmentationAdaptor) buildScan() string {
	tpl := `
{cte}
SELECT {distinct} {fields} FROM {main_table} {order} {limit} {offset}`

	var (
		ctes cte.CTEs
	)
	ctes = append(ctes, cte.CTE{
		Alias: mainTable,
		Query: s.dataTable(false),
	})
	return builder.FormatSQLTemplate(tpl, map[string]any{
		"cte":        ctes.String(),
		"distinct":   lo.If(s.distinct, " DISTINCT ").Else(""),
		"fields":     strings.Join(s.scanColumns, ","),
		"main_table": mainTable,
		"order":      lo.If(len(s.orders) > 0, " ORDER BY "+strings.Join(s.orders, ",")).Else(""),
		"limit":      lo.If(s.limit > 0, fmt.Sprintf(" LIMIT %d ", s.limit)).Else(""),
		"offset":     lo.If(s.offset > 0, fmt.Sprintf(" OFFSET %d ", s.offset)).Else(""),
	})
}

func (s *segmentationAdaptor) Build() string {
	if err := s.Error(); err != nil {
		s.logger.Errorf("error: %s", err)
		return err.Error()
	}

	var sql string
	switch {
	case s.aggregator != nil:
		sql = s.buildAggregation()
	case len(s.scanColumns) > 0:
		sql = s.buildScan()
	default:
		panic("must called after AggregateBy() or ScanBy()")
	}

	s.logger.Debugf("sql: %s", sql)
	switch s.option.FormatMode {
	case FormatModeNone:
		return sql
	case FormatModeViaRewriter:
		if s.option.Rewriter == nil {
			panic("must set rewriter")
		}
		response, err := s.option.Rewriter.Format(s.ctx, &protosrewriter.FormatSQLRequest{Sql: sql})
		if err != nil {
			return sql
		}
		if response.ErrorMessage != "" {
			s.errors = append(s.errors, fmt.Errorf("format error: %s", response.ErrorMessage))
			s.logger.Errorf("format error: %s", response.ErrorMessage)
			return sql
		}
		return response.Sql
	}
	return sql
}
