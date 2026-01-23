package state

import (
	"context"
	"fmt"
)

type State interface {
	GetLastBlock() uint64
	GetIndexerInfos() map[uint64]IndexerInfo
	GetProcessorAllocations() map[string]map[uint64]ProcessorAllocation
	GetHostedProcessors() map[string]bool

	UpdateLastBlock(ctx context.Context, block uint64) error
	UpsertIndexerInfo(ctx context.Context, info IndexerInfo) error
	DeleteIndexerInfo(ctx context.Context, indexerId uint64) error
	UpsertProcessorAllocation(ctx context.Context, allocation ProcessorAllocation) error
	DeleteProcessorAllocation(ctx context.Context, processorId string, indexerId uint64) error
	UpsertHostedProcessor(ctx context.Context, processorId string) error
	DeleteHostedProcessor(ctx context.Context, processorId string) error
	IsHostedProcessor(processorId string) bool
}

type PlainState struct {
	LastBlock            uint64                                    `yaml:"last_block"`
	ProcessorAllocations map[string]map[uint64]ProcessorAllocation `yaml:"processor_allocations"`
	IndexerInfos         map[uint64]IndexerInfo                    `yaml:"indexer_infos"`
	HostedProcessors     map[string]bool                           `yaml:"hosted_processors"`
}

func (s *PlainState) GetLastBlock() uint64 {
	return s.LastBlock
}

func (s *PlainState) GetIndexerInfos() map[uint64]IndexerInfo {
	return s.IndexerInfos
}

func (s *PlainState) GetProcessorAllocations() map[string]map[uint64]ProcessorAllocation {
	return s.ProcessorAllocations
}

func (s *PlainState) GetHostedProcessors() map[string]bool {
	return s.HostedProcessors
}

func (s *PlainState) UpdateLastBlock(_ context.Context, block uint64) error {
	s.LastBlock = block
	return nil
}

func (s *PlainState) UpsertIndexerInfo(_ context.Context, info IndexerInfo) error {
	s.IndexerInfos[info.IndexerId] = info
	return nil
}

func (s *PlainState) DeleteIndexerInfo(_ context.Context, indexerId uint64) error {
	// backward compat
	// if _, ok := s.indexerInfos[indexerId]; !ok {
	// 	panic(fmt.Sprintf("indexer info not found for indexerId: %d", indexerId))
	// }
	delete(s.IndexerInfos, indexerId)
	return nil
}

func (s *PlainState) UpsertProcessorAllocation(_ context.Context, allocation ProcessorAllocation) error {
	if _, ok := s.ProcessorAllocations[allocation.ProcessorId]; !ok {
		s.ProcessorAllocations[allocation.ProcessorId] = map[uint64]ProcessorAllocation{}
	}
	s.ProcessorAllocations[allocation.ProcessorId][allocation.IndexerId] = allocation
	return nil
}

func (s *PlainState) DeleteProcessorAllocation(_ context.Context, processorId string, indexerId uint64) error {
	m, ok := s.ProcessorAllocations[processorId]
	if !ok {
		panic(fmt.Sprintf("processor allocation not found for processorId: %s", processorId))
	}
	delete(m, indexerId)
	return nil
}

func (s *PlainState) UpsertHostedProcessor(_ context.Context, processorId string) error {
	s.HostedProcessors[processorId] = true
	return nil
}

func (s *PlainState) DeleteHostedProcessor(_ context.Context, processorId string) error {
	delete(s.HostedProcessors, processorId)
	return nil
}

func (s *PlainState) IsHostedProcessor(processorId string) bool {
	_, ok := s.HostedProcessors[processorId]
	return ok
}
