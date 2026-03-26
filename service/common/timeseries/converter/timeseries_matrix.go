package converter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/log"
	anyutil "sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timerange"
	adaptor "sentioxyz/sentio-core/service/common/timeseries/adaptor_eventlogs"
	"sentioxyz/sentio-core/service/common/timeseries/matrix"
	protoso11y "sentioxyz/sentio-core/service/observability/protos"

	"github.com/samber/lo"
)

type TimeSeriesMatrix interface {
	AddData(timestamp int64, value float64, labels map[string]string)
	ToProto() *protos.Matrix
	ToO11yProto() *protoso11y.MetricsQueryResponse_Matrix
	Slots() int64
	Apply(matrix matrix.Matrix, withFill bool)

	GetTime() *Time
	GetSamples() map[string]Sample
	GetName() string
}

type timeSeriesMatrix struct {
	segmentationQuery *protos.SegmentationQuery
	rangeQuery        *protos.Query
	timeRange         *timerange.TimeRange
	logger            *log.SentioLogger

	name        string
	displayName string
	samples     map[string]Sample
	time        *Time
	withFill    bool
}

var _ TimeSeriesMatrix = (*timeSeriesMatrix)(nil)

type Query struct {
	*protos.SegmentationQuery
	*protos.Query
}

// NewTimeSeriesMatrix creates a new TimeSeriesMatrix instance
func NewTimeSeriesMatrix(ctx context.Context,
	query Query, timeRange *timerange.TimeRange) TimeSeriesMatrix {
	_, logger := log.FromContext(ctx, "function", "newTimeSeriesMatrix")
	tsm := &timeSeriesMatrix{
		segmentationQuery: query.SegmentationQuery,
		rangeQuery:        query.Query,
		timeRange:         timeRange,
		logger:            logger,
		samples:           make(map[string]Sample),
	}
	tsm.init()
	return tsm
}

func NewTimeSeriesMatrixWithName(ctx context.Context, name string, timeRange *timerange.TimeRange) TimeSeriesMatrix {
	_, logger := log.FromContext(ctx, "function", "newTimeSeriesMatrixWithName", "name", name)
	tsm := &timeSeriesMatrix{
		segmentationQuery: nil,
		rangeQuery:        nil,
		timeRange:         timeRange,
		logger:            logger,
		name:              name,
		samples:           make(map[string]Sample),
	}
	tsm.init()
	return tsm
}

func (m *timeSeriesMatrix) init() {
	m.nameOf()
	m.isWithFill()
}

func (m *timeSeriesMatrix) AddData(timestamp int64, value float64, labels map[string]string) {
	hash := labelHash(labels)
	sample, ok := m.samples[hash]
	if ok {
		sample.SetDataPoint(timestamp, value)
		return
	}
	sample = NewSample(m.logger, m.name, m.displayName, labels, m.time, m.withFill)
	m.samples[hash] = sample
	sample.SetDataPoint(timestamp, value)
}

func (m *timeSeriesMatrix) ToProto() *protos.Matrix {
	var samples []*protos.Matrix_Sample
	for _, s := range m.samples {
		s.WithFill()
		samples = append(samples, s.ToProto())
	}
	return &protos.Matrix{
		Samples:      samples,
		TotalSamples: int32(len(m.samples)),
	}
}

func (m *timeSeriesMatrix) ToO11yProto() *protoso11y.MetricsQueryResponse_Matrix {
	var samples []*protoso11y.MetricsQueryResponse_Sample
	for _, s := range m.samples {
		s.WithFill()
		samples = append(samples, s.ToO11yProto())
	}
	return &protoso11y.MetricsQueryResponse_Matrix{
		Samples:      samples,
		TotalSamples: int32(len(m.samples)),
	}
}

func (m *timeSeriesMatrix) Slots() int64 {
	var slots int64 = 0
	for _, s := range m.samples {
		slots += s.Slots()
	}
	return slots
}

