package sliding_window

import (
	"fmt"
	"strings"
	"time"

	"sentioxyz/sentio-core/driver/timeseries"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"

	"github.com/samber/lo"
)

func opString(valueField, resultAlias, sequence string, labels []string, windowSize time.Duration, operator prebuilt.Operator) (string, error) {
	var (
		orderByClause     = "ORDER BY " + prebuilt.SecondTimestampField + " " + sequence
		partitionClause   = lo.If(len(labels) > 0, "PARTITION BY ("+strings.Join(labels, ",")+")").Else("")
		rangeClause       = fmt.Sprintf("RANGE BETWEEN %d PRECEDING AND CURRENT ROW", int64(windowSize.Seconds()-1))
		windowFunctionTpl = fmt.Sprintf(" OVER (%s %s %s)", partitionClause, orderByClause, rangeClause)
		aggrField         string
	)
	switch operator {
	case prebuilt.OperatorSum:
		aggrField = "sum(" + valueField + ")" + windowFunctionTpl + " AS " + resultAlias
	case prebuilt.OperatorAvg:
		aggrField = "avg(" + valueField + ")" + windowFunctionTpl + " AS " + resultAlias
	case prebuilt.OperatorMin:
		aggrField = "min(" + valueField + ")" + windowFunctionTpl + " AS " + resultAlias
	case prebuilt.OperatorMax:
		aggrField = "max(" + valueField + ")" + windowFunctionTpl + " AS " + resultAlias
	case prebuilt.OperatorFirst:
		aggrField = "argMin(" + valueField + "," + timeseries.SystemTimestamp + ")" + windowFunctionTpl + " AS " + resultAlias
	case prebuilt.OperatorLast:
		aggrField = "argMax(" + valueField + "," + timeseries.SystemTimestamp + ")" + windowFunctionTpl + " AS " + resultAlias
	case prebuilt.OperatorCount:
		aggrField = "count(" + valueField + ")" + windowFunctionTpl + " AS " + resultAlias
	case prebuilt.OperatorDelta:
		aggrField = "argMin(" + valueField + "," + timeseries.SystemTimestamp + ")" + windowFunctionTpl + " AS first_value, " +
			"argMax(" + valueField + "," + timeseries.SystemTimestamp + ")" + windowFunctionTpl + " AS last_value, " +
			"last_value - first_value" + " AS " + resultAlias
	default:
		return "", fmt.Errorf("unsupported operator: %v", operator)
	}
	return aggrField, nil
}
