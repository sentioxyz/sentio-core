package persistent

import (
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"math/big"
	rsh "sentioxyz/sentio-core/common/richstructhelper"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	pb "sentioxyz/sentio-core/service/common/protos"
	"testing"
	"time"
)

func Test_convertRichStruct_normal(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	var bigNum big.Int
	bigNum.SetInt64(456)
	bigDec := decimal.NewFromFloat(123.456)
	ts := time.Unix(1111111111, 111111000)

	edType := sch.GetEntity("EntityD")
	box := EntityBox{
		Data: map[string]any{
			"id": "id",

			"propA1": "pa",
			"propB1": "0x0102030405060708090a",
			"propC1": true,
			"propD1": int32(123),
			"propE1": &bigNum,
			"propF1": bigDec,
			"propG1": "AAA",
			"propH1": ts.UnixMicro(),
			"propI1": float64(456.789),
			"propJ1": int64(999999999999999999),

			"propA2": utils.WrapPointer("pa"),
			"propB2": utils.WrapPointer("0x0102030405060708090a"),
			"propC2": utils.WrapPointer(true),
			"propD2": utils.WrapPointer(int32(123)),
			"propE2": &bigNum,
			"propF2": &bigDec,
			"propG2": utils.WrapPointer("AAA"),
			"propH2": utils.WrapPointer(ts.UnixMicro()),
			"propI2": utils.WrapPointer(float64(456.789)),
			"propJ2": utils.WrapPointer(int64(999999999999999999)),

			"propA3": []string{"pa1", "pa2"},
			"propB3": []string{"0x0102030405060708090a", "0x0102030405060708090b"},
			"propC3": []bool{true, false},
			"propD3": []int32{1, 23, 456},
			"propE3": []*big.Int{&bigNum},
			"propF3": []decimal.Decimal{bigDec},
			"propG3": []string{"AAA", "BBB"},
			"propH3": []int64{ts.UnixMicro()},
			"propI3": []float64{456.789},
			"propJ3": []int64{1, 23, 456},

			"propA4": utils.WrapPointerForArray([]string{"pa1", "pa2"}),
			"propB4": utils.WrapPointerForArray([]string{"0x01", "0x02"}),
			"propC4": utils.WrapPointerForArray([]bool{true, false}),
			"propD4": utils.WrapPointerForArray([]int32{1, 23, 456}),
			"propE4": []*big.Int{&bigNum},
			"propF4": []*decimal.Decimal{&bigDec},
			"propG4": utils.WrapPointerForArray([]string{"AAA", "BBB"}),
			"propH4": utils.WrapPointerForArray([]int64{ts.UnixMicro()}),
			"propI4": utils.WrapPointerForArray([]float64{456.789}),
			"propJ4": utils.WrapPointerForArray([]int64{1, 23, 456}),

			"propA5": utils.WrapPointerForArray([]string{"pa1", "pa2"}),
			"propB5": utils.WrapPointerForArray([]string{"0x01", "0x02"}),
			"propC5": utils.WrapPointerForArray([]bool{true, false}),
			"propD5": utils.WrapPointerForArray([]int32{1, 23, 456}),
			"propE5": []*big.Int{&bigNum},
			"propF5": []*decimal.Decimal{&bigDec},
			"propG5": utils.WrapPointerForArray([]string{"AAA", "BBB"}),
			"propH5": utils.WrapPointerForArray([]int64{ts.UnixMicro()}),
			"propI5": utils.WrapPointerForArray([]float64{456.789}),
			"propJ5": utils.WrapPointerForArray([]int64{1, 23, 456}),

			"propA6": []string{"pa1", "pa2"},
			"propB6": []string{"0x01", "0x02"},
			"propC6": []bool{true, false},
			"propD6": []int32{1, 23, 456},
			"propE6": []*big.Int{&bigNum},
			"propF6": []decimal.Decimal{bigDec},
			"propG6": []string{"AAA", "BBB"},
			"propH6": []int64{ts.UnixMicro()},
			"propI6": []float64{456.789},
			"propJ6": []int64{1, 23, 456},

			"propA7": [][]string{{"pa1", "pa2"}, {"pa3"}},
			"propB7": [][]string{{"0x01"}, {"0x02", "0x03"}},
			"propC7": [][]bool{{true, false}, {}, {false}},
			"propD7": [][]int32{{1, 23, 456}, {123, 45, 6}, {}},
			"propE7": [][]*big.Int{{&bigNum}},
			"propF7": [][]decimal.Decimal{{bigDec}},
			"propG7": [][]string{{"AAA"}, {"BBB", "CCC"}},
			"propH7": [][]int64{{ts.UnixMicro()}},
			"propI7": [][]float64{{456.789}},
			"propJ7": [][]int64{{1, 23, 456}, {123, 45, 6}, {}},

			"propA8": [][]string(nil),
			"propB8": [][]string(nil),
			"propC8": [][]bool{{true, false}, nil, {false}},
			"propD8": [][]int32{{1, 23, 456}, {123, 45, 6}, nil},
			"propE8": [][]*big.Int(nil),
			"propF8": [][]decimal.Decimal(nil),
			"propG8": [][]string(nil),
			"propH8": [][]int64(nil),
			"propI8": [][]float64(nil),
			"propJ8": [][]int64{{1, 23, 456}, {123, 45, 6}, nil},

			"foreign1": "fk1",
			"foreign2": utils.WrapPointer("fk2"),
			"foreign3": []string{"fk3", "fk4"},
			"foreign4": utils.WrapPointerForArray([]string{"fk5", "fk6"}),
			"foreign5": utils.WrapPointerForArray([]string{"fk7", "fk8"}),
			"foreign6": []string{"fk9", "fk10"},
		},
	}
	data := &pb.RichStruct{Fields: map[string]*pb.RichValue{
		"id": rsh.NewStringValue("id"),

		"propA1": rsh.NewStringValue("pa"),
		"propB1": rsh.NewBytesValue([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}),
		"propC1": rsh.NewBoolValue(true),
		"propD1": rsh.NewIntValue(123),
		"propE1": rsh.NewBigIntValue(&bigNum),
		"propF1": rsh.NewBigDecimalValue(bigDec),
		"propG1": rsh.NewStringValue("AAA"),
		"propH1": rsh.NewTimestampValue(ts),
		"propI1": rsh.NewFloatValue(456.789),
		"propJ1": rsh.NewInt64Value(999999999999999999),

		"propA2": rsh.NewStringValue("pa"),
		"propB2": rsh.NewBytesValue([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}),
		"propC2": rsh.NewBoolValue(true),
		"propD2": rsh.NewIntValue(123),
		"propE2": rsh.NewBigIntValue(&bigNum),
		"propF2": rsh.NewBigDecimalValue(bigDec),
		"propG2": rsh.NewStringValue("AAA"),
		"propH2": rsh.NewTimestampValue(ts),
		"propI2": rsh.NewFloatValue(456.789),
		"propJ2": rsh.NewInt64Value(999999999999999999),

		"propA3": rsh.NewListValue(rsh.NewStringValue("pa1"), rsh.NewStringValue("pa2")),
		"propB3": rsh.NewListValue(rsh.NewBytesValue([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}), rsh.NewBytesValue([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 11})),
		"propC3": rsh.NewListValue(rsh.NewBoolValue(true), rsh.NewBoolValue(false)),
		"propD3": rsh.NewListValue(rsh.NewIntValue(1), rsh.NewIntValue(23), rsh.NewIntValue(456)),
		"propE3": rsh.NewListValue(rsh.NewBigIntValue(&bigNum)),
		"propF3": rsh.NewListValue(rsh.NewBigDecimalValue(bigDec)),
		"propG3": rsh.NewListValue(rsh.NewStringValue("AAA"), rsh.NewStringValue("BBB")),
		"propH3": rsh.NewListValue(rsh.NewTimestampValue(ts)),
		"propI3": rsh.NewListValue(rsh.NewFloatValue(456.789)),
		"propJ3": rsh.NewListValue(rsh.NewInt64Value(1), rsh.NewInt64Value(23), rsh.NewInt64Value(456)),

		"propA4": rsh.NewListValue(rsh.NewStringValue("pa1"), rsh.NewStringValue("pa2")),
		"propB4": rsh.NewListValue(rsh.NewBytesValue([]byte{1}), rsh.NewBytesValue([]byte{2})),
		"propC4": rsh.NewListValue(rsh.NewBoolValue(true), rsh.NewBoolValue(false)),
		"propD4": rsh.NewListValue(rsh.NewIntValue(1), rsh.NewIntValue(23), rsh.NewIntValue(456)),
		"propE4": rsh.NewListValue(rsh.NewBigIntValue(&bigNum)),
		"propF4": rsh.NewListValue(rsh.NewBigDecimalValue(bigDec)),
		"propG4": rsh.NewListValue(rsh.NewStringValue("AAA"), rsh.NewStringValue("BBB")),
		"propH4": rsh.NewListValue(rsh.NewTimestampValue(ts)),
		"propI4": rsh.NewListValue(rsh.NewFloatValue(456.789)),
		"propJ4": rsh.NewListValue(rsh.NewInt64Value(1), rsh.NewInt64Value(23), rsh.NewInt64Value(456)),

		"propA5": rsh.NewListValue(rsh.NewStringValue("pa1"), rsh.NewStringValue("pa2")),
		"propB5": rsh.NewListValue(rsh.NewBytesValue([]byte{1}), rsh.NewBytesValue([]byte{2})),
		"propC5": rsh.NewListValue(rsh.NewBoolValue(true), rsh.NewBoolValue(false)),
		"propD5": rsh.NewListValue(rsh.NewIntValue(1), rsh.NewIntValue(23), rsh.NewIntValue(456)),
		"propE5": rsh.NewListValue(rsh.NewBigIntValue(&bigNum)),
		"propF5": rsh.NewListValue(rsh.NewBigDecimalValue(bigDec)),
		"propG5": rsh.NewListValue(rsh.NewStringValue("AAA"), rsh.NewStringValue("BBB")),
		"propH5": rsh.NewListValue(rsh.NewTimestampValue(ts)),
		"propI5": rsh.NewListValue(rsh.NewFloatValue(456.789)),
		"propJ5": rsh.NewListValue(rsh.NewInt64Value(1), rsh.NewInt64Value(23), rsh.NewInt64Value(456)),

		"propA6": rsh.NewListValue(rsh.NewStringValue("pa1"), rsh.NewStringValue("pa2")),
		"propB6": rsh.NewListValue(rsh.NewBytesValue([]byte{1}), rsh.NewBytesValue([]byte{2})),
		"propC6": rsh.NewListValue(rsh.NewBoolValue(true), rsh.NewBoolValue(false)),
		"propD6": rsh.NewListValue(rsh.NewIntValue(1), rsh.NewIntValue(23), rsh.NewIntValue(456)),
		"propE6": rsh.NewListValue(rsh.NewBigIntValue(&bigNum)),
		"propF6": rsh.NewListValue(rsh.NewBigDecimalValue(bigDec)),
		"propG6": rsh.NewListValue(rsh.NewStringValue("AAA"), rsh.NewStringValue("BBB")),
		"propH6": rsh.NewListValue(rsh.NewTimestampValue(ts)),
		"propI6": rsh.NewListValue(rsh.NewFloatValue(456.789)),
		"propJ6": rsh.NewListValue(rsh.NewInt64Value(1), rsh.NewInt64Value(23), rsh.NewInt64Value(456)),

		"propA7": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewStringValue("pa1"),
				rsh.NewStringValue("pa2"),
			),
			rsh.NewListValue(
				rsh.NewStringValue("pa3"),
			),
		),
		"propB7": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewBytesValue([]byte{1}),
			),
			rsh.NewListValue(
				rsh.NewBytesValue([]byte{2}),
				rsh.NewBytesValue([]byte{3}),
			),
		),
		"propC7": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewBoolValue(true),
				rsh.NewBoolValue(false),
			),
			rsh.NewListValue(),
			rsh.NewListValue(
				rsh.NewBoolValue(false),
			),
		),
		"propD7": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewIntValue(1),
				rsh.NewIntValue(23),
				rsh.NewIntValue(456),
			),
			rsh.NewListValue(
				rsh.NewIntValue(123),
				rsh.NewIntValue(45),
				rsh.NewIntValue(6),
			),
			rsh.NewListValue(),
		),
		"propE7": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewBigIntValue(&bigNum),
			),
		),
		"propF7": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewBigDecimalValue(bigDec),
			),
		),
		"propG7": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewStringValue("AAA"),
			),
			rsh.NewListValue(
				rsh.NewStringValue("BBB"),
				rsh.NewStringValue("CCC"),
			),
		),
		"propH7": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewTimestampValue(ts),
			),
		),
		"propI7": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewFloatValue(456.789),
			),
		),
		"propJ7": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewInt64Value(1),
				rsh.NewInt64Value(23),
				rsh.NewInt64Value(456),
			),
			rsh.NewListValue(
				rsh.NewInt64Value(123),
				rsh.NewInt64Value(45),
				rsh.NewInt64Value(6),
			),
			rsh.NewListValue(),
		),

		"propA8": rsh.NewNullValue(),
		"propB8": rsh.NewNullValue(),
		"propC8": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewBoolValue(true),
				rsh.NewBoolValue(false),
			),
			rsh.NewNullValue(),
			rsh.NewListValue(
				rsh.NewBoolValue(false),
			),
		),
		"propD8": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewIntValue(1),
				rsh.NewIntValue(23),
				rsh.NewIntValue(456),
			),
			rsh.NewListValue(
				rsh.NewIntValue(123),
				rsh.NewIntValue(45),
				rsh.NewIntValue(6),
			),
			rsh.NewNullValue(),
		),
		"propE8": rsh.NewNullValue(),
		"propF8": rsh.NewNullValue(),
		"propG8": rsh.NewNullValue(),
		"propH8": rsh.NewNullValue(),
		"propI8": rsh.NewNullValue(),
		"propJ8": rsh.NewListValue(
			rsh.NewListValue(
				rsh.NewInt64Value(1),
				rsh.NewInt64Value(23),
				rsh.NewInt64Value(456),
			),
			rsh.NewListValue(
				rsh.NewInt64Value(123),
				rsh.NewInt64Value(45),
				rsh.NewInt64Value(6),
			),
			rsh.NewNullValue(),
		),

		"foreign1": rsh.NewStringValue("fk1"),
		"foreign2": rsh.NewStringValue("fk2"),
		"foreign3": rsh.NewListValue(
			rsh.NewStringValue("fk3"),
			rsh.NewStringValue("fk4"),
		),
		"foreign4": rsh.NewListValue(
			rsh.NewStringValue("fk5"),
			rsh.NewStringValue("fk6"),
		),
		"foreign5": rsh.NewListValue(
			rsh.NewStringValue("fk7"),
			rsh.NewStringValue("fk8"),
		),
		"foreign6": rsh.NewListValue(
			rsh.NewStringValue("fk9"),
			rsh.NewStringValue("fk10"),
		),
	}}

	assert.NoError(t, err)

	var d *pb.RichStruct
	d, err = box.ToRichStruct(edType)
	assert.NoError(t, err)
	assert.Equal(t, data, d)

	var e EntityBox
	assert.NoError(t, e.FromRichStruct(edType, d))
	assert.Equal(t, box.Data, e.Data)

	//fmt.Println(utils.MustJSONMarshal(box))
}

