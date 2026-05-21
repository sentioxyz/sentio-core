package clickhouse

import (
	"context"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
	"sync"
)

type Option struct {
	// Determines the batch size of the batch insert
	BatchInsertSizeLimit int
}

type Probe interface {
	PreCreateTable(ctx context.Context, tv chx.TableOrView) error
}

type Store struct {
	ctrl   chx.Controller
	option Option

	metaLock sync.Mutex
	meta     storeMeta

	cachedCounterSeriesLatest *utils.SafeMap[string, *counterSeriesLatestCache]

	probe Probe
}

type counterSeriesLatestCache struct {
	labelFields []timeseries.Field
	valueFields []timeseries.Field
	seriesLast  map[string]timeseries.Row // key is series ID
}

var (
	defaultBatchInsertSizeLimit = envconf.LoadUInt64("SENTIO_DEFAULT_TIME_SERIES_BATCH_INSERT_SIZE",
		2000, envconf.WithMin(10), envconf.WithMax(2000))
)

func NewStore(ctrl chx.Controller, option Option, probe Probe) *Store {
	if option.BatchInsertSizeLimit == 0 {
		option.BatchInsertSizeLimit = int(defaultBatchInsertSizeLimit)
	}
	return &Store{
		ctrl:                      ctrl,
		option:                    option,
		cachedCounterSeriesLatest: utils.NewSafeMap[string, *counterSeriesLatestCache](),
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
	return s.ctrl.FullLogicName(meta.GetTableName())
}

func (s *Store) Client() chx.Conn {
	return s.ctrl.GetConnection()
}

var _ timeseries.Store = (*Store)(nil)
