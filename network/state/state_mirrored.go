package state

import (
	"context"
	"fmt"
	"sort"

	"sentioxyz/sentio-core/common/statemirror"
)

type StateMirrored struct {
	inner                    *PlainState
	mirror                   statemirror.Mirror
	indexerInfoCodec         statemirror.JSONCodec[string, IndexerInfo]
	processorAllocationCodec statemirror.JSONCodec[string, []ProcessorAllocation]
	processorInfoCodec       statemirror.JSONCodec[string, ProcessorInfo]
	databaseCodec            statemirror.JSONCodec[string, DatabaseInfo]
	databasePermissionsCodec statemirror.JSONCodec[string, map[string]string]
	operatorsCodec           statemirror.JSONCodec[string, []string]
}

func NewStateMirrored(ctx context.Context, state *PlainState, mirror statemirror.Mirror) (*StateMirrored, error) {
	st := &StateMirrored{
		inner:                    state,
		mirror:                   mirror,
		indexerInfoCodec:         newCodec[IndexerInfo](),
		processorAllocationCodec: newCodec[[]ProcessorAllocation](),
		processorInfoCodec:       newCodec[ProcessorInfo](),
		databaseCodec:            newCodec[DatabaseInfo](),
		databasePermissionsCodec: newCodec[map[string]string](),
		operatorsCodec:           newCodec[[]string](),
	}
	if err := st.SyncMirror(ctx); err != nil {
		return nil, err
	}
	return st, nil
}

// Inner returns the underlying PlainState. The caller must serialize access
// externally — StateMirrored has no internal lock.
func (s *StateMirrored) Inner() *PlainState {
	return s.inner
}

// ReplaceInner atomically substitutes the underlying PlainState and pushes
// the per-mapping delta (both Added and Deleted entries) to the Redis mirror
// so the mirror converges to ps. Use this for transactional batch updates:
// build a working copy via PlainState.Clone, mutate it, then commit here.
//
// The mirror diff is applied before the swap so mirror failures leave the
// in-memory state untouched. The caller must hold an external lock for the
// entire build-mutate-commit window.
func (s *StateMirrored) ReplaceInner(ctx context.Context, ps *PlainState) error {
	if err := diffApply(ctx, s.mirror, statemirror.MappingIndexerInfos, s.indexerInfoCodec,
		stringKeyMap(s.inner.IndexerInfos), stringKeyMap(ps.IndexerInfos)); err != nil {
		return err
	}
	if err := diffApply(ctx, s.mirror, statemirror.MappingProcessorAllocations, s.processorAllocationCodec,
		flattenAllocations(s.inner.ProcessorAllocations), flattenAllocations(ps.ProcessorAllocations)); err != nil {
		return err
	}
	if err := diffApply(ctx, s.mirror, statemirror.MappingProcessorInfos, s.processorInfoCodec,
		s.inner.ProcessorInfos, ps.ProcessorInfos); err != nil {
		return err
	}
	if err := diffApply(ctx, s.mirror, statemirror.MappingDatabases, s.databaseCodec,
		s.inner.Databases, ps.Databases); err != nil {
		return err
	}
	if err := diffApply(ctx, s.mirror, statemirror.MappingDatabasePermissions, s.databasePermissionsCodec,
		s.inner.DatabasePermissions, ps.DatabasePermissions); err != nil {
		return err
	}
	if err := diffApply(ctx, s.mirror, statemirror.MappingOperators, s.operatorsCodec,
		flattenOperators(s.inner.Operators), flattenOperators(ps.Operators)); err != nil {
		return err
	}
	s.inner = ps
	return nil
}

// flattenOperators collapses the nested account→signer-set map into the
// account→sorted-signer-list form the mirror codec stores. Sorted so
// equal logical sets serialise to identical bytes.
func flattenOperators(m map[string]map[string]bool) map[string][]string {
	out := make(map[string][]string, len(m))
	for account, ops := range m {
		if len(ops) == 0 {
			continue
		}
		signers := make([]string, 0, len(ops))
		for s := range ops {
			signers = append(signers, s)
		}
		sort.Strings(signers)
		out[account] = signers
	}
	return out
}