func Test_convertRichStruct_zero(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	edType := sch.GetEntity("EntityD")
	box := EntityBox{
		Data: map[string]any{
			"id": "id",

			"propA1": "",
			"propB1": "0x",
			"propC1": false,
			"propD1": int32(0),
			"propE1": big.NewInt(0),
			"propF1": decimal.Zero,
			"propG1": "",
			"propH1": int64(0),
			"propI1": float64(0),
			"propJ1": int64(0),

			"propA2": (*string)(nil),
			"propB2": (*string)(nil),
			"propC2": (*bool)(nil),
			"propD2": (*int32)(nil),
			"propE2": (*big.Int)(nil),
			"propF2": (*decimal.Decimal)(nil),
			"propG2": (*string)(nil),
			"propH2": (*int64)(nil),
			"propI2": (*float64)(nil),
			"propJ2": (*int64)(nil),

			"propA3": []string(nil),
			"propB3": []string(nil),
			"propC3": []bool(nil),
			"propD3": []int32(nil),
			"propE3": []*big.Int(nil),
			"propF3": []decimal.Decimal(nil),
			"propG3": []string(nil),
			"propH3": []int64(nil),
			"propI3": []float64(nil),
			"propJ3": []int64(nil),

			"propA4": []*string(nil),
			"propB4": []*string(nil),
			"propC4": []*bool(nil),
			"propD4": []*int32(nil),
			"propE4": []*big.Int(nil),
			"propF4": []*decimal.Decimal(nil),
			"propG4": []*string(nil),
			"propH4": []*int64(nil),
			"propI4": []*float64(nil),
			"propJ4": []*int64(nil),

			"propA5": []*string{},
			"propB5": []*string{},
			"propC5": []*bool{},
			"propD5": []*int32{},
			"propE5": []*big.Int{},
			"propF5": []*decimal.Decimal{},
			"propG5": []*string{},
			"propH5": []*int64{},
			"propI5": []*float64{},
			"propJ5": []*int64{},

			"propA6": []string{},
			"propB6": []string{},
			"propC6": []bool{},
			"propD6": []int32{},
			"propE6": []*big.Int{},
			"propF6": []decimal.Decimal{},
			"propG6": []string{},
			"propH6": []int64{},
			"propI6": []float64{},
			"propJ6": []int64{},

			"propA7": [][]string(nil),
			"propB7": [][]string(nil),
			"propC7": [][]bool(nil),
			"propD7": [][]int32(nil),
			"propE7": [][]*big.Int(nil),
			"propF7": [][]decimal.Decimal(nil),
			"propG7": [][]string(nil),
			"propH7": [][]int64(nil),
			"propI7": [][]float64(nil),
			"propJ7": [][]int64(nil),

			"propA8": [][]string(nil),
			"propB8": [][]string(nil),
			"propC8": [][]bool{nil, nil},
			"propD8": [][]int32{nil, nil},
			"propE8": [][]*big.Int(nil),
			"propF8": [][]decimal.Decimal(nil),
			"propG8": [][]string(nil),
			"propH8": [][]int64(nil),
			"propI8": [][]float64(nil),
			"propJ8": [][]int64{nil, nil},

			"foreign1": "",
			"foreign2": (*string)(nil),
			"foreign3": []string(nil),
			"foreign4": []*string(nil),
			"foreign5": []*string{},
			"foreign6": []string{},
		},
	}
	data := &pb.RichStruct{Fields: map[string]*pb.RichValue{
		"id": rsh.NewStringValue("id"),

		"propA1": rsh.NewStringValue(""),
		"propB1": rsh.NewBytesValue([]byte{}),
		"propC1": rsh.NewBoolValue(false),
		"propD1": rsh.NewIntValue(0),
		"propE1": rsh.NewBigIntValue(big.NewInt(0)),
		"propF1": rsh.NewBigDecimalValue(decimal.Zero),
		"propG1": rsh.NewStringValue(""),
		"propH1": rsh.NewTimestampValue(time.UnixMicro(0)),
		"propI1": rsh.NewFloatValue(0),
		"propJ1": rsh.NewInt64Value(0),

		"propA2": rsh.NewNullValue(),
		"propB2": rsh.NewNullValue(),
		"propC2": rsh.NewNullValue(),
		"propD2": rsh.NewNullValue(),
		"propE2": rsh.NewNullValue(),
		"propF2": rsh.NewNullValue(),
		"propG2": rsh.NewNullValue(),
		"propH2": rsh.NewNullValue(),
		"propI2": rsh.NewNullValue(),
		"propJ2": rsh.NewNullValue(),

		"propA3": rsh.NewNullValue(),
		"propB3": rsh.NewNullValue(),
		"propC3": rsh.NewNullValue(),
		"propD3": rsh.NewNullValue(),
		"propE3": rsh.NewNullValue(),
		"propF3": rsh.NewNullValue(),
		"propG3": rsh.NewNullValue(),
		"propH3": rsh.NewNullValue(),
		"propI3": rsh.NewNullValue(),
		"propJ3": rsh.NewNullValue(),

		"propA4": rsh.NewNullValue(),
		"propB4": rsh.NewNullValue(),
		"propC4": rsh.NewNullValue(),
		"propD4": rsh.NewNullValue(),
		"propE4": rsh.NewNullValue(),
		"propF4": rsh.NewNullValue(),
		"propG4": rsh.NewNullValue(),
		"propH4": rsh.NewNullValue(),
		"propI4": rsh.NewNullValue(),
		"propJ4": rsh.NewNullValue(),

		"propA5": rsh.NewListValue(),
		"propB5": rsh.NewListValue(),
		"propC5": rsh.NewListValue(),
		"propD5": rsh.NewListValue(),
		"propE5": rsh.NewListValue(),
		"propF5": rsh.NewListValue(),
		"propG5": rsh.NewListValue(),
		"propH5": rsh.NewListValue(),
		"propI5": rsh.NewListValue(),
		"propJ5": rsh.NewListValue(),

		"propA6": rsh.NewListValue(),
		"propB6": rsh.NewListValue(),
		"propC6": rsh.NewListValue(),
		"propD6": rsh.NewListValue(),
		"propE6": rsh.NewListValue(),
		"propF6": rsh.NewListValue(),
		"propG6": rsh.NewListValue(),
		"propH6": rsh.NewListValue(),
		"propI6": rsh.NewListValue(),
		"propJ6": rsh.NewListValue(),

		"propA7": rsh.NewNullValue(),
		"propB7": rsh.NewNullValue(),
		"propC7": rsh.NewNullValue(),
		"propD7": rsh.NewNullValue(),
		"propE7": rsh.NewNullValue(),
		"propF7": rsh.NewNullValue(),
		"propG7": rsh.NewNullValue(),
		"propH7": rsh.NewNullValue(),
		"propI7": rsh.NewNullValue(),
		"propJ7": rsh.NewNullValue(),

		"propA8": rsh.NewNullValue(),
		"propB8": rsh.NewNullValue(),
		"propC8": rsh.NewListValue(
			rsh.NewNullValue(),
			rsh.NewNullValue(),
		),
		"propD8": rsh.NewListValue(
			rsh.NewNullValue(),
			rsh.NewNullValue(),
		),
		"propE8": rsh.NewNullValue(),
		"propF8": rsh.NewNullValue(),
		"propG8": rsh.NewNullValue(),
		"propH8": rsh.NewNullValue(),
		"propI8": rsh.NewNullValue(),
		"propJ8": rsh.NewListValue(
			rsh.NewNullValue(),
			rsh.NewNullValue(),
		),

		"foreign1": rsh.NewStringValue(""),
		"foreign2": rsh.NewNullValue(),
		"foreign3": rsh.NewNullValue(),
		"foreign4": rsh.NewNullValue(),
		"foreign5": rsh.NewListValue(),
		"foreign6": rsh.NewListValue(),
	}}
	assert.NoError(t, err)

	var d *pb.RichStruct
	d, err = box.ToRichStruct(edType)
	assert.NoError(t, err)
	assert.Equal(t, data, d)

	var e EntityBox
	assert.NoError(t, e.FromRichStruct(edType, d))
	assert.Equal(t, box.Data, e.Data)
}

