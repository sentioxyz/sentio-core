package period

import (
	"fmt"
	"time"
)

type Ticker interface {
	Next() time.Time
}

type ticker struct {
	period   Period
	offset   int64
	location *time.Location
}

func NewTicker(p Period, start time.Time) Ticker {
	tk := &ticker{period: p, location: start.Location()}
	if p.natualMonth == 0 {
		orig, _ := time.Parse(time.RFC3339, "2000-01-01T00:00:00Z")
		tk.offset = (start.Unix()-orig.Unix())/int64(p.seconds)*int64(p.seconds) + orig.Unix()
	} else {
		// origin: 2000-01-01
		tk.offset = int64(start.Year()-2000)*12 + int64(start.Month()) - 1
		tk.offset = tk.offset / int64(p.natualMonth) * int64(p.natualMonth)
	}
	return tk
}

func (t *ticker) Next() time.Time {
	if t.period.natualMonth == 0 {
		t.offset += int64(t.period.seconds)
		return time.Unix(t.offset, 0).In(t.location)
	} else {
		t.offset += int64(t.period.natualMonth)
		year, month := t.offset/12+2000, t.offset%12+1
		next, _ := time.Parse(time.RFC3339, fmt.Sprintf("%04d-%02d-01T00:00:00Z", year, month))
		return next.In(t.location)
	}
}
