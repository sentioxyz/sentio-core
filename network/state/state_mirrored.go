package state

import (
	"context"
	"fmt"

	"sentioxyz/sentio-core/common/statemirror"
)

type StateMirrored struct {
	inner                    *PlainState
	mirror                   statemirror.Mirror
	indexerInfoCodec         statemirror.JSONCodec[string, IndexerInfo]
	processorAllocationCodec statemirror.JSONCodec[string, ProcessorAllocation]
}

func NewStateMirrored(ctx context.Context, state *PlainState, mirror statemirror.Mirror) (*StateMirrored, error) {
	st := &StateMirrored{
		inner:                    state,
		mirror:                   mirror,
		indexerInfoCodec:         newCodec[IndexerInfo](),
		processorAllocationCodec: newCodec[ProcessorAllocation](),
	}
	if err := st.SyncMirror(ctx); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *StateMirrored) GetLastBlock() uint64 {
	return s.inner.GetLastBlock()
}

func (s *StateMirrored) GetIndexerInfos() map[uint64]IndexerInfo {
	return s.inner.GetIndexerInfos()
}

func (s *StateMirrored) GetProcessorAllocations() map[string]map[uint64]ProcessorAllocation {
	return s.inner.GetProcessorAllocations()
}

func (s *StateMirrored) GetHostedProcessors() map[string]bool {
	return s.inner.GetHostedProcessors()
}

func (s *StateMirrored) UpdateLastBlock(ctx context.Context, block uint64) error {
	return s.inner.UpdateLastBlock(ctx, block)
}

func (s *StateMirrored) UpsertIndexerInfo(ctx context.Context, info IndexerInfo) error {
	diff := &statemirror.TypedDiff[string, IndexerInfo]{
		Added: map[string]IndexerInfo{
			fmt.Sprintf("%d", info.IndexerId): info,
		},
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingIndexerInfos, s.indexerInfoCodec, diff); err != nil {
		return err
	}
	return s.inner.UpsertIndexerInfo(ctx, info)
}

func (s *StateMirrored) DeleteIndexerInfo(ctx context.Context, indexerId uint64) error {
	diff := &statemirror.TypedDiff[string, IndexerInfo]{
		Deleted: []string{
			fmt.Sprintf("%d", indexerId),
		},
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingIndexerInfos, s.indexerInfoCodec, diff); err != nil {
		return err
	}
	return s.inner.DeleteIndexerInfo(ctx, indexerId)
}

func (s *StateMirrored) UpsertProcessorAllocation(ctx context.Context, allocation ProcessorAllocation) error {
	diff := &statemirror.TypedDiff[string, ProcessorAllocation]{
		Added: map[string]ProcessorAllocation{
			fmt.Sprintf("%s@%d", allocation.ProcessorId, allocation.IndexerId): allocation,
		},
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingProcessorAllocations, s.processorAllocationCodec, diff); err != nil {
		return err
	}
	return s.inner.UpsertProcessorAllocation(ctx, allocation)
}

func (s *StateMirrored) DeleteProcessorAllocation(ctx context.Context, processorId string, indexerId uint64) error {
	diff := &statemirror.TypedDiff[string, ProcessorAllocation]{
		Deleted: []string{
			fmt.Sprintf("%s@%d", processorId, indexerId),
		},
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingProcessorAllocations, s.processorAllocationCodec, diff); err != nil {
		return err
	}
	return s.inner.DeleteProcessorAllocation(ctx, processorId, indexerId)
}

func (s *StateMirrored) UpsertHostedProcessor(ctx context.Context, processorId string) error {
	return s.inner.UpsertHostedProcessor(ctx, processorId)
}

func (s *StateMirrored) DeleteHostedProcessor(ctx context.Context, processorId string) error {
	return s.inner.DeleteHostedProcessor(ctx, processorId)
}

func (s *StateMirrored) IsHostedProcessor(processorId string) bool {
	return s.inner.IsHostedProcessor(processorId)
}

func (s *StateMirrored) SyncMirror(ctx context.Context) error {
	indexerDiff := &statemirror.TypedDiff[string, IndexerInfo]{
		Added: make(map[string]IndexerInfo),
	}
	for indexerId, info := range s.inner.GetIndexerInfos() {
		indexerDiff.Added[fmt.Sprintf("%d", indexerId)] = info
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingIndexerInfos, s.indexerInfoCodec, indexerDiff); err != nil {
		return err
	}

	processorDiff := &statemirror.TypedDiff[string, ProcessorAllocation]{
		Added: make(map[string]ProcessorAllocation),
	}
	for processorId, m := range s.inner.GetProcessorAllocations() {
		for indexerId, allocation := range m {
			processorDiff.Added[fmt.Sprintf("%s@%d", processorId, indexerId)] = allocation
		}
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingProcessorAllocations, s.processorAllocationCodec, processorDiff); err != nil {
		return err
	}
	return nil
}

func newCodec[V any]() statemirror.JSONCodec[string, V] {
	return statemirror.JSONCodec[string, V]{
		FieldFunc: func(k string) (string, error) {
			return k, nil
		},
		ParseFunc: func(s string) (string, error) {
			return s, nil
		},
	}
}

func applyDiff[K comparable, V any](ctx context.Context, mirror statemirror.Mirror, key statemirror.OnChainKey, codec statemirror.StateCodec[K, V], diff *statemirror.TypedDiff[K, V]) error {
	diffFunc := func(ctx context.Context, key statemirror.OnChainKey) (*statemirror.TypedDiff[K, V], error) {
		return diff, nil
	}
	return mirror.Apply(ctx, key, statemirror.BuildDiffFunc(codec, diffFunc))
}
