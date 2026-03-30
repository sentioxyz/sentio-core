package converter

import (
	"fmt"
	"time"

	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/matrix"

	"github.com/samber/lo"
)

type Time struct {
	*timerange.TimeRange
	reqStartTime time.Time
	reqEndTime   time.Time
}

func newTime(timeRange *timerange.TimeRange, matrix matrix.Matrix) *Time {
	var (
		year  = time.Hour * 24 * 365
		start = lo.If(matrix != nil && matrix.Len() != 0, time.Now().Add(year*10)).Else(timeRange.Start.Add(timeRange.Step * -1))
		end   = lo.If(matrix != nil && matrix.Len() != 0, time.Unix(0, 0)).Else(timeRange.End.Add(timeRange.Step))
	)
	return &Time{
		TimeRange: &timerange.TimeRange{
			Start:      start,
			End:        end,
			Step:       timeRange.Step,
			Timezone:   timeRange.Timezone,
			SampleRate: timeRange.SampleRate,
		},
		reqStartTime: timeRange.Start,
		reqEndTime:   timeRange.End,
	}
}

func (t *Time) String() string {
	return fmt.Sprintf("[start:%s(%s),end:%s(%s),step:%s,timezone:%s]",
		t.Start.Format(time.RFC3339),
		t.reqStartTime.Format(time.RFC3339),
		t.End.Format(time.RFC3339),
		t.reqEndTime.Format(time.RFC3339),
		t.Step.String(),
		t.Timezone.String())
}
