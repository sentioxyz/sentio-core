package clientpool

import (
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type downgradeStatWindow struct {
	startAt          time.Time
	priorityDuration map[uint32]time.Duration
	lastPriority     uint32
	lastPriorityFrom time.Time
}

func newDowngradeStatWindow(curPriority uint32) *downgradeStatWindow {
	now := time.Now()
	return &downgradeStatWindow{
		startAt:          now,
		lastPriority:     curPriority,
		lastPriorityFrom: now,
	}
}

func (w *downgradeStatWindow) GetStartAt() time.Time {
	return w.startAt
}

func (w *downgradeStatWindow) Merge(a *downgradeStatWindow) {
	if len(a.priorityDuration) == 0 && a.lastPriority == w.lastPriority {
		return
	}
	if w.priorityDuration == nil {
		w.priorityDuration = make(map[uint32]time.Duration)
	}
	for p, d := range a.priorityDuration {
		w.priorityDuration[p] += d
	}
	w.priorityDuration[w.lastPriority] += a.startAt.Sub(w.lastPriorityFrom)
	w.lastPriority, w.lastPriorityFrom = a.lastPriority, a.lastPriorityFrom
}

func (w *downgradeStatWindow) Snapshot(endAt time.Time) any {
	pd := utils.CopyMap(w.priorityDuration)
	pd[w.lastPriority] += endAt.Sub(w.lastPriorityFrom)
	return map[string]any{
		"startAt":          w.startAt.String(),
		"endAt":            endAt.String(),
		"duration":         endAt.Sub(w.startAt).String(),
		"priorityDuration": utils.MapMapNoError(pd, time.Duration.String),
	}
}
