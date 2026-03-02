package filter

import (
	"fmt"
	"strings"
	"time"

	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/timerange"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"

	"github.com/samber/lo"
)

type filterFunction struct {
	*prebuilt.BaseFunction
}

func NewFilterFunction(meta timeseries.Meta, store timeseries.Store) prebuilt.FilterFunction {
	return &filterFunction{
		BaseFunction: prebuilt.NewBaseFunction(meta, store, "filter"),
	}
}

func (f *filterFunction) Filter() prebuilt.FilterFunction {
	defer f.Init(f)
	return f
}

func (f *filterFunction) startingUnionData() string {
	if f.TimeRange == nil {
		return ""
	}

	const tpl = `
	SELECT
		{starting} AS {timestamp},
		{label_fields}
		{result_alias} AS {result_alias}
	FROM (
		SELECT
			{label_fields}
			argMax({value_field}, {sort_tuple}) AS {result_alias}
		FROM {table} {where_clause}
		{group_by_labels}
		{order_by_labels}
	) AS starting_aggr
	UNION ALL
`
	var (
		conditions []string
		sortTuple  string
		starting   time.Time
	)
	if f.TimeRange.RangeMode == timerange.LeftOpenRange || f.TimeRange.RangeMode == timerange.BothOpenRange {
		conditions = append(conditions,
			timeseries.SystemTimestamp+"<="+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", f.TimeRange.Start.UTC().Format("2006-01-02 15:04:05")))
		starting = f.TimeRange.Start.Add(time.Second)
	} else {
		conditions = append(conditions,
			timeseries.SystemTimestamp+"<"+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", f.TimeRange.Start.UTC().Format("2006-01-02 15:04:05")))
		starting = f.TimeRange.Start
	}
	if f.Selector != nil {
		conditions = append(conditions, f.Selector.Cond())
	}

	if len(f.Meta.GetFieldsByRole(timeseries.FieldRoleAggInterval)) != 0 {
		sortTuple = timeseries.SystemFieldPrefix + "block_number"
	} else {
		sortTuple = "(" + timeseries.SystemFieldPrefix + "block_number" + "," + timeseries.SystemFieldPrefix + "transaction_index" + "," + timeseries.SystemFieldPrefix + "log_index" + ")"
	}

	return builder.FormatSQLTemplate(tpl, map[string]any{
		"starting":        fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", starting.UTC().Format("2006-01-02 15:04:05")),
		"timestamp":       timeseries.SystemTimestamp,
		"label_fields":    f.GetLabelFields(),
		"value_field":     f.GetValueField(),
		"sort_tuple":      sortTuple,
		"result_alias":    f.GetResultAlias(),
		"group_by_labels": lo.If(f.GetLabelFields() != "", " GROUP BY "+strings.TrimSuffix(f.GetLabelFields(), ",")).Else(""),
		"order_by_labels": lo.If(f.GetLabelFields() != "", " ORDER BY "+strings.TrimSuffix(f.GetLabelFields(), ",")).Else(""),
		"table":           f.GetTableName(),
		"where_clause":    " WHERE " + strings.Join(conditions, " AND "),
	})
}

func (f *filterFunction) Generate() (string, error) {
	const tpl = `
	{starting_data}
	SELECT
		{timestamp},
		{label_fields}
		{value_field} AS {result_alias}
	FROM {table} {where_clause}
	ORDER BY {label_fields} {timestamp} ASC
`
	return builder.FormatSQLTemplate(tpl, map[string]any{
		"starting_data": f.startingUnionData(),
		"timestamp":     timeseries.SystemTimestamp,
		"label_fields":  f.GetLabelFields(),
		"result_alias":  f.GetResultAlias(),
		"value_field":   f.GetValueField(),
		"table":         f.GetTableName(),
		"where_clause":  f.WhereClause(f.TimeRange),
	}), nil
}

func (f *filterFunction) GetFuncName() string {
	return "filter_function"
}
