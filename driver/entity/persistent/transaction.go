package persistent

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
)

// TxnReport holds statistics observed by a ReportMonitor during one commit
// cycle (i.e. between two successive Controller.Commit calls).
type TxnReport struct {
	TxnUsed       time.Duration
	TxnCommitUsed time.Duration

	// set-entity statistics
	TotalSet       int
	TotalSetNil    int
	TotalSetPartly int
	TotalSetUsed   time.Duration

	// commit statistics
	TotalCommit       map[string]int
	TotalCommitCreate map[string]int
	TotalCommitType   int

	// get-entity statistics
	TotalGet                int
	TotalGetInBlock         int
	TotalGetUsed            time.Duration
	TotalGetFrom            map[string]map[string]int
	TotalGetFromUsed        map[string]map[string]time.Duration

	// list-entity statistics
	TotalList               int
	TotalListForLoadRelated int
	TotalListUsed           time.Duration
	TotalListFrom           map[string]map[string]int
	TotalListFromUsed       map[string]map[string]time.Duration

	TotalCacheEvicted int
}

func newTxnReport() TxnReport {
	return TxnReport{
		TotalCommit:       make(map[string]int),
		TotalCommitCreate: make(map[string]int),
		TotalGetFrom:      make(map[string]map[string]int),
		TotalGetFromUsed:  make(map[string]map[string]time.Duration),
		TotalListFrom:     make(map[string]map[string]int),
		TotalListFromUsed: make(map[string]map[string]time.Duration),
	}
}

// ReportMonitor is a Monitor implementation that accumulates per-commit
// statistics in its Report field and logs a summary on each OnCommit call.
// It also delegates metric recording to an embedded MetricsMonitor.
//
// After OnCommit returns, Report is reset so the next commit cycle starts
// with a clean slate.  Callers may read Report at any point during a cycle
// to observe in-progress statistics.
type ReportMonitor struct {
	// Report holds the statistics accumulated since the last OnCommit (or since
	// construction).  It is reset automatically after each OnCommit.
	Report TxnReport

	start             time.Time
	storeCacheEvicted int
	store             ChainStore // used only for CacheEvicted()
	metrics           MetricsMonitor
}

// NewReportMonitor creates a ReportMonitor bound to the given store.
// usedMetric may be nil if latency recording is not required.
func NewReportMonitor(store ChainStore, usedMetric metric.Float64Histogram) *ReportMonitor {
	return &ReportMonitor{
		start:             time.Now(),
		storeCacheEvicted: store.CacheEvicted(),
		store:             store,
		metrics:           MetricsMonitor{UsedMetric: usedMetric},
		Report:            newTxnReport(),
	}
}

func (m *ReportMonitor) OnGet(
	ctx context.Context,
	entity string,
	id string,
	blockNumber uint64,
	inBlock bool,
	from string,
	used time.Duration,
) {
	m.Report.TotalGet++
	if inBlock {
		m.Report.TotalGetInBlock++
	}
	m.Report.TotalGetUsed += used
	utils.UpdateK2Map(m.Report.TotalGetFrom, from, entity,
		func(v int) int { return v + 1 })
	utils.UpdateK2Map(m.Report.TotalGetFromUsed, from, entity,
		func(v time.Duration) time.Duration { return v + used })
	m.metrics.OnGet(ctx, entity, id, blockNumber, inBlock, from, used)
}

func (m *ReportMonitor) OnList(
	ctx context.Context,
	entity string,
	blockNumber uint64,
	loadRelated bool,
	from string,
	resultLen int,
	resultPersistentLen int,
	used time.Duration,
) {
	m.Report.TotalList++
	if loadRelated {
		m.Report.TotalListForLoadRelated++
	}
	m.Report.TotalListUsed += used
	utils.UpdateK2Map(m.Report.TotalListFrom, from, entity,
		func(v int) int { return v + 1 })
	utils.UpdateK2Map(m.Report.TotalListFromUsed, from, entity,
		func(v time.Duration) time.Duration { return v + used })
	m.metrics.OnList(ctx, entity, blockNumber, loadRelated, from, resultLen, resultPersistentLen, used)
}

func (m *ReportMonitor) OnSet(
	ctx context.Context,
	entity string,
	id string,
	blockNumber uint64,
	remove bool,
	hasOperator bool,
	used time.Duration,
) {
	m.Report.TotalSet++
	m.Report.TotalSetUsed += used
	if remove {
		m.Report.TotalSetNil++
	}
	if hasOperator {
		m.Report.TotalSetPartly++
	}
	m.metrics.OnSet(ctx, entity, id, blockNumber, remove, hasOperator, used)
}

// OnCommit finalises the current cycle's Report (logging it), then resets
// Report so the next cycle begins with a clean slate.
func (m *ReportMonitor) OnCommit(
	ctx context.Context,
	blockNumber uint64,
	created map[string]int,
	updated map[string]int,
	used time.Duration,
) {
	_, logger := log.FromContext(ctx)
	m.Report.TotalCommit = utils.MergeMapSum(created, updated)
	m.Report.TotalCommitCreate = created
	m.Report.TotalCommitType = len(m.Report.TotalCommit)
	m.Report.TotalCacheEvicted = m.store.CacheEvicted() - m.storeCacheEvicted
	m.Report.TxnUsed = time.Since(m.start)
	m.Report.TxnCommitUsed = used
	if utils.SumMap(m.Report.TotalCommit) == 0 {
		logger.Debugw("commit changes of all entities succeed", "report", m.Report)
	} else {
		logger.Infow("commit changes of all entities succeed", "report", m.Report)
	}
	// Reset for the next commit cycle.
	m.start = time.Now()
	m.storeCacheEvicted = m.store.CacheEvicted()
	m.Report = newTxnReport()
}

type keyMetricAttrs struct{}

var keyMetricAttrs_ = keyMetricAttrs{}

func WithMetricAttrs(parent context.Context, attrs []attribute.KeyValue) context.Context {
	return context.WithValue(parent, keyMetricAttrs_, attrs)
}

func GetMetricAttrs(ctx context.Context) []attribute.KeyValue {
	if attrs := ctx.Value(keyMetricAttrs_); attrs != nil {
		return attrs.([]attribute.KeyValue)
	}
	return nil
}
