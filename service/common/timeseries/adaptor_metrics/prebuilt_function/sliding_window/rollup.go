package sliding_window

import (
	"fmt"
	"time"

	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/driver/timeseries"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"

	"github.com/samber/lo"
)

type rollupSlidingWindowFunction struct {
	*prebuilt.BaseFunction
	aggregatedWindowSize time.Duration
	rollupWindowSize     time.Duration
}

func NewRollupSlidingWindowFunction(meta timeseries.Meta, store timeseries.Store) prebuilt.RollupFunction {
	return &rollupSlidingWindowFunction{
		BaseFunction: prebuilt.NewBaseFunction(meta, store, "rollup"),
	}
}

func (f *rollupSlidingWindowFunction) AggregatedWindowSize(aggregatedWindowSize time.Duration) prebuilt.AggregatedOverTimeFunction {
	defer f.Init(f)
	f.aggregatedWindowSize = aggregatedWindowSize
	return f
}

func (f *rollupSlidingWindowFunction) RollupWindowSize(rollupWindowSize time.Duration) prebuilt.RollupFunction {
	defer f.Init(f)
	f.rollupWindowSize = rollupWindowSize
	return f
}

func (f *rollupSlidingWindowFunction) Generate() (string, error) {
	if f.aggregatedWindowSize == 0 {
		return "", fmt.Errorf("window size is required")
	}
	var (
		labelFields    = f.GetLabelFields()
		orderByClause  = "ORDER BY " + prebuilt.MilliTimestampField + " ASC"
		whereClause    = f.WhereClause(f.TimeRange)
		aggrField, err = opString(f.GetValueField(), f.ResultAlias, "DESC", f.Labels, f.aggregatedWindowSize, f.Operator)
	)
	if err != nil {
		return "", err
	}

	const tpl = `
	SELECT
		{timestamp},
		{milli_timestamp},
		{label_fields}
		{result_alias} AS {result_alias}
	FROM (
		SELECT
			{timestamp},
			{second_timestamp},
			{label_fields}
			{aggr_field}
		FROM {table} {where_clause} {order_by_clause}
	) AS aggr_table
	WHERE {timestamp} = date_trunc('{step_unit}', {timestamp}, '{timezone}') AND {result_alias} IS NOT NULL
`

	return builder.FormatSQLTemplate(tpl, map[string]any{
		"timestamp":        timeseries.SystemTimestamp,
		"milli_timestamp":  prebuilt.MilliTimestamp,
		"second_timestamp": prebuilt.SecondTimestamp,
		"label_fields":     labelFields,
		"aggr_field":       aggrField,
		"result_alias":     f.GetResultAlias(),
		"table":            f.GetTableName(),
		"where_clause":     whereClause,
		"order_by_clause":  orderByClause,
		"step_unit":        f.StepUnit(f.TimeRange),
		"timezone":         lo.IfF(f.TimeRange != nil, func() string { return f.TimeRange.Timezone.String() }).Else("UTC"),
	}), nil
}

func (f *rollupSlidingWindowFunction) GetFuncName() string {
	return "rollup_sliding_window_function"
}
