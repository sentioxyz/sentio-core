package prebuilt_function

import (
	"time"

	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/selector"
)

type Operator int

const (
	OperatorSum Operator = iota
	OperatorAvg
	OperatorMin
	OperatorMax
	OperatorFirst
	OperatorLast
	OperatorCount
	OperatorDelta
	OperatorAbs
	OperatorCeil
	OperatorFloor
	OperatorRound
	OperatorLog2
	OperatorLog10
	OperatorLn
	OperatorTopK
	OperatorBottomK
	OperatorTimestamp
	OperatorDayOfYear
	OperatorDayOfMonth
	OperatorDayOfWeek
	OperatorYear
	OperatorMonth
	OperatorHour
	OperatorMinute
	OperatorRate
	OperatorIRate
)

type Function interface {
	WithTimeRange(timeRange *timerange.TimeRange) Function
	WithLabels(labels []string) Function
	WithResultAlias(resultAlias string) Function
	WithOp(op Operator) Function
	WithTable(table string) Function
	WithValueField(valueField string) Function
	WithSelector(selector selector.Selector) Function

	Generate() (string, error)
	GetTableName() string
	GetValueField() string
	GetResultAlias() string
	GetFuncName() string
}

type AggregatedOverTimeFunction interface {
	Function
	AggregatedWindowSize(aggregatedWindowSize time.Duration) AggregatedOverTimeFunction
}

type RollupFunction interface {
	AggregatedOverTimeFunction
	RollupWindowSize(rollupWindow time.Duration) RollupFunction
}

type MathFunction interface {
	Function
	Math() MathFunction
}

type RankFunction interface {
	Function
	Rank(k int) RankFunction
}

type TimeFunction interface {
	Function
	Time() TimeFunction
}

type RateFunction interface {
	Function
	Rate(step time.Duration) RateFunction
}

// FilterFunction is a function that filters out data points based on a condition
// usually used as first in a cascade of functions
type FilterFunction interface {
	Function
	Filter() FilterFunction
}

// SampleFunction is a function that samples data points
// usually used as last in a cascade of functions
type SampleFunction interface {
	Function
	Sample(d time.Duration) SampleFunction
}