func Test_convertRichStruct_zero2(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	edType := sch.GetEntity("EntityD")
	box1 := EntityBox{
		Data: map[string]any{
			"id": "id",

			"propA1": nil,
			"propB1": nil,
			"propC1": nil,
			"propD1": nil,
			"propE1": nil,
			"propF1": nil,
			"propG1": nil,
			"propH1": nil,
			"propI1": nil,
			"propJ1": nil,

			"propA2": nil,
			"propB2": nil,
			"propC2": nil,
			"propD2": nil,
			"propE2": nil,
			"propF2": nil,
			"propG2": nil,
			"propH2": nil,
			"propI2": nil,
			"propJ2": nil,
		},
	}
	box2 := EntityBox{
		Data: map[string]any{
			"id": "id",

			"propA1": "",
			"propB1": "0x",
			"propC1": false,
			"propD1": int32(0),
			"propE1": big.NewInt(0),
			"propF1": decimal.Zero,
			"propG1": "",
			"propH1": int64(0),
			"propI1": float64(0),
			"propJ1": int64(0),

			"propA2": (*string)(nil),
			"propB2": (*string)(nil),
			"propC2": (*bool)(nil),
			"propD2": (*int32)(nil),
			"propE2": (*big.Int)(nil),
			"propF2": (*decimal.Decimal)(nil),
			"propG2": (*string)(nil),
			"propH2": (*int64)(nil),
			"propI2": (*float64)(nil),
			"propJ2": (*int64)(nil),

			"propA3": []string(nil),
			"propB3": []string(nil),
			"propC3": []bool(nil),
			"propD3": []int32(nil),
			"propE3": []*big.Int(nil),
			"propF3": []decimal.Decimal(nil),
			"propG3": []string(nil),
			"propH3": []int64(nil),
			"propI3": []float64(nil),
			"propJ3": []int64(nil),

			"propA4": []*string(nil),
			"propB4": []*string(nil),
			"propC4": []*bool(nil),
			"propD4": []*int32(nil),
			"propE4": []*big.Int(nil),
			"propF4": []*decimal.Decimal(nil),
			"propG4": []*string(nil),
			"propH4": []*int64(nil),
			"propI4": []*float64(nil),
			"propJ4": []*int64(nil),

			"propA5": make([]*string, 0),
			"propB5": make([]*string, 0),
			"propC5": make([]*bool, 0),
			"propD5": make([]*int32, 0),
			"propE5": make([]*big.Int, 0),
			"propF5": make([]*decimal.Decimal, 0),
			"propG5": make([]*string, 0),
			"propH5": make([]*int64, 0),
			"propI5": make([]*float64, 0),
			"propJ5": make([]*int64, 0),

			"propA6": make([]string, 0),
			"propB6": make([]string, 0),
			"propC6": make([]bool, 0),
			"propD6": make([]int32, 0),
			"propE6": make([]*big.Int, 0),
			"propF6": make([]decimal.Decimal, 0),
			"propG6": make([]string, 0),
			"propH6": make([]int64, 0),
			"propI6": make([]float64, 0),
			"propJ6": make([]int64, 0),

			"propA7": [][]string(nil),
			"propB7": [][]string(nil),
			"propC7": [][]bool(nil),
			"propD7": [][]int32(nil),
			"propE7": [][]*big.Int(nil),
			"propF7": [][]decimal.Decimal(nil),
			"propG7": [][]string(nil),
			"propH7": [][]int64(nil),
			"propI7": [][]float64(nil),
			"propJ7": [][]int64(nil),

			"propA8": [][]string(nil),
			"propB8": [][]string(nil),
			"propC8": [][]bool(nil),
			"propD8": [][]int32(nil),
			"propE8": [][]*big.Int(nil),
			"propF8": [][]decimal.Decimal(nil),
			"propG8": [][]string(nil),
			"propH8": [][]int64(nil),
			"propI8": [][]float64(nil),
			"propJ8": [][]int64(nil),

			"foreign1": "",
			"foreign2": (*string)(nil),
			"foreign3": []string(nil),
			"foreign4": []*string(nil),
			"foreign5": make([]*string, 0),
			"foreign6": make([]string, 0),
		},
	}
	data := &pb.RichStruct{Fields: map[string]*pb.RichValue{
		"id": rsh.NewStringValue("id"),

		"propA1": rsh.NewStringValue(""),
		"propB1": rsh.NewBytesValue([]byte{}),
		"propC1": rsh.NewBoolValue(false),
		"propD1": rsh.NewIntValue(0),
		"propE1": rsh.NewBigIntValue(big.NewInt(0)),
		"propF1": rsh.NewBigDecimalValue(decimal.Zero),
		"propG1": rsh.NewStringValue(""),
		"propH1": rsh.NewTimestampValue(time.UnixMicro(0)),
		"propI1": rsh.NewFloatValue(0),
		"propJ1": rsh.NewInt64Value(0),

		"propA2": rsh.NewNullValue(),
		"propB2": rsh.NewNullValue(),
		"propC2": rsh.NewNullValue(),
		"propD2": rsh.NewNullValue(),
		"propE2": rsh.NewNullValue(),
		"propF2": rsh.NewNullValue(),
		"propG2": rsh.NewNullValue(),
		"propH2": rsh.NewNullValue(),
		"propI2": rsh.NewNullValue(),
		"propJ2": rsh.NewNullValue(),
	}}

	var d *pb.RichStruct
	d, err = box1.ToRichStruct(edType)
	assert.NoError(t, err)
	assert.Equal(t, data, d)

	var e EntityBox
	assert.NoError(t, e.FromRichStruct(edType, d))
	assert.Equal(t, box2.Data, e.Data)
}

