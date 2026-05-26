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

type timeStatWindow struct {
	startAt    time.Time
	reorg      timehist.Histogram
	commit     timehist.Histogram
	entityStat map[string]entityTimeStat
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
}

func (w *timeStatWindow) Snapshot(endAt time.Time) any {
	return map[string]any{
		"startAt":  w.startAt.String(),
		"endAt":    endAt.String(),
		"duration": endAt.Sub(w.startAt).String(),
		"reorg":    w.reorg.Snapshot(),
		"commit":   w.commit.Snapshot(),
		"entity":   utils.MapMapNoError(w.entityStat, entityTimeStat.Snapshot),
	}
}
