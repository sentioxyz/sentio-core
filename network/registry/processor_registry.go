package registry

import (
	"context"
	"strconv"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/statemirror"
	"sentioxyz/sentio-core/network/state"

	"github.com/go-faster/errors"
)

type ProcessorRegistry interface {
	RetrieveProcessorAllocation(ctx context.Context, processorId string) ([]state.ProcessorAllocation, error)
	RetrieveShardingByProcessor(ctx context.Context, processorId string, ckhManager ckhmanager.Manager) (sharding ckhmanager.Sharding, err error)
	RetrieveProcessorInfo(ctx context.Context, processorId string) (state.ProcessorInfo, error)
}

type processorRegistry struct {
	ProcessorAllocation statemirror.MirrorReadOnlyState[string, []state.ProcessorAllocation]
	IndexerInfo         statemirror.MirrorReadOnlyState[string, state.IndexerInfo]
	ProcessorInfo       statemirror.MirrorReadOnlyState[string, state.ProcessorInfo]
}

func NewProcessorRegistry(mirror statemirror.Mirror) ProcessorRegistry {
	if mirror == nil {
		return &processorRegistry{}
	}
	return &processorRegistry{
		ProcessorAllocation: statemirror.NewTypedMirror(mirror, statemirror.MappingProcessorAllocations, statemirror.JSONCodec[string, []state.ProcessorAllocation]{
			FieldFunc: func(k string) (string, error) {
				return k, nil
			},
			ParseFunc: func(s string) (string, error) {
				return s, nil
			},
		}),
		IndexerInfo: statemirror.NewTypedMirror(mirror, statemirror.MappingIndexerInfos, statemirror.JSONCodec[string, state.IndexerInfo]{
			FieldFunc: func(k string) (string, error) {
				return k, nil
			},
			ParseFunc: func(s string) (string, error) {
				return s, nil
			},
		}),
		ProcessorInfo: statemirror.NewTypedMirror(mirror, statemirror.MappingProcessorInfos, statemirror.JSONCodec[string, state.ProcessorInfo]{
			FieldFunc: func(k string) (string, error) {
				return k, nil
			},
			ParseFunc: func(s string) (string, error) {
				return s, nil
			},
		}),
	}
}

func (r *processorRegistry) RetrieveProcessorAllocation(ctx context.Context, processorId string) ([]state.ProcessorAllocation, error) {
	if r.ProcessorAllocation == nil {
		return nil, errors.New("processor allocation state is not initialized")
	}
	ctx, logger := log.FromContext(ctx)
	pa, ok, err := r.ProcessorAllocation.Get(ctx, processorId)
	if err != nil {
		logger.Errorf("failed to get processor allocation: %s", err.Error())
		return nil, errors.Wrap(err, "failed to get processor allocation")
	}
	if !ok {
		logger.Errorf("processor allocation not found for processor %s", processorId)
		return nil, errors.Errorf("processor allocation not found for processor %s", processorId)
	}
	return pa, nil
}

func (r *processorRegistry) RetrieveProcessorInfo(ctx context.Context, processorId string) (state.ProcessorInfo, error) {
	if r.ProcessorInfo == nil {
		return state.ProcessorInfo{}, errors.New("processor info state is not initialized")
	}
	ctx, logger := log.FromContext(ctx)
	pi, ok, err := r.ProcessorInfo.Get(ctx, processorId)
	if err != nil {
		logger.Errorf("failed to get processor info: %s", err.Error())
		return state.ProcessorInfo{}, errors.Wrap(err, "failed to get processor info")
	}
	if !ok {
		logger.Errorf("processor info not found for processor %s", processorId)
		return state.ProcessorInfo{}, errors.Errorf("processor info not found for processor %s", processorId)
	}
	return pi, nil
}

func (r *processorRegistry) RetrieveShardingByProcessor(ctx context.Context, processorId string, ckhManager ckhmanager.Manager) (sharding ckhmanager.Sharding, err error) {
	if r.IndexerInfo == nil {
		return nil, errors.New("indexer info state is not initialized")
	}
	processorAllocation, err := r.RetrieveProcessorAllocation(ctx, processorId)
	if err != nil {
		return nil, err
	}

	ctx, logger := log.FromContext(ctx)
	for _, allocation := range processorAllocation {
		logger.Debugf("found allocation for network-processor %s: %d", processorId, allocation.IndexerId)
		if indexer, exists, err := r.IndexerInfo.Get(ctx, strconv.FormatUint(allocation.IndexerId, 10)); err == nil && exists {
			logger.Debugf("found sharding for network-processor %s, sharding: %d", processorId, indexer.IndexerId)
			sharding = ckhManager.NewShardByStateIndexer(indexer)
			break
		}
	}
	return
}