// stringKeyMap converts a map keyed by uint64 (used for indexer IDs in the
// inner state) to the string-keyed form expected by the mirror codecs.
func stringKeyMap(m map[uint64]IndexerInfo) map[string]IndexerInfo {
	out := make(map[string]IndexerInfo, len(m))
	for k, v := range m {
		out[fmt.Sprintf("%d", k)] = v
	}
	return out
}

// flattenAllocations collapses the nested per-indexer allocation map into the
// per-processor slice form the mirror stores.
func flattenAllocations(m map[string]map[uint64]ProcessorAllocation) map[string][]ProcessorAllocation {
	out := make(map[string][]ProcessorAllocation, len(m))
	for procId, byIndexer := range m {
		allocations := make([]ProcessorAllocation, 0, len(byIndexer))
		for _, a := range byIndexer {
			allocations = append(allocations, a)
		}
		out[procId] = allocations
	}
	return out
}

// diffApply pushes the delta between old and new (both keyed by K) to mirror
// as a single TypedDiff. Keys present only in old are emitted as Deleted;
// keys present in new are emitted as Added (the mirror codec treats Added as
// upsert, so equal entries are a harmless no-op write).
func diffApply[K comparable, V any](ctx context.Context, mirror statemirror.Mirror, key statemirror.OnChainKey, codec statemirror.StateCodec[K, V], old, new map[K]V) error {
	diff := &statemirror.TypedDiff[K, V]{
		Added: make(map[K]V, len(new)),
	}
	for k, v := range new {
		diff.Added[k] = v
	}
	for k := range old {
		if _, ok := new[k]; !ok {
			diff.Deleted = append(diff.Deleted, k)
		}
	}
	return applyDiff(ctx, mirror, key, codec, diff)
}

func (s *StateMirrored) GetLastBlock() uint64 {
	return s.inner.GetLastBlock()
}

func (s *StateMirrored) GetIndexerInfos() map[uint64]IndexerInfo {
	return s.inner.GetIndexerInfos()
}

func (s *StateMirrored) GetIndexerInfo(indexerId uint64) (IndexerInfo, bool) {
	return s.inner.GetIndexerInfo(indexerId)
}

func (s *StateMirrored) GetProcessorAllocations() map[string]map[uint64]ProcessorAllocation {
	return s.inner.GetProcessorAllocations()
}

func (s *StateMirrored) GetProcessorInfos() map[string]ProcessorInfo {
	return s.inner.GetProcessorInfos()
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
	if err := s.inner.UpsertProcessorAllocation(ctx, allocation); err != nil {
		return err
	}
	return s.syncProcessorAllocations(ctx, allocation.ProcessorId)
}

func (s *StateMirrored) DeleteProcessorAllocation(ctx context.Context, processorId string, indexerId uint64) error {
	if err := s.inner.DeleteProcessorAllocation(ctx, processorId, indexerId); err != nil {
		return err
	}
	return s.syncProcessorAllocations(ctx, processorId)
}

func (s *StateMirrored) syncProcessorAllocations(ctx context.Context, processorId string) error {
	var allocations []ProcessorAllocation
	for _, alloc := range s.inner.ProcessorAllocations[processorId] {
		allocations = append(allocations, alloc)
	}
	var diff statemirror.TypedDiff[string, []ProcessorAllocation]
	if len(allocations) > 0 {
		diff = statemirror.TypedDiff[string, []ProcessorAllocation]{
			Added: map[string][]ProcessorAllocation{
				processorId: allocations,
			},
		}
	} else {
		diff = statemirror.TypedDiff[string, []ProcessorAllocation]{
			Deleted: []string{processorId},
		}
	}
	return applyDiff(ctx, s.mirror, statemirror.MappingProcessorAllocations, s.processorAllocationCodec, &diff)
}

func (s *StateMirrored) UpsertProcessorInfo(ctx context.Context, info ProcessorInfo) error {
	diff := &statemirror.TypedDiff[string, ProcessorInfo]{
		Added: map[string]ProcessorInfo{
			info.ProcessorId: info,
		},
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingProcessorInfos, s.processorInfoCodec, diff); err != nil {
		return err
	}
	return s.inner.UpsertProcessorInfo(ctx, info)
}

