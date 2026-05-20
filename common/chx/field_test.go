package chx

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_BuildFieldType(t *testing.T) {
	assert.Equal(t, FieldTypeString, BuildFieldType("String"))
	assert.Equal(t, FieldTypeBool, BuildFieldType("Bool"))
	assert.Equal(t, FieldTypeInt8, BuildFieldType("Int8"))
	assert.Equal(t, FieldTypeInt32, BuildFieldType("Int32"))
	assert.Equal(t, FieldTypeInt64, BuildFieldType("Int64"))
	assert.Equal(t, FieldTypeInt256, BuildFieldType("Int256"))
	assert.Equal(t, FieldTypeUInt8, BuildFieldType("UInt8"))
	assert.Equal(t, FieldTypeUInt32, BuildFieldType("UInt32"))
	assert.Equal(t, FieldTypeUInt64, BuildFieldType("UInt64"))
	assert.Equal(t, FieldTypeUInt256, BuildFieldType("UInt256"))
	assert.Equal(t, FieldTypeFloat32, BuildFieldType("Float32"))
	assert.Equal(t, FieldTypeFloat64, BuildFieldType("Float64"))
	assert.Equal(t, FieldTypeDecimal{Precision: 76, Scale: 30}, BuildFieldType("Decimal(76, 30)"))
	assert.Equal(t, FieldTypeEnum{"aaa", "bbb"}, BuildFieldType("Enum8('aaa' = 1, 'bbb' = 2)"))
	assert.Equal(t, FieldTypeEnum{"aaa", "bbb"}, BuildFieldType("Enum('aaa', 'bbb')"))
	assert.Equal(t, FieldTypeDateTime64{Precision: 6, Timezone: "UTC"}, BuildFieldType("DateTime64(6, 'UTC')"))
	assert.Equal(t, FieldTypeDateTime64{Precision: 3, Timezone: "UTC"}, BuildFieldType("DateTime64(3, 'UTC')"))
	assert.Equal(t, FieldTypeNullable{Inner: FieldTypeBool}, BuildFieldType("Nullable(Bool)"))
	assert.Equal(t, FieldTypeArray{
		Inner: FieldTypeArray{
			Inner: FieldTypeNullable{
				Inner: FieldTypeEnum{"aaa", "bbb"},
			},
		},
	}, BuildFieldType("Array(Array(Nullable(Enum8('aaa' = 1, 'bbb' = 2))))"))
}

func Test_SameAs(t *testing.T) {
	{
		a := BuildFieldType("Enum8('AAA' = 1, 'BBB' = 2, 'CCC' = 3)")
		b := BuildFieldType("Enum('AAA', 'BBB', 'CCC')")
		assert.True(t, a.SameAs(b))
	}
	{
		a := BuildFieldType("Enum8('AAA' = 1, 'BBB' = 2, 'CCC' = 3)")
		b := BuildFieldType("Enum('AAA', 'BBB', 'CCC', 'DDD')")
		assert.False(t, a.SameAs(b))
	}
	{
		a := BuildFieldType("Array(Nullable(Enum8('AAA' = 1, 'BBB' = 2, 'CCC' = 3)))")
		b := BuildFieldType("Array(Nullable(Enum('AAA', 'BBB', 'CCC')))")
		assert.True(t, a.SameAs(b))
	}
	{
		a := BuildFieldType("Array(Nullable(Enum8('AAA' = 1, 'BBB' = 2, 'CCC' = 3)))")
		b := BuildFieldType("Array(Nullable(Enum('AAA', 'BBB', 'CCC', 'DDD')))")
		assert.False(t, a.SameAs(b))
	}
	{
		a := BuildFieldType("Array(Array(Decimal(76, 30)))")
		b := FieldTypeArray{Inner: FieldTypeArray{Inner: FieldTypeDecimal{Precision: 76, Scale: 30}}}
		assert.True(t, a.SameAs(b))
	}
	{
		a := BuildFieldType("Array(Array(Decimal(76, 20)))")
		b := FieldTypeArray{Inner: FieldTypeArray{Inner: FieldTypeDecimal{Precision: 76, Scale: 30}}}
		assert.False(t, a.SameAs(b))
	}
}

