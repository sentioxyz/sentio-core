package state

import (
	"context"
	"fmt"
)

type State interface {
	GetLastBlock() uint64
	GetIndexerInfos() map[uint64]IndexerInfo
	GetProcessorAllocations() map[string]map[uint64]ProcessorAllocation
	GetProcessorInfos() map[string]ProcessorInfo
	GetHostedProcessors() map[string]bool
	GetDatabases() map[string]DatabaseInfo
	GetDatabase(databaseId string) (DatabaseInfo, bool)

	UpdateLastBlock(ctx context.Context, block uint64) error
	UpsertIndexerInfo(ctx context.Context, info IndexerInfo) error
	DeleteIndexerInfo(ctx context.Context, indexerId uint64) error
	UpsertProcessorAllocation(ctx context.Context, allocation ProcessorAllocation) error
	DeleteProcessorAllocation(ctx context.Context, processorId string, indexerId uint64) error
	UpsertProcessorInfo(ctx context.Context, info ProcessorInfo) error
	DeleteProcessorInfo(ctx context.Context, processorId string) error
	UpsertHostedProcessor(ctx context.Context, processorId string) error
	DeleteHostedProcessor(ctx context.Context, processorId string) error
	IsHostedProcessor(processorId string) bool

	UpsertDatabase(ctx context.Context, info DatabaseInfo) error
	DeleteDatabase(ctx context.Context, databaseId string) error
	SetDatabaseOwner(ctx context.Context, databaseId string, owner string) error
	AddDatabaseOperator(ctx context.Context, databaseId string, operator string) error
	RemoveDatabaseOperator(ctx context.Context, databaseId string, operator string) error
	UpsertDatabaseAllocation(ctx context.Context, databaseId string, allocation DatabaseAllocation) error
	DeleteDatabaseAllocation(ctx context.Context, databaseId string, indexerId uint64) error
	UpsertDatabaseTable(ctx context.Context, databaseId string, table TableInfo) error
	DeleteDatabaseTable(ctx context.Context, tableId string) error
}

type PlainState struct {
	LastBlock            uint64                                    `yaml:"last_block"`
	ProcessorAllocations map[string]map[uint64]ProcessorAllocation `yaml:"processor_allocations"`
	ProcessorInfos       map[string]ProcessorInfo                  `yaml:"processor_infos"`
	IndexerInfos         map[uint64]IndexerInfo                    `yaml:"indexer_infos"`
	HostedProcessors     map[string]bool                           `yaml:"hosted_processors"`
	Databases            map[string]DatabaseInfo                   `yaml:"databases"`
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

func (s *PlainState) GetProcessorInfos() map[string]ProcessorInfo {
	return s.ProcessorInfos
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
	if len(m) == 0 {
		delete(s.ProcessorAllocations, processorId)
	}
	return nil
}

func (s *PlainState) UpsertProcessorInfo(_ context.Context, info ProcessorInfo) error {
	s.ProcessorInfos[info.ProcessorId] = info
	return nil
}

func (s *PlainState) DeleteProcessorInfo(_ context.Context, processorId string) error {
	delete(s.ProcessorInfos, processorId)
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

func (s *PlainState) GetDatabases() map[string]DatabaseInfo {
	return s.Databases
}

func (s *PlainState) GetDatabase(databaseId string) (DatabaseInfo, bool) {
	info, ok := s.Databases[databaseId]
	return info, ok
}

func (s *PlainState) UpsertDatabase(_ context.Context, info DatabaseInfo) error {
	s.Databases[info.DatabaseId] = info
	return nil
}

func (s *PlainState) DeleteDatabase(_ context.Context, databaseId string) error {
	delete(s.Databases, databaseId)
	return nil
}

func (s *PlainState) SetDatabaseOwner(_ context.Context, databaseId string, owner string) error {
	info, ok := s.Databases[databaseId]
	if !ok {
		return fmt.Errorf("database not found: %s", databaseId)
	}
	info.Owner = owner
	s.Databases[databaseId] = info
	return nil
}

func (s *PlainState) AddDatabaseOperator(_ context.Context, databaseId string, operator string) error {
	info, ok := s.Databases[databaseId]
	if !ok {
		return fmt.Errorf("database not found: %s", databaseId)
	}
	for _, op := range info.Operators {
		if op == operator {
			return nil
		}
	}
	info.Operators = append(info.Operators, operator)
	s.Databases[databaseId] = info
	return nil
}

func (s *PlainState) RemoveDatabaseOperator(_ context.Context, databaseId string, operator string) error {
	info, ok := s.Databases[databaseId]
	if !ok {
		return fmt.Errorf("database not found: %s", databaseId)
	}
	filtered := info.Operators[:0]
	for _, op := range info.Operators {
		if op != operator {
			filtered = append(filtered, op)
		}
	}
	info.Operators = filtered
	s.Databases[databaseId] = info
	return nil
}

func (s *PlainState) UpsertDatabaseAllocation(_ context.Context, databaseId string, allocation DatabaseAllocation) error {
	info, ok := s.Databases[databaseId]
	if !ok {
		return fmt.Errorf("database not found: %s", databaseId)
	}
	replaced := false
	for i, a := range info.Allocations {
		if a.IndexerId == allocation.IndexerId {
			info.Allocations[i] = allocation
			replaced = true
			break
		}
	}
	if !replaced {
		info.Allocations = append(info.Allocations, allocation)
	}
	s.Databases[databaseId] = info
	return nil
}

func (s *PlainState) DeleteDatabaseAllocation(_ context.Context, databaseId string, indexerId uint64) error {
	info, ok := s.Databases[databaseId]
	if !ok {
		return fmt.Errorf("database not found: %s", databaseId)
	}
	filtered := info.Allocations[:0]
	for _, a := range info.Allocations {
		if a.IndexerId != indexerId {
			filtered = append(filtered, a)
		}
	}
	info.Allocations = filtered
	s.Databases[databaseId] = info
	return nil
}

func (s *PlainState) UpsertDatabaseTable(_ context.Context, databaseId string, table TableInfo) error {
	info, ok := s.Databases[databaseId]
	if !ok {
		return fmt.Errorf("database not found: %s", databaseId)
	}
	replaced := false
	for i, t := range info.Tables {
		if t.TableId == table.TableId {
			info.Tables[i] = table
			replaced = true
			break
		}
	}
	if !replaced {
		info.Tables = append(info.Tables, table)
	}
	s.Databases[databaseId] = info
	return nil
}

func (s *PlainState) DeleteDatabaseTable(_ context.Context, tableId string) error {
	for databaseId, info := range s.Databases {
		filtered := info.Tables[:0]
		removed := false
		for _, t := range info.Tables {
			if t.TableId == tableId {
				removed = true
				continue
			}
			filtered = append(filtered, t)
		}
		if removed {
			info.Tables = filtered
			s.Databases[databaseId] = info
			return nil
		}
	}
	return nil
}
