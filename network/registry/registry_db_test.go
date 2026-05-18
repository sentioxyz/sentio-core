package registry

import (
	"context"
	"testing"

	"sentioxyz/sentio-core/common/statemirror"
	"sentioxyz/sentio-core/network/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDatabaseInfoMirror implements MirrorReadOnlyState for DatabaseInfo
type mockDatabaseInfoMirror struct {
	data map[string]state.DatabaseInfo
	err  error
}

func (m *mockDatabaseInfoMirror) Get(ctx context.Context, field string) (state.DatabaseInfo, bool, error) {
	if m.err != nil {
		return state.DatabaseInfo{}, false, m.err
	}
	info, ok := m.data[field]
	return info, ok, nil
}

func (m *mockDatabaseInfoMirror) MGet(ctx context.Context, fields ...string) (map[string]state.DatabaseInfo, error) {
	result := make(map[string]state.DatabaseInfo)
	for _, f := range fields {
		if info, ok := m.data[f]; ok {
			result[f] = info
		}
	}
	return result, nil
}

func (m *mockDatabaseInfoMirror) GetAll(ctx context.Context) (map[string]state.DatabaseInfo, error) {
	return m.data, m.err
}

func (m *mockDatabaseInfoMirror) Scan(ctx context.Context, cursor uint64, match string, count int) (uint64, map[string]state.DatabaseInfo, error) {
	return 0, m.data, m.err
}

// mockPermissionMirror implements MirrorReadOnlyState for permissions
type mockPermissionMirror struct {
	data map[string]map[string]string
	err  error
}

func (m *mockPermissionMirror) Get(ctx context.Context, field string) (map[string]string, bool, error) {
	if m.err != nil {
		return nil, false, m.err
	}
	perms, ok := m.data[field]
	return perms, ok, nil
}

func (m *mockPermissionMirror) MGet(ctx context.Context, fields ...string) (map[string]map[string]string, error) {
	result := make(map[string]map[string]string)
	for _, f := range fields {
		if perms, ok := m.data[f]; ok {
			result[f] = perms
		}
	}
	return result, nil
}

func (m *mockPermissionMirror) GetAll(ctx context.Context) (map[string]map[string]string, error) {
	return m.data, m.err
}

func (m *mockPermissionMirror) Scan(ctx context.Context, cursor uint64, match string, count int) (uint64, map[string]map[string]string, error) {
	return 0, m.data, m.err
}

// mockMirror implements statemirror.Mirror for testing
type mockMirror struct{}

func (m *mockMirror) Upsert(ctx context.Context, key statemirror.OnChainKey, syncF statemirror.SyncFunc) error {
	return nil
}

func (m *mockMirror) UpsertStreaming(ctx context.Context, key statemirror.OnChainKey, syncF statemirror.StreamingSyncFunc) error {
	return nil
}

func (m *mockMirror) Apply(ctx context.Context, key statemirror.OnChainKey, diffF statemirror.DiffFunc) error {
	return nil
}

func (m *mockMirror) Get(ctx context.Context, key statemirror.OnChainKey, field string) (value string, ok bool, err error) {
	return "", false, nil
}

func (m *mockMirror) MGet(ctx context.Context, key statemirror.OnChainKey, fields ...string) (map[string]string, error) {
	return nil, nil
}

func (m *mockMirror) GetAll(ctx context.Context, key statemirror.OnChainKey) (map[string]string, error) {
	return nil, nil
}

func (m *mockMirror) Scan(ctx context.Context, key statemirror.OnChainKey, cursor uint64, match string, count int) (nextCursor uint64, kv map[string]string, err error) {
	return 0, nil, nil
}

func TestNewDbRegistry_NilMirror(t *testing.T) {
	reg := NewDbRegistry(nil)
	assert.NotNil(t, reg)

	// Methods should return "mirror is not initialized" errors
	ctx := context.Background()
	_, err := reg.RetrieveDatabaseInfo(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestDbRegistry_RetrieveDatabaseInfo(t *testing.T) {
	ctx := context.Background()

	activeDB := state.DatabaseInfo{
		DatabaseId:    "db_123",
		DbType:        state.DatabaseTypeUser,
		IndexerId:     1,
		PendingDelete: false,
	}
	pendingDeleteDB := state.DatabaseInfo{
		DatabaseId:    "db_pending",
		DbType:        state.DatabaseTypeProcessor,
		IndexerId:     2,
		PendingDelete: true,
	}

	tests := []struct {
		name        string
		database    Database
		data        map[string]state.DatabaseInfo
		mirrorErr   error
		expected    state.DatabaseInfo
		expectErr   bool
		errContains string
	}{
		{
			name:     "Successfully retrieve active database",
			database: "db_123",
			data: map[string]state.DatabaseInfo{
				"db_123": activeDB,
			},
			expected: activeDB,
		},
		{
			name:        "Database not found",
			database:    "db_404",
			data: map[string]state.DatabaseInfo{
				"db_123": activeDB,
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name:     "Pending delete database returns not found",
			database: "db_pending",
			data: map[string]state.DatabaseInfo{
				"db_pending": pendingDeleteDB,
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name:        "Mirror error propagates",
			database:    "db_error",
			mirrorErr:   assert.AnError,
			expectErr:   true,
			errContains: "failed to get database info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbMirror := &mockDatabaseInfoMirror{
				data: tt.data,
				err:  tt.mirrorErr,
			}
			reg := &dbRegistry{
				mirror:         &mockMirror{}, // non-nil to pass initialization check
				databaseMirror: dbMirror,
			}

			result, err := reg.RetrieveDatabaseInfo(ctx, tt.database)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDbRegistry_RetrieveAllDatabaseInfos(t *testing.T) {
	ctx := context.Background()

	activeDB1 := state.DatabaseInfo{
		DatabaseId:    "db_1",
		DbType:        state.DatabaseTypeUser,
		PendingDelete: false,
	}
	activeDB2 := state.DatabaseInfo{
		DatabaseId:    "db_2",
		DbType:        state.DatabaseTypeProcessor,
		PendingDelete: false,
	}
	pendingDeleteDB := state.DatabaseInfo{
		DatabaseId:    "db_pending",
		PendingDelete: true,
	}

	tests := []struct {
		name        string
		data        map[string]state.DatabaseInfo
		mirrorErr   error
		expected    map[Database]state.DatabaseInfo
		expectErr   bool
		errContains string
	}{
		{
			name: "Successfully retrieve all databases, excludes pending delete",
			data: map[string]state.DatabaseInfo{
				"db_1":       activeDB1,
				"db_2":       activeDB2,
				"db_pending": pendingDeleteDB,
			},
			expected: map[Database]state.DatabaseInfo{
				"db_1": activeDB1,
				"db_2": activeDB2,
				// db_pending should be excluded
			},
		},
		{
			name:     "Empty database list",
			data:     map[string]state.DatabaseInfo{},
			expected: map[Database]state.DatabaseInfo{},
		},
		{
			name:        "Mirror error propagates",
			mirrorErr:   assert.AnError,
			expectErr:   true,
			errContains: "failed to get all database infos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbMirror := &mockDatabaseInfoMirror{
				data: tt.data,
				err:  tt.mirrorErr,
			}
			reg := &dbRegistry{
				mirror:         &mockMirror{}, // non-nil to pass initialization check
				databaseMirror: dbMirror,
			}

			result, err := reg.RetrieveAllDatabaseInfos(ctx)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDbRegistry_AccountHasPermission(t *testing.T) {
	ctx := context.Background()

	activeDB := state.DatabaseInfo{
		DatabaseId:    "db_1",
		DbType:        state.DatabaseTypeUser,
		PendingDelete: false,
	}
	pendingDeleteDB := state.DatabaseInfo{
		DatabaseId:    "db_pending",
		PendingDelete: true,
	}

	tests := []struct {
		name          string
		address       Address
		database      Database
		action        Action
		dbData        map[string]state.DatabaseInfo
		permData      map[string]map[string]string
		dbMirrorErr   error
		permMirrorErr error
		expected      bool
		expectErr     bool
		errContains   string
	}{
		{
			name:    "User has read permission",
			address: "0xABC",
			database: "db_1",
			action:  Read,
			dbData: map[string]state.DatabaseInfo{
				"db_1": activeDB,
			},
			permData: map[string]map[string]string{
				"0xabc": {"db_1": "1"}, // Read
				string(WildcardAddress): {},
			},
			expected: true,
		},
		{
			name:    "User has write permission (implies read)",
			address: "0xABC",
			database: "db_1",
			action:  Read,
			dbData: map[string]state.DatabaseInfo{
				"db_1": activeDB,
			},
			permData: map[string]map[string]string{
				"0xabc": {"db_1": "2"}, // Write
				string(WildcardAddress): {},
			},
			expected: true,
		},
		{
			name:    "User has write permission",
			address: "0xABC",
			database: "db_1",
			action:  Write,
			dbData: map[string]state.DatabaseInfo{
				"db_1": activeDB,
			},
			permData: map[string]map[string]string{
				"0xabc": {"db_1": "2"}, // Write
				string(WildcardAddress): {},
			},
			expected: true,
		},
		{
			name:    "User does not have write permission",
			address: "0xABC",
			database: "db_1",
			action:  Write,
			dbData: map[string]state.DatabaseInfo{
				"db_1": activeDB,
			},
			permData: map[string]map[string]string{
				"0xabc": {"db_1": "1"}, // Read only
				string(WildcardAddress): {},
			},
			expected: false,
		},
		{
			name:    "Wildcard grants read permission",
			address: "0xABC",
			database: "db_1",
			action:  Read,
			dbData: map[string]state.DatabaseInfo{
				"db_1": activeDB,
			},
			permData: map[string]map[string]string{
				"0xabc": {},
				string(WildcardAddress): {"db_1": "1"},
			},
			expected: true,
		},
		{
			name:    "Database not found",
			address: "0xABC",
			database: "db_404",
			action:  Read,
			dbData: map[string]state.DatabaseInfo{
				"db_1": activeDB,
			},
			permData: map[string]map[string]string{
				string(WildcardAddress): {},
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name:    "Pending delete database returns no permission",
			address: "0xABC",
			database: "db_pending",
			action:  Read,
			dbData: map[string]state.DatabaseInfo{
				"db_pending": pendingDeleteDB,
			},
			permData: map[string]map[string]string{
				"0xabc": {"db_pending": "8"}, // Owner
			},
			expectErr:   true,
			errContains: "not found",
		},
		{
			name:    "Address is lowercased",
			address: "0xABCDEF",
			database: "db_1",
			action:  Read,
			dbData: map[string]state.DatabaseInfo{
				"db_1": activeDB,
			},
			permData: map[string]map[string]string{
				"0xabcdef": {"db_1": "1"}, // Stored as lowercase
				string(WildcardAddress): {},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbMirror := &mockDatabaseInfoMirror{
				data: tt.dbData,
				err:  tt.dbMirrorErr,
			}
			permMirror := &mockPermissionMirror{
				data: tt.permData,
				err:  tt.permMirrorErr,
			}
			reg := &dbRegistry{
				databaseMirror:   dbMirror,
					mirror:           &mockMirror{},
				permissionMirror: permMirror,
			}

			result, err := reg.AccountHasPermission(ctx, tt.address, tt.database, tt.action)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDbRegistry_RetrievePermissionsByAccount(t *testing.T) {
	ctx := context.Background()

	activeDB1 := state.DatabaseInfo{
		DatabaseId:    "db_1",
		PendingDelete: false,
	}
	activeDB2 := state.DatabaseInfo{
		DatabaseId:    "db_2",
		PendingDelete: false,
	}
	pendingDeleteDB := state.DatabaseInfo{
		DatabaseId:    "db_pending",
		PendingDelete: true,
	}

	tests := []struct {
		name          string
		address       Address
		dbData        map[string]state.DatabaseInfo
		permData      map[string]map[string]string
		expected      map[Database]DbAuth
		expectErr     bool
		errContains   string
	}{
		{
			name:    "Successfully retrieve permissions",
			address: "0xABC",
			dbData: map[string]state.DatabaseInfo{
				"db_1": activeDB1,
				"db_2": activeDB2,
			},
			permData: map[string]map[string]string{
				"0xabc": {
					"db_1": "1", // Read
					"db_2": "2", // Write
				},
				string(WildcardAddress): {},
			},
			expected: map[Database]DbAuth{
				"db_1": DbAuthRead,
				"db_2": DbAuthWrite | DbAuthRead,
			},
		},
		{
			name:    "Permissions for pending delete database are filtered out",
			address: "0xABC",
			dbData: map[string]state.DatabaseInfo{
				"db_1":       activeDB1,
				"db_pending": pendingDeleteDB,
			},
			permData: map[string]map[string]string{
				"0xabc": {
					"db_1": "1",
					"db_pending": "8", // Owner on pending delete
				},
				string(WildcardAddress): {},
			},
			expected: map[Database]DbAuth{
				"db_1": DbAuthRead,
				// db_pending should be filtered
			},
		},
		{
			name:    "Empty permissions when account has none",
			address: "0xNOPERM",
			dbData: map[string]state.DatabaseInfo{
				"db_1": activeDB1,
			},
			permData: map[string]map[string]string{
				string(WildcardAddress): {},
			},
			expected: map[Database]DbAuth{},
		},
		{
			name:    "Wildcard permissions merged",
			address: "0xABC",
			dbData: map[string]state.DatabaseInfo{
				"db_1": activeDB1,
				"db_2": activeDB2,
			},
			permData: map[string]map[string]string{
				"0xabc": {
					"db_1": "1",
				},
				string(WildcardAddress): {
					"db_2": "2", // Everyone gets write on db_2
				},
			},
			expected: map[Database]DbAuth{
				"db_1": DbAuthRead,
				"db_2": DbAuthWrite | DbAuthRead,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbMirror := &mockDatabaseInfoMirror{data: tt.dbData}
			permMirror := &mockPermissionMirror{data: tt.permData}
			reg := &dbRegistry{
				databaseMirror:   dbMirror,
				mirror:           &mockMirror{},
				permissionMirror: permMirror,
			}

			result, err := reg.RetrievePermissionsByAccount(ctx, tt.address)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