func Test_CheckModify(t *testing.T) {
	{
		a := BuildFieldType("Array(Nullable(Enum('AAA', 'BBB', 'CCC')))")
		b := BuildFieldType("Array(Nullable(Enum('AAA', 'BBB', 'CCC', 'DDD')))")
		assert.True(t, a.CheckModify(b))
	}
	{
		a := BuildFieldType("Array(Nullable(Enum('AAA', 'BBB', 'CCC')))")
		b := BuildFieldType("Array(Enum('AAA', 'BBB', 'CCC'))")
		assert.False(t, a.CheckModify(b))
	}
	{
		a := BuildFieldType("Array(Nullable(Enum('AAA', 'BBB', 'CCC')))")
		b := BuildFieldType("Nullable(Enum('AAA', 'BBB', 'CCC', 'DDD'))")
		assert.False(t, a.CheckModify(b))
	}
}

func Test_BuildFieldType_Tuple(t *testing.T) {
	// unnamed tuple
	assert.Equal(t, FieldTypeTuple{
		Parts: []TuplePart{
			{Name: "", Type: FieldTypeInt64},
			{Name: "", Type: FieldTypeString},
		},
	}, BuildFieldType("Tuple(Int64, String)"))

	// named tuple
	assert.Equal(t, FieldTypeTuple{
		Parts: []TuplePart{
			{Name: "a", Type: FieldTypeInt64},
			{Name: "b", Type: FieldTypeString},
		},
	}, BuildFieldType("Tuple(a Int64, b String)"))

	// nested: Tuple containing Array and Nullable
	assert.Equal(t, FieldTypeTuple{
		Parts: []TuplePart{
			{Name: "ids", Type: FieldTypeArray{Inner: FieldTypeUInt64}},
			{Name: "label", Type: FieldTypeNullable{Inner: FieldTypeString}},
		},
	}, BuildFieldType("Tuple(ids Array(UInt64), label Nullable(String))"))

	// nested: Array of Tuple
	assert.Equal(t, FieldTypeArray{
		Inner: FieldTypeTuple{
			Parts: []TuplePart{
				{Name: "x", Type: FieldTypeInt32},
				{Name: "y", Type: FieldTypeInt32},
			},
		},
	}, BuildFieldType("Array(Tuple(x Int32, y Int32))"))
}

func Test_FieldTypeTuple_String(t *testing.T) {
	assert.Equal(t, "Tuple(Int64, String)", FieldTypeTuple{
		Parts: []TuplePart{
			{Name: "", Type: FieldTypeInt64},
			{Name: "", Type: FieldTypeString},
		},
	}.String())

	assert.Equal(t, "Tuple(a Int64, b String)", FieldTypeTuple{
		Parts: []TuplePart{
			{Name: "a", Type: FieldTypeInt64},
			{Name: "b", Type: FieldTypeString},
		},
	}.String())
}

func Test_FieldTypeTuple_SameAs(t *testing.T) {
	a := BuildFieldType("Tuple(a Int64, b String)")

	// same
	assert.True(t, a.SameAs(BuildFieldType("Tuple(a Int64, b String)")))
	// different field name
	assert.False(t, a.SameAs(BuildFieldType("Tuple(a Int64, c String)")))
	// different field type
	assert.False(t, a.SameAs(BuildFieldType("Tuple(a Int64, b Int64)")))
	// different count
	assert.False(t, a.SameAs(BuildFieldType("Tuple(a Int64)")))
	// different kind entirely
	assert.False(t, a.SameAs(FieldTypeString))
}

func Test_FieldTypeTuple_CheckModify(t *testing.T) {
	// Tuple does not support online schema modification
	a := BuildFieldType("Tuple(a Int64, b String)")
	assert.False(t, a.CheckModify(BuildFieldType("Tuple(a Int64, b String, c Int32)")))
}
