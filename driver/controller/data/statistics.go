package data

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/queue"
	"sentioxyz/sentio-core/common/timehist"
	"sentioxyz/sentio-core/common/timewin"
	"sentioxyz/sentio-core/common/utils"
)

func getCaller() string {
	return fmt.Sprintf("%+v", errors.Errorf(""))
}

type call struct {
	Caller   string
	Method   string
	Params   string
	Err      error
	WaitUsed time.Duration
	Used     time.Duration
	EndAt    time.Time
}

func (c call) Snapshot() any {
	r := map[string]any{
		"endAt":    c.EndAt.String(),
		"waitUsed": c.WaitUsed.String(),
		"used":     c.Used.String(),
		"method":   c.Method,
		"caller":   c.Caller,
	}
	if len(c.Params) > 0 {
		r["params"] = c.Params
	}
	if c.Err != nil {
		r["err"] = c.Err.Error()
	}
	return r
}

type statWindow struct {
	StartAt       time.Time
	Count         map[string]int
	FailedCount   map[string]int
	TotalUsed     map[string]time.Duration
	TotalWaitUsed map[string]time.Duration
	Used          map[string]timehist.Histogram
	WaitUsed      map[string]timehist.Histogram
}

func newStatWindow(c call) *statWindow {
	w := &statWindow{
		StartAt:       c.EndAt,
		Count:         make(map[string]int),
		FailedCount:   make(map[string]int),
		TotalUsed:     make(map[string]time.Duration),
		TotalWaitUsed: make(map[string]time.Duration),
		Used:          make(map[string]timehist.Histogram),
		WaitUsed:      make(map[string]timehist.Histogram),
	}
	w.Count[c.Method] += 1
	if c.Err != nil {
		w.FailedCount[c.Method] += 1
	}
	w.TotalUsed[c.Method] += c.Used
	w.Used[c.Method] = w.Used[c.Method].Incr(c.Used)
	w.TotalWaitUsed[c.Method] += c.WaitUsed
	w.WaitUsed[c.Method] = w.WaitUsed[c.Method].Incr(c.WaitUsed)
	return w
}

func (w *statWindow) GetStartAt() time.Time {
	return w.StartAt
}

func (w *statWindow) Merge(a *statWindow) {
	for method, v := range a.Count {
		w.Count[method] += v
	}
	for method, v := range a.FailedCount {
		w.FailedCount[method] += v
	}
	for method, v := range a.TotalUsed {
		w.TotalUsed[method] += v
	}
	for method, v := range a.TotalWaitUsed {
		w.TotalWaitUsed[method] += v
	}
	for method, th := range a.Used {
		w.Used[method] = w.Used[method].Add(th)
	}
	for method, th := range a.WaitUsed {
		w.WaitUsed[method] = w.WaitUsed[method].Add(th)
	}
}

func (w *statWindow) Snapshot(endAt time.Time) any {
	return map[string]any{
		"startAt":   w.StartAt.String(),
		"endAt":     endAt.String(),
		"duration":  endAt.Sub(w.StartAt).String(),
		"count":     w.Count,
		"failed":    w.FailedCount,
		"totalUsed": utils.MapMapNoError(w.TotalUsed, time.Duration.String),
		"used":      utils.MapMapNoError(w.Used, timehist.Histogram.String),
		// waitUsed is the time spent waiting for a resource-manager concurrency token before
		// the actual RPC; (totalUsed - totalWaitUsed) is therefore the real on-the-wire latency.
		// Surfacing it lets us tell network-bound calls apart from token-contention-bound ones.
		"totalWaitUsed": utils.MapMapNoError(w.TotalWaitUsed, time.Duration.String),
		"waitUsed":      utils.MapMapNoError(w.WaitUsed, timehist.Histogram.String),
	}
}

type CallStatistics struct {
	latest queue.Circular[call]
	stat   *timewin.TimeWindowsManager[*statWindow]
}

var (
	clientKeepRecentRequestCount = envconf.LoadUInt64("SENTIO_CLIENT_KEEP_RECENT_REQUEST_COUNT",
		1000, envconf.WithMin(10))
	clientStatTimeWindowWidth = envconf.LoadDuration("SENTIO_CLIENT_STAT_TIME_WINDOW_WIDTH",
		time.Minute, envconf.WithMinDuration(time.Second*30))
)

func NewDefaultCallStatistics() *CallStatistics {
	return NewCallStatistics(int(clientKeepRecentRequestCount), clientStatTimeWindowWidth)
}

func NewCallStatistics(latestNum int, winWidth time.Duration) *CallStatistics {
	return &CallStatistics{
		latest: queue.NewSafeCircular[call](latestNum),
		stat:   timewin.NewTimeWindowsManager[*statWindow](winWidth),
	}
}

func (s *CallStatistics) Called(method string, args []any, err error, startAt, waitEndAt time.Time) {
	c := call{
		Caller: getCaller(),
		Method: method,
		Err:    err,
		EndAt:  time.Now(),
	}
	c.Used = c.EndAt.Sub(startAt)
	c.WaitUsed = waitEndAt.Sub(startAt)
	if len(args) > 0 {
		c.Params = utils.MustJSONMarshal(args)
	}
	s.latest.Push(c)
	s.stat.Append(newStatWindow(c))
}

func (s *CallStatistics) Snapshot() any {
	return map[string]any{
		"recent":     utils.MapSliceNoError(s.latest.Dump(true), call.Snapshot),
		"statistics": s.stat.Snapshot(),
	}
}
