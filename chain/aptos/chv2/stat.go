package chv2

import (
	"context"
	"sentioxyz/sentio-core/common/histogram"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/timehist"
	"sentioxyz/sentio-core/common/utils"
	"sync"
	"time"
)

var (
	queryTxGotLadder      = histogram.Ladder[int]{10, 30, 60, 100, 250, getTransactionsMaxReturn}
	queryChangesGotLadder = histogram.Ladder[int]{10, 100, 1000, 10000, 100000}
)

type statistic struct {
	mu sync.Mutex

	queryTxUsed     map[string]timehist.Histogram
	queryTxGot      map[string]histogram.Histogram
	queryTxOverSize map[string]int

	queryChangeStatUsed map[string]timehist.Histogram

	queryChangesUsed map[string]timehist.Histogram
	queryChangesGot  map[string]histogram.Histogram
}

func (m *statistic) init() {
	m.queryTxUsed = make(map[string]timehist.Histogram)
	m.queryTxGot = make(map[string]histogram.Histogram)
	m.queryTxOverSize = make(map[string]int)
	m.queryChangeStatUsed = make(map[string]timehist.Histogram)
	m.queryChangesUsed = make(map[string]timehist.Histogram)
	m.queryChangesGot = make(map[string]histogram.Histogram)
}

func (m *statistic) getSource(ctx context.Context) string {
	if ctxData := jsonrpc.GetCtxData(ctx); ctxData != nil {
		return ctxData.ReqSrc.Summary()
	}
	return ""
}

func (m *statistic) recordQueryTx(ctx context.Context, used time.Duration, txLen int) {
	src := m.getSource(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryTxUsed[src] = m.queryTxUsed[src].Incr(used)
	m.queryTxGot[src] = queryTxGotLadder.Incr(m.queryTxGot[src], txLen)
	if txLen >= getTransactionsMaxReturn {
		m.queryTxOverSize[src] += 1
	}
}

func (m *statistic) recordQueryChangeStat(ctx context.Context, used time.Duration) {
	src := m.getSource(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryChangeStatUsed[src] = m.queryChangeStatUsed[src].Incr(used)
}

func (m *statistic) recordQueryChanges(ctx context.Context, used time.Duration, chLen int) {
	src := m.getSource(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryChangesUsed[src] = m.queryChangesUsed[src].Incr(used)
	m.queryChangesGot[src] = queryChangesGotLadder.Incr(m.queryChangesGot[src], chLen)
}

func (m *statistic) Snapshot() any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{
		"queryTx": map[string]any{
			"used":     utils.MapMapNoError(m.queryTxUsed, timehist.Histogram.String),
			"count":    utils.MapMapNoError(m.queryTxUsed, timehist.Histogram.Sum),
			"got":      utils.MapMapNoError(m.queryTxGot, queryTxGotLadder.Snapshot),
			"overSize": m.queryTxOverSize,
		},
		"queryChangeStat": map[string]any{
			"used":  utils.MapMapNoError(m.queryChangeStatUsed, timehist.Histogram.String),
			"count": utils.MapMapNoError(m.queryChangeStatUsed, timehist.Histogram.Sum),
		},
		"queryChanges": map[string]any{
			"used":  utils.MapMapNoError(m.queryChangesUsed, timehist.Histogram.String),
			"count": utils.MapMapNoError(m.queryChangesUsed, timehist.Histogram.Sum),
			"got":   utils.MapMapNoError(m.queryChangesGot, queryChangesGotLadder.Snapshot),
		},
	}
}
