package adaptor_metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timerange"
	adaptor "sentioxyz/sentio-core/service/common/timeseries/adaptor_eventlogs"
	"sentioxyz/sentio-core/service/common/timeseries/matrix"
	"sentioxyz/sentio-core/service/common/timeseries/util"

	"github.com/samber/lo"
)

type QueryRangeAdaptor interface {
	Name() string
	Alias() string
	Id() string
	Parameter() *Parameters

	Error() error
	Build() (string, error)
	Scan(ctx context.Context, scan adaptor.ScanFunc, sql string, args ...any) (matrix.Matrix, error)
}

type queryRangeAdaptor struct {
	functionAdaptor FunctionAdaptor

	params *Parameters
	err    error
}

func NewQueryRangeAdaptor(fa FunctionAdaptor, params *Parameters) QueryRangeAdaptor {
	return &queryRangeAdaptor{
		functionAdaptor: fa,
		params:          params,
	}
}

func (qra *queryRangeAdaptor) Name() string {
	return qra.params.name
}

func (qra *queryRangeAdaptor) Alias() string {
	return qra.params.alias
}

func (qra *queryRangeAdaptor) Id() string {
	return qra.params.id
}

func (qra *queryRangeAdaptor) Parameter() *Parameters {
	return qra.params
}

func (qra *queryRangeAdaptor) histogram(d time.Duration, f, tz string) string {
	return util.HistogramFunction(d, f, tz)
}

func (qra *queryRangeAdaptor) buildAggregation() string {
	if qra.params.operator != nil {
		switch *qra.params.operator {
		case protos.Aggregate_AVG:
			return "avg(" + qra.functionAdaptor.ValueAlias() + ")"
		case protos.Aggregate_SUM:
			return "sum(" + qra.functionAdaptor.ValueAlias() + ")"
		case protos.Aggregate_MIN:
			return "min(" + qra.functionAdaptor.ValueAlias() + ")"
		case protos.Aggregate_MAX:
			return "max(" + qra.functionAdaptor.ValueAlias() + ")"
		case protos.Aggregate_COUNT:
			return "count(" + qra.functionAdaptor.ValueAlias() + ")"
		default:
			return "avg(" + qra.functionAdaptor.ValueAlias() + ")"
		}
	} else {
		return "last_value(" + qra.functionAdaptor.ValueAlias() + ")"
	}
}

func (qra *queryRangeAdaptor) whereClause() string {
	if qra.params == nil {
		return ""
	}
	var conditions []string
	if qra.params.timeRange.RangeMode == timerange.LeftOpenRange || qra.params.timeRange.RangeMode == timerange.BothOpenRange {
		conditions = append(conditions,
			timeseries.SystemTimestamp+">"+
				util.HistogramFunction(qra.params.timeRange.Step,
					fmt.Sprintf("toDateTime64('%s', 6, 'UTC')",
						qra.params.timeRange.Start.UTC().Format("2006-01-02 15:04:05")),
					qra.params.timeRange.Timezone.String()))
	} else {
		conditions = append(conditions,
			timeseries.SystemTimestamp+">="+
				util.HistogramFunction(qra.params.timeRange.Step,
					fmt.Sprintf("toDateTime64('%s', 6, 'UTC')",
						qra.params.timeRange.Start.UTC().Format("2006-01-02 15:04:05")),
					qra.params.timeRange.Timezone.String()))
	}
	if qra.params.timeRange.RangeMode == timerange.RightOpenRange || qra.params.timeRange.RangeMode == timerange.BothOpenRange {
		conditions = append(conditions,
			timeseries.SystemTimestamp+"<"+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", qra.params.timeRange.End.UTC().Format("2006-01-02 15:04:05")))
	} else {
		conditions = append(conditions,
			timeseries.SystemTimestamp+"<="+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", qra.params.timeRange.End.UTC().Format("2006-01-02 15:04:05")))
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}

func (qra *queryRangeAdaptor) Build() (string, error) {
	if qra.params == nil || qra.params.timeRange == nil {
		return "", fmt.Errorf("time range is required")
	}

	const (
		tpl = `
	{cte}
	SELECT
		{histogram} AS {time_alias},
		{label}
		{agg_field} AS {agg_alias}
		FROM {table} {where_clause} GROUP BY {label} {time_alias}
`
	)

	snippets, err := qra.functionAdaptor.Snippets()
	if err != nil {
		qra.err = err
		return "", err
	}
	var (
		cte = lo.IfF(len(snippets) > 0, func() string {
			l := lo.MapToSlice(snippets, func(k, v string) string {
				return k + " AS (" + v + ")"
			})
			return "WITH " + strings.Join(l, ",") + " "
		}).Else("")
		labels = lo.IfF(qra.params != nil && qra.params.operator != nil && len(qra.params.groups) > 0, func() string {
			return strings.Join(qra.params.groups, ",") + ","
		}).ElseIfF(qra.params.operator == nil, func() string {
			return strings.Join(qra.functionAdaptor.SeriesLabel(), ",") + ","
		}).Else("")
	)
	return builder.FormatSQLTemplate(tpl, map[string]any{
		"cte":          cte,
		"histogram":    qra.histogram(qra.params.timeRange.Step, timeseries.SystemTimestamp, qra.params.timeRange.Timezone.String()),
		"time_alias":   matrix.TimeFieldName,
		"label":        labels,
		"agg_field":    qra.buildAggregation(),
		"agg_alias":    matrix.AggFieldName,
		"table":        qra.functionAdaptor.TableAlias(),
		"where_clause": qra.whereClause(),
	}), nil
}

func (qra *queryRangeAdaptor) Error() error {
	return qra.err
}

func (qra *queryRangeAdaptor) Scan(ctx context.Context, scan adaptor.ScanFunc, sql string, args ...any) (matrix.Matrix, error) {
	if err := qra.Error(); err != nil {
		return nil, err
	}
	rows, err := scan(ctx, sql, args)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	return matrix.NewMatrix(rows)
}
