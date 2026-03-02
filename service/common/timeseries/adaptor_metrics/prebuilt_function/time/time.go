package time

import (
	"fmt"

	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/driver/timeseries"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"
)

type timeFunction struct {
	*prebuilt.BaseFunction
}

func NewTimeFunction(meta timeseries.Meta, store timeseries.Store) prebuilt.TimeFunction {
	return &timeFunction{
		BaseFunction: prebuilt.NewBaseFunction(meta, store, "time"),
	}
}

func (f *timeFunction) Time() prebuilt.TimeFunction {
	defer f.Init(f)
	return f
}

func (f *timeFunction) OpString() (string, error) {
	switch f.Operator {
	case prebuilt.OperatorTimestamp:
		return "toUnixTimestamp(" + timeseries.SystemTimestamp + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorDayOfYear:
		return "toDayOfYear(" + timeseries.SystemTimestamp + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorDayOfMonth:
		return "toDayOfMonth(" + timeseries.SystemTimestamp + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorDayOfWeek:
		return "toDayOfWeek(" + timeseries.SystemTimestamp + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorYear:
		return "toYear(" + timeseries.SystemTimestamp + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorMonth:
		return "toMonth(" + timeseries.SystemTimestamp + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorHour:
		return "toHour(" + timeseries.SystemTimestamp + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorMinute:
		return "toMinute(" + timeseries.SystemTimestamp + ") AS " + f.ResultAlias, nil
	default:
		return "", fmt.Errorf("unsupported operator: %v", f.Operator)
	}
}

func (f *timeFunction) Generate() (string, error) {
	var (
		labelFields    = f.GetLabelFields()
		orderByClause  = "ORDER BY " + prebuilt.MilliTimestampField + " ASC"
		whereClause    = f.WhereClause(f.TimeRange)
		aggrField, err = f.OpString()
	)
	if err != nil {
		return "", err
	}

	const tpl = `SELECT {timestamp}, {milli_timestamp}, {label_fields} {aggr_field} FROM {table} {where_clause} {order_by_clause}`
	return builder.FormatSQLTemplate(tpl, map[string]any{
		"timestamp":       timeseries.SystemTimestamp,
		"milli_timestamp": prebuilt.MilliTimestamp,
		"label_fields":    labelFields,
		"aggr_field":      aggrField,
		"table":           f.GetTableName(),
		"where_clause":    whereClause,
		"order_by_clause": orderByClause,
	}), nil
}

func (f *timeFunction) GetFuncName() string {
	return "time_function"
}