func Test_convertRichStruct_missFields(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	var bigNum big.Int
	bigNum.SetInt64(456)
	bigDec := decimal.NewFromFloat(123.456)

	edType := sch.GetEntity("EntityD")
	box := EntityBox{
		Data: map[string]any{
			"id": "id",

			"propA1": "pa",
			"propB1": "0x0102",
			"propC1": true,
			"propD1": int32(123),
			"propE1": &bigNum,
			"propF1": bigDec,
			"propG1": "AAA",
			"propH1": int64(1111111111111111),
			"propI1": float64(1111.11111),
			"propJ1": int64(123),

			"foreign1": "fk1",
			"foreign2": utils.WrapPointer("fk2"),
			"foreign3": []string{"fk3", "fk4"},
		},
	}
	boxFull := EntityBox{
		Data: map[string]any{
			"id": "id",

			"propA1": "pa",
			"propB1": "0x0102",
			"propC1": true,
			"propD1": int32(123),
			"propE1": &bigNum,
			"propF1": bigDec,
			"propG1": "AAA",
			"propH1": int64(1111111111111111),
			"propI1": float64(1111.11111),
			"propJ1": int64(123),

			"propA2": (*string)(nil),
			"propB2": (*string)(nil),
			"propC2": (*bool)(nil),
			"propD2": (*int32)(nil),
			"propE2": (*big.Int)(nil),
			"propF2": (*decimal.Decimal)(nil),
			"propG2": (*string)(nil),
			"propH2": (*int64)(nil),
			"propI2": (*float64)(nil),
			"propJ2": (*int64)(nil),

			"propA3": []string(nil),
			"propB3": []string(nil),
			"propC3": []bool(nil),
			"propD3": []int32(nil),
			"propE3": []*big.Int(nil),
			"propF3": []decimal.Decimal(nil),
			"propG3": []string(nil),
			"propH3": []int64(nil),
			"propI3": []float64(nil),
			"propJ3": []int64(nil),

			"propA4": []*string(nil),
			"propB4": []*string(nil),
			"propC4": []*bool(nil),
			"propD4": []*int32(nil),
			"propE4": []*big.Int(nil),
			"propF4": []*decimal.Decimal(nil),
			"propG4": []*string(nil),
			"propH4": []*int64(nil),
			"propI4": []*float64(nil),
			"propJ4": []*int64(nil),

			"propA5": make([]*string, 0),
			"propB5": make([]*string, 0),
			"propC5": make([]*bool, 0),
			"propD5": make([]*int32, 0),
			"propE5": make([]*big.Int, 0),
			"propF5": make([]*decimal.Decimal, 0),
			"propG5": make([]*string, 0),
			"propH5": make([]*int64, 0),
			"propI5": make([]*float64, 0),
			"propJ5": make([]*int64, 0),

			"propA6": make([]string, 0),
			"propB6": make([]string, 0),
			"propC6": make([]bool, 0),
			"propD6": make([]int32, 0),
			"propE6": make([]*big.Int, 0),
			"propF6": make([]decimal.Decimal, 0),
			"propG6": make([]string, 0),
			"propH6": make([]int64, 0),
			"propI6": make([]float64, 0),
			"propJ6": make([]int64, 0),

			"propA7": [][]string(nil),
			"propB7": [][]string(nil),
			"propC7": [][]bool(nil),
			"propD7": [][]int32(nil),
			"propE7": [][]*big.Int(nil),
			"propF7": [][]decimal.Decimal(nil),
			"propG7": [][]string(nil),
			"propH7": [][]int64(nil),
			"propI7": [][]float64(nil),
			"propJ7": [][]int64(nil),

			"propA8": [][]string(nil),
			"propB8": [][]string(nil),
			"propC8": [][]bool(nil),
			"propD8": [][]int32(nil),
			"propE8": [][]*big.Int(nil),
			"propF8": [][]decimal.Decimal(nil),
			"propG8": [][]string(nil),
			"propH8": [][]int64(nil),
			"propI8": [][]float64(nil),
			"propJ8": [][]int64(nil),

			"foreign1": "fk1",
			"foreign2": utils.WrapPointer("fk2"),
			"foreign3": []string{"fk3", "fk4"},
			"foreign4": []*string(nil),
			"foreign5": make([]*string, 0),
			"foreign6": make([]string, 0),
		},
	}
	data := &pb.RichStruct{Fields: map[string]*pb.RichValue{
		"id": rsh.NewStringValue("id"),

		"propA1": rsh.NewStringValue("pa"),
		"propB1": rsh.NewBytesValue([]byte{1, 2}),
		"propC1": rsh.NewBoolValue(true),
		"propD1": rsh.NewIntValue(123),
		"propE1": rsh.NewBigIntValue(&bigNum),
		"propF1": rsh.NewBigDecimalValue(bigDec),
		"propG1": rsh.NewStringValue("AAA"),
		"propH1": rsh.NewTimestampValue(time.UnixMicro(1111111111111111)),
		"propI1": rsh.NewFloatValue(1111.11111),
		"propJ1": rsh.NewInt64Value(123),

		"foreign1": rsh.NewStringValue("fk1"),
		"foreign2": rsh.NewStringValue("fk2"),
		"foreign3": rsh.NewListValue(
			rsh.NewStringValue("fk3"),
			rsh.NewStringValue("fk4"),
		),
	}}
	dataFull := &pb.RichStruct{Fields: map[string]*pb.RichValue{
		"id": rsh.NewStringValue("id"),

		"propA1": rsh.NewStringValue("pa"),
		"propB1": rsh.NewBytesValue([]byte{1, 2}),
		"propC1": rsh.NewBoolValue(true),
		"propD1": rsh.NewIntValue(123),
		"propE1": rsh.NewBigIntValue(&bigNum),
		"propF1": rsh.NewBigDecimalValue(bigDec),
		"propG1": rsh.NewStringValue("AAA"),
		"propH1": rsh.NewTimestampValue(time.UnixMicro(1111111111111111)),
		"propI1": rsh.NewFloatValue(1111.11111),
		"propJ1": rsh.NewInt64Value(123),

		"propA2": rsh.NewNullValue(),
		"propB2": rsh.NewNullValue(),
		"propC2": rsh.NewNullValue(),
		"propD2": rsh.NewNullValue(),
		"propE2": rsh.NewNullValue(),
		"propF2": rsh.NewNullValue(),
		"propG2": rsh.NewNullValue(),
		"propH2": rsh.NewNullValue(),
		"propI2": rsh.NewNullValue(),
		"propJ2": rsh.NewNullValue(),

		"propA3": rsh.NewNullValue(),
		"propB3": rsh.NewNullValue(),
		"propC3": rsh.NewNullValue(),
		"propD3": rsh.NewNullValue(),
		"propE3": rsh.NewNullValue(),
		"propF3": rsh.NewNullValue(),
		"propG3": rsh.NewNullValue(),
		"propH3": rsh.NewNullValue(),
		"propI3": rsh.NewNullValue(),
		"propJ3": rsh.NewNullValue(),

		"propA4": rsh.NewNullValue(),
		"propB4": rsh.NewNullValue(),
		"propC4": rsh.NewNullValue(),
		"propD4": rsh.NewNullValue(),
		"propE4": rsh.NewNullValue(),
		"propF4": rsh.NewNullValue(),
		"propG4": rsh.NewNullValue(),
		"propH4": rsh.NewNullValue(),
		"propI4": rsh.NewNullValue(),
		"propJ4": rsh.NewNullValue(),

		"propA5": rsh.NewListValue(),
		"propB5": rsh.NewListValue(),
		"propC5": rsh.NewListValue(),
		"propD5": rsh.NewListValue(),
		"propE5": rsh.NewListValue(),
		"propF5": rsh.NewListValue(),
		"propG5": rsh.NewListValue(),
		"propH5": rsh.NewListValue(),
		"propI5": rsh.NewListValue(),
		"propJ5": rsh.NewListValue(),

		"propA6": rsh.NewListValue(),
		"propB6": rsh.NewListValue(),
		"propC6": rsh.NewListValue(),
		"propD6": rsh.NewListValue(),
		"propE6": rsh.NewListValue(),
		"propF6": rsh.NewListValue(),
		"propG6": rsh.NewListValue(),
		"propH6": rsh.NewListValue(),
		"propI6": rsh.NewListValue(),
		"propJ6": rsh.NewListValue(),

		"propA7": rsh.NewNullValue(),
		"propB7": rsh.NewNullValue(),
		"propC7": rsh.NewNullValue(),
		"propD7": rsh.NewNullValue(),
		"propE7": rsh.NewNullValue(),
		"propF7": rsh.NewNullValue(),
		"propG7": rsh.NewNullValue(),
		"propH7": rsh.NewNullValue(),
		"propI7": rsh.NewNullValue(),
		"propJ7": rsh.NewNullValue(),

		"propA8": rsh.NewNullValue(),
		"propB8": rsh.NewNullValue(),
		"propC8": rsh.NewNullValue(),
		"propD8": rsh.NewNullValue(),
		"propE8": rsh.NewNullValue(),
		"propF8": rsh.NewNullValue(),
		"propG8": rsh.NewNullValue(),
		"propH8": rsh.NewNullValue(),
		"propI8": rsh.NewNullValue(),
		"propJ8": rsh.NewNullValue(),

		"foreign1": rsh.NewStringValue("fk1"),
		"foreign2": rsh.NewStringValue("fk2"),
		"foreign3": rsh.NewListValue(
			rsh.NewStringValue("fk3"),
			rsh.NewStringValue("fk4"),
		),
		"foreign4": rsh.NewNullValue(),
		"foreign5": rsh.NewListValue(),
		"foreign6": rsh.NewListValue(),
	}}

	// box -> data -> box
	var d *pb.RichStruct
	d, err = box.ToRichStruct(edType)
	assert.NoError(t, err)
	assert.Equal(t, data, d)
	var e EntityBox
	assert.NoError(t, e.FromRichStruct(edType, d))
	assert.Equal(t, boxFull.Data, e.Data)

	// data -> box -> data
	assert.NoError(t, e.FromRichStruct(edType, data))
	assert.Equal(t, boxFull.Data, e.Data)
	d, err = e.ToRichStruct(edType)
	assert.NoError(t, err)
	assert.Equal(t, dataFull, d)
}

