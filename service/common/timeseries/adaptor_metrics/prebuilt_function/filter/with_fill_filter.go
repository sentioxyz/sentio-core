package filter

import (
	"fmt"

	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/driver/timeseries"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"
)

type withFillFilterFunction struct {
	*filterFunction
}

func NewWithFillFilterFunction(meta timeseries.Meta, store timeseries.Store) prebuilt.FilterFunction {
	return &withFillFilterFunction{
		filterFunction: &filterFunction{
			BaseFunction: prebuilt.NewBaseFunction(meta, store, "filter"),
		},
	}
}

func (f *withFillFilterFunction) Filter() prebuilt.FilterFunction {
	defer f.Init(f)
	return f
}

func (f *withFillFilterFunction) Generate() (string, error) {
	if f.TimeRange == nil {
		return "", fmt.Errorf("with fill function must have timerange")
	}

	const tpl = `
	SELECT
		{timestamp},
		{label_fields}
		toNullable({result_alias}) AS {result_alias}
	FROM (
		{starting_data}
		SELECT
			{timestamp},
			{label_fields}
			{value_field} AS {result_alias}
		FROM
			{table} {where_clause}
	) AS unioned
	ORDER BY {label_fields} {timestamp} ASC
	WITH FILL FROM {start_time} TO {end_time} STEP {step}
	INTERPOLATE ({result_alias} AS NULL)
`

	return builder.FormatSQLTemplate(tpl, map[string]any{
		"starting_data": f.startingUnionData(),
		"timestamp":     timeseries.SystemTimestamp,
		"label_fields":  f.GetLabelFields(),
		"result_alias":  f.GetResultAlias(),
		"value_field":   f.GetValueField(),
		"table":         f.GetTableName(),
		"where_clause":  f.WhereClause(f.TimeRange),
		"start_time":    f.StartAlignedTime(f.TimeRange),
		"end_time":      f.EndAlignedTime(f.TimeRange),
		"step":          f.StepAlignedInterval(f.TimeRange),
	}), nil
}

func (f *withFillFilterFunction) GetFuncName() string {
	return "with_fill_filter_function"
}
