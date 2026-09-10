package persistent

import (
	"time"

	"sentioxyz/sentio-core/common/timehist"
	"sentioxyz/sentio-core/common/utils"
)

type entityTimeStat struct {
	get       map[string]timehist.Histogram // map[from]
	list      map[string]timehist.Histogram // map[from]
	set       map[string]timehist.Histogram // map[mode]
	getTotal  map[string]time.Duration      // map[from]
	listTotal map[string]time.Duration      // map[from]
	setTotal  map[string]time.Duration      // map[mode]
}

func (s entityTimeStat) Merge(a entityTimeStat) (r entityTimeStat) {
	r.get = utils.CopyMap(s.get)
	for from, hist := range a.get {
		r.get[from] = r.get[from].Add(hist)
	}
	r.list = utils.CopyMap(s.list)
	for from, hist := range a.list {
		r.list[from] = r.list[from].Add(hist)
	}
	r.set = utils.CopyMap(s.set)
	for mode, hist := range a.set {
		r.set[mode] = r.set[mode].Add(hist)
	}
	r.getTotal = utils.MapAdd(s.getTotal, a.getTotal)
	r.listTotal = utils.MapAdd(s.listTotal, a.listTotal)
	r.setTotal = utils.MapAdd(s.setTotal, a.setTotal)
	return r
}

func (s entityTimeStat) Snapshot() any {
	themeSnapshot := func(hist map[string]timehist.Histogram, total map[string]time.Duration) map[string]any {
		sn := map[string]any{}
		for k := range hist {
			ic, it := hist[k], total[k]
			count := ic.Sum()
			var avg time.Duration
			if count > 0 {
				avg = it / time.Duration(count)
			}
			sn[k] = map[string]any{
				"count": count,
				"dist":  ic.String(),
				"total": it.String(),
				"avg":   avg.String(),
			}
		}
		return sn
	}
	return map[string]any{
		"get":  themeSnapshot(s.get, s.getTotal),
		"list": themeSnapshot(s.list, s.listTotal),
		"set":  themeSnapshot(s.set, s.setTotal),
	}
}

// storeReadStat counts the store reads withStoreVersions makes for the get / list / commit paths:
// the ones without the controller lock (the normal case) and the ones under it (the fallback after
// maxStoreVersionRounds rounds of concurrent writes or commits), so that a snapshot shows how often
// a store round trip still holds the lock.
type storeReadStat struct {
	offLock           timehist.Histogram // one prefetchStoreVersions call without the lock
	offLockTotal      time.Duration
	offLockVersions   int                // versions read by those calls
	underLock         timehist.Histogram // one prefetchStoreVersions call under the lock
	underLockTotal    time.Duration
	underLockVersions int
	stale             int // rounds whose versions were dropped because a commit landed meanwhile
}

func (s storeReadStat) Merge(a storeReadStat) storeReadStat {
	return storeReadStat{
		offLock:           s.offLock.Add(a.offLock),
		offLockTotal:      s.offLockTotal + a.offLockTotal,
		offLockVersions:   s.offLockVersions + a.offLockVersions,
		underLock:         s.underLock.Add(a.underLock),
		underLockTotal:    s.underLockTotal + a.underLockTotal,
		underLockVersions: s.underLockVersions + a.underLockVersions,
		stale:             s.stale + a.stale,
	}
}

func (s storeReadStat) Snapshot() any {
	reads := func(hist timehist.Histogram, total time.Duration, versions int) map[string]any {
		count := hist.Sum()
		var avg time.Duration
		if count > 0 {
			avg = total / time.Duration(count)
		}
		return map[string]any{
			"calls":    count,
			"versions": versions,
			"dist":     hist.String(),
			"total":    total.String(),
			"avg":      avg.String(),
		}
	}
	calls := s.offLock.Sum() + s.underLock.Sum()
	var underLockShare float64
	if calls > 0 {
		underLockShare = float64(s.underLock.Sum()) / float64(calls)
	}
	return map[string]any{
		"offLock":        reads(s.offLock, s.offLockTotal, s.offLockVersions),
		"underLock":      reads(s.underLock, s.underLockTotal, s.underLockVersions),
		"underLockShare": underLockShare,
		"staleRounds":    s.stale,
	}
}

type timeStatWindow struct {
	startAt    time.Time
	reorg      timehist.Histogram
	commit     timehist.Histogram
	entityStat map[string]entityTimeStat
	// storeReads is what withStoreVersions did, and lockWait how long SetEntity waited for the
	// controller lock: together they show whether store round trips still stall the writers
	storeReads    storeReadStat
	lockWait      timehist.Histogram
	lockWaitTotal time.Duration
}

func (w *timeStatWindow) GetStartAt() time.Time {
	return w.startAt
}

func (w *timeStatWindow) Merge(a *timeStatWindow) {
	w.reorg = w.reorg.Add(a.reorg)
	w.commit = w.commit.Add(a.commit)
	if w.entityStat == nil {
		w.entityStat = make(map[string]entityTimeStat)
	}
	for entity, stat := range a.entityStat {
		w.entityStat[entity] = w.entityStat[entity].Merge(stat)
	}
	w.storeReads = w.storeReads.Merge(a.storeReads)
	w.lockWait = w.lockWait.Add(a.lockWait)
	w.lockWaitTotal += a.lockWaitTotal
}

func (w *timeStatWindow) Snapshot(endAt time.Time) any {
	var lockWaitAvg time.Duration
	if count := w.lockWait.Sum(); count > 0 {
		lockWaitAvg = w.lockWaitTotal / time.Duration(count)
	}
	return map[string]any{
		"startAt":    w.startAt.String(),
		"endAt":      endAt.String(),
		"duration":   endAt.Sub(w.startAt).String(),
		"reorg":      w.reorg.Snapshot(),
		"commit":     w.commit.Snapshot(),
		"entity":     utils.MapMapNoError(w.entityStat, entityTimeStat.Snapshot),
		"storeReads": w.storeReads.Snapshot(),
		"setLockWait": map[string]any{
			"count": w.lockWait.Sum(),
			"dist":  w.lockWait.String(),
			"total": w.lockWaitTotal.String(),
			"avg":   lockWaitAvg.String(),
		},
	}
}