func (m *timeSeriesMatrix) nameOfSegmentationQuery() (name string) {
	var query = m.segmentationQuery
	if resource := query.GetResource(); resource != nil {
		switch resource.GetType() {
		case protos.SegmentationQuery_EVENTS:
			if eventName := resource.GetName(); eventName != "" {
				name = eventName + " - "
			} else if eventNames := resource.GetMultipleNames(); len(eventNames) > 0 {
				name = "<" + strings.Join(eventNames, ",") + "> - "
			} else {
				name = "<All Events> - "
			}
			switch query.GetAggregation().Value.(type) {
			case *protos.SegmentationQuery_Aggregation_Total_:
				name += "Total Count"
			case *protos.SegmentationQuery_Aggregation_Unique_:
				name += "Unique Count"
			case *protos.SegmentationQuery_Aggregation_CountUnique_:
				d := timerange.ParseTimeDuration(query.GetAggregation().GetCountUnique().GetDuration())
				switch d {
				case 0:
					name += "AAU"
				case time.Hour * 24:
					name += "DAU"
				case time.Hour * 24 * 7:
					name += "WAU"
				case time.Hour * 24 * 30:
					name += "MAU"
				}
			case *protos.SegmentationQuery_Aggregation_AggregateProperties_:
				var propertyName = query.GetAggregation().GetAggregateProperties().GetPropertyName()
				switch query.GetAggregation().GetAggregateProperties().GetType() {
				case protos.SegmentationQuery_Aggregation_AggregateProperties_SUM:
					name += "(Sum of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_AVG:
					name += "(Average of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_MIN:
					name += "(Minimum of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_MAX:
					name += "(Maximum of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_DISTINCT_COUNT:
					name += "(Distinct count of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_FIRST:
					name += "(First of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_LAST:
					name += "(Last of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_SUM:
					name += "(Cumulative sum of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_DISTINCT_COUNT:
					name += "(Cumulative distinct Count of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_FIRST:
					name += "(Cumulative first of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_LAST:
					name += "(Cumulative last of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_COUNT:
					name += "(Cumulative count of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_PERCENTILE_25TH:
					name += "(25th percentile of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_MEDIAN:
					name += "(Median of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_PERCENTILE_75TH:
					name += "(75th percentile of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_PERCENTILE_90TH:
					name += "(90th percentile of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_PERCENTILE_95TH:
					name += "(95th percentile of " + propertyName + ")"
				case protos.SegmentationQuery_Aggregation_AggregateProperties_PERCENTILE_99TH:
					name += "(99th percentile of " + propertyName + ")"
				default:
					name += "(Aggregate of " + propertyName + ")"
				}
			}
		case protos.SegmentationQuery_COHORTS:
			switch resource.GetCohortsValue().(type) {
			case *protos.SegmentationQuery_Resource_CohortsId:
				// TODO need read name from db
				name = "Cohorts<" + resource.GetCohortsId() + "> Segmentation"
			case *protos.SegmentationQuery_Resource_CohortsQuery:
				name = resource.GetCohortsQuery().GetName()
			}
		}
	} else {
		name = "<Nil Resource> - "
	}
	return name
}

func (m *timeSeriesMatrix) nameOfRangeQuery() string {
	var (
		query          = m.rangeQuery
		prefix, suffix string
	)
	switch {
	case query.Aggregate != nil:
		var op string
		switch query.Aggregate.Op {
		case protos.Aggregate_AVG:
			op = "avg"
		case protos.Aggregate_SUM:
			op = "sum"
		case protos.Aggregate_MIN:
			op = "min"
		case protos.Aggregate_MAX:
			op = "max"
		case protos.Aggregate_COUNT:
			op = "count"
		}
		if len(query.Aggregate.Grouping) > 0 {
			prefix = op + " by (" + strings.Join(query.Aggregate.Grouping, ",") + ") ("
			suffix = ")"
		} else {
			prefix = op + " ("
			suffix = ")"
		}
	}

	if len(query.GetFunctions()) == 0 {
		return prefix + query.GetQuery() + suffix
	}

	var funcExpression = query.GetQuery()
	for idx := len(query.GetFunctions()) - 1; idx >= 0; idx-- {
		funcExpression = query.GetFunctions()[idx].GetName() + "(" + funcExpression
		var args []string
		for _, arg := range query.GetFunctions()[idx].GetArguments() {
			switch arg.ArgumentValue.(type) {
			case *protos.Argument_BoolValue:
				args = append(args, lo.If(arg.GetBoolValue(), "True").Else("False"))
			case *protos.Argument_StringValue:
				args = append(args, "\""+arg.GetStringValue()+"\"")
			case *protos.Argument_IntValue:
				args = append(args, fmt.Sprintf("%d", arg.GetIntValue()))
			case *protos.Argument_DoubleValue:
				args = append(args, fmt.Sprintf("%.2f", arg.GetDoubleValue()))
			case *protos.Argument_DurationValue:
				args = append(args, fmt.Sprintf("%d%s", int64(arg.GetDurationValue().GetValue()), arg.GetDurationValue().GetUnit()))
			}
		}
		if len(args) > 0 {
			funcExpression += ", " + strings.Join(args, ", ") + ")"
		} else {
			funcExpression += ")"
		}
	}
	return prefix + funcExpression + suffix
}