func Test_convertRichStruct_missFields2(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	edType := sch.GetEntity("EntityD")
	box := EntityBox{
		Data: map[string]any{
			"id": "id",
		},
	}
	data := &pb.RichStruct{Fields: map[string]*pb.RichValue{
		"id": rsh.NewStringValue("id"),

		"propA1": rsh.NewStringValue(""),
		"propB1": rsh.NewBytesValue([]byte{}),
		"propC1": rsh.NewBoolValue(false),
		"propD1": rsh.NewIntValue(0),
		"propE1": rsh.NewBigIntValue(big.NewInt(0)),
		"propF1": rsh.NewBigDecimalValue(decimal.Zero),
		"propG1": rsh.NewStringValue("AAA"),
		"propH1": rsh.NewTimestampValue(time.UnixMicro(0)),
		"propI1": rsh.NewFloatValue(0),
		"propJ1": rsh.NewInt64Value(0),

		"propA2": rsh.NewNullValue(),
		"propB2": rsh.NewNullValue(),
		"propC2": rsh.NewNullValue(),
		"propD2": rsh.NewNullValue(),
		"propE2": rsh.NewNullValue(),
		"propF2": rsh.NewNullValue(),
		"propG2": rsh.NewNullValue(),
		"propH2": rsh.NewNullValue(),
		"propI2": rsh.NewNullValue(),
		"propJ2": rsh.NewNullValue(),

		"propA3": rsh.NewNullValue(),
		"propB3": rsh.NewNullValue(),
		"propC3": rsh.NewNullValue(),
		"propD3": rsh.NewNullValue(),
		"propE3": rsh.NewNullValue(),
		"propF3": rsh.NewNullValue(),
		"propG3": rsh.NewNullValue(),
		"propH3": rsh.NewNullValue(),
		"propI3": rsh.NewNullValue(),
		"propJ3": rsh.NewNullValue(),

		"propA4": rsh.NewNullValue(),
		"propB4": rsh.NewNullValue(),
		"propC4": rsh.NewNullValue(),
		"propD4": rsh.NewNullValue(),
		"propE4": rsh.NewNullValue(),
		"propF4": rsh.NewNullValue(),
		"propG4": rsh.NewNullValue(),
		"propH4": rsh.NewNullValue(),
		"propI4": rsh.NewNullValue(),
		"propJ4": rsh.NewNullValue(),

		"propA5": rsh.NewListValue(),
		"propB5": rsh.NewListValue(),
		"propC5": rsh.NewListValue(),
		"propD5": rsh.NewListValue(),
		"propE5": rsh.NewListValue(),
		"propF5": rsh.NewListValue(),
		"propG5": rsh.NewListValue(),
		"propH5": rsh.NewListValue(),
		"propI5": rsh.NewListValue(),
		"propJ5": rsh.NewListValue(),

		"propA6": rsh.NewListValue(),
		"propB6": rsh.NewListValue(),
		"propC6": rsh.NewListValue(),
		"propD6": rsh.NewListValue(),
		"propE6": rsh.NewListValue(),
		"propF6": rsh.NewListValue(),
		"propG6": rsh.NewListValue(),
		"propH6": rsh.NewListValue(),
		"propI6": rsh.NewListValue(),
		"propJ6": rsh.NewListValue(),

		"propA7": rsh.NewNullValue(),
		"propB7": rsh.NewNullValue(),
		"propC7": rsh.NewNullValue(),
		"propD7": rsh.NewNullValue(),
		"propE7": rsh.NewNullValue(),
		"propF7": rsh.NewNullValue(),
		"propG7": rsh.NewNullValue(),
		"propH7": rsh.NewNullValue(),
		"propI7": rsh.NewNullValue(),
		"propJ7": rsh.NewNullValue(),

		"propA8": rsh.NewNullValue(),
		"propB8": rsh.NewNullValue(),
		"propC8": rsh.NewNullValue(),
		"propD8": rsh.NewNullValue(),
		"propE8": rsh.NewNullValue(),
		"propF8": rsh.NewNullValue(),
		"propG8": rsh.NewNullValue(),
		"propH8": rsh.NewNullValue(),
		"propI8": rsh.NewNullValue(),
		"propJ8": rsh.NewNullValue(),

		"foreign1": rsh.NewStringValue(""),
		"foreign2": rsh.NewNullValue(),
		"foreign3": rsh.NewNullValue(),
		"foreign4": rsh.NewNullValue(),
		"foreign5": rsh.NewListValue(),
		"foreign6": rsh.NewListValue(),
	}}

	box.FillLostFields(make(map[string]any), edType)
	var d *pb.RichStruct
	d, err = box.ToRichStruct(edType)
	assert.NoError(t, err)
	assert.Equal(t, data, d)

	assert.Equal(t, 0, decimal.Zero.Cmp(decimal.Decimal{}))
}

