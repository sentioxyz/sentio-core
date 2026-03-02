package math

import (
	"fmt"

	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/driver/timeseries"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"
)

type mathFunction struct {
	*prebuilt.BaseFunction
}

func NewMathFunction(meta timeseries.Meta, store timeseries.Store) prebuilt.MathFunction {
	return &mathFunction{
		BaseFunction: prebuilt.NewBaseFunction(meta, store, "math"),
	}
}

func (f *mathFunction) Math() prebuilt.MathFunction {
	defer f.Init(f)
	return f
}

func (f *mathFunction) OpString() (string, error) {
	switch f.Operator {
	case prebuilt.OperatorAbs:
		return "abs(" + f.GetValueField() + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorCeil:
		return "ceil(" + f.GetValueField() + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorFloor:
		return "floor(" + f.GetValueField() + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorRound:
		return "round(" + f.GetValueField() + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorLog2:
		return "log2(" + f.GetValueField() + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorLog10:
		return "log10(" + f.GetValueField() + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorLn:
		return "log(" + f.GetValueField() + ") AS " + f.ResultAlias, nil
	default:
		return "", fmt.Errorf("unsupported operator: %v", f.Operator)
	}
}

func (f *mathFunction) Generate() (string, error) {
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

func (f *mathFunction) GetFuncName() string {
	return "math_function"
}
