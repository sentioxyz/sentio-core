package persistent

import (
	"context"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

type TxnReport struct {
	TxnUsed       time.Duration
	TxnCommitUsed time.Duration

	// about update entity
	TotalSet          int
	TotalSetNil       int
	TotalSetPartly    int
	TotalSetUsed      time.Duration
	TotalCommit       map[string]int
	TotalCommitCreate map[string]int
	TotalCommitType   int

	// about get entity
	TotalGet                int
	TotalGetInBlock         int
	TotalGetUsed            time.Duration
	TotalGetFrom            map[string]map[string]int
	TotalGetFromUsed        map[string]map[string]time.Duration
	TotalList               int
	TotalListForLoadRelated int
	TotalListUsed           time.Duration
	TotalListFrom           map[string]map[string]int
	TotalListFromUsed       map[string]map[string]time.Duration
	TotalCacheEvicted       int
}

type Txn struct {
	start             time.Time
	storeCacheEvicted int
	report            TxnReport

	recordMetric SimpleNoticeController

	*Controller
}

func (t *Txn) NoticeGet(
	ctx context.Context,
	entity string,
	id string,
	blockNumber uint64,
	inBlock bool,
	from string,
	used time.Duration,
) {
	t.report.TotalGet++
	if inBlock {
		t.report.TotalGetInBlock++
	}
	t.report.TotalGetUsed += used
	utils.UpdateK2Map(t.report.TotalGetFrom, from, entity,
		func(v int) int { return v + 1 })
	utils.UpdateK2Map(t.report.TotalGetFromUsed, from, entity,
		func(v time.Duration) time.Duration { return v + used })
	t.recordMetric.NoticeGet(ctx, entity, id, blockNumber, inBlock, from, used)
}

func (t *Txn) NoticeList(
	ctx context.Context,
	entity string,
	blockNumber uint64,
	loadRelated bool,
	from string,
	resultLen int,
	resultPersistentLen int,
	used time.Duration,
) {
	t.report.TotalList++
	if loadRelated {
		t.report.TotalListForLoadRelated++
	}
	t.report.TotalListUsed += used
	utils.UpdateK2Map(t.report.TotalListFrom, from, entity,
		func(v int) int { return v + 1 })
	utils.UpdateK2Map(t.report.TotalListFromUsed, from, entity,
		func(v time.Duration) time.Duration { return v + used })
	t.recordMetric.NoticeList(ctx, entity, blockNumber, loadRelated, from, resultLen, resultPersistentLen, used)
}

func (t *Txn) NoticeSet(
	ctx context.Context,
	entity string,
	id string,
	blockNumber uint64,
	remove bool,
	hasOperator bool,
	used time.Duration,
) {
	t.report.TotalSet++
	t.report.TotalSetUsed += used
	if remove {
		t.report.TotalSetNil++
	}
	if hasOperator {
		t.report.TotalSetPartly++
	}
	t.recordMetric.NoticeSet(ctx, entity, id, blockNumber, remove, hasOperator, used)
}

func (t *Txn) NoticeCommit(
	ctx context.Context,
	blockNumber uint64,
	created map[string]int,
	updated map[string]int,
	used time.Duration,
) {
	_, logger := log.FromContext(ctx)
	t.report.TotalCommit = utils.MergeMapSum(created, updated)
	t.report.TotalCommitCreate = created
	t.report.TotalCommitType = len(t.report.TotalCommit)
	t.report.TotalCacheEvicted = t.store.cacheEvicted - t.storeCacheEvicted
	t.report.TxnUsed = time.Since(t.start)
	t.report.TxnCommitUsed = used
	if utils.SumMap(t.report.TotalCommit) == 0 {
		logger.Debugw("commit changes of all entities succeed", "report", t.report)
	} else {
		logger.Infow("commit changes of all entities succeed", "report", t.report)
	}
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
