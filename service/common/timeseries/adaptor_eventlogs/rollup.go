package adaptor_eventlogs

import (
	"fmt"
	"math"
	"time"
)

type RollupParams interface {
	ToDate() int
	ToSeconds() int
}

type rollupUnit int

const (
	RollupByDay rollupUnit = iota
	RollupByWeek
	RollupByMonth
)

const (
	day   = 24 * time.Hour
	week  = 7 * day
	month = 30 * day
)

type rollupOptions struct {
	Unit  rollupUnit
	Value int
}

func NewRollupParams(d time.Duration, unit string) (RollupParams, error) {
	switch unit {
	case "d":
		return &rollupOptions{
			Unit:  RollupByDay,
			Value: int(d / day),
		}, nil
	case "w":
		return &rollupOptions{
			Unit:  RollupByWeek,
			Value: int(d / week),
		}, nil
	case "M":
		return &rollupOptions{
			Unit:  RollupByMonth,
			Value: int(d / month),
		}, nil
	default:
		return nil, fmt.Errorf("unknown time unit %s", unit)
	}
}

func (r *rollupOptions) ToDate() int {
	switch r.Unit {
	case RollupByDay:
		return int(math.Max(0, float64(r.Value-1)))
	case RollupByWeek:
		return int(math.Max(0, float64(r.Value*7-1)))
	case RollupByMonth:
		return int(math.Max(0, float64(r.Value*30-1)))
	}
	return 0
}

func (r *rollupOptions) ToSeconds() int {
	switch r.Unit {
	case RollupByDay:
		return int(math.Max(0, float64(r.Value-1)*86400))
	case RollupByWeek:
		return int(math.Max(0, float64(r.Value*7-1)*86400))
	case RollupByMonth:
		return int(math.Max(0, float64(r.Value*30-1)*86400))
	}
	return 0
}