func Test_FromRichStruct_emptyArray(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	edType := sch.GetEntity("EntityD")

	data := &pb.RichStruct{Fields: map[string]*pb.RichValue{
		"id":     rsh.NewStringValue("id"),
		"propA6": rsh.NewListValue(),
	}}
	var a EntityBox
	assert.NoError(t, a.FromRichStruct(edType, data))
	assert.Equal(t, []string{}, a.Data["propA6"]) // not []string(nil), it is important

	data = &pb.RichStruct{Fields: map[string]*pb.RichValue{
		"id":     rsh.NewStringValue("id"),
		"propA6": rsh.NewListValue(rsh.NewStringValue("abc")),
	}}
	assert.NoError(t, a.FromRichStruct(edType, data))
	assert.Equal(t, []string{"abc"}, a.Data["propA6"])

	data = &pb.RichStruct{Fields: map[string]*pb.RichValue{
		"id": rsh.NewStringValue("id"),
		"propA6": rsh.NewListValue(
			rsh.NewStringValue("abc1"),
			rsh.NewStringValue("abc2"),
			rsh.NewStringValue("abc3"),
		),
	}}
	assert.NoError(t, a.FromRichStruct(edType, data))
	assert.Equal(t, []string{"abc1", "abc2", "abc3"}, a.Data["propA6"])
}

