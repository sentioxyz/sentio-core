package ex

import (
	"sentioxyz/sentio-core/common/timehist"
	"sentioxyz/sentio-core/common/timewin"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type statWindow struct {
	startAt time.Time
	hasErr  map[string]int
	used    map[string]timehist.Histogram
}

func newStatWin(key string, used time.Duration, hasErr bool) *statWindow {
	now := time.Now()
	w := &statWindow{
		startAt: now,
		hasErr:  make(map[string]int),
		used:    make(map[string]timehist.Histogram),
	}
	if hasErr {
		w.hasErr[key] = 1
	}
	w.used[key] = timehist.Histogram{}.Incr(used)
	return w
}

func (w *statWindow) GetStartAt() time.Time {
	return w.startAt
}

func (w *statWindow) Merge(a *statWindow) {
	for method, c := range a.hasErr {
		w.hasErr[method] += c
	}
	for method, hist := range a.used {
		w.used[method] = w.used[method].Add(hist)
	}
}

func (w *statWindow) Snapshot(endAt time.Time) any {
	return map[string]any{
		"startAt":  w.startAt.String(),
		"endAt":    endAt.String(),
		"duration": endAt.Sub(w.startAt).String(),
		"used":     utils.MapMapNoError(w.used, timehist.Histogram.String),
		"count":    utils.MapMapNoError(w.used, timehist.Histogram.Sum),
		"hasErr":   w.hasErr,
	}
}

type StatWinManager struct {
	m *timewin.TimeWindowsManager[*statWindow]
}

func NewStatWinManager(winLen time.Duration) *StatWinManager {
	return &StatWinManager{
		m: timewin.NewTimeWindowsManager[*statWindow](winLen),
	}
}

func (m *StatWinManager) Snapshot() any {
	return m.m.Snapshot()
}

func (m *StatWinManager) Record(key string, used time.Duration, hasErr bool) {
	m.m.Append(newStatWin(key, used, hasErr))
}
