package ch

import (
	"context"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/histogram"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/timehist"
	"sentioxyz/sentio-core/common/utils"
)

var queryGotLadder = histogram.Ladder[int]{100, 300, 1000, 3000, 10000, 30000, 100000}

// statistic records per-method query latency and result-count histograms keyed by request source,
// and provides the Snapshot the launcher tracks.
type statistic struct {
	mu sync.Mutex

	queryUsed map[string]timehist.Histogram
	queryGot  map[string]histogram.Histogram
}

func (m *statistic) init() {
	m.queryUsed = make(map[string]timehist.Histogram)
	m.queryGot = make(map[string]histogram.Histogram)
}

func (m *statistic) getSource(ctx context.Context) string {
	if ctxData := jsonrpc.GetCtxData(ctx); ctxData != nil {
		return ctxData.ReqSrc.Summary()
	}
	return ""
}

func (m *statistic) record(ctx context.Context, method string, used time.Duration, count int) {
	key := method + "/" + m.getSource(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryUsed[key] = m.queryUsed[key].Incr(used)
	m.queryGot[key] = queryGotLadder.Incr(m.queryGot[key], count)
}

func (m *statistic) Snapshot() any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{
		"used":  utils.MapMapNoError(m.queryUsed, timehist.Histogram.Snapshot),
		"count": utils.MapMapNoError(m.queryUsed, timehist.Histogram.Sum),
		"got":   utils.MapMapNoError(m.queryGot, queryGotLadder.Snapshot),
	}
}
