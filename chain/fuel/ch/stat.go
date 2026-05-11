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

var queryTxGotLadder = histogram.Ladder[int]{100, 300, 1000, 3000, 10000, 30000, 100000}

type statistic struct {
	mu sync.Mutex

	queryTxUsed map[string]timehist.Histogram
	queryTxGot  map[string]histogram.Histogram

	queryContractStartUsed map[string]timehist.Histogram
}

func (m *statistic) init() {
	m.queryTxUsed = make(map[string]timehist.Histogram)
	m.queryTxGot = make(map[string]histogram.Histogram)
	m.queryContractStartUsed = make(map[string]timehist.Histogram)
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
	m.queryTxUsed[src] = m.queryTxUsed[src].Incr(used)
	m.queryTxGot[src] = queryTxGotLadder.Incr(m.queryTxGot[src], count)
}

func (m *statistic) recordQueryContractStart(ctx context.Context, used time.Duration) {
	src := m.getSource(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryContractStartUsed[src] = m.queryContractStartUsed[src].Incr(used)
}

func (m *statistic) Snapshot() any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{
		"queryTx": map[string]any{
			"used":  utils.MapMapNoError(m.queryTxUsed, timehist.Histogram.Snapshot),
			"count": utils.MapMapNoError(m.queryTxUsed, timehist.Histogram.Sum),
			"got":   utils.MapMapNoError(m.queryTxGot, queryTxGotLadder.Snapshot),
		},
		"queryContractStart": map[string]any{
			"used":  utils.MapMapNoError(m.queryContractStartUsed, timehist.Histogram.Snapshot),
			"count": utils.MapMapNoError(m.queryContractStartUsed, timehist.Histogram.Sum),
		},
	}
}
