package prebuilt_function

import (
	"context"
	"fmt"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/gonanoid"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/selector"
	"sentioxyz/sentio-core/service/common/timeseries/util"

	"github.com/samber/lo"
)

const (
	MilliTimestampField  = timeseries.SystemTimestamp + "_milli"
	MilliTimestamp       = "toUnixTimestamp64Milli(" + timeseries.SystemTimestamp + ") AS " + MilliTimestampField
	SecondTimestampField = timeseries.SystemTimestamp + "_second"
	SecondTimestamp      = "toUnixTimestamp64Second(" + timeseries.SystemTimestamp + ") AS " + SecondTimestampField
)

type BaseFunction struct {
	Meta  timeseries.Meta
	Store timeseries.Store

	TableName   string
	Operator    Operator
	TimeRange   *timerange.TimeRange
	Labels      []string
	ValueField  string
	ResultAlias string
	Selector    selector.Selector

	self Function
}

func NewBaseFunction(meta timeseries.Meta, store timeseries.Store, category string) *BaseFunction {
	return &BaseFunction{
		Meta:        meta,
		Store:       store,
		ResultAlias: timeseries.MetricValueFieldName + "_" + category + "_" + gonanoid.Must(5),
	}
}

func (f *BaseFunction) Init(self Function) {
	f.self = self
}

func (f *BaseFunction) timeRange(specifiedTimeRange *timerange.TimeRange) (timeRange *timerange.TimeRange) {
	if specifiedTimeRange != nil {
		timeRange = specifiedTimeRange
	} else {
		timeRange = f.TimeRange
	}
	return
}

func (f *BaseFunction) TimeRangeCondString(specifiedTimeRange *timerange.TimeRange) string {
	var timeRange = f.timeRange(specifiedTimeRange)
	if timeRange == nil {
		return "1"
	}
	var conditions []string
	if timeRange.RangeMode == timerange.LeftOpenRange || timeRange.RangeMode == timerange.BothOpenRange {
		conditions = append(conditions,
			timeseries.SystemTimestamp+">"+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", timeRange.Start.UTC().Format("2006-01-02 15:04:05")))
	} else {
		conditions = append(conditions,
			timeseries.SystemTimestamp+">="+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", timeRange.Start.UTC().Format("2006-01-02 15:04:05")))
	}
	if timeRange.RangeMode == timerange.RightOpenRange || timeRange.RangeMode == timerange.BothOpenRange {
		conditions = append(conditions,
			timeseries.SystemTimestamp+"<"+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", timeRange.End.UTC().Format("2006-01-02 15:04:05")))
	} else {
		conditions = append(conditions,
			timeseries.SystemTimestamp+"<="+fmt.Sprintf("toDateTime64('%s', 6, 'UTC')", timeRange.End.UTC().Format("2006-01-02 15:04:05")))
	}
	return strings.Join(conditions, " AND ")
}

func (f *BaseFunction) clickhouseAlignedTime(t time.Time, step time.Duration) string {
	var (
		ckTimeLayout   = "2006-01-02 15:04:05"
		outputFormat   = "toDateTime64(date_trunc('%s', toDateTime64('%s', 6, 'UTC'), 'UTC'), 6, 'UTC')"
		fallbackLayout = "toDateTime64('%s', 6, 'UTC')"
	)
	if unit, ok := util.HistogramTimeUnitMap[step]; ok {
		return fmt.Sprintf(outputFormat, unit, t.Format(ckTimeLayout))
	} else {
		return fmt.Sprintf(fallbackLayout, t.Format(ckTimeLayout))
	}
}

func (f *BaseFunction) StartAlignedTime(specifiedTimeRange *timerange.TimeRange) string {
	var timeRange = f.timeRange(specifiedTimeRange)
	if timeRange == nil {
		return "now()"
	}
	return f.clickhouseAlignedTime(timeRange.Start, timeRange.Step)
}

func (f *BaseFunction) EndAlignedTime(specifiedTimeRange *timerange.TimeRange) string {
	var timeRange = f.timeRange(specifiedTimeRange)
	if timeRange == nil {
		return "now()"
	}
	return f.clickhouseAlignedTime(timeRange.End, timeRange.Step)
}

func (f *BaseFunction) StepAlignedInterval(specifiedTimeRange *timerange.TimeRange) string {
	var timeRange = f.timeRange(specifiedTimeRange)
	if timeRange == nil {
		return "toIntervalSecond(1)"
	}
	return fmt.Sprintf("toIntervalSecond(%d)", int64(timeRange.Step.Seconds()))
}

func (f *BaseFunction) StepUnit(specifiedTimeRange *timerange.TimeRange) string {
	var timeRange = f.timeRange(specifiedTimeRange)
	if timeRange == nil {
		return "second"
	}
	if unit, ok := util.HistogramTimeUnitMap[timeRange.Step]; ok {
		return unit
	}
	return "second"
}

func (f *BaseFunction) WhereClause(specifiedTimeRange *timerange.TimeRange) string {
	var conditions []string

	tr := lo.If(specifiedTimeRange == nil, f.TimeRange).Else(specifiedTimeRange)
	if tr != nil {
		conditions = append(conditions, f.TimeRangeCondString(specifiedTimeRange))
	}

	if f.Selector != nil {
		conditions = append(conditions, f.Selector.Cond())
	}

	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}

func (f *BaseFunction) WithSelector(selector selector.Selector) Function {
	f.Selector = selector
	return f
}

func (f *BaseFunction) WithTimeRange(timeRange *timerange.TimeRange) Function {
	f.TimeRange = timeRange
	return f
}

func (f *BaseFunction) WithLabels(labels []string) Function {
	f.Labels = labels
	return f
}

func (f *BaseFunction) WithResultAlias(resultAlias string) Function {
	f.ResultAlias = resultAlias
	return f
}

func (f *BaseFunction) WithOp(op Operator) Function {
	f.Operator = op
	return f
}

func (f *BaseFunction) WithTable(table string) Function {
	f.TableName = table
	return f
}

func (f *BaseFunction) WithValueField(valueField string) Function {
	f.ValueField = valueField
	return f
}

func (f *BaseFunction) Generate() (string, error) {
	if f.self == nil {
		return "", fmt.Errorf("function is not initialized")
	}

	_, logger := log.FromContext(context.Background())
	logger.With("name", f.GetFuncName())
	generated, err := f.self.Generate()
	if err != nil {
		logger.Errorw("failed to generate function",
			"error", err,
			"store", f.Store.Meta().String(),
			"meta", f.Meta.GetFullName())
		return "", err
	}
	return generated, nil
}

func (f *BaseFunction) GetFuncName() string {
	if f.self == nil {
		return "base_function"
	}

	return f.self.GetFuncName()
}

func (f *BaseFunction) GetTableName() string {
	return lo.IfF(f.TableName == "", func() string { return f.Store.MetaTable(f.Meta) }).Else(f.TableName)
}

func (f *BaseFunction) GetResultAlias() string {
	return f.ResultAlias
}

func (f *BaseFunction) GetValueField() string {
	return lo.If(f.ValueField == "", timeseries.MetricValueFieldName).Else(f.ValueField)
}

func (f *BaseFunction) GetLabelFields() string {
	return lo.If(len(f.Labels) > 0, strings.Join(f.Labels, ",")+",").Else("")
}
