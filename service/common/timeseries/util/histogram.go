package util

import (
	"fmt"
	"time"
)

const (
	normalDateTruncTpl   = "dateTrunc('%s', %s, '%s')"
	wrappedTpl           = "toDateTime64(%s, 6, '%s')"
	format               = "%F %T"
	toStartOfIntervalTpl = "toDateTime64(formatDateTime(toStartOfInterval(%s, toIntervalSecond(%d)), '%s', 'UTC'), 6, '%s')"
)

var (
	HistogramTimeUnitMap = map[time.Duration]string{
		time.Second:             "second",
		time.Minute:             "minute",
		time.Hour:               "hour",
		time.Hour * 24:          "day",
		time.Hour * 24 * 7:      "week",
		time.Hour * 24 * 30:     "month",
		time.Hour * 24 * 30 * 3: "quarter",
		time.Hour * 24 * 365:    "year",
	}

	HistogramFunction = func(d time.Duration, f, tz string) string {
		if unit, ok := HistogramTimeUnitMap[d]; ok {
			field := fmt.Sprintf(normalDateTruncTpl, unit, f, tz)
			if d > time.Hour*24 {
				field = fmt.Sprintf(wrappedTpl, field, tz)
			}
			return field
		}
		return fmt.Sprintf(toStartOfIntervalTpl, f, int(d.Seconds()), format, tz)
	}

	HistogramCeilFunction = func(d time.Duration, f, tz string) string {
		original := HistogramFunction(d, f, tz)
		needCeil := fmt.Sprintf("(%s != %s)", original, f)
		return HistogramFunction(d, f+"+if("+needCeil+",interval "+fmt.Sprintf("%d", int64(d.Seconds()))+" second,interval 0 second)", tz)
	}
)
