package timerange

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sentioxyz/sentio-core/service/common/protos"

	"github.com/pkg/errors"
)

func ResolveTimeLike(t *protos.TimeRange_TimeLike, asBegin bool, tz *time.Location) (ret time.Time, err error) {
	if t == nil {
		err = errors.Errorf("time is nil")
		return
	}
	switch t.TimeLike.(type) {
	case *protos.TimeRange_TimeLike_AbsoluteTime:
		ret = time.Unix(t.GetAbsoluteTime(), 0).UTC()
	case *protos.TimeRange_TimeLike_RelativeTime:
		unit := t.GetRelativeTime().GetUnit()
		value := t.GetRelativeTime().GetValue()
		align := t.GetRelativeTime().GetAlign()
		s := "now"
		if value != 0 && unit != "" {
			s = fmt.Sprintf("%d%s", value, shortUnit(unit))
			if value > 0 {
				s = "now+" + s
			} else {
				s = "now" + s
			}
		}
		if align != "" {
			s += "/" + shortUnit(align)
		}
		return ResolveTimeStrWithAlign(s, asBegin, tz)
	}
	return
}

func shortUnit(unit string) string {
	if unit == "month" || unit == "months" {
		return "M"
	}
	return unit[:1]
}

func ResolveTimeStr(s string) (ret time.Time, err error) {
	var re = regexp.MustCompile(`(?s)^(\d+|now)?\s*(([+-])\s*((\d+)([dMmshwy])))?$`)
	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return time.Time{}, fmt.Errorf("invalid time string %s", s)
	}
	m := matches[0]
	var t time.Time
	switch m[1] {
	case "":
		t = time.Now().UTC()
	case "now":
		t = time.Now().UTC()
	default:
		value, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid time string %s", s)
		}
		if value > 1e12 {
			t = time.UnixMilli(int64(value)).UTC()
		} else {
			t = time.Unix(int64(value), 0).UTC()
		}
	}
	if m[2] == "" {
		return t, nil
	}
	sign := 1
	if m[3] == "-" {
		sign = -1
	}
	unit := m[6]
	value, err := strconv.ParseUint(m[5], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time string %s", s)
	}
	switch unit {
	case "M":
		return t.AddDate(0, int(value)*sign, 0), nil
	case "y":
		return t.AddDate(int(value)*sign, 0, 0), nil
	case "w":
		return t.AddDate(0, 0, int(value)*sign*7), nil
	case "d":
		return t.AddDate(0, 0, int(value)*sign), nil
	default:
		if u, ok := unitMap[unit]; ok {
			return t.Add(time.Duration(value) * u * time.Duration(sign)), nil
		}
		return time.Time{}, fmt.Errorf("invalid time string %s", s)
	}
}

func resolveTimeStrWithAlign(s string, align string, tz *time.Location) (ret time.Time, err error) {
	var re = regexp.MustCompile(`(?s)^(\d+|now)?\s*(([+-])\s*((\d+)([dMmshwy])))?$`)
	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return time.Time{}, fmt.Errorf("invalid time string %s", s)
	}
	m := matches[0]
	// get base time
	var t time.Time
	switch m[1] {
	case "", "now":
		t = time.Now().UTC()
	default:
		value, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid time string %s", s)
		}
		if value > 1e12 {
			t = time.UnixMilli(int64(value)).UTC()
		} else {
			t = time.Unix(int64(value), 0).UTC()
		}
	}
	t = t.In(tz)
	// align
	switch align {
	case "y":
		t = time.Date(t.Year(), 1, 1, 0, 0, 0, 0, tz)
	case "M":
		t = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, tz)
	case "w":
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tz).AddDate(0, 0, -int(t.Weekday()))
	case "d":
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tz)
	case "h":
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, tz)
	}
	// return if no offset
	if m[2] == "" {
		return t, nil
	}
	// add offset
	sign := 1
	if m[3] == "-" {
		sign = -1
	}
	unit := m[6]
	value, err := strconv.ParseUint(m[5], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time string %s", s)
	}
	switch unit {
	case "M":
		return t.AddDate(0, int(value)*sign, 0), nil
	case "y":
		return t.AddDate(int(value)*sign, 0, 0), nil
	case "w":
		return t.AddDate(0, 0, int(value)*sign*7), nil
	case "d":
		return t.AddDate(0, 0, int(value)*sign), nil
	default:
		if u, ok := unitMap[unit]; ok {
			return t.Add(time.Duration(value) * u * time.Duration(sign)), nil
		}
		return time.Time{}, fmt.Errorf("invalid time string %s", s)
	}
}

