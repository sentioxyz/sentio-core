package period

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Period only one of all attributes will take effect, and they are arranged in order of priority from high to low
type Period struct {
	natualMonth uint64
	seconds     uint64
}

const (
	minute = 60
	hour   = minute * 60
	day    = hour * 24
	week   = day * 7
)

var (
	Zero   = Period{}
	Second = Period{seconds: 1}
	Minute = Period{seconds: minute}
	Hour   = Period{seconds: hour}
	Day    = Period{seconds: day}
	Week   = Period{seconds: week}
	Month  = Period{natualMonth: 1}
	Year   = Period{natualMonth: 12}

	units = []struct {
		Period
		suffix string
	}{
		{Period: Year, suffix: "year"},
		{Period: Month, suffix: "month"},
		{Period: Week, suffix: "w"},
		{Period: Day, suffix: "d"},
		{Period: Hour, suffix: "h"},
		{Period: Minute, suffix: "m"},
		{Period: Second, suffix: "s"},
	}

	ErrPeriodInvalid = errors.New("invalid period")
)

func Parse(orig string) (Period, error) {
	for _, unit := range units {
		if strings.HasSuffix(orig, unit.suffix) {
			k, err := strconv.ParseUint(strings.TrimSuffix(orig, unit.suffix), 10, 64)
			if err != nil {
				return Zero, fmt.Errorf("%w: %s", ErrPeriodInvalid, err.Error())
			}
			p := unit.Multi(k)
			return p, nil
		}
	}
	return Zero, fmt.Errorf("%w: %q without a valid suffix", ErrPeriodInvalid, orig)
}

func MustParse(orig string) Period {
	p, err := Parse(orig)
	if err != nil {
		panic(err)
	}
	return p
}

func ParseFromPGPolicyInterval(orig string) (p Period, err error) {
	// format will be: {hour}:{minute}:{second}|{year} year[s]* {month} mon[s]* {day} day[s]*
	if orig == "" {
		return Zero, nil
	}
	plus := func(num string, secondUnit, monthUnit uint64) error {
		x, parseErr := strconv.ParseUint(num, 10, 64)
		if parseErr != nil {
			return fmt.Errorf("%q format error: %w", orig, parseErr)
		}
		p.seconds += x * secondUnit
		p.natualMonth += x * monthUnit
		return nil
	}
	if strings.Contains(orig, ":") {
		sections := strings.Split(orig, ":")
		if len(sections) != 3 {
			return Zero, fmt.Errorf("%q format error, should be '{hour}:{minute}:{second}'", orig)
		}
		if err = plus(sections[0], hour, 0); err != nil {
			return Zero, err
		}
		if err = plus(sections[1], minute, 0); err != nil {
			return Zero, err
		}
		if err = plus(sections[2], 1, 0); err != nil {
			return Zero, err
		}
	} else {
		sections := strings.Split(orig, " ")
		if len(sections)%2 != 0 {
			return Zero, fmt.Errorf("%q format error, should be '{year} year[s] {month} mon[s] {day} day[s]'", orig)
		}
		for i := 0; i < len(sections); i += 2 {
			switch sections[i+1][:3] {
			case "yea":
				err = plus(sections[i], 0, 12)
			case "mon":
				err = plus(sections[i], 0, 1)
			case "day":
				err = plus(sections[i], day, 0)
			}
			if err != nil {
				return Zero, err
			}
		}
	}
	if p.natualMonth > 0 && p.seconds > 0 {
		return Zero, fmt.Errorf("%w: natualMonth and seconds can only has one", ErrPeriodInvalid)
	}
	return p, nil
}

func (p Period) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

func (p *Period) UnmarshalJSON(b []byte) error {
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return err
	}
	if v, err := Parse(str); err != nil {
		return err
	} else {
		*p = v
		return nil
	}
}

func (p Period) Multi(x uint64) Period {
	return Period{natualMonth: p.natualMonth * x, seconds: p.seconds * x}
}

func (p Period) IsZero() bool {
	return p.natualMonth == 0 && p.seconds == 0
}

func (p Period) LT(a Period) bool {
	// a natual month will be treated as 30 days
	return p.natualMonth*day*30+p.seconds < a.natualMonth*day*30+a.seconds
}

// Div return p / a, p % a == 0
// x month / x day will return 0 and true
func (p Period) Div(a Period) (uint64, bool) {
	if a.IsZero() {
		panic("div zero")
	}
	if p.IsZero() {
		return 0, true
	}
	if a == p {
		return 1, true
	}
	if p.natualMonth > 0 && a.natualMonth > 0 {
		return p.natualMonth / a.natualMonth, p.natualMonth%a.natualMonth == 0
	}
	if p.seconds > 0 && a.seconds > 0 {
		return p.seconds / a.seconds, p.seconds%a.seconds == 0
	}
	if p.seconds > 0 && a.natualMonth > 0 {
		return 0, false
	}
	// p.natualMonth > 0 and a.seconds > 0
	return 0, a.seconds <= day && day%a.seconds == 0
}

func (p Period) String() string {
	if p.IsZero() {
		return "0s"
	}
	for _, unit := range units {
		if k, can := p.Div(unit.Period); can {
			return fmt.Sprintf("%d%s", k, unit.suffix)
		}
	}
	panic("unreachable")
}

func (p Period) TimeBucket(t time.Time) time.Time {
	if p.natualMonth > 0 {
		year, month, _ := t.UTC().Date()
		before := (year-2000)*12 + int(month) - 1
		after := before / int(p.natualMonth) * int(p.natualMonth)
		if before < after {
			after -= int(p.natualMonth)
		}
		newYear := after/12 + 2000
		newMonth := after%12 + 1
		return time.Date(newYear, time.Month(newMonth), 1, 0, 0, 0, 0, time.UTC)
	}
	base := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	before := t.Unix() - base.Unix()
	after := before / int64(p.seconds) * int64(p.seconds)
	if before < after {
		after -= int64(p.seconds)
	}
	return base.Add(time.Second * time.Duration(after))
}

func (p Period) TimeBucketField() string {
	return "time_bucket_" + p.String()
}

func (p Period) PGInterval() string {
	if p.natualMonth > 0 {
		return fmt.Sprintf("INTERVAL '%d month'", p.natualMonth)
	}
	if p.seconds%week == 0 {
		return fmt.Sprintf("INTERVAL '%d week'", p.seconds/week)
	}
	if p.seconds%day == 0 {
		return fmt.Sprintf("INTERVAL '%d day'", p.seconds/day)
	}
	if p.seconds%hour == 0 {
		return fmt.Sprintf("INTERVAL '%d hour'", p.seconds/hour)
	}
	return fmt.Sprintf("INTERVAL '%d seconds'", p.seconds)
}
