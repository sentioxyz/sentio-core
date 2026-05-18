package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPermissionSource implements PermissionSource for testing
type mockPermissionSource struct {
	permissions map[string]map[string]string
	notFound    bool
	err         error
}

func (m *mockPermissionSource) GetAccountPermissions(ctx context.Context, account string) (map[string]string, bool, error) {
	if m.err != nil {
		return nil, false, m.err
	}
	if m.notFound {
		return nil, false, nil
	}
	perms, ok := m.permissions[account]
	return perms, ok, nil
}

func TestExpandAuth(t *testing.T) {
	tests := []struct {
		name     string
		input    DbAuth
		expected DbAuth
	}{
		{
			name:     "Owner expands to all permissions",
			input:    DbAuthOwner,
			expected: DbAuthOwner | DbAuthAdmin | DbAuthWrite | DbAuthRead,
		},
		{
			name:     "Write expands to Read",
			input:    DbAuthWrite,
			expected: DbAuthWrite | DbAuthRead,
		},
		{
			name:     "Admin does not expand",
			input:    DbAuthAdmin,
			expected: DbAuthAdmin,
		},
		{
			name:     "Read does not expand",
			input:    DbAuthRead,
			expected: DbAuthRead,
		},
		{
			name:     "Owner + Admin keeps all",
			input:    DbAuthOwner | DbAuthAdmin,
			expected: DbAuthOwner | DbAuthAdmin | DbAuthWrite | DbAuthRead,
		},
		{
			name:     "Write + Admin expands Write only",
			input:    DbAuthWrite | DbAuthAdmin,
			expected: DbAuthWrite | DbAuthAdmin | DbAuthRead,
		},
		{
			name:     "No permissions",
			input:    0,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandAuth(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseAuth(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  DbAuth
		expectErr bool
	}{
		{
			name:     "Valid read permission",
			input:    "1",
			expected: DbAuthRead,
		},
		{
			name:     "Valid write permission",
			input:    "2",
			expected: DbAuthWrite,
		},
		{
			name:     "Valid owner permission",
			input:    "8",
			expected: DbAuthOwner,
		},
		{
			name:     "Combined permissions",
			input:    "3",
			expected: DbAuthRead | DbAuthWrite,
		},
		{
			name:     "Empty string returns zero",
			input:    "",
			expected: 0,
		},
		{
			name:      "Invalid format",
			input:     "abc",
			expectErr: true,
		},
		{
			name:     "Negative number parses but won't match permissions",
			input:    "-1",
			expected: -1, // Parses to -1, which won't match any positive permission bits
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAuth(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMergeAccountPermissions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		source    *mockPermissionSource
		account   string
		expected  map[Database]DbAuth
		expectErr bool
	}{
		{
			name: "User permissions only",
			source: &mockPermissionSource{
				permissions: map[string]map[string]string{
					"0xabc": {
						"db1": "1", // Read
						"db2": "2", // Write
					},
					string(WildcardAddress): {},
				},
			},
			account: "0xabc",
			expected: map[Database]DbAuth{
				"db1": DbAuthRead,
				"db2": DbAuthWrite | DbAuthRead, // Write expands to Read
			},
		},
		{
			name: "User + wildcard permissions merge",
			source: &mockPermissionSource{
				permissions: map[string]map[string]string{
					"0xabc": {
						"db1": "1", // Read
					},
					string(WildcardAddress): {
						"db2": "2", // Write for everyone
					},
				},
			},
			account: "0xabc",
			expected: map[Database]DbAuth{
				"db1": DbAuthRead,
				"db2": DbAuthWrite | DbAuthRead, // Wildcard write
			},
		},
		{
			name: "Wildcard permissions inherit",
			source: &mockPermissionSource{
				permissions: map[string]map[string]string{
					"0xabc": {
						"db1": "1",
					},
					string(WildcardAddress): {
						"db1": "2", // User + wildcard = Read|Write
					},
				},
			},
			account: "0xabc",
			expected: map[Database]DbAuth{
				"db1": DbAuthRead | DbAuthWrite, // 1 | 2 = 3, Write expands
			},
		},
		{
			name: "Owner permission expands",
			source: &mockPermissionSource{
				permissions: map[string]map[string]string{
					"0xabc": {
						"db1": "8", // Owner
					},
					string(WildcardAddress): {},
				},
			},
			account: "0xabc",
			expected: map[Database]DbAuth{
				"db1": DbAuthOwner | DbAuthAdmin | DbAuthWrite | DbAuthRead,
			},
		},
		{
			name: "No permissions returns empty map",
			source: &mockPermissionSource{
				permissions: map[string]map[string]string{
					string(WildcardAddress): {},
				},
				notFound: true,
			},
			account:   "0xabc",
			expected:  map[Database]DbAuth{},
		},
		{
			name: "Wildcard address query skips wildcard merge",
			source: &mockPermissionSource{
				permissions: map[string]map[string]string{
					string(WildcardAddress): {
						"db1": "1",
					},
				},
			},
			account: string(WildcardAddress),
			expected: map[Database]DbAuth{
				"db1": DbAuthRead,
			},
		},
		{
			name: "Source error propagates",
			source: &mockPermissionSource{
				err: assert.AnError,
			},
			account:   "0xabc",
			expectErr: true,
		},
		{
			name: "Invalid permission string is skipped",
			source: &mockPermissionSource{
				permissions: map[string]map[string]string{
					"0xabc": {
						"db1": "1",
						"db2": "invalid", // Skipped
					},
					string(WildcardAddress): {},
				},
			},
			account: "0xabc",
			expected: map[Database]DbAuth{
				"db1": DbAuthRead,
				// db2 skipped due to invalid format
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MergeAccountPermissions(ctx, tt.source, tt.account)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
