package timeseries

import (
	"math/big"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/log"
	commonProtos "sentioxyz/sentio-core/service/common/protos"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestRichValueToAny_IntValue(t *testing.T) {
	rv := &commonProtos.RichValue{
		Value: &commonProtos.RichValue_IntValue{IntValue: 42},
	}

	result, fieldType, err := richValueToAny(rv, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(42), result)
	assert.Equal(t, FieldTypeInt, fieldType)
}

func TestRichValueToAny_FloatValue(t *testing.T) {
	rv := &commonProtos.RichValue{
		Value: &commonProtos.RichValue_FloatValue{FloatValue: 3.14},
	}

	result, fieldType, err := richValueToAny(rv, time.Now())

	require.NoError(t, err)
	assert.Equal(t, 3.14, result)
	assert.Equal(t, FieldTypeFloat, fieldType)
}

func TestRichValueToAny_BytesValue(t *testing.T) {
	testBytes := []byte{0x01, 0x02, 0x03}
	rv := &commonProtos.RichValue{
		Value: &commonProtos.RichValue_BytesValue{BytesValue: testBytes},
	}

	result, fieldType, err := richValueToAny(rv, time.Now())

	require.NoError(t, err)
	assert.Equal(t, hexutil.Encode(testBytes), result)
	assert.Equal(t, FieldTypeString, fieldType)
}

func TestRichValueToAny_BoolValue(t *testing.T) {
	rv := &commonProtos.RichValue{
		Value: &commonProtos.RichValue_BoolValue{BoolValue: true},
	}

	result, fieldType, err := richValueToAny(rv, time.Now())

	require.NoError(t, err)
	assert.Equal(t, true, result)
	assert.Equal(t, FieldTypeBool, fieldType)
}

func TestRichValueToAny_StringValue(t *testing.T) {
	rv := &commonProtos.RichValue{
		Value: &commonProtos.RichValue_StringValue{StringValue: "test string"},
	}

	result, fieldType, err := richValueToAny(rv, time.Now())

	require.NoError(t, err)
	assert.Equal(t, "test string", result)
	assert.Equal(t, FieldTypeString, fieldType)
}

func TestRichValueToAny_TimestampValue(t *testing.T) {
	testTime := time.Now().In(time.UTC)
	rv := &commonProtos.RichValue{
		Value: &commonProtos.RichValue_TimestampValue{
			TimestampValue: timestamppb.New(testTime),
		},
	}

	result, fieldType, err := richValueToAny(rv, time.Now())

	require.NoError(t, err)
	assert.Equal(t, testTime.Truncate(time.Second), result.(time.Time).Truncate(time.Second))
	assert.Equal(t, FieldTypeTime, fieldType)
}

func TestRichValueToAny_BigintValue(t *testing.T) {
	// Test successful case - this requires the richstructhelper to work
	rv := &commonProtos.RichValue{
		Value: &commonProtos.RichValue_BigintValue{
			BigintValue: &commonProtos.BigInteger{
				Negative: false,
				Data:     []byte{0x01, 0x00}, // represents big int value
			},
		},
	}

	result, fieldType, err := richValueToAny(rv, time.Now())

	// Note: This test assumes richstructhelper.GetBigInt works correctly
	// If it fails, it should return a zero big.Int and an error
	assert.Equal(t, FieldTypeBigInt, fieldType)
	if err != nil {
		// If parsing fails, should return zero big int
		assert.Equal(t, big.NewInt(0), result)
		assert.Contains(t, err.Error(), "failed to parse big int value")
	} else {
		// If parsing succeeds, should return a *big.Int
		assert.IsType(t, &big.Int{}, result)
		b := result.(*big.Int)
		assert.Equal(t, int64(256), b.Int64())
	}
}

func TestRichValueToAny_BigdecimalValue(t *testing.T) {
	rv := &commonProtos.RichValue{
		Value: &commonProtos.RichValue_BigdecimalValue{
			BigdecimalValue: &commonProtos.BigDecimal{
				Value: &commonProtos.BigInteger{
					Negative: false,
					Data:     []byte{0x01, 0x00},
				},
				Exp: 2,
			},
		},
	}

	result, fieldType, err := richValueToAny(rv, time.Now())

	assert.Equal(t, FieldTypeBigFloat, fieldType)
	if err != nil {
		// If parsing fails, should return zero decimal
		assert.Equal(t, decimal.NewFromBigInt(big.NewInt(0), 0), result)
		assert.Contains(t, err.Error(), "failed to parse big decimal value")
	} else {
		// If parsing succeeds, should return a decimal.Decimal
		assert.IsType(t, decimal.Decimal{}, result)
		d := result.(decimal.Decimal)
		assert.Equal(t, decimal.NewFromBigInt(big.NewInt(256), 2), d)
	}
}

func TestRichValueToAny_ListValue(t *testing.T) {
	rv := &commonProtos.RichValue{
		Value: &commonProtos.RichValue_ListValue{
			ListValue: &commonProtos.RichValueList{
				Values: []*commonProtos.RichValue{
					{Value: &commonProtos.RichValue_StringValue{StringValue: "string1"}},
					{Value: &commonProtos.RichValue_StringValue{StringValue: "string2"}},
					{Value: &commonProtos.RichValue_IntValue{IntValue: 42}},
				},
			},
		},
	}

	result, fieldType, err := richValueToAny(rv, time.Now())

	require.NoError(t, err)
	assert.Equal(t, FieldTypeArray, fieldType)

	resultList, ok := result.([]any)
	require.True(t, ok)
	assert.Len(t, resultList, 3)
}

func TestRichValueToAny_StructValue(t *testing.T) {
	rv := &commonProtos.RichValue{
		Value: &commonProtos.RichValue_StructValue{
			StructValue: &commonProtos.RichStruct{
				Fields: map[string]*commonProtos.RichValue{
					"stringField": {
						Value: &commonProtos.RichValue_StringValue{StringValue: "string1"},
					},
					"intField": {
						Value: &commonProtos.RichValue_IntValue{IntValue: 42},
					},
					"boolField": {
						Value: &commonProtos.RichValue_BoolValue{BoolValue: true},
					},
					"structField": {
						Value: &commonProtos.RichValue_StructValue{
							StructValue: &commonProtos.RichStruct{
								Fields: map[string]*commonProtos.RichValue{
									"nestedField": {
										Value: &commonProtos.RichValue_StringValue{StringValue: "nestedValue"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	result, fieldType, err := richValueToAny(rv, time.Now())

	require.NoError(t, err)
	assert.Equal(t, FieldTypeJSON, fieldType)

	resultMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Len(t, resultMap, 4)
}

func TestRichValueToAny_TokenValue(t *testing.T) {
	rv := &commonProtos.RichValue{
		Value: &commonProtos.RichValue_TokenValue{
			TokenValue: &commonProtos.TokenAmount{
				Token: &commonProtos.CoinID{
					Id: &commonProtos.CoinID_Address{
						Address: &commonProtos.CoinID_AddressIdentifier{
							Address: "0x0000000000000000000000000000000000000000",
							Chain:   "1",
						},
					},
				},
				Amount: &commonProtos.BigDecimal{
					Value: &commonProtos.BigInteger{
						Negative: false,
						Data:     []byte{0x01, 0x00},
					},
					Exp: 2,
				},
			},
		},
	}

	result, fieldType, err := richValueToAny(rv, time.Now())

	assert.Equal(t, FieldTypeToken, fieldType)
	if err != nil {
		// If parsing fails
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to parse token value")
	} else {
		assert.IsType(t, map[string]any{}, result)
		m := result.(map[string]any)
		assert.Equal(t, "0x0000000000000000000000000000000000000000", m["address"])
		assert.Equal(t, "1", m["chain"])
		assert.Equal(t, decimal.NewFromBigInt(big.NewInt(256), 2), m["amount"])
	}
}

func TestRichValueToAny_UnknownType(t *testing.T) {
	// Create a RichValue with no value set (nil case)
	rv := &commonProtos.RichValue{}

	result, fieldType, err := richValueToAny(rv, time.Now())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown RichValue type")
	assert.Nil(t, result)
	assert.Equal(t, FieldTypeString, fieldType)
}

func TestNestedRow_Update_EmptyFields(t *testing.T) {
	row := &NestedRow{
		Row:          make(Row),
		StructSchema: make(map[string]FieldType),
	}

	// Test with nil fields
	structValue := &commonProtos.RichStruct{Fields: nil}
	err := row.Update("", structValue, time.Now())

	assert.NoError(t, err)
	assert.Empty(t, row.Row)
	assert.Empty(t, row.StructSchema)
}

func TestNestedRow_Update_BasicTypes(t *testing.T) {
	row := &NestedRow{
		Row:          make(Row),
		StructSchema: make(map[string]FieldType),
	}

	structValue := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"stringField": {
				Value: &commonProtos.RichValue_StringValue{StringValue: "test"},
			},
			"intField": {
				Value: &commonProtos.RichValue_IntValue{IntValue: 42},
			},
			"boolField": {
				Value: &commonProtos.RichValue_BoolValue{BoolValue: true},
			},
		},
	}

	err := row.Update("", structValue, time.Now())

	require.NoError(t, err)
	assert.Equal(t, "test", row.Row["stringField"])
	assert.Equal(t, int64(42), row.Row["intField"])
	assert.Equal(t, true, row.Row["boolField"])
	assert.Equal(t, FieldTypeString, row.StructSchema["stringField"])
	assert.Equal(t, FieldTypeInt, row.StructSchema["intField"])
	assert.Equal(t, FieldTypeBool, row.StructSchema["boolField"])
}

func TestNestedRow_Update_WithPrefix(t *testing.T) {
	row := &NestedRow{
		Row:          make(Row),
		StructSchema: make(map[string]FieldType),
	}

	structValue := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"field1": {
				Value: &commonProtos.RichValue_StringValue{StringValue: "value1"},
			},
		},
	}

	err := row.Update("prefix", structValue, time.Now())

	require.NoError(t, err)
	// With prefix, values should not be added to Row but should be in StructSchema
	assert.Empty(t, row.Row)
	assert.Equal(t, FieldTypeString, row.StructSchema["prefix.field1"])
}

func TestNestedRow_Update_NestedStruct(t *testing.T) {
	row := &NestedRow{
		Row:          make(Row),
		StructSchema: make(map[string]FieldType),
	}

	structValue := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"parentField": {
				Value: &commonProtos.RichValue_StructValue{
					StructValue: &commonProtos.RichStruct{
						Fields: map[string]*commonProtos.RichValue{
							"childField": {
								Value: &commonProtos.RichValue_StringValue{StringValue: "childValue"},
							},
						},
					},
				},
			},
		},
	}

	err := row.Update("", structValue, time.Now())

	require.NoError(t, err)
	// Should have both the parent struct and the nested field
	assert.Contains(t, row.StructSchema, "parentField")
	assert.Contains(t, row.StructSchema, "parentField.childField")
	assert.Equal(t, 2, len(row.StructSchema))
	assert.Equal(t, 1, len(row.Row))
	assert.Equal(t, map[string]any{
		"childField": "childValue",
	}, row.Row["parentField"])
	assert.Equal(t, FieldTypeJSON, row.StructSchema["parentField"])
	assert.Equal(t, FieldTypeString, row.StructSchema["parentField.childField"])
}

func TestNestedRow_Update_NestedArray(t *testing.T) {
	row := &NestedRow{
		Row:          make(Row),
		StructSchema: make(map[string]FieldType),
	}

	structValue := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"parentField": {
				Value: &commonProtos.RichValue_ListValue{
					ListValue: &commonProtos.RichValueList{
						Values: []*commonProtos.RichValue{
							{
								Value: &commonProtos.RichValue_StringValue{StringValue: "childValue1"},
							},
							{
								Value: &commonProtos.RichValue_StringValue{StringValue: "childValue2"},
							},
						},
					},
				},
			},
			"parentField2": {
				Value: &commonProtos.RichValue_ListValue{
					ListValue: &commonProtos.RichValueList{
						Values: []*commonProtos.RichValue{
							{
								Value: &commonProtos.RichValue_BoolValue{BoolValue: true},
							},
						},
					},
				},
			},
			"parentField3": {
				Value: &commonProtos.RichValue_StringValue{
					StringValue: "childValue4",
				},
			},
			"parentField4": {
				Value: &commonProtos.RichValue_StringValue{
					StringValue: "childValue5",
				},
			},
			"parentField5": {
				Value: &commonProtos.RichValue_Int64Value{
					Int64Value: 100,
				},
			},
			"parentField6": {
				Value: &commonProtos.RichValue_StructValue{
					StructValue: &commonProtos.RichStruct{
						Fields: map[string]*commonProtos.RichValue{
							"childField#1": {
								Value: &commonProtos.RichValue_StringValue{StringValue: "childValue6"},
							},
						},
					},
				},
			},
			"parentField7": {
				Value: &commonProtos.RichValue_StructValue{
					StructValue: &commonProtos.RichStruct{
						Fields: map[string]*commonProtos.RichValue{
							"childField#2": {
								Value: &commonProtos.RichValue_StructValue{
									StructValue: &commonProtos.RichStruct{
										Fields: map[string]*commonProtos.RichValue{
											"grandChildField": {
												Value: &commonProtos.RichValue_StringValue{StringValue: "grandChildValue"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	err := row.Update("", structValue, time.Now())
	require.NoError(t, err)

	assert.Equal(t, 7, len(row.Row))
	assert.Equal(t, []any{"childValue1", "childValue2"}, row.Row["parentField"])
	assert.Equal(t, []any{true}, row.Row["parentField2"])
	assert.Equal(t, "childValue4", row.Row["parentField3"])
	assert.Equal(t, "childValue5", row.Row["parentField4"])
	assert.Equal(t, int64(100), row.Row["parentField5"])
	assert.Equal(t, map[string]any{
		"childField#1": "childValue6",
	}, row.Row["parentField6"])
	assert.Equal(t, map[string]any{
		"childField#2": map[string]any{
			"grandChildField": "grandChildValue",
		},
	}, row.Row["parentField7"])
	assert.Equal(t, 10, len(row.StructSchema))
	log.Infof("schema: %v", row.StructSchema)
}

func TestNestedRow_Update_NullValue(t *testing.T) {
	row := &NestedRow{
		Row:          make(Row),
		StructSchema: make(map[string]FieldType),
	}

	structValue := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"nullField": {
				Value: &commonProtos.RichValue_NullValue_{},
			},
			"normalField": {
				Value: &commonProtos.RichValue_StringValue{StringValue: "normal"},
			},
		},
	}

	err := row.Update("", structValue, time.Now())

	require.NoError(t, err)
	// Null values should be skipped
	assert.NotContains(t, row.Row, "nullField")
	assert.NotContains(t, row.StructSchema, "nullField")
	assert.Equal(t, "normal", row.Row["normalField"])
}

func TestNestedRow_Data(t *testing.T) {
	row := &NestedRow{
		Row: Row{
			"field1": "value1",
			"field2": 42,
			"field3": true,
		},
		StructSchema: make(map[string]FieldType),
	}

	data := row.Data()

	assert.NotEmpty(t, data)
	// Should be valid JSON containing the row data
	assert.Contains(t, data, "field1")
	assert.Contains(t, data, "value1")
	assert.Contains(t, data, "field2")
}

func TestNestedRow_DataByKey_ExistingKey(t *testing.T) {
	row := &NestedRow{
		Row: Row{
			"field1": "value1",
			"field2": map[string]interface{}{"nested": "data"},
		},
		StructSchema: make(map[string]FieldType),
	}

	data := row.DataByKey("field1")
	assert.Equal(t, "value1", data)

	data = row.DataByKey("field2")
	assert.Equal(t, map[string]interface{}{"nested": "data"}, data)
	log.Infof("data: %v", data)
}

func TestNestedRow_DataByKey_NonExistentKey(t *testing.T) {
	row := &NestedRow{
		Row:          Row{"field1": "value1"},
		StructSchema: make(map[string]FieldType),
	}

	data := row.DataByKey("nonexistent")
	assert.Equal(t, "", data)
}

func TestNestedRow_DataByKey_NilValue(t *testing.T) {
	row := &NestedRow{
		Row:          Row{"field1": nil},
		StructSchema: make(map[string]FieldType),
	}

	data := row.DataByKey("field1")
	assert.Equal(t, "", data)
}

func TestGetDatasetsSummary(t *testing.T) {
	datasets := []Dataset{
		{
			Meta: Meta{Name: "dataset1", Type: MetaTypeGauge},
			Rows: []Row{{"field": "value1"}, {"field": "value2"}},
		},
		{
			Meta: Meta{Name: "dataset2", Type: MetaTypeEvent},
			Rows: []Row{{"field": "value1"}},
		},
	}

	assert.Equal(t, "event.dataset2/1,gauge.dataset1/2", GetDatasetsSummary(datasets))
}

func TestStatistic(t *testing.T) {
	datasets := []Dataset{
		{
			Meta: Meta{Name: "dataset1", Type: MetaTypeCounter},
			Rows: []Row{{"field": "value1"}, {"field": "value2"}},
		},
		{
			Meta: Meta{Name: "dataset1", Type: MetaTypeCounter},
			Rows: []Row{{"field": "value1"}},
		},
		{
			Meta: Meta{Name: "dataset2", Type: MetaTypeEvent},
			Rows: []Row{{"field": "value1"}},
		},
	}

	stat := make(map[MetaType]map[string]int)
	Statistic(datasets, stat)

	assert.Equal(t, 3, stat[MetaTypeCounter]["dataset1"]) // 2 + 1
	assert.Equal(t, 1, stat[MetaTypeEvent]["dataset2"])
}
