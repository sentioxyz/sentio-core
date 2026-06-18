package controller

import (
	"time"

	"sentioxyz/sentio-core/common/timewin"
	"sentioxyz/sentio-core/common/utils"
)

type taskStatWindow struct {
	startAt            time.Time
	taskCount          map[string]int
	taskUsed           map[string]time.Duration
	fetchWaitUsed      time.Duration
	taskPreWaitUsed    time.Duration
	taskPostWaitUsed   time.Duration
	makeCheckpointUsed time.Duration
}

func (w *taskStatWindow) GetStartAt() time.Time {
	return w.startAt
}

func (w *taskStatWindow) Merge(a *taskStatWindow) {
	for handlerID, v := range a.taskCount {
		w.taskCount[handlerID] += v
	}
	for handlerID, v := range a.taskUsed {
		w.taskUsed[handlerID] += v
	}
	w.fetchWaitUsed += a.fetchWaitUsed
	w.taskPreWaitUsed += a.taskPreWaitUsed
	w.taskPostWaitUsed += a.taskPostWaitUsed
	w.makeCheckpointUsed += a.makeCheckpointUsed
}

func (w *taskStatWindow) Snapshot(endAt time.Time) any {
	taskStat := make(map[string]map[string]any)
	for hid, count := range w.taskCount {
		used := w.taskUsed[hid]
		taskStat[hid] = map[string]any{
			"count":     count,
			"avgUsed":   (w.taskUsed[hid] / time.Duration(count)).String(),
			"totalUsed": used.String(),
		}
	}
	return map[string]any{
		"startAt":       w.startAt.String(),
		"endAt":         endAt.String(),
		"duration":      endAt.Sub(w.startAt).String(),
		"task":          taskStat,
		"taskTotalUsed": utils.SumMap(w.taskUsed).String(),
		// fetchWaitUsed is the time the single producer goroutine blocked inside blockBuilder.Next()
		// waiting for the data fetcher. It is NOT covered by taskPreWaitUsed (which only starts after
		// Next() returns), so without it a fetch-bound pipeline looks idle on the consumer side.
		"fetchWaitUsed":      w.fetchWaitUsed.String(),
		"taskPreWaitUsed":    w.taskPreWaitUsed.String(),
		"taskPostWaitUsed":   w.taskPostWaitUsed.String(),
		"makeCheckpointUsed": w.makeCheckpointUsed.String(),
	}
}

type analyser struct {
	*timewin.TimeWindowsManager[*taskStatWindow]
}

func newAnalyser() analyser {
	return analyser{
		TimeWindowsManager: timewin.NewTimeWindowsManager[*taskStatWindow](time.Minute),
	}
}

func (a *analyser) fetchWait(used time.Duration) {
	a.Append(&taskStatWindow{
		startAt:       time.Now(),
		taskCount:     make(map[string]int),
		taskUsed:      make(map[string]time.Duration),
		fetchWaitUsed: used,
	})
}

func (a *analyser) taskSent(preWait time.Duration) {
	a.Append(&taskStatWindow{
		startAt:            time.Now(),
		taskCount:          make(map[string]int),
		taskUsed:           make(map[string]time.Duration),
		taskPreWaitUsed:    preWait,
		taskPostWaitUsed:   0,
		makeCheckpointUsed: 0,
	})
}

func (a *analyser) taskComplete(handlerID string, used, postWait time.Duration) {
	a.Append(&taskStatWindow{
		startAt:            time.Now(),
		taskCount:          map[string]int{handlerID: 1},
		taskUsed:           map[string]time.Duration{handlerID: used},
		taskPreWaitUsed:    0,
		taskPostWaitUsed:   postWait,
		makeCheckpointUsed: 0,
	})
}

func (a *analyser) makeCheckpoint(used time.Duration) {
	a.Append(&taskStatWindow{
		startAt:            time.Now(),
		taskCount:          make(map[string]int),
		taskUsed:           make(map[string]time.Duration),
		taskPreWaitUsed:    0,
		taskPostWaitUsed:   0,
		makeCheckpointUsed: used,
	})
}

type checkpointStatWindow struct {
	startAt              time.Time
	checkOverQuotaUsed   time.Duration
	commitTimeSeriesUsed time.Duration
	commitEntityUsed     time.Duration
	commitWebhookUsed    time.Duration
	saveUsageUsed        time.Duration
	saveCheckpointUsed   time.Duration
	totalBinding         uint64
	failedCount          int
	count                int
}

func (w *checkpointStatWindow) GetStartAt() time.Time {
	return w.startAt
}

func (w *checkpointStatWindow) Merge(a *checkpointStatWindow) {
	w.checkOverQuotaUsed += a.checkOverQuotaUsed
	w.commitTimeSeriesUsed += a.commitTimeSeriesUsed
	w.commitEntityUsed += a.commitEntityUsed
	w.commitWebhookUsed += a.commitWebhookUsed
	w.saveUsageUsed += a.saveUsageUsed
	w.saveCheckpointUsed += a.saveCheckpointUsed
	w.totalBinding += a.totalBinding
	w.failedCount += a.failedCount
	w.count += a.count
}

func (w *checkpointStatWindow) Snapshot(endAt time.Time) any {
	dur := endAt.Sub(w.startAt)
	total := w.checkOverQuotaUsed +
		w.commitTimeSeriesUsed +
		w.commitEntityUsed +
		w.commitWebhookUsed +
		w.saveUsageUsed +
		w.saveCheckpointUsed
	return map[string]any{
		"startAt":              w.startAt.String(),
		"endAt":                endAt.String(),
		"duration":             dur.String(),
		"checkOverQuotaUsed":   w.checkOverQuotaUsed.String(),
		"commitTimeSeriesUsed": w.commitTimeSeriesUsed.String(),
		"commitEntityUsed":     w.commitEntityUsed.String(),
		"commitWebhookUsed":    w.commitWebhookUsed.String(),
		"saveUsageUsed":        w.saveUsageUsed.String(),
		"saveCheckpointUsed":   w.saveCheckpointUsed.String(),
		"totalBinding":         w.totalBinding,
		"failedCount":          w.failedCount,
		"count":                w.count,
		"pressure":             float64(total) / float64(dur),
	}
}
