package converter

import (
	"fmt"
	"time"

	"sentioxyz/sentio-core/service/common/timerange"
)

type Time struct {
	*timerange.TimeRange
	reqStartTime time.Time
	reqEndTime   time.Time
}

func newTime(timeRange *timerange.TimeRange) *Time {
	return &Time{
		TimeRange: &timerange.TimeRange{
			Start:    time.Now().Add(time.Hour * 24 * 365),
			End:      time.Unix(0, 0),
			Step:     timeRange.Step,
			Timezone: timeRange.Timezone,
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
