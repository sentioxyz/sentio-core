package registry

import (
	"context"
	"fmt"
	"strconv"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/statemirror"
	"sentioxyz/sentio-core/network/state"

	"github.com/go-faster/errors"
)

type indexerRegistry struct {
	mirror        statemirror.Mirror
	indexerMirror statemirror.MirrorReadOnlyState[string, state.IndexerInfo]
}

func NewIndexerRegistry(mirror statemirror.Mirror) IndexerRegistry {
	if mirror == nil {
		return &indexerRegistry{}
	}
	return &indexerRegistry{
		mirror: mirror,
		indexerMirror: statemirror.NewTypedMirror(mirror, statemirror.MappingIndexerInfos, statemirror.JSONCodec[string, state.IndexerInfo]{
			FieldFunc: func(k string) (string, error) {
				return k, nil
			},
			ParseFunc: func(s string) (string, error) {
				return s, nil
			},
		}),
	}
}

func (i *indexerRegistry) RetrieveIndexerInfo(ctx context.Context, indexerId IndexerId) (state.IndexerInfo, error) {
	if i.mirror == nil {
		return state.IndexerInfo{}, errors.New("indexer mirror is nil")
	}
	_, logger := log.FromContext(ctx, "indexer", fmt.Sprint(indexerId))
	indexer, ok, err := i.indexerMirror.Get(ctx, strconv.FormatUint(uint64(indexerId), 10))
	if err != nil {
		logger.Errorf("failed to retrieve indexer info for indexer %d: %v", indexerId, err)
		return state.IndexerInfo{}, errors.Wrapf(err, "failed to retrieve indexer info for indexer %d", indexerId)
	}
	if !ok {
		logger.Warnf("indexer info not found for indexer %d", indexerId)
		return state.IndexerInfo{}, errors.New("indexer info not found")
	}
	return indexer, nil
}

func (i *indexerRegistry) RetrieveAllIndexers(ctx context.Context) (map[IndexerId]state.IndexerInfo, error) {
	if i.mirror == nil {
		return nil, errors.New("indexer mirror is nil")
	}
	_, logger := log.FromContext(ctx)
	indexers, err := i.indexerMirror.GetAll(ctx)
	if err != nil {
		logger.Errorf("failed to retrieve all indexer infos: %v", err)
		return nil, errors.Wrap(err, "failed to retrieve all indexer infos")
	}
	result := make(map[IndexerId]state.IndexerInfo, len(indexers))
	for _, indexer := range indexers {
		result[IndexerId(indexer.IndexerId)] = indexer
	}
	return result, nil
}
