package adaptor_metrics

import (
	"fmt"
	"runtime/debug"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/timeseries"
	protoscommon "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timerange"
	cascade "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/cascade_function"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/filter"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/math"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/rank"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/rate"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/sample"
	slidingwindow "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/sliding_window"
	prebuilttime "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/time"

	"github.com/samber/lo"
)

type FunctionAdaptor interface {
	Generate() (string, error)
	Snippets() (map[string]string, error)
	TableAlias() string
	ValueAlias() string
	Parameter() *Parameters
	SeriesLabel() []string
}

type functionAdaptor struct {
	meta      timeseries.Meta
	store     timeseries.Store
	functions []*protoscommon.Function
	labels    []string

	// additional parameters
	parameter       *Parameters
	extendTimeRange *timerange.TimeRange

	prebuilt []prebuilt.Function
	cascade  cascade.Functions
}

func NewFunctionAdaptor(meta timeseries.Meta, store timeseries.Store,
	functions []*protoscommon.Function, params *Parameters) (FunctionAdaptor, error) {
	fa := &functionAdaptor{
		meta:      meta,
		store:     store,
		functions: functions,
		parameter: params,
		cascade:   cascade.NewCascadeFunctions(),
	}
	fa.labels = append(fa.labels, meta.GetChainIDField().Name)
	var seriesLabel []string
	lo.ForEach(meta.GetFieldsByRole(timeseries.FieldRoleSeriesLabel), func(f timeseries.Field, _ int) {
		seriesLabel = append(seriesLabel, f.Name)
	})
	fa.labels = append(fa.labels, seriesLabel...)
	if err := fa.convert(); err != nil {
		log.Errorf("convert function error: %v", err)
		return nil, err
	}
	return fa, nil
}

func (fa *functionAdaptor) SeriesLabel() []string {
	return fa.labels
}

func (fa *functionAdaptor) convertDurationValue(v *protoscommon.Duration) time.Duration {
	if v == nil {
		panic(fmt.Errorf("duration value is nil"))
	}
	switch v.Unit {
	case "s":
		return time.Duration(v.Value) * time.Second
	case "m":
		return time.Duration(v.Value) * time.Minute
	case "h":
		return time.Duration(v.Value) * time.Hour
	case "d":
		return time.Duration(v.Value) * time.Hour * 24
	case "w":
		return time.Duration(v.Value) * time.Hour * 24 * 7
	default:
		return time.Duration(v.Value) * time.Second
	}
}

func (fa *functionAdaptor) verifyArguments(arguments []*protoscommon.Argument, idx int) error {
	if len(arguments) <= idx {
		return fmt.Errorf("missing argument at index %d", idx)
	}
	return nil
}