func (s *StateMirrored) DeleteProcessorInfo(ctx context.Context, processorId string) error {
	diff := &statemirror.TypedDiff[string, ProcessorInfo]{
		Deleted: []string{processorId},
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingProcessorInfos, s.processorInfoCodec, diff); err != nil {
		return err
	}
	return s.inner.DeleteProcessorInfo(ctx, processorId)
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

func (s *StateMirrored) GetDatabases() map[string]DatabaseInfo {
	return s.inner.GetDatabases()
}

func (s *StateMirrored) GetDatabase(databaseId string) (DatabaseInfo, bool) {
	return s.inner.GetDatabase(databaseId)
}

func (s *StateMirrored) UpsertDatabase(ctx context.Context, info DatabaseInfo) error {
	if err := s.inner.UpsertDatabase(ctx, info); err != nil {
		return err
	}
	return s.syncDatabase(ctx, info.DatabaseId)
}

func (s *StateMirrored) DeleteDatabase(ctx context.Context, databaseId string) error {
	// Snapshot accounts whose perm map contains this dbId before the
	// inner call strips it — used after the cascade to re-sync each
	// affected account's redis hash entry.
	var affected []string
	for account, perms := range s.inner.DatabasePermissions {
		if _, has := perms[databaseId]; has {
			affected = append(affected, account)
		}
	}
	if err := s.inner.DeleteDatabase(ctx, databaseId); err != nil {
		return err
	}
	if err := s.syncDatabase(ctx, databaseId); err != nil {
		return err
	}
	for _, account := range affected {
		if err := s.syncAccountDatabasePermissions(ctx, account); err != nil {
			return err
		}
	}
	return nil
}

func (s *StateMirrored) MarkDatabasePendingDelete(ctx context.Context, databaseId string) error {
	if err := s.inner.MarkDatabasePendingDelete(ctx, databaseId); err != nil {
		return err
	}
	return s.syncDatabase(ctx, databaseId)
}

func (s *StateMirrored) UpsertDatabaseTable(ctx context.Context, databaseId string, table TableInfo) error {
	if err := s.inner.UpsertDatabaseTable(ctx, databaseId, table); err != nil {
		return err
	}
	return s.syncDatabase(ctx, databaseId)
}

func (s *StateMirrored) DeleteDatabaseTable(ctx context.Context, databaseId string, tableId string) error {
	if err := s.inner.DeleteDatabaseTable(ctx, databaseId, tableId); err != nil {
		return err
	}
	return s.syncDatabase(ctx, databaseId)
}

func (s *StateMirrored) GetDatabasePermissions() map[string]map[string]string {
	return s.inner.GetDatabasePermissions()
}

func (s *StateMirrored) GetAccountDatabasePermissions(account string) map[string]string {
	return s.inner.GetAccountDatabasePermissions(account)
}

func (s *StateMirrored) SetDatabasePermission(ctx context.Context, account string, databaseId string, permission string) error {
	if err := s.inner.SetDatabasePermission(ctx, account, databaseId, permission); err != nil {
		return err
	}
	return s.syncAccountDatabasePermissions(ctx, account)
}

func (s *StateMirrored) DeleteDatabasePermission(ctx context.Context, account string, databaseId string) error {
	if err := s.inner.DeleteDatabasePermission(ctx, account, databaseId); err != nil {
		return err
	}
	return s.syncAccountDatabasePermissions(ctx, account)
}

func (s *StateMirrored) IsOperator(account, signer string) bool {
	return s.inner.IsOperator(account, signer)
}

func (s *StateMirrored) GetOperators() map[string]map[string]bool {
	return s.inner.GetOperators()
}

func (s *StateMirrored) AddOperator(ctx context.Context, account, signer string) error {
	if err := s.inner.AddOperator(ctx, account, signer); err != nil {
		return err
	}
	return s.syncOperatorsForAccount(ctx, account)
}

func (s *StateMirrored) RemoveOperator(ctx context.Context, account, signer string) error {
	if err := s.inner.RemoveOperator(ctx, account, signer); err != nil {
		return err
	}
	return s.syncOperatorsForAccount(ctx, account)
}

func (s *StateMirrored) syncOperatorsForAccount(ctx context.Context, account string) error {
	ops, ok := s.inner.Operators[account]
	var diff statemirror.TypedDiff[string, []string]
	if ok && len(ops) > 0 {
		signers := make([]string, 0, len(ops))
		for op := range ops {
			signers = append(signers, op)
		}
		sort.Strings(signers)
		diff = statemirror.TypedDiff[string, []string]{
			Added: map[string][]string{account: signers},
		}
	} else {
		diff = statemirror.TypedDiff[string, []string]{
			Deleted: []string{account},
		}
	}
	return applyDiff(ctx, s.mirror, statemirror.MappingOperators, s.operatorsCodec, &diff)
}

func (s *StateMirrored) syncAccountDatabasePermissions(ctx context.Context, account string) error {
	perms, ok := s.inner.DatabasePermissions[account]
	var diff statemirror.TypedDiff[string, map[string]string]
	if ok && len(perms) > 0 {
		diff = statemirror.TypedDiff[string, map[string]string]{
			Added: map[string]map[string]string{account: perms},
		}
	} else {
		diff = statemirror.TypedDiff[string, map[string]string]{
			Deleted: []string{account},
		}
	}
	return applyDiff(ctx, s.mirror, statemirror.MappingDatabasePermissions, s.databasePermissionsCodec, &diff)
}

func (s *StateMirrored) syncDatabase(ctx context.Context, databaseId string) error {
	info, ok := s.inner.Databases[databaseId]
	var diff statemirror.TypedDiff[string, DatabaseInfo]
	if ok {
		diff = statemirror.TypedDiff[string, DatabaseInfo]{
			Added: map[string]DatabaseInfo{databaseId: info},
		}
	} else {
		diff = statemirror.TypedDiff[string, DatabaseInfo]{
			Deleted: []string{databaseId},
		}
	}
	return applyDiff(ctx, s.mirror, statemirror.MappingDatabases, s.databaseCodec, &diff)
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

	processorAllocationDiff := &statemirror.TypedDiff[string, []ProcessorAllocation]{
		Added: make(map[string][]ProcessorAllocation),
	}
	for processorId, m := range s.inner.GetProcessorAllocations() {
		var allocations []ProcessorAllocation
		for _, alloc := range m {
			allocations = append(allocations, alloc)
		}
		processorAllocationDiff.Added[processorId] = allocations
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingProcessorAllocations, s.processorAllocationCodec, processorAllocationDiff); err != nil {
		return err
	}

	processorInfoDiff := &statemirror.TypedDiff[string, ProcessorInfo]{
		Added: make(map[string]ProcessorInfo),
	}
	for processorId, info := range s.inner.GetProcessorInfos() {
		processorInfoDiff.Added[processorId] = info
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingProcessorInfos, s.processorInfoCodec, processorInfoDiff); err != nil {
		return err
	}

	databaseDiff := &statemirror.TypedDiff[string, DatabaseInfo]{
		Added: make(map[string]DatabaseInfo),
	}
	for databaseId, info := range s.inner.GetDatabases() {
		databaseDiff.Added[databaseId] = info
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingDatabases, s.databaseCodec, databaseDiff); err != nil {
		return err
	}

	databasePermissionsDiff := &statemirror.TypedDiff[string, map[string]string]{
		Added: make(map[string]map[string]string),
	}
	for account, perms := range s.inner.GetDatabasePermissions() {
		databasePermissionsDiff.Added[account] = perms
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingDatabasePermissions, s.databasePermissionsCodec, databasePermissionsDiff); err != nil {
		return err
	}

	operatorsDiff := &statemirror.TypedDiff[string, []string]{
		Added: flattenOperators(s.inner.Operators),
	}
	if err := applyDiff(ctx, s.mirror, statemirror.MappingOperators, s.operatorsCodec, operatorsDiff); err != nil {
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
