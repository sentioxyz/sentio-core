package ch

import (
	"context"
	"sentioxyz/sentio-core/common/histogram"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/timehist"
	"sentioxyz/sentio-core/common/utils"
	"sync"
	"time"
)

var queryGotLadder = histogram.Ladder[int]{100, 300, 1000, 3000, 10000, 30000, 100000}

type queryStat struct {
	used map[string]timehist.Histogram
	got  map[string]histogram.Histogram
}

func newQueryStat() queryStat {
	return queryStat{
		used: make(map[string]timehist.Histogram),
		got:  make(map[string]histogram.Histogram),
	}
}

func (q queryStat) Snapshot() any {
	return map[string]any{
		"used": utils.MapMapNoError(q.used, timehist.Histogram.Snapshot),
		"got":  utils.MapMapNoError(q.got, queryGotLadder.Snapshot),
	}
}

type statistic struct {
	mu sync.Mutex

	ready bool

	queryLog   queryStat
	queryTrace queryStat
}

func (m *statistic) init() {
	if m.ready {
		return
	}
	m.queryLog = newQueryStat()
	m.queryTrace = newQueryStat()
	m.ready = true
}

func (m *statistic) getSource(ctx context.Context) string {
	if ctxData := jsonrpc.GetCtxData(ctx); ctxData != nil {
		return ctxData.ReqSrc.Summary()
	}
	return ""
}

func (m *statistic) recordQueryLog(ctx context.Context, used time.Duration, count int) {
	src := m.getSource(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	m.queryLog.used[src] = m.queryLog.used[src].Incr(used)
	m.queryLog.got[src] = queryGotLadder.Incr(m.queryLog.got[src], count)
}

func (m *statistic) recordQueryTrace(ctx context.Context, used time.Duration, count int) {
	src := m.getSource(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	m.queryTrace.used[src] = m.queryTrace.used[src].Incr(used)
	m.queryTrace.got[src] = queryGotLadder.Incr(m.queryTrace.got[src], count)
}

func (m *statistic) Snapshot() any {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	return map[string]any{
		"queryLog":   m.queryLog.Snapshot(),
		"queryTrace": m.queryTrace.Snapshot(),
	}
}
