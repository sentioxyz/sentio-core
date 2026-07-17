package clickhouse

import (
	"context"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
	"sync"
)

type Option struct {
	// Determines the batch size of the batch insert
	BatchInsertSizeLimit int
	// Maximum number of distinct metrics (counters and gauges combined) across all chains,
	// as metric tables are shared between chains. 0 means the default
	MetricNameLimit int
	// Maximum number of distinct event types across all chains, 0 means the default
	EventNameLimit int
	// Maximum number of time series of a single metric on one chain — each chain has its own
	// label combinations. 0 means the default
	SeriesLimitPerMetric int
	// Maximum total number of time series summed over all tracked metrics and chains,
	// 0 means the default
	SeriesLimitTotal int
}

type Probe interface {
	PreCreateTable(ctx context.Context, tv chx.TableOrView) error
}

type emptyProbe struct{}

func (p emptyProbe) PreCreateTable(_ context.Context, _ chx.TableOrView) error { return nil }

type Store struct {
	ctrl   chx.Controller
	option Option

	metaLock sync.Mutex
	meta     storeMeta

	// keys of both series caches are built by seriesCountKey (per metric per chain)
	cachedCounterSeriesLatest *utils.SafeMap[string, *counterSeriesLatestCache]
	cachedGaugeSeriesIDs      *utils.SafeMap[string, *gaugeSeriesIDCache]
	// number of known time series of each metric, key is built by seriesCountKey
	seriesCount *utils.SafeMap[string, int]
	// chains whose series counts have been backfilled from the storage, guarded by metaLock
	seriesCountLoadedChains set.Set[string]

	probe Probe
}

type counterSeriesLatestCache struct {
	labelFields []timeseries.Field
	valueFields []timeseries.Field
	seriesLast  map[string]timeseries.Row // key is series ID
}

type gaugeSeriesIDCache struct {
	labelFields []timeseries.Field
	ids         set.Set[string] // series IDs
	// the metric already had more series than the limit when this cache was loaded,
	// so the full ID set is unknown and the series limit is not enforced for it
	overLimitOnLoad bool
}

var (
	defaultBatchInsertSizeLimit = envconf.LoadUInt64("SENTIO_DEFAULT_TIME_SERIES_BATCH_INSERT_SIZE",
		2000, envconf.WithMin(10), envconf.WithMax(2000))
)

func NewStore(ctrl chx.Controller, option Option, probe Probe) *Store {
	if option.BatchInsertSizeLimit == 0 {
		option.BatchInsertSizeLimit = int(defaultBatchInsertSizeLimit)
	}
	if option.MetricNameLimit == 0 {
		option.MetricNameLimit = int(defaultMetricNameLimit)
	}
	if option.EventNameLimit == 0 {
		option.EventNameLimit = int(defaultEventNameLimit)
	}
	if option.SeriesLimitPerMetric == 0 {
		option.SeriesLimitPerMetric = int(defaultSeriesLimitPerMetric)
	}
	if option.SeriesLimitTotal == 0 {
		option.SeriesLimitTotal = int(defaultSeriesLimitTotal)
	}
	if probe == nil {
		probe = emptyProbe{}
	}
	return &Store{
		ctrl:                      ctrl,
		option:                    option,
		cachedCounterSeriesLatest: utils.NewSafeMap[string, *counterSeriesLatestCache](),
		cachedGaugeSeriesIDs:      utils.NewSafeMap[string, *gaugeSeriesIDCache](),
		seriesCount:               utils.NewSafeMap[string, int](),
		seriesCountLoadedChains:   set.New[string](),
		probe:                     probe,
	}
}

func (s *Store) Init(ctx context.Context) error {
	s.metaLock.Lock()
	defer s.metaLock.Unlock()
	if err := s.fetchMetas(ctx, false); err != nil {
		return err
	}
	for _, item := range s.meta {
		if err := s.syncMeta(ctx, timeseries.Dataset{Meta: item.meta}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) MetaTableName(meta timeseries.Meta) string {
	return s.ctrl.LogicName(meta.GetTableName())
}

func (s *Store) Client() chx.Conn {
	return s.ctrl.GetConnection()
}

var _ timeseries.Store = (*Store)(nil)
