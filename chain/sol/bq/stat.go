package bq

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

// queryBytesLadder buckets the bytes billed per BigQuery query (the on-demand cost signal): from
// 10MiB up to 100GiB.
var queryBytesLadder = histogram.Ladder[int64]{
	10 << 20, 100 << 20, 1 << 30, 10 << 30, 50 << 30, 100 << 30,
}

// statistic records per-method query latency, result-count, and bytes-billed histograms keyed by
// request source, and provides the Snapshot the launcher tracks. It mirrors sol/ch.statistic, with
// an extra bytes-billed histogram for BigQuery cost visibility.
type statistic struct {
	mu sync.Mutex

	queryUsed       map[string]timehist.Histogram
	queryGot        map[string]histogram.Histogram
	queryBytes      map[string]histogram.Histogram // distribution of bytes billed per query
	queryTotalBytes map[string]int64               // running total of bytes billed (cumulative cost)

	// notifier, when set, is invoked once per recorded operation. Optional (nil = not reported).
	notifier Notifier
}

func (m *statistic) init() {
	m.queryUsed = make(map[string]timehist.Histogram)
	m.queryGot = make(map[string]histogram.Histogram)
	m.queryBytes = make(map[string]histogram.Histogram)
	m.queryTotalBytes = make(map[string]int64)
}

func (m *statistic) getSource(ctx context.Context) string {
	if ctxData := jsonrpc.GetCtxData(ctx); ctxData != nil {
		return ctxData.ReqSrc.Summary()
	}
	return ""
}

// record adds one observation for method: latency, returned element count, the distribution of bytes
// billed, and the cumulative total of bytes billed — all keyed by method+source. It also invokes the
// notifier (when set) so the launcher can emit external metrics with its own attributes.
func (m *statistic) record(ctx context.Context, method string, used time.Duration, count int, bytesBilled int64) {
	source := m.getSource(ctx)
	if m.notifier != nil {
		m.notifier(ctx, method, source, used, count, bytesBilled)
	}
	key := method + "/" + source
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryUsed[key] = m.queryUsed[key].Incr(used)
	m.queryGot[key] = queryGotLadder.Incr(m.queryGot[key], count)
	m.queryBytes[key] = queryBytesLadder.Incr(m.queryBytes[key], bytesBilled)
	m.queryTotalBytes[key] += bytesBilled
}

func (m *statistic) Snapshot() any {
	m.mu.Lock()
	defer m.mu.Unlock()
	totalBytes := make(map[string]int64, len(m.queryTotalBytes))
	for k, v := range m.queryTotalBytes {
		totalBytes[k] = v
	}
	return map[string]any{
		"used":        utils.MapMapNoError(m.queryUsed, timehist.Histogram.Snapshot),
		"count":       utils.MapMapNoError(m.queryUsed, timehist.Histogram.Sum),
		"got":         utils.MapMapNoError(m.queryGot, queryGotLadder.Snapshot),
		"bytesBilled": utils.MapMapNoError(m.queryBytes, queryBytesLadder.Snapshot),
		"totalBytes":  totalBytes,
	}
}
