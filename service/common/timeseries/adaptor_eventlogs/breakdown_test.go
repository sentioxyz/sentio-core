package adaptor_eventlogs

import (
	"testing"
)

func TestBreakdown_String(t *testing.T) {
	tests := []struct {
		name      string
		breakdown Breakdown
		addComma  bool
		expected  string
	}{
		{
			name:      "empty breakdown without comma",
			breakdown: Breakdown{},
			addComma:  false,
			expected:  "",
		},
		{
			name:      "empty breakdown with comma",
			breakdown: Breakdown{},
			addComma:  true,
			expected:  "",
		},
		{
			name:      "single field without comma",
			breakdown: Breakdown{"field1"},
			addComma:  false,
			expected:  "`field1`",
		},
		{
			name:      "single field with comma",
			breakdown: Breakdown{"field1"},
			addComma:  true,
			expected:  ",`field1`",
		},
		{
			name:      "multiple fields without comma",
			breakdown: Breakdown{"field1", "field2", "field3"},
			addComma:  false,
			expected:  "`field1`,`field2`,`field3`",
		},
		{
			name:      "multiple fields with comma",
			breakdown: Breakdown{"field1", "field2", "field3"},
			addComma:  true,
			expected:  ",`field1`,`field2`,`field3`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.breakdown.String(tt.addComma)
			if result != tt.expected {
				t.Errorf("Breakdown.String() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
