package timehist

import (
	"sentioxyz/sentio-core/common/histogram"
	"time"
)

var timeLadder = histogram.Ladder[time.Duration]{
	time.Millisecond * 10,
	time.Millisecond * 100,
	time.Millisecond * 500,
	time.Second,
	time.Second * 5,
	time.Second * 10,
	time.Minute,
}

type Histogram [8]int

func (t Histogram) Incr(d time.Duration) Histogram {
	timeLadder.Incr(t[:], d)
	return t
}

func (t Histogram) Snapshot() any {
	return timeLadder.Snapshot(t[:])
}

func (t Histogram) Sum() (s int) {
	return timeLadder.Sum(t[:])
}

func (t Histogram) String() string {
	return timeLadder.ToString(t[:])
}

func (t Histogram) Add(a Histogram) Histogram {
	_ = timeLadder.Merge(t[:], t[:], a[:])
	return t
}
