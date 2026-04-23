package clickhouse

import (
	"context"
	"sync"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/registrar"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/processor/models"
)

type Option struct {
	// Determines the batch size of the batch insert
	BatchInsertSizeLimit int
}

type Store struct {
	client                chx.Conn
	cluster               string
	database              string
	tableSettings         string
	processorID           string
	processorReplica      int
	processorTablePattern models.TablePattern
	option                Option

	meta *storeMeta

	cachedCounterSeriesLatest *utils.SafeMap[string, *counterSeriesLatestCache]

	// registrar mirrors table creations to the on-chain Databases contract.
	// Only consulted when processorTablePattern == TablePatternNetworkV1.
	// Nil disables on-chain registration.
	registrar registrar.OnChain

	// onChainDatabaseEnsured guards a single EnsureDatabase call per Store
	// lifetime — timeseries tables are created lazily in syncMeta, so
	// there is no startup phase to hook.
	onChainDatabaseEnsured sync.Once
	onChainDatabaseErr     error
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
	processorReplica int,
	processorTablePattern models.TablePattern,
	option Option,
	reg registrar.OnChain,
) *Store {
	if option.BatchInsertSizeLimit == 0 {
		option.BatchInsertSizeLimit = int(defaultBatchInsertSizeLimit)
	}
	return &Store{
		client:                client,
		cluster:               cluster,
		database:              database,
		processorID:           processorID,
		processorReplica:      processorReplica,
		processorTablePattern: processorTablePattern,
		option:                option,
		meta: &storeMeta{
			Metas: make(map[timeseries.MetaType]map[string]timeseries.Meta),
		},
		cachedCounterSeriesLatest: utils.NewSafeMap[string, *counterSeriesLatestCache](),
		registrar:                 reg,
	}
}

func (s *Store) Init(ctx context.Context, overWriteMeta bool) error {
	return s.fetchMetas(ctx, overWriteMeta)
}

// onChainRegistrationEnabled reports whether the store should mirror table
// creations to the on-chain Databases contract.
func (s *Store) onChainRegistrationEnabled() bool {
	return s.processorTablePattern == models.TablePatternNetworkV1 && s.registrar != nil
}

// ensureOnChainDatabase ensures the on-chain database record for this
// replica exists, caching the result for subsequent table creations.
func (s *Store) ensureOnChainDatabase(ctx context.Context) error {
	s.onChainDatabaseEnsured.Do(func() {
		s.onChainDatabaseErr = s.registrar.EnsureDatabase(ctx, s.processorID, uint32(s.processorReplica))
	})
	return s.onChainDatabaseErr
}

func (s *Store) Client() timeseries.QueryClient {
	return s.client
}

var _ timeseries.Store = (*Store)(nil)
