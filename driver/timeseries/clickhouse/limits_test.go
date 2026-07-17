package clickhouse

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
)

func buildStoreMeta(counters, gauges, events int) storeMeta {
	var items []metaAndTable
	add := func(t timeseries.MetaType, count int) {
		for i := 0; i < count; i++ {
			items = append(items, metaAndTable{meta: timeseries.Meta{
				Type: t,
				Name: fmt.Sprintf("%s_%03d", t, i),
			}})
		}
	}
	add(timeseries.MetaTypeCounter, counters)
	add(timeseries.MetaTypeGauge, gauges)
	add(timeseries.MetaTypeEvent, events)
	return newStoreMeta(items)
}

func Test_checkMetaNameLimit(t *testing.T) {
	sm := buildStoreMeta(3, 2, 4)

	// under the limit
	assert.NoError(t, checkMetaNameLimit(sm,
		timeseries.Meta{Type: timeseries.MetaTypeCounter, Name: "new"}, 6, 5))
	assert.NoError(t, checkMetaNameLimit(sm,
		timeseries.Meta{Type: timeseries.MetaTypeEvent, Name: "new"}, 5, 5))

	// counters and gauges share the metric limit
	err := checkMetaNameLimit(sm, timeseries.Meta{Type: timeseries.MetaTypeGauge, Name: "new"}, 5, 100)
	assert.ErrorIs(t, err, timeseries.ErrTooManyMetrics)
	assert.ErrorContains(t, err, `cannot create metric "new"`)
	assert.ErrorContains(t, err, "has 5 distinct metrics, over the limit 5")

	// events have their own limit and are not affected by the metric limit
	err = checkMetaNameLimit(sm, timeseries.Meta{Type: timeseries.MetaTypeEvent, Name: "new"}, 100, 4)
	assert.ErrorIs(t, err, timeseries.ErrTooManyEventTypes)
	assert.ErrorContains(t, err, `cannot create event type "new"`)
	assert.ErrorContains(t, err, "has 4 distinct event types, over the limit 4")

	// the name preview is truncated
	sm = buildStoreMeta(30, 0, 0)
	err = checkMetaNameLimit(sm, timeseries.Meta{Type: timeseries.MetaTypeCounter, Name: "new"}, 30, 30)
	assert.ErrorIs(t, err, timeseries.ErrTooManyMetrics)
	assert.ErrorContains(t, err, "First 20 existing metrics")
	assert.ErrorContains(t, err, "counter_019")
	assert.NotContains(t, err.Error(), "counter_020")
}

func Test_collectNewSeriesIDs(t *testing.T) {
	labelFields := []timeseries.Field{{Name: "token", Type: timeseries.FieldTypeString}}
	rows := []timeseries.Row{
		{"token": "a"},
		{"token": "a"}, // duplicate of the row above
		{"token": "b"},
		{"token": "c"},
	}
	existing := set.New(buildSeriesID(timeseries.Row{"token": "b"}, labelFields))
	newIDs := collectNewSeriesIDs(rows, labelFields, existing.Contains)
	assert.Equal(t, 2, newIDs.Size())
	assert.True(t, newIDs.Contains(buildSeriesID(timeseries.Row{"token": "a"}, labelFields)))
	assert.True(t, newIDs.Contains(buildSeriesID(timeseries.Row{"token": "c"}, labelFields)))
}

func Test_checkSeriesLimit(t *testing.T) {
	newTestStore := func(perMetric, total int) *Store {
		return &Store{
			option:      Option{SeriesLimitPerMetric: perMetric, SeriesLimitTotal: total},
			seriesCount: utils.NewSafeMap[string, int](),
		}
	}
	counterMeta := timeseries.Meta{Type: timeseries.MetaTypeCounter, Name: "vol"}
	gaugeMeta := timeseries.Meta{Type: timeseries.MetaTypeGauge, Name: "price"}

	// under both limits
	s := newTestStore(10, 100)
	assert.NoError(t, s.checkSeriesLimit(counterMeta, "1", 5, 5))

	// over the per-metric limit
	s = newTestStore(10, 100)
	err := s.checkSeriesLimit(counterMeta, "1", 5, 6)
	assert.ErrorIs(t, err, timeseries.ErrTooManySeries)
	assert.ErrorContains(t, err, "of metric counter.vol would reach 11, over the limit 10")

	// no new series never fails, even if the metric is already over the limit
	s = newTestStore(10, 100)
	assert.NoError(t, s.checkSeriesLimit(counterMeta, "1", 20, 0))

	// over the total limit across metrics
	s = newTestStore(100, 15)
	assert.NoError(t, s.checkSeriesLimit(counterMeta, "1", 0, 10))
	err = s.checkSeriesLimit(gaugeMeta, "1", 0, 6)
	assert.ErrorIs(t, err, timeseries.ErrTooManySeries)
	assert.ErrorContains(t, err, "total number of time series would reach 16, over the limit 15")
	assert.ErrorContains(t, err, "counter.vol@1: 10")
	assert.ErrorContains(t, err, "gauge.price@1: 6")

	// the count of a metric is refreshed instead of accumulated
	s = newTestStore(100, 15)
	assert.NoError(t, s.checkSeriesLimit(counterMeta, "1", 0, 10))
	assert.NoError(t, s.checkSeriesLimit(counterMeta, "1", 10, 2))
	assert.NoError(t, s.checkSeriesLimit(gaugeMeta, "1", 0, 3))
}

func Test_ensureSeriesCountsLoaded_skipsWithoutQuery(t *testing.T) {
	labeled := timeseries.Meta{
		Type: timeseries.MetaTypeCounter,
		Name: "labeled",
		Fields: timeseries.BuildFields(
			timeseries.Field{Name: "token", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleSeriesLabel},
		),
	}
	unlabeled := timeseries.Meta{Type: timeseries.MetaTypeGauge, Name: "unlabeled"}
	event := timeseries.Meta{Type: timeseries.MetaTypeEvent, Name: "transfer"}
	agg := timeseries.Meta{
		Type:        timeseries.MetaTypeCounter,
		Name:        "agg",
		Aggregation: &timeseries.Aggregation{Source: "labeled"},
		Fields:      labeled.Fields,
	}
	counted := timeseries.Meta{Type: timeseries.MetaTypeCounter, Name: "counted", Fields: labeled.Fields}

	// the ctrl is nil, so any attempt to run a count query would panic;
	// every meta here must be skipped without querying the storage
	s := &Store{
		option:                  Option{SeriesLimitPerMetric: 10, SeriesLimitTotal: 100},
		meta:                    newStoreMeta([]metaAndTable{{meta: labeled}, {meta: unlabeled}, {meta: event}, {meta: agg}, {meta: counted}}),
		seriesCount:             utils.NewSafeMap[string, int](),
		seriesCountLoadedChains: set.New[string](),
	}
	s.seriesCount.Put(seriesCountKey(counted, "1"), 3)

	assert.NoError(t, s.ensureSeriesCountsLoaded(context.Background(), "1", set.New(labeled.GetFullName())))
	assert.True(t, s.seriesCountLoadedChains.Contains("1"))

	// once marked as loaded, the chain is never scanned again even without the skip list
	assert.NoError(t, s.ensureSeriesCountsLoaded(context.Background(), "1", set.New[string]()))

	// another chain is independent: "labeled" is no longer skipped, so it would query and panic
	assert.Panics(t, func() {
		_ = s.ensureSeriesCountsLoaded(context.Background(), "2", set.New[string]())
	})
}
