package timerange

import "time"

type TimePoint struct {
	time.Time
	step time.Duration
	tz   *time.Location
}

func NewTimePoint(t time.Time,
	step time.Duration,
	tz *time.Location) *TimePoint {
	tp := &TimePoint{
		Time: t,
		step: step}
	if tz != nil {
		tp.tz = tz
	} else {
		tp.tz = time.UTC
	}
	return tp
}

func (tp *TimePoint) Next() *TimePoint {
	if tp.step < time.Hour*24 {
		return NewTimePoint(tp.Add(tp.step).In(tp.tz), tp.step, tp.tz)
	}
	switch tp.step {
	case time.Hour * 24:
		return NewTimePoint(tp.In(tp.tz).AddDate(0, 0, 1).In(tp.tz), tp.step, tp.tz)
	case time.Hour * 24 * 7:
		return NewTimePoint(tp.In(tp.tz).AddDate(0, 0, 7).In(tp.tz), tp.step, tp.tz)
	case time.Hour * 24 * 30:
		return NewTimePoint(tp.In(tp.tz).AddDate(0, 1, 0).In(tp.tz), tp.step, tp.tz)
	case time.Hour * 24 * 30 * 3:
		return NewTimePoint(tp.In(tp.tz).AddDate(0, 3, 0).In(tp.tz), tp.step, tp.tz)
	case time.Hour * 24 * 365:
		return NewTimePoint(tp.In(tp.tz).AddDate(1, 0, 0).In(tp.tz), tp.step, tp.tz)
	default:
		return NewTimePoint(tp.Add(tp.step).In(tp.tz), tp.step, tp.tz)
	}
}

func (tp *TimePoint) Pre() *TimePoint {
	if tp.step < time.Hour*24 {
		return NewTimePoint(tp.Add(-tp.step), tp.step, tp.tz)
	}
	switch tp.step {
	case time.Hour * 24:
		return NewTimePoint(tp.In(tp.tz).AddDate(0, 0, -1), tp.step, tp.tz)
	case time.Hour * 24 * 7:
		return NewTimePoint(tp.In(tp.tz).AddDate(0, 0, -7), tp.step, tp.tz)
	case time.Hour * 24 * 30:
		return NewTimePoint(tp.In(tp.tz).AddDate(0, -1, 0), tp.step, tp.tz)
	case time.Hour * 24 * 30 * 3:
		return NewTimePoint(tp.In(tp.tz).AddDate(0, -3, 0), tp.step, tp.tz)
	case time.Hour * 24 * 365:
		return NewTimePoint(tp.In(tp.tz).AddDate(-1, 0, 0), tp.step, tp.tz)
	default:
		return NewTimePoint(tp.Add(-tp.step).In(tp.tz), tp.step, tp.tz)
	}
}
