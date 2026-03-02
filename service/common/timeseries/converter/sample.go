package converter

import (
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timerange"
	protoso11y "sentioxyz/sentio-core/service/observability/protos"
)

type Sample interface {
	Hash() string
	AddDataPoint(timestamp int64, value *float64)
	SetDataPoint(timestamp int64, value float64)
	WithFill()
	ToProto() *protos.Matrix_Sample
	ToO11yProto() *protoso11y.MetricsQueryResponse_Sample
	Weight() float64
	Slots() int64
}

type DataPoint struct {
	Timestamp int64
	Value     *float64
	Previous  *DataPoint
	Next      *DataPoint
}

func NewDataPoint(logger *log.SentioLogger, timestamp int64, value *float64, prev *DataPoint) *DataPoint {
	d := &DataPoint{
		Timestamp: timestamp,
		Value:     value,
		Previous:  prev,
		Next:      nil,
	}
	if prev != nil {
		if prev.Next != nil {
			logger.Debug("datapoint: previous datapoint has next datapoint, will drop and replace it")
		}
		prev.Next = d
	}
	return d
}

type sample struct {
	logger    *log.SentioLogger
	metric    Metric
	last      *DataPoint
	head      *DataPoint
	timeIndex map[int64]*DataPoint
	withFill  bool
	time      *Time
	slots     int64
}

func NewSample(logger *log.SentioLogger, name, displayName string, labels map[string]string,
	time *Time, withFill bool) Sample {
	metric := NewMetric(name, displayName, NewLabel(labels))
	sample := &sample{
		logger:    logger,
		metric:    metric,
		timeIndex: make(map[int64]*DataPoint),
		withFill:  withFill,
		time:      time,
	}
	if time != nil && time.TimeRange != nil {
		start := timerange.NewTimePoint(time.Start, time.Step, time.Timezone)
		end := timerange.NewTimePoint(time.End, time.Step, time.Timezone)
		for t := start; t.Before(end.Time) || t.Equal(end.Time); t = t.Next() {
			sample.AddDataPoint(t.Unix(), nil)
		}
	}
	return sample
}

func (s *sample) Hash() string {
	if s.metric == nil {
		return ""
	}
	return s.metric.Hash()
}

func (s *sample) AddDataPoint(timestamp int64, value *float64) {
	if dp, ok := s.timeIndex[timestamp]; ok {
		dp.Value = value
		return
	}
	s.timeIndex[timestamp] = NewDataPoint(s.logger, timestamp, value, s.last)
	s.last = s.timeIndex[timestamp]
	if s.head == nil {
		s.head = s.last
	}
}

func (s *sample) SetDataPoint(timestamp int64, value float64) {
	if dp, ok := s.timeIndex[timestamp]; ok {
		dp.Value = &value
		s.slots++
	} else {
		s.logger.Debugf("sample: datapoint with timestamp %d does not exist, will skip it", timestamp)
	}
}

func (s *sample) WithFill() {
	if s.head == nil || !s.withFill {
		return
	}

	cur := s.head
	for cur != nil {
		if cur.Value == nil && cur.Previous != nil {
			cur.Value = cur.Previous.Value
			s.slots++
		}
		cur = cur.Next
	}
}

func (s *sample) toProtoValues() []*protos.Matrix_Value {
	if s.head == nil {
		return nil
	}
	var values []*protos.Matrix_Value
	cur := s.head
	for cur != nil {
		// use source time range to filter out datapoints
		if cur.Timestamp >= s.time.reqStartTime.Unix() && cur.Timestamp <= s.time.reqEndTime.Unix() {
			if cur.Value != nil {
				values = append(values, &protos.Matrix_Value{
					Timestamp: cur.Timestamp,
					Value:     *cur.Value,
				})
			} else {
				values = append(values, &protos.Matrix_Value{
					Timestamp: cur.Timestamp,
					Value:     0,
				})
			}
		}
		cur = cur.Next
	}
	return values
}

func (s *sample) toO11yProtoValues() []*protoso11y.MetricsQueryResponse_Value {
	if s.head == nil {
		return nil
	}
	var values []*protoso11y.MetricsQueryResponse_Value
	cur := s.head
	for cur != nil {
		// use source time range to filter out datapoints
		if cur.Timestamp >= s.time.reqStartTime.Unix() && cur.Timestamp <= s.time.reqEndTime.Unix() {
			if cur.Value != nil {
				values = append(values, &protoso11y.MetricsQueryResponse_Value{
					Timestamp: cur.Timestamp,
					Value:     *cur.Value,
				})
			} else {
				values = append(values, &protoso11y.MetricsQueryResponse_Value{
					Timestamp: cur.Timestamp,
					Value:     0,
				})
			}
		}
		cur = cur.Next
	}
	return values
}

func (s *sample) ToProto() *protos.Matrix_Sample {
	if s == nil {
		return nil
	}
	return &protos.Matrix_Sample{
		Metric: s.metric.ToProto(),
		Values: s.toProtoValues(),
	}
}

func (s *sample) ToO11yProto() *protoso11y.MetricsQueryResponse_Sample {
	if s == nil {
		return nil
	}
	return &protoso11y.MetricsQueryResponse_Sample{
		Metric: s.metric.ToO11yProto(),
		Values: s.toO11yProtoValues(),
	}
}

func (s *sample) checkNil() bool {
	return s == nil || s.head == nil
}

func (s *sample) Weight() float64 {
	if s.checkNil() {
		return 0
	}
	var weight float64 = 0
	cur := s.head
	for cur != nil {
		if cur.Value != nil {
			weight = *cur.Value
		} else {
			weight = 0
		}
		cur = cur.Next
	}
	return weight
}

func (s *sample) Slots() int64 {
	return s.slots
}
