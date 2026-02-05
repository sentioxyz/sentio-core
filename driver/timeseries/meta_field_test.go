package timeseries

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_FieldDiff(t *testing.T) {
	type testSet struct {
		FieldA     Field
		FieldB     Field
		Compatible bool
		Diff       FieldDiff
	}

	var testCases = []testSet{
		{
			FieldA: Field{
				Name: "a",
				Type: FieldTypeFloat,
			},
			FieldB: Field{
				Name: "a",
				Type: FieldTypeFloat,
			},
			Compatible: true,
		},
		{
			FieldA: Field{
				Name: "a",
				Type: FieldTypeFloat,
			},
			FieldB: Field{
				Name: "a",
				Type: FieldTypeInt,
			},
			Compatible: false,
			Diff: FieldDiff{
				Before: Field{
					Name: "a",
					Type: FieldTypeFloat,
				},
				After: Field{
					Name: "a",
					Type: FieldTypeInt,
				},
			},
		},
		{
			FieldA: Field{
				Name: "a",
				Type: FieldTypeFloat,
			},
			FieldB: Field{
				Name: "a",
				Type: FieldTypeFloat,
			},
			Compatible: true,
		},
		{
			FieldA: Field{
				Name: "a",
				Type: FieldTypeJSON,
				NestedStructSchema: map[string]FieldType{
					"b": FieldTypeFloat,
					"c": FieldTypeInt,
				},
			},
			FieldB: Field{
				Name: "a",
				Type: FieldTypeJSON,
				NestedStructSchema: map[string]FieldType{
					"b": FieldTypeFloat,
					"c": FieldTypeInt,
				},
			},
			Compatible: true,
		},
		{
			FieldA: Field{
				Name: "a",
				Type: FieldTypeJSON,
				NestedStructSchema: map[string]FieldType{
					"b": FieldTypeFloat,
					"c": FieldTypeInt,
				},
			},
			FieldB: Field{
				Name: "a",
				Type: FieldTypeJSON,
				NestedStructSchema: map[string]FieldType{
					"b": FieldTypeFloat,
					"c": FieldTypeInt,
					"d": FieldTypeString,
				},
			},
			Compatible: true,
		},
		{
			FieldA: Field{
				Name: "a",
				Type: FieldTypeJSON,
				NestedStructSchema: map[string]FieldType{
					"b": FieldTypeFloat,
					"c": FieldTypeInt,
				},
			},
			FieldB: Field{
				Name: "a",
				Type: FieldTypeJSON,
				NestedStructSchema: map[string]FieldType{
					"b": FieldTypeFloat,
					"c": FieldTypeString,
					"d": FieldTypeString,
				},
			},
			Compatible: false,
			Diff: FieldDiff{
				Before: Field{
					Name: "a.c",
					Type: FieldTypeInt,
				},
				After: Field{
					Name: "a.c",
					Type: FieldTypeString,
				},
			},
		},
		{
			FieldA: Field{
				Name: "a",
				Type: FieldTypeJSON,
				NestedStructSchema: map[string]FieldType{
					"b": FieldTypeFloat,
					"c": FieldTypeString,
					"d": FieldTypeString,
				},
			},
			FieldB: Field{
				Name: "a",
				Type: FieldTypeJSON,
				NestedStructSchema: map[string]FieldType{
					"b": FieldTypeFloat,
					"c": FieldTypeString,
				},
			},
			Compatible: true,
		},
	}

	for _, tc := range testCases {
		compatible := tc.FieldA.Compatible(tc.FieldB)
		if compatible != tc.Compatible {
			t.Errorf("FieldA: %v, FieldB: %v, Compatible: %v, Expected: %v", tc.FieldA, tc.FieldB, compatible, tc.Compatible)
		}
		if !tc.Compatible {
			diff := tc.FieldA.CompatibleDiff(tc.FieldB)
			assert.Equal(t, tc.Diff.Before, diff.Before)
			assert.Equal(t, tc.Diff.After, diff.After)
		}
	}
}
