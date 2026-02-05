package clickhouse

import (
	"context"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
)

type Option struct {
	// Determines the batch size of the batch insert
	BatchInsertSizeLimit int
}

type Store struct {
	client        chx.Conn
	cluster       string
	database      string
	tableSettings string
	processorID   string
	option        Option

	meta *storeMeta

	cachedCounterSeriesLatest *utils.SafeMap[string, *counterSeriesLatestCache]
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

func NewStore(
	client chx.Conn,
	cluster string,
	database string,
	processorID string,
	option Option,
) *Store {
	if option.BatchInsertSizeLimit == 0 {
		option.BatchInsertSizeLimit = int(defaultBatchInsertSizeLimit)
	}
	return &Store{
		client:      client,
		cluster:     cluster,
		database:    database,
		processorID: processorID,
		option:      option,
		meta: &storeMeta{
			Metas: make(map[timeseries.MetaType]map[string]timeseries.Meta),
		},
		cachedCounterSeriesLatest: utils.NewSafeMap[string, *counterSeriesLatestCache](),
	}
}

func (s *Store) Init(ctx context.Context, overWriteMeta bool) error {
	return s.fetchMetas(ctx, overWriteMeta)
}

func (s *Store) Client() timeseries.QueryClient {
	return s.client
}

var _ timeseries.Store = (*Store)(nil)
