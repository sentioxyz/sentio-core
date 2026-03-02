package rate

import (
	"fmt"
	"strings"
	"time"

	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/driver/timeseries"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"

	"github.com/samber/lo"
)

type rateFunction struct {
	*prebuilt.BaseFunction
	step time.Duration
}

func NewRateFunction(meta timeseries.Meta, store timeseries.Store) prebuilt.RateFunction {
	return &rateFunction{
		BaseFunction: prebuilt.NewBaseFunction(meta, store, "rate"),
	}
}

func (f *rateFunction) Rate(step time.Duration) prebuilt.RateFunction {
	defer f.Init(f)
	f.step = step
	return f
}

func (f *rateFunction) rateDiffTable() string {
	var (
		labelFields     = f.GetLabelFields()
		orderByClause   = "ORDER BY " + timeseries.SystemTimestamp + " ASC"
		partitionClause = lo.If(len(f.Labels) > 0, "PARTITION BY ("+strings.Join(f.Labels, ",")+")").Else("")
		whereClause     = f.WhereClause(f.TimeRange)
	)

	const tpl = `
	SELECT
		{timestamp},
		{label_fields}
		{value_field},
		if (lagInFrame(toNullable({value_field})) OVER w IS NULL OR {value_field} < lagInFrame(toNullable({value_field})) OVER w, 0, {value_field} - lagInFrame(toNullable({value_field})) OVER w) AS delta,
		if (lagInFrame(toNullable({timestamp})) OVER w IS NULL, 0, intDiv(toUnixTimestamp({timestamp}) - toUnixTimestamp(lagInFrame(toNullable({timestamp})) OVER w), 1)) AS delta_seconds
	FROM {table} {where_clause}
	WINDOW w AS (
		{partition_clause}
		{order_by_clause}
		ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
	)
`

	return builder.FormatSQLTemplate(tpl, map[string]any{
		"timestamp":        timeseries.SystemTimestamp,
		"label_fields":     labelFields,
		"value_field":      f.GetValueField(),
		"table":            f.GetTableName(),
		"partition_clause": partitionClause,
		"order_by_clause":  orderByClause,
		"where_clause":     whereClause,
	})
}

func (f *rateFunction) rate() string {
	const (
		tpl = `
	SELECT
		{timestamp},
		{second_timestamp},
		{label_fields}
		{value_field},
		sum(delta) OVER rw AS sum_delta,
		sum(delta_seconds) OVER rw AS sum_delta_seconds,
		if (sum_delta_seconds = 0, 0, sum_delta / sum_delta_seconds) AS {result_alias}
	FROM ({diff_table}) AS diff_table
	WINDOW rw AS (
		{partition_clause}
		{order_by_clause}
		RANGE BETWEEN {step} PRECEDING AND CURRENT ROW
	)
`
	)

	var (
		labelFields     = f.GetLabelFields()
		orderByClause   = "ORDER BY " + prebuilt.SecondTimestampField + " ASC"
		partitionClause = lo.If(len(f.Labels) > 0, "PARTITION BY ("+strings.Join(f.Labels, ",")+")").Else("")
	)
	return builder.FormatSQLTemplate(tpl, map[string]any{
		"timestamp":        timeseries.SystemTimestamp,
		"second_timestamp": prebuilt.SecondTimestamp,
		"label_fields":     labelFields,
		"value_field":      f.GetValueField(),
		"step":             int64(f.step.Seconds()),
		"diff_table":       f.rateDiffTable(),
		"partition_clause": partitionClause,
		"order_by_clause":  orderByClause,
		"result_alias":     f.GetResultAlias(),
	})
}

func (f *rateFunction) iRate() string {
	var (
		labelFields     = f.GetLabelFields()
		orderByClause   = "ORDER BY " + prebuilt.SecondTimestampField + " ASC"
		partitionClause = lo.If(len(f.Labels) > 0, "PARTITION BY ("+strings.Join(f.Labels, ",")+")").Else("")
		whereClause     = f.WhereClause(f.TimeRange)
	)

	const tpl = `
	SELECT
		{timestamp},
		{milli_timestamp},
		{label_fields}
		{value_field},
		prev_value,
		prev_timestamp,
		if (prev_value is NULL or {value_field} < prev_value, 0, {value_field} - prev_value) AS delta,
		if (prev_timestamp IS NULL, 0, intDiv(toUnixTimestamp({timestamp}) - toUnixTimestamp(prev_timestamp), 1)) AS delta_seconds,
		if (delta_seconds = 0, 0, delta / delta_seconds) AS {result_alias}
	FROM (
		SELECT
			{timestamp},
			{second_timestamp},
			{label_fields}
			{value_field},
			lagInFrame(toNullable({value_field})) OVER w AS prev_value,
			lagInFrame(toNullable({timestamp})) OVER w AS prev_timestamp
		FROM {table} {where_clause}
		WINDOW w AS (
			{partition_clause}
			{order_by_clause}
			RANGE BETWEEN {step} PRECEDING AND CURRENT ROW
		)
	) AS prev_table
`
	return builder.FormatSQLTemplate(tpl, map[string]any{
		"timestamp":        timeseries.SystemTimestamp,
		"milli_timestamp":  prebuilt.MilliTimestamp,
		"second_timestamp": prebuilt.SecondTimestamp,
		"label_fields":     labelFields,
		"value_field":      f.GetValueField(),
		"table":            f.GetTableName(),
		"partition_clause": partitionClause,
		"order_by_clause":  orderByClause,
		"where_clause":     whereClause,
		"result_alias":     f.GetResultAlias(),
		"step":             f.step.Seconds(),
	})
}

func (f *rateFunction) Generate() (string, error) {
	switch f.Operator {
	case prebuilt.OperatorRate:
		return f.rate(), nil
	case prebuilt.OperatorIRate:
		return f.iRate(), nil
	default:
		return "", fmt.Errorf("unsupported operator: %v", f.Operator)
	}
}

func (f *rateFunction) GetFuncName() string {
	return "rate_function"
}