func Test_ToRichStruct_bigintArray0(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	prop: [BigInt]!
}
`)
	assert.NoError(t, err)

	etype := sch.GetEntity("EntityX")

	data := &pb.RichStruct{Fields: map[string]*pb.RichValue{
		"id": rsh.NewStringValue("id-0"),
		"prop": rsh.NewListValue(
			rsh.NewBigIntValue(big.NewInt(123)),
			rsh.NewNullValue(),
		),
	}}

	var num big.Int
	num.SetInt64(123)
	box0 := EntityBox{
		Data: map[string]any{
			"id":   "id-0",
			"prop": []any{num, nil},
		},
	}
	d0, err := box0.ToRichStruct(etype)
	assert.NoError(t, err)
	assert.Equal(t, data, d0)

	box1 := EntityBox{
		Data: map[string]any{
			"id":   "id-0",
			"prop": []any{big.NewInt(123), nil},
		},
	}
	d1, err := box1.ToRichStruct(etype)
	assert.NoError(t, err)
	assert.Equal(t, data, d1)

}

func Test_ToRichStruct_bigintArray1(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	prop: [BigInt!]!
}
`)
	assert.NoError(t, err)

	etype := sch.GetEntity("EntityX")

	data := &pb.RichStruct{Fields: map[string]*pb.RichValue{
		"id": rsh.NewStringValue("id-0"),
		"prop": rsh.NewListValue(
			rsh.NewBigIntValue(big.NewInt(123)),
			rsh.NewBigIntValue(big.NewInt(0)),
		),
	}}

	var num big.Int
	num.SetInt64(123)
	box0 := EntityBox{
		Data: map[string]any{
			"id":   "id-0",
			"prop": []any{num, nil},
		},
	}
	d0, err := box0.ToRichStruct(etype)
	assert.NoError(t, err)
	assert.Equal(t, data, d0)

	box1 := EntityBox{
		Data: map[string]any{
			"id":   "id-0",
			"prop": []any{big.NewInt(123), nil},
		},
	}
	d1, err := box1.ToRichStruct(etype)
	assert.NoError(t, err)
	assert.Equal(t, data, d1)
}
