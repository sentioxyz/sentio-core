package functions

import (
	"time"

	"sentioxyz/sentio-core/service/common/protos"
)

type Delta struct {
	duration time.Duration
}

func NewDeltaHandler(arguments []*protos.Argument) *Delta {
	if len(arguments) == 0 {
		return &Delta{duration: 0}
	}
	return &Delta{duration: parseArgument2Duration(arguments[0])}
}

func (d *Delta) Handle(matrix *protos.Matrix) (*protos.Matrix, error) {
	for _, sample := range matrix.Samples {
		tsValue := map[time.Time]float64{}
		var min, max *time.Time
		for _, v := range sample.Values {
			ts := time.Unix(v.Timestamp, 0).UTC()
			tsValue[ts] = v.Value
			if min == nil || ts.Before(*min) {
				min = &ts
			}
			if max == nil || ts.After(*max) {
				max = &ts
			}
		}
		var values []*protos.Matrix_Value
		for _, v := range sample.Values {
			deltaTimestamp := time.Unix(v.Timestamp, 0).UTC().Add(-d.duration)
			if deltaTimestamp.Before(*min) || deltaTimestamp.After(*max) {
				continue
			}
			if value, ok := tsValue[deltaTimestamp]; ok {
				values = append(values, &protos.Matrix_Value{
					Timestamp: v.Timestamp,
					Value:     v.Value - value,
				})
			} else {
				values = append(values, &protos.Matrix_Value{
					Timestamp: v.Timestamp,
					Value:     v.Value,
				})
			}
		}
		sample.Values = values
	}
	return matrix, nil
}

func (d *Delta) Category() string {
	return "aggregate"
}
