package cte

import (
	"testing"
)

func TestCTE_String(t *testing.T) {
	tests := []struct {
		name     string
		ctes     CTEs
		expected string
	}{
		{
			name:     "empty CTEs",
			ctes:     CTEs{},
			expected: "",
		},
		{
			name: "single CTE",
			ctes: CTEs{
				{Alias: "test_table", Query: "SELECT * FROM users"},
			},
			expected: "WITH test_table AS (SELECT * FROM users)",
		},
		{
			name: "multiple CTEs",
			ctes: CTEs{
				{Alias: "users_table", Query: "SELECT id, name FROM users"},
				{Alias: "orders_table", Query: "SELECT user_id, amount FROM orders"},
			},
			expected: "WITH users_table AS (SELECT id, name FROM users), orders_table AS (SELECT user_id, amount FROM orders)",
		},
		{
			name: "CTE with complex query",
			ctes: CTEs{
				{Alias: "aggregated_data", Query: "SELECT user_id, count(*) as total FROM events WHERE timestamp >= '2023-01-01' GROUP BY user_id"},
			},
			expected: "WITH aggregated_data AS (SELECT user_id, count(*) as total FROM events WHERE timestamp >= '2023-01-01' GROUP BY user_id)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ctes.String()
			if result != tt.expected {
				t.Errorf("CTEs.String() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