func (fa *functionAdaptor) convert() (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			log.Errorf("panic in convert: %v\n%s", r, stack)
			err = fmt.Errorf("panic in convert: %v\n%s", r, stack)
		}
	}()

	var (
		withFill                   = false
		extendPrevious, extendNext time.Duration
	)
	if fa.parameter != nil && fa.parameter.timeRange != nil {
		fa.extendTimeRange = fa.parameter.timeRange.Copy()
	}
	for _, f := range fa.functions {
		var pf prebuilt.Function
		switch f.Name {
		case "abs":
			pf = math.NewMathFunction(fa.meta, fa.store).Math().WithOp(prebuilt.OperatorAbs)
		case "ceil":
			pf = math.NewMathFunction(fa.meta, fa.store).Math().WithOp(prebuilt.OperatorCeil)
		case "floor":
			pf = math.NewMathFunction(fa.meta, fa.store).Math().WithOp(prebuilt.OperatorFloor)
		case "round":
			pf = math.NewMathFunction(fa.meta, fa.store).Math().WithOp(prebuilt.OperatorRound)
		case "log2":
			pf = math.NewMathFunction(fa.meta, fa.store).Math().WithOp(prebuilt.OperatorLog2)
		case "log10":
			pf = math.NewMathFunction(fa.meta, fa.store).Math().WithOp(prebuilt.OperatorLog10)
		case "ln":
			pf = math.NewMathFunction(fa.meta, fa.store).Math().WithOp(prebuilt.OperatorLn)
		case "rollup_avg":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendNext == 0 || extendNext < d {
				extendNext = d
			}
			pf = slidingwindow.NewRollupSlidingWindowFunction(fa.meta, fa.store).
				RollupWindowSize(d).AggregatedWindowSize(d).WithOp(prebuilt.OperatorAvg)
		case "rollup_sum":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendNext == 0 || extendNext < d {
				extendNext = d
			}
			pf = slidingwindow.NewRollupSlidingWindowFunction(fa.meta, fa.store).
				RollupWindowSize(d).AggregatedWindowSize(d).WithOp(prebuilt.OperatorSum)
		case "rollup_min":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendNext == 0 || extendNext < d {
				extendNext = d
			}
			pf = slidingwindow.NewRollupSlidingWindowFunction(fa.meta, fa.store).
				RollupWindowSize(d).AggregatedWindowSize(d).WithOp(prebuilt.OperatorMin)
		case "rollup_max":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendNext == 0 || extendNext < d {
				extendNext = d
			}
			pf = slidingwindow.NewRollupSlidingWindowFunction(fa.meta, fa.store).
				RollupWindowSize(d).AggregatedWindowSize(d).WithOp(prebuilt.OperatorMax)
		case "rollup_count":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendNext == 0 || extendNext < d {
				extendNext = d
			}
			pf = slidingwindow.NewRollupSlidingWindowFunction(fa.meta, fa.store).
				RollupWindowSize(d).AggregatedWindowSize(d).WithOp(prebuilt.OperatorCount)
		case "rollup_first":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendNext == 0 || extendNext < d {
				extendNext = d
			}
			pf = slidingwindow.NewRollupSlidingWindowFunction(fa.meta, fa.store).
				RollupWindowSize(d).AggregatedWindowSize(d).WithOp(prebuilt.OperatorFirst)
		case "rollup_last":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendNext == 0 || extendNext < d {
				extendNext = d
			}
			pf = slidingwindow.NewRollupSlidingWindowFunction(fa.meta, fa.store).
				RollupWindowSize(d).AggregatedWindowSize(d).WithOp(prebuilt.OperatorLast)
		case "rollup_delta":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendNext == 0 || extendNext < d {
				extendNext = d
			}
			pf = slidingwindow.NewRollupSlidingWindowFunction(fa.meta, fa.store).
				RollupWindowSize(d).AggregatedWindowSize(d).WithOp(prebuilt.OperatorDelta)
		case "avg_over_time":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendPrevious == 0 || extendPrevious < d {
				extendPrevious = d
			}
			pf = slidingwindow.NewAggregatedSlidingWindowFunction(fa.meta, fa.store).
				AggregatedWindowSize(d).
				WithOp(prebuilt.OperatorAvg)
		case "sum_over_time":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendPrevious == 0 || extendPrevious < d {
				extendPrevious = d
			}
			pf = slidingwindow.NewAggregatedSlidingWindowFunction(fa.meta, fa.store).
				AggregatedWindowSize(d).
				WithOp(prebuilt.OperatorSum)
		case "min_over_time":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendPrevious == 0 || extendPrevious < d {
				extendPrevious = d
			}
			pf = slidingwindow.NewAggregatedSlidingWindowFunction(fa.meta, fa.store).
				AggregatedWindowSize(d).
				WithOp(prebuilt.OperatorMin)
		case "max_over_time":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendPrevious == 0 || extendPrevious < d {
				extendPrevious = d
			}
			pf = slidingwindow.NewAggregatedSlidingWindowFunction(fa.meta, fa.store).
				AggregatedWindowSize(d).
				WithOp(prebuilt.OperatorMax)
		case "count_over_time":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendPrevious == 0 || extendPrevious < d {
				extendPrevious = d
			}
			pf = slidingwindow.NewAggregatedSlidingWindowFunction(fa.meta, fa.store).
				AggregatedWindowSize(d).
				WithOp(prebuilt.OperatorCount)
		case "delta_over_time":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendPrevious == 0 || extendPrevious < d {
				extendPrevious = d
			}
			pf = slidingwindow.NewAggregatedSlidingWindowFunction(fa.meta, fa.store).
				AggregatedWindowSize(d).
				WithOp(prebuilt.OperatorDelta)
		case "first_over_time":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendPrevious == 0 || extendPrevious < d {
				extendPrevious = d
			}
			pf = slidingwindow.NewAggregatedSlidingWindowFunction(fa.meta, fa.store).
				AggregatedWindowSize(d).
				WithOp(prebuilt.OperatorFirst)
		case "last_over_time":
			withFill = true
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			d := fa.convertDurationValue(f.Arguments[0].GetDurationValue())
			if extendPrevious == 0 || extendPrevious < d {
				extendPrevious = d
			}
			pf = slidingwindow.NewAggregatedSlidingWindowFunction(fa.meta, fa.store).
				AggregatedWindowSize(d).
				WithOp(prebuilt.OperatorLast)
		case "topk":
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			pf = rank.NewRankFunction(fa.meta, fa.store).Rank(int(f.Arguments[0].GetIntValue())).
				WithOp(prebuilt.OperatorTopK)
		case "bottomk":
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			pf = rank.NewRankFunction(fa.meta, fa.store).Rank(int(f.Arguments[0].GetIntValue())).
				WithOp(prebuilt.OperatorBottomK)
		case "timestamp":
			pf = prebuilttime.NewTimeFunction(fa.meta, fa.store).Time().WithOp(prebuilt.OperatorTimestamp)
		case "day_of_year":
			pf = prebuilttime.NewTimeFunction(fa.meta, fa.store).Time().WithOp(prebuilt.OperatorDayOfYear)
		case "day_of_month":
			pf = prebuilttime.NewTimeFunction(fa.meta, fa.store).Time().WithOp(prebuilt.OperatorDayOfMonth)
		case "day_of_week":
			pf = prebuilttime.NewTimeFunction(fa.meta, fa.store).Time().WithOp(prebuilt.OperatorDayOfWeek)
		case "year":
			pf = prebuilttime.NewTimeFunction(fa.meta, fa.store).Time().WithOp(prebuilt.OperatorYear)
		case "month":
			pf = prebuilttime.NewTimeFunction(fa.meta, fa.store).Time().WithOp(prebuilt.OperatorMonth)
		case "hour":
			pf = prebuilttime.NewTimeFunction(fa.meta, fa.store).Time().WithOp(prebuilt.OperatorHour)
		case "minute":
			pf = prebuilttime.NewTimeFunction(fa.meta, fa.store).Time().WithOp(prebuilt.OperatorMinute)
		case "rate":
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			pf = rate.NewRateFunction(fa.meta, fa.store).Rate(fa.convertDurationValue(f.Arguments[0].GetDurationValue())).WithOp(prebuilt.OperatorRate)
		case "irate":
			if fa.verifyArguments(f.Arguments, 0) != nil {
				return fmt.Errorf("missing argument at index 0")
			}
			pf = rate.NewRateFunction(fa.meta, fa.store).Rate(fa.convertDurationValue(f.Arguments[0].GetDurationValue())).WithOp(prebuilt.OperatorIRate)
		default:
			return fmt.Errorf("unknown function: %s", f.Name)
		}

		pf = pf.WithLabels(fa.labels).WithSelector(fa.parameter.labelSelector)
		fa.prebuilt = append(fa.prebuilt, pf)
	}

	var filterFunction prebuilt.Function
	if withFill {
		filterFunction = filter.NewWithFillFilterFunction(fa.meta, fa.store).Filter().WithLabels(fa.labels).WithSelector(fa.parameter.labelSelector)
	} else {
		filterFunction = filter.NewFilterFunction(fa.meta, fa.store).Filter().WithLabels(fa.labels).WithSelector(fa.parameter.labelSelector)
	}
	fa.prebuilt = append([]prebuilt.Function{filterFunction}, fa.prebuilt...)

	if fa.extendTimeRange != nil {
		fa.extendTimeRange.Start = fa.extendTimeRange.Start.Add(-extendPrevious)
		fa.extendTimeRange.End = fa.extendTimeRange.End.Add(extendNext)
		for _, f := range fa.prebuilt {
			f.WithTimeRange(fa.extendTimeRange)
		}
		fa.prebuilt = append(fa.prebuilt, sample.NewSampleFunction(fa.meta, fa.store).
			Sample(fa.extendTimeRange.Step).WithTimeRange(fa.extendTimeRange).
			WithLabels(fa.labels).WithSelector(fa.parameter.labelSelector))
	}

	for _, f := range fa.prebuilt {
		fa.cascade.Add(f)
	}
	return nil
}

func (fa *functionAdaptor) Generate() (string, error) {
	return fa.cascade.Generate()
}

func (fa *functionAdaptor) Snippets() (map[string]string, error) {
	return fa.cascade.Snippets()
}

func (fa *functionAdaptor) Parameter() *Parameters {
	return fa.parameter
}

func (fa *functionAdaptor) TableAlias() string {
	return fa.cascade.LastTableAlias()
}

func (fa *functionAdaptor) ValueAlias() string {
	if len(fa.prebuilt) == 0 {
		return timeseries.MetricValueFieldName
	}
	return fa.cascade.LastValueAlias()
}
