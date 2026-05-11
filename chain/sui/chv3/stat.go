package chv3

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

	queryTx  queryStat
	queryObj queryStat
}

func (m *statistic) init() {
	m.queryTx = newQueryStat()
	m.queryObj = newQueryStat()
}

func (m *statistic) getSource(ctx context.Context) string {
	if ctxData := jsonrpc.GetCtxData(ctx); ctxData != nil {
		return ctxData.ReqSrc.Summary()
	}
	return ""
}
func (m *statistic) recordQueryTx(ctx context.Context, used time.Duration, count int) {
	src := m.getSource(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryTx.used[src] = m.queryTx.used[src].Incr(used)
	m.queryTx.got[src] = queryGotLadder.Incr(m.queryTx.got[src], count)
}

func (m *statistic) recordQueryObj(ctx context.Context, used time.Duration, count int) {
	src := m.getSource(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryObj.used[src] = m.queryObj.used[src].Incr(used)
	m.queryObj.got[src] = queryGotLadder.Incr(m.queryObj.got[src], count)
}

func (m *statistic) Snapshot() any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{
		"queryTx":  m.queryTx.Snapshot(),
		"queryObj": m.queryObj.Snapshot(),
	}
}
