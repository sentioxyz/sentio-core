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

func TestUpdateEvents_BasicStringField(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"testField": {
				Value: &commonProtos.RichValue_StringValue{StringValue: "test value"},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, "test value", row["testField"])
	assert.Equal(t, Field{Name: "testField", Type: FieldTypeString, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["testField"])
}

func TestUpdateEvents_IntField(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"intField": {
				Value: &commonProtos.RichValue_IntValue{IntValue: 42},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, int64(42), row["intField"])
	assert.Equal(t, Field{Name: "intField", Type: FieldTypeInt, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["intField"])
}

func TestUpdateEvents_Int64Field(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"int64Field": {
				Value: &commonProtos.RichValue_Int64Value{Int64Value: 9223372036854775807},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, int64(9223372036854775807), row["int64Field"])
	assert.Equal(t, Field{Name: "int64Field", Type: FieldTypeInt, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["int64Field"])
}

func TestUpdateEvents_BoolField(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"boolField": {
				Value: &commonProtos.RichValue_BoolValue{BoolValue: true},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, true, row["boolField"])
	assert.Equal(t, Field{Name: "boolField", Type: FieldTypeBool, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["boolField"])
}

func TestUpdateEvents_FloatField(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"floatField": {
				Value: &commonProtos.RichValue_FloatValue{FloatValue: 3.14},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, 3.14, row["floatField"])
	assert.Equal(t, Field{Name: "floatField", Type: FieldTypeFloat, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["floatField"])
}

func TestUpdateEvents_BytesField(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()
	testBytes := []byte{0x01, 0x02, 0x03}

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"bytesField": {
				Value: &commonProtos.RichValue_BytesValue{BytesValue: testBytes},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, hexutil.Encode(testBytes), row["bytesField"])
	assert.Equal(t, Field{Name: "bytesField", Type: FieldTypeString, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["bytesField"])
}

func TestUpdateEvents_TimestampField(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()
	testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"timestampField": {
				Value: &commonProtos.RichValue_TimestampValue{
					TimestampValue: timestamppb.New(testTime),
				},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, testTime, row["timestampField"])
	assert.Equal(t, Field{Name: "timestampField", Type: FieldTypeTime, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["timestampField"])
}

func TestUpdateEvents_BigintField(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"bigintField": {
				Value: &commonProtos.RichValue_BigintValue{
					BigintValue: &commonProtos.BigInteger{
						Negative: false,
						Data:     []byte{0x01, 0x00},
					},
				},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, Field{Name: "bigintField", Type: FieldTypeBigInt, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["bigintField"])
	// Note: The actual value depends on richstructhelper.GetBigInt implementation
	bigIntVal, ok := row["bigintField"].(*big.Int)
	require.True(t, ok)
	assert.NotNil(t, bigIntVal)
	assert.Equal(t, int64(256), bigIntVal.Int64())
}

func TestUpdateEvents_BigdecimalField(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"bigdecimalField": {
				Value: &commonProtos.RichValue_BigdecimalValue{
					BigdecimalValue: &commonProtos.BigDecimal{
						Value: &commonProtos.BigInteger{
							Negative: false,
							Data:     []byte{0x01, 0x00},
						},
						Exp: 2,
					},
				},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, Field{Name: "bigdecimalField", Type: FieldTypeBigFloat, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["bigdecimalField"])
	// Note: The actual value depends on richstructhelper.GetBigDecimal implementation
	bigFloatVal, ok := row["bigdecimalField"].(decimal.Decimal)
	require.True(t, ok)
	assert.NotNil(t, bigFloatVal)
}

func TestUpdateEvents_ListValue(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"listField": {
				Value: &commonProtos.RichValue_ListValue{
					ListValue: &commonProtos.RichValueList{
						Values: []*commonProtos.RichValue{
							{Value: &commonProtos.RichValue_StringValue{StringValue: "item1"}},
							{Value: &commonProtos.RichValue_StringValue{StringValue: "item2"}},
							{Value: &commonProtos.RichValue_Int64Value{Int64Value: 1}},
						},
					},
				},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, Field{Name: "listField", Type: FieldTypeArray, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["listField"])
	assert.NotNil(t, row["listField"])
	assert.Equal(t, []any{"item1", "item2", int64(1)}, row["listField"])
}

func TestUpdateEvents_StructValue(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"structField": {
				Value: &commonProtos.RichValue_StructValue{
					StructValue: &commonProtos.RichStruct{
						Fields: map[string]*commonProtos.RichValue{
							"nestedField": {
								Value: &commonProtos.RichValue_StringValue{StringValue: "nested value"},
							},
							"middleField": {
								Value: &commonProtos.RichValue_StructValue{
									StructValue: &commonProtos.RichStruct{
										Fields: map[string]*commonProtos.RichValue{
											"leafField": {
												Value: &commonProtos.RichValue_ListValue{
													ListValue: &commonProtos.RichValueList{
														Values: []*commonProtos.RichValue{
															{Value: &commonProtos.RichValue_StringValue{StringValue: "leaf value 1"}},
															{Value: &commonProtos.RichValue_StringValue{StringValue: "leaf value 2"}},
															{Value: &commonProtos.RichValue_Int64Value{Int64Value: 1}},
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
				},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, Field{Name: "structField", Type: FieldTypeJSON, NestedStructSchema: map[string]FieldType{
		"nestedField":           FieldTypeString,
		"middleField":           FieldTypeJSON,
		"middleField.leafField": FieldTypeArray,
	}, NestedIndex: make(map[string]FieldType)}, meta.Fields["structField"])
	assert.NotNil(t, row["structField"])
	assert.IsType(t, "", row["structField"])
	log.Infof("struct value: %v", row["structField"])
	// The actual structure depends on NestedRow.Data() implementation
	// which returns a JSON string representation
}

func TestUpdateEvents_StructValue_Merge(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name: "test_event",
		Type: MetaTypeEvent,
		Fields: map[string]Field{
			"structField": {
				Name: "structField",
				Type: FieldTypeJSON,
				NestedStructSchema: map[string]FieldType{
					"nestedField": FieldTypeString,
				},
			},
		},
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"structField": {
				Value: &commonProtos.RichValue_StructValue{
					StructValue: &commonProtos.RichStruct{
						Fields: map[string]*commonProtos.RichValue{
							"nestedField": {
								Value: &commonProtos.RichValue_StringValue{StringValue: "nested value"},
							},
							"middleField": {
								Value: &commonProtos.RichValue_StructValue{
									StructValue: &commonProtos.RichStruct{
										Fields: map[string]*commonProtos.RichValue{
											"leafField": {
												Value: &commonProtos.RichValue_ListValue{
													ListValue: &commonProtos.RichValueList{
														Values: []*commonProtos.RichValue{
															{Value: &commonProtos.RichValue_StringValue{StringValue: "leaf value 1"}},
															{Value: &commonProtos.RichValue_StringValue{StringValue: "leaf value 2"}},
															{Value: &commonProtos.RichValue_Int64Value{Int64Value: 1}},
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
				},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, Field{Name: "structField", Type: FieldTypeJSON, NestedStructSchema: map[string]FieldType{
		"nestedField":           FieldTypeString,
		"middleField":           FieldTypeJSON,
		"middleField.leafField": FieldTypeArray,
	},
		NestedIndex: make(map[string]FieldType)}, meta.Fields["structField"])
	assert.NotNil(t, row["structField"])
	assert.IsType(t, "", row["structField"])
	log.Infof("struct value: %v", row["structField"])
	// The actual structure depends on NestedRow.Data() implementation
	// which returns a JSON string representation
}

func TestUpdateEvents_TokenValue(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	now := time.Now().In(time.UTC)
	blockTime := now

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"tokenField": {
				Value: &commonProtos.RichValue_TokenValue{
					TokenValue: &commonProtos.TokenAmount{
						Token: &commonProtos.CoinID{
							Id: &commonProtos.CoinID_Address{
								Address: &commonProtos.CoinID_AddressIdentifier{
									Address: "0x123",
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
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, Field{Name: "tokenField", Type: FieldTypeToken, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["tokenField"])
	assert.Equal(t, map[string]any{
		"address":   "0x123",
		"chain":     "1",
		"amount":    decimal.NewFromBigInt(big.NewInt(256), 2),
		"timestamp": now,
		"symbol":    "",
	}, row["tokenField"])
}

func TestUpdateEvents_NullValue_Skipped(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"nullField": {
				Value: &commonProtos.RichValue_NullValue_{},
			},
			"normalField": {
				Value: &commonProtos.RichValue_StringValue{StringValue: "normal"},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	// Null field should be skipped
	assert.NotContains(t, row, "nullField")
	assert.NotContains(t, meta.Fields, "nullField")
	// Normal field should be processed
	assert.Equal(t, "normal", row["normalField"])
}

func TestUpdateEvents_SeverityFieldRenamed(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"severity": {
				Value: &commonProtos.RichValue_StringValue{StringValue: "high"},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	// Field should be renamed to meta.severity
	assert.Equal(t, "high", row["meta.severity"])
	assert.Equal(t, Field{Name: "meta.severity", Type: FieldTypeString, BuiltIn: true, NestedStructSchema: make(map[string]FieldType), NestedIndex: make(map[string]FieldType)}, meta.Fields["meta.severity"])
}

func TestUpdateEvents_DistinctEntityIdRenamed(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"distinctEntityId": {
				Value: &commonProtos.RichValue_StringValue{StringValue: "user123"},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	// Field should be renamed to distinctId
	assert.Equal(t, "user123", row["distinctId"])
	assert.Equal(t, Field{Name: "distinctId",
		Type:               FieldTypeString,
		BuiltIn:            true,
		NestedStructSchema: make(map[string]FieldType),
		NestedIndex:        make(map[string]FieldType),
	}, meta.Fields["distinctId"])
}

func TestUpdateEvents_DistinctEntityId_Empty(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"distinctEntityId": {
				Value: &commonProtos.RichValue_StringValue{StringValue: ""},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	// Empty distinctEntityId should be skipped
	assert.Contains(t, row, "distinctId")
	assert.Contains(t, meta.Fields, "distinctId")
}

func TestUpdateEvents_DistinctEntityId_NullSkipped(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"distinctEntityId": {
				Value: &commonProtos.RichValue_NullValue_{},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	// Null distinctEntityId should be skipped
	assert.NotContains(t, row, "distinctId")
	assert.NotContains(t, meta.Fields, "distinctId")
}

func TestUpdateEvents_ReservedFieldError(t *testing.T) {
	row := make(Row)
	row["testField"] = "existing value" // Pre-populate row with reserved field
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"testField": {
				Value: &commonProtos.RichValue_StringValue{StringValue: "new value"},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "testField is reserved")
}

func TestUpdateEvents_CompatibleFieldTypes(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name: "test_event",
		Type: MetaTypeEvent,
		Fields: map[string]Field{
			"existingField": {Name: "existingField", Type: FieldTypeString},
		},
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"existingField": {
				Value: &commonProtos.RichValue_StringValue{StringValue: "new value"},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, "new value", row["existingField"])
	assert.Equal(t, Field{Name: "existingField",
		Type:               FieldTypeString,
		NestedStructSchema: make(map[string]FieldType),
		NestedIndex:        make(map[string]FieldType),
	},
		meta.Fields["existingField"])
}

func TestUpdateEvents_IncompatibleFieldTypes(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name: "test_event",
		Type: MetaTypeEvent,
		Fields: map[string]Field{
			"existingField": {Name: "existingField", Type: FieldTypeString},
		},
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"existingField": {
				Value: &commonProtos.RichValue_IntValue{IntValue: 42},
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "types of fields")
	assert.Contains(t, err.Error(), "are not uniform")
	assert.Contains(t, err.Error(), "String")
	assert.Contains(t, err.Error(), "Int")
}

func TestUpdateEvents_MultipleFields(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
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

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Equal(t, "test", row["stringField"])
	assert.Equal(t, int64(42), row["intField"])
	assert.Equal(t, true, row["boolField"])
	assert.Len(t, meta.Fields, 3)
}

func TestUpdateEvents_EmptyData(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	data := &commonProtos.RichStruct{
		Fields: nil,
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.NoError(t, err)
	assert.Empty(t, row)
	assert.Empty(t, meta.Fields)
}

func TestUpdateEvents_UnsupportedValueType(t *testing.T) {
	row := make(Row)
	meta := &Meta{
		Name:   "test_event",
		Type:   MetaTypeEvent,
		Fields: make(map[string]Field),
	}
	blockTime := time.Now()

	// Create a RichValue with no value set to trigger default case
	data := &commonProtos.RichStruct{
		Fields: map[string]*commonProtos.RichValue{
			"invalidField": {
				// No Value set - should trigger default case
			},
		},
	}

	err := UpdateEvents(data, &row, meta, blockTime)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalidField has invalid type")
}
