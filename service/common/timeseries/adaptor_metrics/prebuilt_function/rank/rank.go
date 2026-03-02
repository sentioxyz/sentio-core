package rank

import (
	"fmt"
	"strconv"

	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/driver/timeseries"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"
)

type rankFunction struct {
	*prebuilt.BaseFunction
	k int
}

func NewRankFunction(meta timeseries.Meta, store timeseries.Store) prebuilt.RankFunction {
	return &rankFunction{
		BaseFunction: prebuilt.NewBaseFunction(meta, store, "rank"),
	}
}

func (f *rankFunction) Rank(k int) prebuilt.RankFunction {
	defer f.Init(f)
	f.k = k
	return f
}

func (f *rankFunction) OpString() (string, error) {
	switch f.Operator {
	case prebuilt.OperatorTopK:
		return "topK(" + strconv.FormatInt(int64(f.k), 10) + ")(" + f.GetValueField() + ") AS " + f.ResultAlias, nil
	case prebuilt.OperatorBottomK:
		return "", fmt.Errorf("bottomK is not supported yet")
	default:
		return "", fmt.Errorf("unsupported operator: %v", f.Operator)
	}
}

func (f *rankFunction) Generate() (string, error) {
	if f.k <= 0 {
		return "", fmt.Errorf("k must be positive")
	}

	var (
		labelFields    = f.GetLabelFields()
		orderByClause  = "ORDER BY " + prebuilt.MilliTimestampField + " ASC"
		whereClause    = f.WhereClause(f.TimeRange)
		aggrField, err = f.OpString()
	)
	if err != nil {
		return "", err
	}

	const tpl = `SELECT {timestamp}, {milli_timestamp_field}, {label_fields} {result_alias} FROM (
	SELECT {timestamp}, {milli_timestamp}, {label_fields} {aggr_field} FROM {table} GROUP BY {label_fields} {timestamp}, {milli_timestamp_field}
) AS rank_table {array_join} {where_clause} {order_by_clause}`

	return builder.FormatSQLTemplate(tpl, map[string]any{
		"timestamp":             timeseries.SystemTimestamp,
		"milli_timestamp":       prebuilt.MilliTimestamp,
		"milli_timestamp_field": prebuilt.MilliTimestampField,
		"label_fields":          labelFields,
		"result_alias":          f.ResultAlias,
		"aggr_field":            aggrField,
		"array_join":            " ARRAY JOIN " + f.ResultAlias,
		"table":                 f.GetTableName(),
		"where_clause":          whereClause,
		"order_by_clause":       orderByClause,
	}), nil
}

func (f *rankFunction) GetFuncName() string {
	return "rank_function"
}
