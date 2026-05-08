package state

import (
	"context"
	"fmt"
	"maps"
	"slices"
)

type State interface {
	GetLastBlock() uint64
	GetIndexerInfos() map[uint64]IndexerInfo
	GetIndexerInfo(indexerId uint64) (IndexerInfo, bool)
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
	MarkDatabasePendingDelete(ctx context.Context, databaseId string) error
	UpsertDatabaseTable(ctx context.Context, databaseId string, table TableInfo) error
	DeleteDatabaseTable(ctx context.Context, databaseId string, tableId string) error

	GetDatabasePermissions() map[string]map[string]string
	GetAccountDatabasePermissions(account string) map[string]string
	SetDatabasePermission(ctx context.Context, account string, databaseId string, permission string) error
	DeleteDatabasePermission(ctx context.Context, account string, databaseId string) error

	IsOperator(account, signer string) bool
	AddOperator(ctx context.Context, account, signer string) error
	RemoveOperator(ctx context.Context, account, signer string) error
	GetOperators() map[string]map[string]bool
}

type PlainState struct {
	LastBlock            uint64                                    `yaml:"last_block"`
	ProcessorAllocations map[string]map[uint64]ProcessorAllocation `yaml:"processor_allocations"`
	ProcessorInfos       map[string]ProcessorInfo                  `yaml:"processor_infos"`
	IndexerInfos         map[uint64]IndexerInfo                    `yaml:"indexer_infos"`
	HostedProcessors     map[string]bool                           `yaml:"hosted_processors"`
	Databases            map[string]DatabaseInfo                   `yaml:"databases"`
	DatabasePermissions  map[string]map[string]string              `yaml:"database_permissions"`
	Operators            map[string]map[string]bool                `yaml:"operators"`
}

// Clone returns a deep copy of s suitable for use as an isolated working
// copy: mutations to the clone never alias back into the source. Slices and
// nested maps are duplicated; struct values are copied by assignment.
func (s *PlainState) Clone() *PlainState {
	clone := &PlainState{
		LastBlock:            s.LastBlock,
		ProcessorAllocations: make(map[string]map[uint64]ProcessorAllocation, len(s.ProcessorAllocations)),
		ProcessorInfos:       maps.Clone(s.ProcessorInfos),
		IndexerInfos:         maps.Clone(s.IndexerInfos),
		HostedProcessors:     maps.Clone(s.HostedProcessors),
		Databases:            make(map[string]DatabaseInfo, len(s.Databases)),
		DatabasePermissions:  make(map[string]map[string]string, len(s.DatabasePermissions)),
		Operators:            make(map[string]map[string]bool, len(s.Operators)),
	}
	for procId, byIndexer := range s.ProcessorAllocations {
		clone.ProcessorAllocations[procId] = maps.Clone(byIndexer)
	}
	for dbId, info := range s.Databases {
		// DatabaseInfo.Tables is a slice — duplicate so handler appends to
		// the working copy don't bleed into the source.
		info.Tables = slices.Clone(info.Tables)
		clone.Databases[dbId] = info
	}
	for account, perms := range s.DatabasePermissions {
		clone.DatabasePermissions[account] = maps.Clone(perms)
	}
	for account, ops := range s.Operators {
		clone.Operators[account] = maps.Clone(ops)
	}
	if clone.ProcessorInfos == nil {
		clone.ProcessorInfos = map[string]ProcessorInfo{}
	}
	if clone.IndexerInfos == nil {
		clone.IndexerInfos = map[uint64]IndexerInfo{}
	}
	if clone.HostedProcessors == nil {
		clone.HostedProcessors = map[string]bool{}
	}
	return clone
}

func (s *PlainState) GetLastBlock() uint64 {
	return s.LastBlock
}

func (s *PlainState) GetIndexerInfos() map[uint64]IndexerInfo {
	return s.IndexerInfos
}

func (s *PlainState) GetIndexerInfo(indexerId uint64) (IndexerInfo, bool) {
	info, ok := s.IndexerInfos[indexerId]
	return info, ok
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
	// The contract's _cascadeDelete doesn't iterate _dbPermissions[dbId][*]
	// (no on-chain account index per db) and emits only a single
	// DatabaseDeleted event, so this loop is the only place where orphan
	// permission entries get cleared. Without it housegate's
	// buildDatabaseMap keeps surfacing the deleted db via SHOW DATABASES.
	for account, perms := range s.DatabasePermissions {
		if _, has := perms[databaseId]; !has {
			continue
		}
		delete(perms, databaseId)
		if len(perms) == 0 {
			delete(s.DatabasePermissions, account)
		}
	}
	return nil
}

func (s *PlainState) MarkDatabasePendingDelete(_ context.Context, databaseId string) error {
	info, ok := s.Databases[databaseId]
	if !ok {
		return fmt.Errorf("database not found: %s", databaseId)
	}
	info.PendingDelete = true
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

func (s *PlainState) DeleteDatabaseTable(_ context.Context, databaseId string, tableId string) error {
	info, ok := s.Databases[databaseId]
	if !ok {
		return nil
	}
	filtered := info.Tables[:0]
	for _, t := range info.Tables {
		if t.TableId == tableId {
			continue
		}
		filtered = append(filtered, t)
	}
	info.Tables = filtered
	s.Databases[databaseId] = info
	return nil
}

func (s *PlainState) GetDatabasePermissions() map[string]map[string]string {
	return s.DatabasePermissions
}

func (s *PlainState) GetAccountDatabasePermissions(account string) map[string]string {
	return s.DatabasePermissions[account]
}

func (s *PlainState) SetDatabasePermission(_ context.Context, account string, databaseId string, permission string) error {
	perms, ok := s.DatabasePermissions[account]
	if !ok {
		perms = map[string]string{}
		s.DatabasePermissions[account] = perms
	}
	perms[databaseId] = permission
	return nil
}

func (s *PlainState) DeleteDatabasePermission(_ context.Context, account string, databaseId string) error {
	perms, ok := s.DatabasePermissions[account]
	if !ok {
		return nil
	}
	delete(perms, databaseId)
	if len(perms) == 0 {
		delete(s.DatabasePermissions, account)
	}
	return nil
}

func (s *PlainState) IsOperator(account, signer string) bool {
	if account == "" || signer == "" {
		return false
	}
	if account == signer {
		return true
	}
	ops, ok := s.Operators[account]
	if !ok {
		return false
	}
	return ops[signer]
}

func (s *PlainState) AddOperator(_ context.Context, account, signer string) error {
	if s.Operators == nil {
		s.Operators = map[string]map[string]bool{}
	}
	ops, ok := s.Operators[account]
	if !ok {
		ops = map[string]bool{}
		s.Operators[account] = ops
	}
	ops[signer] = true
	return nil
}

func (s *PlainState) RemoveOperator(_ context.Context, account, signer string) error {
	ops, ok := s.Operators[account]
	if !ok {
		return nil
	}
	delete(ops, signer)
	if len(ops) == 0 {
		delete(s.Operators, account)
	}
	return nil
}

func (s *PlainState) GetOperators() map[string]map[string]bool {
	return s.Operators
}