func (m *timeSeriesMatrix) nameOf() {
	var (
		name        string
		displayName string
	)
	switch {
	case m.rangeQuery != nil:
		displayName = m.nameOfRangeQuery()
		name = m.rangeQuery.Query
	case m.segmentationQuery != nil:
		name = m.nameOfSegmentationQuery()
		displayName = name
	default:
		if m.name != "" {
			name = m.name
			displayName = m.name
		} else {
			name = "<Nil>"
		}
	}
	m.name = name
	m.displayName = displayName
	m.logger = m.logger.With("name", name, "display_name", displayName)
}

func (m *timeSeriesMatrix) isWithFill() {
	switch {
	case m.rangeQuery != nil:
		m.withFill = true
	case m.segmentationQuery == nil:
		m.withFill = false
	default:
		switch m.segmentationQuery.GetAggregation().Value.(type) {
		case *protos.SegmentationQuery_Aggregation_AggregateProperties_:
			m.withFill = adaptor.IsCumulativeAggregationOp(m.segmentationQuery.GetAggregation().GetAggregateProperties().GetType())
		case *protos.SegmentationQuery_Aggregation_CountUnique_:
			if m.segmentationQuery.GetAggregation().GetCountUnique().GetDuration().GetValue() == 0 {
				m.withFill = true
			}
		}
	}
	m.logger = m.logger.With("cumulative", m.withFill)
}

func (m *timeSeriesMatrix) processTimeRange(matrix matrix.Matrix, withFill bool) {
	t := newTime(m.timeRange, matrix)
	for idx := 0; idx < matrix.Len(); idx++ {
		timeSlot := matrix.TimeSeriesTimeValue(idx)
		if timeSlot.IsZero() {
			continue
		}
		if timeSlot.Before(t.Start) || t.Start.IsZero() {
			t.Start = timeSlot
		}
		if timeSlot.After(t.End) || t.End.IsZero() {
			t.End = timeSlot
		}
	}

	previous := timerange.NewTimePoint(t.Start, t.Step, t.Timezone)
	next := timerange.NewTimePoint(t.End, t.Step, t.Timezone)
	if withFill {
		for {
			if previous.After(t.reqStartTime) || previous.Equal(t.reqStartTime) {
				previous = previous.Pre()
			} else {
				break
			}
		}
		t.Start = previous.Time

		for {
			if next.Before(t.reqEndTime) || next.Equal(t.reqEndTime) {
				next = next.Next()
			} else {
				break
			}
		}
		t.End = next.Time
	} else {
		if !previous.Pre().Before(t.reqStartTime) {
			t.Start = previous.Pre().Time
		}
		if !next.Next().After(t.reqEndTime) {
			t.End = next.Next().Time
		}
	}
	m.time = t
	m.logger = m.logger.With("time", m.time.String())
}

func (m *timeSeriesMatrix) Apply(matrix matrix.Matrix, withFill bool) {
	m.processTimeRange(matrix, withFill)

	for idx := 0; idx < matrix.Len(); idx++ {
		timeSlot := matrix.TimeSeriesTimeValue(idx)
		if timeSlot.IsZero() {
			m.logger.Warnw("can not found time slot",
				"data", matrix.DataByRow(idx),
				"columns", matrix.ColumnTypes())
			continue
		}
		value, err := anyutil.Any2Float(matrix.TimeSeriesAggValue(idx))
		if err != nil {
			m.logger.Warnw("can not found value",
				"err", err.Error(),
				"data", matrix.DataByRow(idx),
				"columns", matrix.ColumnTypes())
			continue
		}
		var labels = make(map[string]string)
		for k, v := range matrix.TimeSeriesLabelsValue(idx) {
			labels[k] = anyutil.Any2String(v)
		}

		m.AddData(timeSlot.Unix(), value, labels)
	}
}

func (m *timeSeriesMatrix) GetTime() *Time {
	return m.time
}

func (m *timeSeriesMatrix) GetSamples() map[string]Sample {
	return m.samples
}

func (m *timeSeriesMatrix) GetName() string {
	return m.name
}
