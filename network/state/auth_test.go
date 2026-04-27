package state

import (
	"context"
	"testing"
)

// fakeAuthState implements the subset of State that IsDatabaseWriter
// touches: GetDatabase + GetDatabasePermissions. Other methods panic so a
// regression that tries to call them surfaces immediately.
type fakeAuthState struct {
	databases    map[string]DatabaseInfo
	permissions  map[string]map[string]string
	indexerInfos map[uint64]IndexerInfo
}

func (f fakeAuthState) GetDatabase(id string) (DatabaseInfo, bool) {
	db, ok := f.databases[id]
	return db, ok
}

func (f fakeAuthState) GetDatabasePermissions() map[string]map[string]string {
	return f.permissions
}

func (f fakeAuthState) GetIndexerInfo(indexerId uint64) (IndexerInfo, bool) {
	info, ok := f.indexerInfos[indexerId]
	return info, ok
}

func (f fakeAuthState) GetLastBlock() uint64 { panic("not used") }
func (f fakeAuthState) GetIndexerInfos() map[uint64]IndexerInfo {
	panic("not used")
}
func (f fakeAuthState) GetProcessorAllocations() map[string]map[uint64]ProcessorAllocation {
	panic("not used")
}
func (f fakeAuthState) GetProcessorInfos() map[string]ProcessorInfo {
	panic("not used")
}
func (f fakeAuthState) GetHostedProcessors() map[string]bool { panic("not used") }
func (f fakeAuthState) GetDatabases() map[string]DatabaseInfo {
	panic("not used")
}
func (f fakeAuthState) UpdateLastBlock(context.Context, uint64) error    { panic("not used") }
func (f fakeAuthState) UpsertIndexerInfo(context.Context, IndexerInfo) error {
	panic("not used")
}
func (f fakeAuthState) DeleteIndexerInfo(context.Context, uint64) error { panic("not used") }
func (f fakeAuthState) UpsertProcessorAllocation(context.Context, ProcessorAllocation) error {
	panic("not used")
}
func (f fakeAuthState) DeleteProcessorAllocation(context.Context, string, uint64) error {
	panic("not used")
}
func (f fakeAuthState) UpsertProcessorInfo(context.Context, ProcessorInfo) error {
	panic("not used")
}
func (f fakeAuthState) DeleteProcessorInfo(context.Context, string) error {
	panic("not used")
}
func (f fakeAuthState) UpsertHostedProcessor(context.Context, string) error {
	panic("not used")
}
func (f fakeAuthState) DeleteHostedProcessor(context.Context, string) error {
	panic("not used")
}
func (f fakeAuthState) IsHostedProcessor(string) bool { panic("not used") }
func (f fakeAuthState) UpsertDatabase(context.Context, DatabaseInfo) error {
	panic("not used")
}
func (f fakeAuthState) DeleteDatabase(context.Context, string) error { panic("not used") }
func (f fakeAuthState) SetDatabaseOwner(context.Context, string, string) error {
	panic("not used")
}
func (f fakeAuthState) AddDatabaseOperator(context.Context, string, string) error {
	panic("not used")
}
func (f fakeAuthState) RemoveDatabaseOperator(context.Context, string, string) error {
	panic("not used")
}
func (f fakeAuthState) UpsertDatabaseTable(context.Context, string, TableInfo) error {
	panic("not used")
}
func (f fakeAuthState) DeleteDatabaseTable(context.Context, string, string) error {
	panic("not used")
}
func (f fakeAuthState) GetAccountDatabasePermissions(string) map[string]string {
	panic("not used")
}
func (f fakeAuthState) SetDatabasePermission(context.Context, string, string, string) error {
	panic("not used")
}
func (f fakeAuthState) DeleteDatabasePermission(context.Context, string, string) error {
	panic("not used")
}

func TestIsDatabaseWriter(t *testing.T) {
	const dbID = "foo"
	const owner = "0xAAaA000000000000000000000000000000000001"
	const operator = "0xBBbB000000000000000000000000000000000002"
	const stranger = "0xCCcC000000000000000000000000000000000003"
	const indexerSigner = "0xDDdD000000000000000000000000000000000004"

	state := fakeAuthState{
		databases: map[string]DatabaseInfo{
			dbID: {DatabaseId: dbID, Owner: owner, IndexerId: 1},
		},
		permissions: map[string]map[string]string{
			operator: {dbID: WritePermission},
			owner:    {dbID: WritePermission},
		},
		indexerInfos: map[uint64]IndexerInfo{
			1: {IndexerId: 1, Signer: indexerSigner},
		},
	}

	tests := []struct {
		name string
		addr string
		dbID string
		want bool
	}{
		{"owner exact case", owner, dbID, true},
		{"owner lower case", "0xaaaa000000000000000000000000000000000001", dbID, true},
		{"owner upper case", "0xAAAA000000000000000000000000000000000001", dbID, true},
		{"operator exact case", operator, dbID, true},
		{"operator lower case", "0xbbbb000000000000000000000000000000000002", dbID, true},
		{"indexer signer exact case", indexerSigner, dbID, true},
		{"indexer signer lower case", "0xdddd000000000000000000000000000000000004", dbID, true},
		{"stranger rejected", stranger, dbID, false},
		{"empty addr rejected", "", dbID, false},
		{"empty dbID rejected", owner, "", false},
		{"missing db, no permission entry", owner, "missing-db", false},
		{"missing db, permission entry exists for other db", operator, "missing-db", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsDatabaseWriter(state, tc.addr, tc.dbID); got != tc.want {
				t.Errorf("IsDatabaseWriter(%q, %q) = %v, want %v", tc.addr, tc.dbID, got, tc.want)
			}
		})
	}

	t.Run("operator without permission entry is rejected", func(t *testing.T) {
		bareState := fakeAuthState{
			databases: map[string]DatabaseInfo{
				dbID: {DatabaseId: dbID, Owner: owner, Operators: []string{operator}, IndexerId: 1},
			},
			permissions:  map[string]map[string]string{},
			indexerInfos: map[uint64]IndexerInfo{1: {IndexerId: 1, Signer: indexerSigner}},
		}
		if IsDatabaseWriter(bareState, operator, dbID) {
			t.Error("operator from DatabaseInfo.Operators should not authorize without a permission map entry")
		}
		if !IsDatabaseWriter(bareState, owner, dbID) {
			t.Error("owner should authorize even without a permission map entry")
		}
		if !IsDatabaseWriter(bareState, indexerSigner, dbID) {
			t.Error("indexer signer should authorize via GetIndexerInfo")
		}
	})

	t.Run("indexer signer with empty Signer field is rejected", func(t *testing.T) {
		noSignerState := fakeAuthState{
			databases: map[string]DatabaseInfo{
				dbID: {DatabaseId: dbID, Owner: owner, IndexerId: 2},
			},
			permissions: map[string]map[string]string{},
			indexerInfos: map[uint64]IndexerInfo{
				2: {IndexerId: 2, Signer: ""},
			},
		}
		if IsDatabaseWriter(noSignerState, "0xwhat", dbID) {
			t.Error("empty Signer should not authorize")
		}
	})
}
