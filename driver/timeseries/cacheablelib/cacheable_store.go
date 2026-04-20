package cacheablelib

import (
	"context"
	"fmt"
	"time"

	"sentioxyz/sentio-core/common/cache"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/driver/timeseries/clickhouse"
	"sentioxyz/sentio-core/service/processor/models"
)

type CacheableMeta struct {
	ProcessorID      string
	ProcessorVersion int
	ProjectID        string

	conn            ckhmanager.Conn
	ttl             time.Duration
	refreshInterval time.Duration
	store           timeseries.Store
	reload          func(ctx context.Context) (timeseries.Store, error)
}

func (t *CacheableMeta) Key() string {
	return t.ProcessorID + "/" + t.conn.GetHost()
}

func (t *CacheableMeta) TTL() time.Duration {
	return t.ttl
}

func (t *CacheableMeta) RefreshInterval() time.Duration {
	return t.refreshInterval
}

func (t *CacheableMeta) Reload(ctx context.Context) (timeseries.Store, error) {
	if t.reload != nil {
		return t.reload(ctx)
	}
	return t.store, nil
}

func NewCacheableMeta(
	processorId string,
	processorVersion int,
	processorReplica int,
	processorTablePattern models.TablePattern,
	projectId string,
	conn ckhmanager.Conn,
) (cache.Cacheable[timeseries.Store], error) {
	return NewCacheableMetaWithTTL(
		processorId,
		processorVersion,
		processorReplica,
		processorTablePattern,
		projectId,
		conn,
		time.Hour,
		time.Minute*5,
	)
}

func NewCacheableMetaWithTTL(
	processorId string,
	processorVersion int,
	processorReplica int,
	processorTablePattern models.TablePattern,
	projectId string,
	conn ckhmanager.Conn,
	ttl, refreshInterval time.Duration,
) (cache.Cacheable[timeseries.Store], error) {
	if conn == nil {
		return nil, fmt.Errorf("processor %s clickhouse conn is nil", processorId)
	}
	tsm := &CacheableMeta{
		ProcessorID:      processorId,
		ProjectID:        projectId,
		ProcessorVersion: processorVersion,
		conn:             conn,
		ttl:              ttl,
		refreshInterval:  refreshInterval,
	}
	tsm.store = clickhouse.NewStore(
		conn,
		conn.GetCluster(),
		conn.GetDatabase(),
		processorId,
		processorReplica,
		processorTablePattern,
		clickhouse.Option{},
	)
	tsm.reload = func(ctx context.Context) (timeseries.Store, error) {
		if err := tsm.store.ReloadMeta(ctx, false); err != nil {
			return nil, err
		}
		return tsm.store, nil
	}
	return tsm, nil
}
