package bq

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

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

	queryUsed  map[string]timehist.Histogram
	queryGot   map[string]histogram.Histogram
	queryBytes map[string]histogram.Histogram

	// costCounter, when set, accumulates total BigQuery bytes billed (the on-demand cost driver),
	// labelled by method, as an OpenTelemetry counter. Optional (nil = not reported).
	costCounter metric.Int64Counter
}

func (m *statistic) init() {
	m.queryUsed = make(map[string]timehist.Histogram)
	m.queryGot = make(map[string]histogram.Histogram)
	m.queryBytes = make(map[string]histogram.Histogram)
}

func (m *statistic) getSource(ctx context.Context) string {
	if ctxData := jsonrpc.GetCtxData(ctx); ctxData != nil {
		return ctxData.ReqSrc.Summary()
	}
	return ""
}

// record adds one observation for method: latency, returned element count, and total bytes billed
// across all BigQuery jobs the method ran. It also accumulates bytes billed into the OpenTelemetry
// cost counter (when configured), labelled by method.
func (m *statistic) record(ctx context.Context, method string, used time.Duration, count int, bytesBilled int64) {
	if m.costCounter != nil && bytesBilled > 0 {
		m.costCounter.Add(ctx, bytesBilled, metric.WithAttributes(attribute.String("method", method)))
	}
	key := method + "/" + m.getSource(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryUsed[key] = m.queryUsed[key].Incr(used)
	m.queryGot[key] = queryGotLadder.Incr(m.queryGot[key], count)
	m.queryBytes[key] = queryBytesLadder.Incr(m.queryBytes[key], bytesBilled)
}

func (m *statistic) Snapshot() any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{
		"used":        utils.MapMapNoError(m.queryUsed, timehist.Histogram.Snapshot),
		"count":       utils.MapMapNoError(m.queryUsed, timehist.Histogram.Sum),
		"got":         utils.MapMapNoError(m.queryGot, queryGotLadder.Snapshot),
		"bytesBilled": utils.MapMapNoError(m.queryBytes, queryBytesLadder.Snapshot),
	}
}