func ResolveTimeStrWithAlign(s string, asBegin bool, tz *time.Location) (time.Time, error) {
	var parts = strings.Split(s, "/")
	if len(parts) == 1 {
		return ResolveTimeStr(parts[0])
	}
	t, err := resolveTimeStrWithAlign(parts[0], parts[1], tz)
	if err != nil {
		return t, err
	}
	if asBegin {
		return t, nil
	}
	switch parts[1] {
	case "":
		return t, nil
	case "d":
		t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 1e+9-1, tz)
	case "M":
		t = time.Date(t.Year(), t.Month(), 1, 23, 59, 59, 1e+9-1, tz).AddDate(0, 1, -1)
	case "h":
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 59, 59, 1e+9-1, tz)
	case "w":
		t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 1e+9-1, tz).AddDate(0, 0, 6-int(t.Weekday()))
	case "y":
		t = time.Date(t.Year(), 12, 31, 23, 59, 59, 1e+9-1, tz)
	default:
		return time.Time{}, fmt.Errorf("invalid time string %s", s)
	}
	if tz != nil {
		if t.After(time.Now().In(tz)) {
			t = time.Now().In(tz)
		}
	} else {
		if t.After(time.Now().UTC()) {
			t = time.Now().UTC()
		}
	}
	return t, nil
}

var unitMap = map[string]time.Duration{
	"s": time.Second,
	"m": time.Minute,
	"h": time.Hour,
}

func AlignStartTime(t time.Time, step time.Duration, tz *time.Location) time.Time {
	_, offset := t.In(tz).Zone()
	offsetNano := float64((time.Duration(offset) * time.Second).Nanoseconds())
	stepNano := float64(step.Nanoseconds())
	return time.Unix(0, int64(math.Floor((float64(t.UnixNano())+offsetNano)/stepNano)*stepNano-offsetNano)).UTC()
}

func AlignEndTime(t time.Time, step time.Duration, tz *time.Location) time.Time {
	_, offset := t.In(tz).Zone()
	offsetNano := float64((time.Duration(offset) * time.Second).Nanoseconds())
	stepNano := float64(step.Nanoseconds())
	return time.Unix(0, int64(math.Floor((float64(t.UnixNano())+offsetNano)/stepNano)*stepNano-offsetNano)).UTC()
}

func GetMetricsMinStep(start time.Time, end time.Time) time.Duration {
	const MaxPointsAllowed = 10000

	diff := end.Sub(start)
	minStep := diff / MaxPointsAllowed
	if minStep < 10*time.Second {
		minStep = 10 * time.Second
	}
	return minStep
}

func GetEventMinStep(start, end time.Time) (minStep time.Duration) {
	if end.Sub(start) > 24*time.Hour*7*2 {
		// two weeks
		minStep = 1 * time.Hour
	} else {
		minStep = 1 * time.Minute
	}
	return
}

func TimezoneOffsetStr(tz *time.Location) string {
	if tz == nil {
		return "+00:00"
	}
	_, offset := time.Now().In(tz).Zone()
	hours := offset / 3600
	minutes := (offset % 3600) / 60
	return fmt.Sprintf("%+03d:%02d", hours, minutes)
}

// TimezoneOffset return the offset based on now()
// do not use this function anymore!
func TimezoneOffset(tz *time.Location) time.Duration {
	if tz == nil {
		return 0
	}
	_, offset := time.Now().In(tz).Zone()
	return time.Duration(offset) * time.Second
}

func TimezoneHistoryOffset(tz *time.Location, t time.Time) time.Duration {
	if tz == nil {
		return 0
	}
	_, offset := t.In(tz).Zone()
	return time.Duration(offset) * time.Second
}

func ParseTimeDuration(duration *protos.Duration) time.Duration {
	switch duration.GetUnit() {
	case "day":
		return time.Duration(duration.GetValue()) * time.Hour * 24
	case "week":
		return time.Duration(duration.GetValue()) * time.Hour * 24 * 7
	case "month":
		return time.Duration(duration.GetValue()) * time.Hour * 24 * 30
	default:
		return 0
	}
}
