package persistent

import (
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"math/big"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	"testing"
)

func prepareTestBox() EntityBox {
	var bigNum big.Int
	bigNum.SetInt64(456)
	var bigDec decimal.Decimal
	bigDec, _ = decimal.NewFromString("123.456")
	return EntityBox{
		Data: map[string]any{
			"id": "id",

			"propA1": "pa",
			"propB1": "pb",
			"propC1": true,
			"propD1": int32(123),
			"propE1": &bigNum,
			"propF1": bigDec,
			"propG1": "AAA",
			"propH1": int64(123),
			"propI1": float64(456.789),

			"propA2": utils.WrapPointer("pa"),
			"propB2": utils.WrapPointer("pb"),
			"propC2": utils.WrapPointer(true),
			"propD2": utils.WrapPointer(int32(123)),
			"propE2": &bigNum,
			"propF2": &bigDec,
			"propG2": utils.WrapPointer("AAA"),
			"propH2": utils.WrapPointer(int64(123)),
			"propI2": utils.WrapPointer(float64(456.789)),

			"propA3": []string{"pa1", "pa2"},
			"propB3": []string{"pb1", "pb2"},
			"propC3": []bool{true, false},
			"propD3": []int32{1, 23, 456},
			"propE3": []*big.Int{&bigNum},
			"propF3": []decimal.Decimal{bigDec},
			"propG3": []string{"AAA", "BBB"},
			"propH3": []int64{123},
			"propI3": []float64{456.789},

			"propA4": utils.WrapPointerForArray([]string{"pa1", "pa2"}),
			"propB4": utils.WrapPointerForArray([]string{"pb1", "pb2"}),
			"propC4": utils.WrapPointerForArray([]bool{true, false}),
			"propD4": utils.WrapPointerForArray([]int32{1, 23, 456}),
			"propE4": []*big.Int{&bigNum},
			"propF4": []*decimal.Decimal{&bigDec},
			"propG4": utils.WrapPointerForArray([]string{"AAA", "BBB"}),
			"propH4": utils.WrapPointerForArray([]int64{123}),
			"propI4": utils.WrapPointerForArray([]float64{456.789}),

			"propA5": utils.WrapPointerForArray([]string{"pa1", "pa2"}),
			"propB5": utils.WrapPointerForArray([]string{"pb1", "pb2"}),
			"propC5": utils.WrapPointerForArray([]bool{true, false}),
			"propD5": utils.WrapPointerForArray([]int32{1, 23, 456}),
			"propE5": []*big.Int{&bigNum},
			"propF5": []*decimal.Decimal{&bigDec},
			"propG5": utils.WrapPointerForArray([]string{"AAA", "BBB"}),
			"propH5": utils.WrapPointerForArray([]int64{123}),
			"propI5": utils.WrapPointerForArray([]float64{456.789}),

			"propA6": []string{"pa1", "pa2"},
			"propB6": []string{"pb1", "pb2"},
			"propC6": []bool{true, false},
			"propD6": []int32{1, 23, 456},
			"propE6": []*big.Int{&bigNum},
			"propF6": []decimal.Decimal{bigDec},
			"propG6": []string{"AAA", "BBB"},
			"propH6": []int64{123},
			"propI6": []float64{456.789},

			"propA7": [][]string{{"pa1", "pa2"}, {"pa3"}},
			"propB7": [][]string{{"pb1"}, {"pb2", "pb3"}},
			"propC7": [][]bool{{true, false}, nil, {false}},
			"propD7": [][]int32{{1, 23, 456}, {123, 45, 6}, nil},
			"propE7": [][]*big.Int{{&bigNum}},
			"propF7": [][]decimal.Decimal{{bigDec}},
			"propG7": [][]string{{"AAA"}, {"BBB", "CCC"}},
			"propH7": [][]int64{{123}},
			"propI7": [][]float64{{456.789}},

			"propC8": [][]bool{{true, false}, nil, {false}},
			"propD8": [][]int32{{1, 23, 456}, {123, 45, 6}, nil},

			"foreign1": "fk1",
			"foreign2": utils.WrapPointer("fk2"),
			"foreign3": []string{"fk3", "fk4"},
			"foreign4": utils.WrapPointerForArray([]string{"fk5", "fk6"}),
			"foreign5": utils.WrapPointerForArray([]string{"fk7", "fk8"}),
			"foreign6": []string{"fk9", "fk10"},
		},
	}
}

func Test_checkFilter(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	entityType := sch.GetEntity("EntityD")

	var cr bool
	box := prepareTestBox()

	type Testcase struct {
		F string
		O EntityFilterOp
		V []any
		R bool
	}
	var testcases []Testcase

	// about id
	testcases = append(testcases, []Testcase{
		// eq
		{F: "id", O: EntityFilterOpEq, V: []any{"id"}, R: true},
		{F: "id", O: EntityFilterOpEq, V: []any{"xx"}, R: false},
		// ne
		{F: "id", O: EntityFilterOpNe, V: []any{"id"}, R: false},
		{F: "id", O: EntityFilterOpNe, V: []any{"xx"}, R: true},
		// gt
		{F: "id", O: EntityFilterOpGt, V: []any{"aa"}, R: true},
		{F: "id", O: EntityFilterOpGt, V: []any{"id"}, R: false},
		{F: "id", O: EntityFilterOpGt, V: []any{"zz"}, R: false},
		// ge
		{F: "id", O: EntityFilterOpGe, V: []any{"aa"}, R: true},
		{F: "id", O: EntityFilterOpGe, V: []any{"id"}, R: true},
		{F: "id", O: EntityFilterOpGe, V: []any{"zz"}, R: false},
		// lt
		{F: "id", O: EntityFilterOpLt, V: []any{"aa"}, R: false},
		{F: "id", O: EntityFilterOpLt, V: []any{"id"}, R: false},
		{F: "id", O: EntityFilterOpLt, V: []any{"zz"}, R: true},
		// le
		{F: "id", O: EntityFilterOpLe, V: []any{"aa"}, R: false},
		{F: "id", O: EntityFilterOpLe, V: []any{"id"}, R: true},
		{F: "id", O: EntityFilterOpLe, V: []any{"zz"}, R: true},
		// in
		{F: "id", O: EntityFilterOpIn, V: []any{"aa"}, R: false},
		{F: "id", O: EntityFilterOpIn, V: []any{"id"}, R: true},
		{F: "id", O: EntityFilterOpIn, V: []any{"id", "aa"}, R: true},
		{F: "id", O: EntityFilterOpIn, V: []any{}, R: false},
		// not in
		{F: "id", O: EntityFilterOpNotIn, V: []any{"aa"}, R: true},
		{F: "id", O: EntityFilterOpNotIn, V: []any{"id"}, R: false},
		{F: "id", O: EntityFilterOpNotIn, V: []any{"id", "aa"}, R: false},
		{F: "id", O: EntityFilterOpNotIn, V: []any{}, R: true},
		// like
		{F: "id", O: EntityFilterOpLike, V: []any{"id%"}, R: true},
		{F: "id", O: EntityFilterOpLike, V: []any{"i%"}, R: true},
		{F: "id", O: EntityFilterOpLike, V: []any{"aa%"}, R: false},
		// not like
		{F: "id", O: EntityFilterOpNotLike, V: []any{"id%"}, R: false},
		{F: "id", O: EntityFilterOpNotLike, V: []any{"i%"}, R: false},
		{F: "id", O: EntityFilterOpNotLike, V: []any{"aa%"}, R: true},
	}...)

	// about scalar field
	for gx := 1; gx <= 2; gx++ {
		suf := fmt.Sprintf("%d", gx)
		testcases = append(testcases, []Testcase{
			// eq
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{"pa"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{"px"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{utils.WrapPointer("pa")}, R: true},
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{utils.WrapPointer("px")}, R: false},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{"pb"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{"px"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{utils.WrapPointer("pb")}, R: true},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{utils.WrapPointer("px")}, R: false},
			{F: "propC" + suf, O: EntityFilterOpEq, V: []any{true}, R: true},
			{F: "propC" + suf, O: EntityFilterOpEq, V: []any{false}, R: false},
			{F: "propC" + suf, O: EntityFilterOpEq, V: []any{utils.WrapPointer(true)}, R: true},
			{F: "propC" + suf, O: EntityFilterOpEq, V: []any{utils.WrapPointer(false)}, R: false},
			{F: "propD" + suf, O: EntityFilterOpEq, V: []any{int32(123)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpEq, V: []any{int32(321)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpEq, V: []any{big.NewInt(456)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpEq, V: []any{big.NewInt(123)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpEq, V: []any{decimal.NewFromFloat(123.456)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpEq, V: []any{decimal.NewFromInt(1)}, R: false},
			{F: "propG" + suf, O: EntityFilterOpEq, V: []any{"AAA"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpEq, V: []any{"BBB"}, R: false},
			{F: "propH" + suf, O: EntityFilterOpEq, V: []any{int64(123)}, R: true},
			{F: "propH" + suf, O: EntityFilterOpEq, V: []any{int64(321)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpEq, V: []any{float64(456.789)}, R: true},
			{F: "propI" + suf, O: EntityFilterOpEq, V: []any{float64(789.456)}, R: false},
			// ne
			{F: "propA" + suf, O: EntityFilterOpNe, V: []any{"pa"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpNe, V: []any{"px"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpNe, V: []any{"pb"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpNe, V: []any{"px"}, R: true},
			{F: "propC" + suf, O: EntityFilterOpNe, V: []any{true}, R: false},
			{F: "propC" + suf, O: EntityFilterOpNe, V: []any{false}, R: true},
			{F: "propD" + suf, O: EntityFilterOpNe, V: []any{int32(123)}, R: false},
			{F: "propD" + suf, O: EntityFilterOpNe, V: []any{int32(321)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpNe, V: []any{big.NewInt(456)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpNe, V: []any{big.NewInt(123)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpNe, V: []any{decimal.NewFromFloat(123.456)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpNe, V: []any{decimal.NewFromInt(1)}, R: true},
			{F: "propG" + suf, O: EntityFilterOpNe, V: []any{"AAA"}, R: false},
			{F: "propG" + suf, O: EntityFilterOpNe, V: []any{"BBB"}, R: true},
			{F: "propH" + suf, O: EntityFilterOpNe, V: []any{int64(123)}, R: false},
			{F: "propH" + suf, O: EntityFilterOpNe, V: []any{int64(321)}, R: true},
			{F: "propI" + suf, O: EntityFilterOpNe, V: []any{float64(456.789)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpNe, V: []any{float64(789.456)}, R: true},
			// gt
			{F: "propA" + suf, O: EntityFilterOpGt, V: []any{"aa"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpGt, V: []any{"pa"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpGt, V: []any{"zz"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpGt, V: []any{"aa"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpGt, V: []any{"pb"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpGt, V: []any{"zz"}, R: false},
			{F: "propC" + suf, O: EntityFilterOpGt, V: []any{false}, R: true},
			{F: "propC" + suf, O: EntityFilterOpGt, V: []any{true}, R: false},
			{F: "propD" + suf, O: EntityFilterOpGt, V: []any{int32(100)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpGt, V: []any{int32(123)}, R: false},
			{F: "propD" + suf, O: EntityFilterOpGt, V: []any{int32(321)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpGt, V: []any{big.NewInt(123)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpGt, V: []any{big.NewInt(456)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpGt, V: []any{big.NewInt(500)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpGt, V: []any{decimal.NewFromInt(1)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpGt, V: []any{decimal.NewFromFloat(123.456)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpGt, V: []any{decimal.NewFromInt(500)}, R: false},
			{F: "propG" + suf, O: EntityFilterOpGt, V: []any{"A"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpGt, V: []any{"AAA"}, R: false},
			{F: "propG" + suf, O: EntityFilterOpGt, V: []any{"BBB"}, R: false},
			{F: "propH" + suf, O: EntityFilterOpGt, V: []any{int64(122)}, R: true},
			{F: "propH" + suf, O: EntityFilterOpGt, V: []any{int64(123)}, R: false},
			{F: "propH" + suf, O: EntityFilterOpGt, V: []any{int64(124)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpGt, V: []any{float64(456.78)}, R: true},
			{F: "propI" + suf, O: EntityFilterOpGt, V: []any{float64(456.789)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpGt, V: []any{float64(456.799)}, R: false},
			// ge
			{F: "propA" + suf, O: EntityFilterOpGe, V: []any{"aa"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpGe, V: []any{"pa"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpGe, V: []any{"zz"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpGe, V: []any{"aa"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpGe, V: []any{"pb"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpGe, V: []any{"zz"}, R: false},
			{F: "propC" + suf, O: EntityFilterOpGe, V: []any{false}, R: true},
			{F: "propC" + suf, O: EntityFilterOpGe, V: []any{true}, R: true},
			{F: "propD" + suf, O: EntityFilterOpGe, V: []any{int32(100)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpGe, V: []any{int32(123)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpGe, V: []any{int32(321)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpGe, V: []any{big.NewInt(123)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpGe, V: []any{big.NewInt(456)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpGe, V: []any{big.NewInt(500)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpGe, V: []any{decimal.NewFromInt(1)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpGe, V: []any{decimal.NewFromFloat(123.456)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpGe, V: []any{decimal.NewFromInt(500)}, R: false},
			{F: "propG" + suf, O: EntityFilterOpGe, V: []any{"A"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpGe, V: []any{"AAA"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpGe, V: []any{"BBB"}, R: false},
			{F: "propH" + suf, O: EntityFilterOpGe, V: []any{int64(122)}, R: true},
			{F: "propH" + suf, O: EntityFilterOpGe, V: []any{int64(123)}, R: true},
			{F: "propH" + suf, O: EntityFilterOpGe, V: []any{int64(124)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpGe, V: []any{float64(456.78)}, R: true},
			{F: "propI" + suf, O: EntityFilterOpGe, V: []any{float64(456.789)}, R: true},
			{F: "propI" + suf, O: EntityFilterOpGe, V: []any{float64(456.799)}, R: false},
			// lt
			{F: "propA" + suf, O: EntityFilterOpLt, V: []any{"aa"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpLt, V: []any{"pa"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpLt, V: []any{"zz"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpLt, V: []any{"aa"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpLt, V: []any{"pb"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpLt, V: []any{"zz"}, R: true},
			{F: "propC" + suf, O: EntityFilterOpLt, V: []any{false}, R: false},
			{F: "propC" + suf, O: EntityFilterOpLt, V: []any{true}, R: false},
			{F: "propD" + suf, O: EntityFilterOpLt, V: []any{int32(100)}, R: false},
			{F: "propD" + suf, O: EntityFilterOpLt, V: []any{int32(123)}, R: false},
			{F: "propD" + suf, O: EntityFilterOpLt, V: []any{int32(321)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpLt, V: []any{big.NewInt(123)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpLt, V: []any{big.NewInt(456)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpLt, V: []any{big.NewInt(500)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpLt, V: []any{decimal.NewFromInt(1)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpLt, V: []any{decimal.NewFromFloat(123.456)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpLt, V: []any{decimal.NewFromInt(500)}, R: true},
			{F: "propG" + suf, O: EntityFilterOpLt, V: []any{"A"}, R: false},
			{F: "propG" + suf, O: EntityFilterOpLt, V: []any{"AAA"}, R: false},
			{F: "propG" + suf, O: EntityFilterOpLt, V: []any{"BBB"}, R: true},
			{F: "propH" + suf, O: EntityFilterOpLt, V: []any{int64(122)}, R: false},
			{F: "propH" + suf, O: EntityFilterOpLt, V: []any{int64(123)}, R: false},
			{F: "propH" + suf, O: EntityFilterOpLt, V: []any{int64(124)}, R: true},
			{F: "propI" + suf, O: EntityFilterOpLt, V: []any{float64(456.78)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpLt, V: []any{float64(456.789)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpLt, V: []any{float64(456.799)}, R: true},
			// le
			{F: "propA" + suf, O: EntityFilterOpLe, V: []any{"aa"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpLe, V: []any{"pa"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpLe, V: []any{"zz"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpLe, V: []any{"aa"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpLe, V: []any{"pb"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpLe, V: []any{"zz"}, R: true},
			{F: "propC" + suf, O: EntityFilterOpLe, V: []any{false}, R: false},
			{F: "propC" + suf, O: EntityFilterOpLe, V: []any{true}, R: true},
			{F: "propD" + suf, O: EntityFilterOpLe, V: []any{int32(100)}, R: false},
			{F: "propD" + suf, O: EntityFilterOpLe, V: []any{int32(123)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpLe, V: []any{int32(321)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpLe, V: []any{big.NewInt(123)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpLe, V: []any{big.NewInt(456)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpLe, V: []any{big.NewInt(500)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpLe, V: []any{decimal.NewFromInt(1)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpLe, V: []any{decimal.NewFromFloat(123.456)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpLe, V: []any{decimal.NewFromInt(500)}, R: true},
			{F: "propG" + suf, O: EntityFilterOpLe, V: []any{"A"}, R: false},
			{F: "propG" + suf, O: EntityFilterOpLe, V: []any{"AAA"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpLe, V: []any{"BBB"}, R: true},
			{F: "propH" + suf, O: EntityFilterOpLe, V: []any{int64(122)}, R: false},
			{F: "propH" + suf, O: EntityFilterOpLe, V: []any{int64(123)}, R: true},
			{F: "propH" + suf, O: EntityFilterOpLe, V: []any{int64(124)}, R: true},
			{F: "propI" + suf, O: EntityFilterOpLe, V: []any{float64(456.78)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpLe, V: []any{float64(456.789)}, R: true},
			{F: "propI" + suf, O: EntityFilterOpLe, V: []any{float64(456.799)}, R: true},
			// in
			{F: "propA" + suf, O: EntityFilterOpIn, V: []any{"aa"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpIn, V: []any{"pa"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpIn, V: []any{"aa", "pa"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpIn, V: []any{}, R: false},
			{F: "propB" + suf, O: EntityFilterOpIn, V: []any{"aa"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpIn, V: []any{"pb"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpIn, V: []any{"aa", "pb"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpIn, V: []any{}, R: false},
			{F: "propC" + suf, O: EntityFilterOpIn, V: []any{false}, R: false},
			{F: "propC" + suf, O: EntityFilterOpIn, V: []any{true}, R: true},
			{F: "propC" + suf, O: EntityFilterOpIn, V: []any{true, false}, R: true},
			{F: "propC" + suf, O: EntityFilterOpIn, V: []any{}, R: false},
			{F: "propD" + suf, O: EntityFilterOpIn, V: []any{int32(100)}, R: false},
			{F: "propD" + suf, O: EntityFilterOpIn, V: []any{int32(123)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpIn, V: []any{int32(123), int32(321)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpIn, V: []any{}, R: false},
			{F: "propE" + suf, O: EntityFilterOpIn, V: []any{big.NewInt(123)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpIn, V: []any{big.NewInt(456)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpIn, V: []any{big.NewInt(500), big.NewInt(456)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpIn, V: []any{}, R: false},
			{F: "propF" + suf, O: EntityFilterOpIn, V: []any{decimal.NewFromInt(1)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpIn, V: []any{decimal.NewFromFloat(123.456)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpIn, V: []any{decimal.NewFromInt(500), decimal.NewFromFloat(123.456)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpIn, V: []any{}, R: false},
			{F: "propG" + suf, O: EntityFilterOpIn, V: []any{"BBB"}, R: false},
			{F: "propG" + suf, O: EntityFilterOpIn, V: []any{"AAA"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpIn, V: []any{"AAA", "BBB"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpIn, V: []any{}, R: false},
			{F: "propH" + suf, O: EntityFilterOpIn, V: []any{int64(100)}, R: false},
			{F: "propH" + suf, O: EntityFilterOpIn, V: []any{int64(123)}, R: true},
			{F: "propH" + suf, O: EntityFilterOpIn, V: []any{int64(123), int64(321)}, R: true},
			{F: "propH" + suf, O: EntityFilterOpIn, V: []any{}, R: false},
			{F: "propI" + suf, O: EntityFilterOpIn, V: []any{float64(456.788)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpIn, V: []any{float64(456.789)}, R: true},
			{F: "propI" + suf, O: EntityFilterOpIn, V: []any{float64(456.788), float64(456.789)}, R: true},
			{F: "propI" + suf, O: EntityFilterOpIn, V: []any{}, R: false},
			// not in
			{F: "propA" + suf, O: EntityFilterOpNotIn, V: []any{"aa"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpNotIn, V: []any{"pa"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpNotIn, V: []any{"aa", "pa"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpNotIn, V: []any{}, R: true},
			{F: "propB" + suf, O: EntityFilterOpNotIn, V: []any{"aa"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpNotIn, V: []any{"pb"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpNotIn, V: []any{"aa", "pb"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpNotIn, V: []any{}, R: true},
			{F: "propC" + suf, O: EntityFilterOpNotIn, V: []any{false}, R: true},
			{F: "propC" + suf, O: EntityFilterOpNotIn, V: []any{true}, R: false},
			{F: "propC" + suf, O: EntityFilterOpNotIn, V: []any{true, false}, R: false},
			{F: "propC" + suf, O: EntityFilterOpNotIn, V: []any{}, R: true},
			{F: "propD" + suf, O: EntityFilterOpNotIn, V: []any{int32(100)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpNotIn, V: []any{int32(123)}, R: false},
			{F: "propD" + suf, O: EntityFilterOpNotIn, V: []any{int32(123), int32(321)}, R: false},
			{F: "propD" + suf, O: EntityFilterOpNotIn, V: []any{}, R: true},
			{F: "propE" + suf, O: EntityFilterOpNotIn, V: []any{big.NewInt(123)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpNotIn, V: []any{big.NewInt(456)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpNotIn, V: []any{big.NewInt(500), big.NewInt(456)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpNotIn, V: []any{}, R: true},
			{F: "propF" + suf, O: EntityFilterOpNotIn, V: []any{decimal.NewFromInt(1)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpNotIn, V: []any{decimal.NewFromFloat(123.456)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpNotIn, V: []any{decimal.NewFromInt(500), decimal.NewFromFloat(123.456)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpNotIn, V: []any{}, R: true},
			{F: "propG" + suf, O: EntityFilterOpNotIn, V: []any{"BBB"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpNotIn, V: []any{"AAA"}, R: false},
			{F: "propG" + suf, O: EntityFilterOpNotIn, V: []any{"AAA", "BBB"}, R: false},
			{F: "propG" + suf, O: EntityFilterOpNotIn, V: []any{}, R: true},
			{F: "propH" + suf, O: EntityFilterOpNotIn, V: []any{int64(100)}, R: true},
			{F: "propH" + suf, O: EntityFilterOpNotIn, V: []any{int64(123)}, R: false},
			{F: "propH" + suf, O: EntityFilterOpNotIn, V: []any{int64(123), int64(321)}, R: false},
			{F: "propH" + suf, O: EntityFilterOpNotIn, V: []any{}, R: true},
			{F: "propI" + suf, O: EntityFilterOpNotIn, V: []any{float64(456.788)}, R: true},
			{F: "propI" + suf, O: EntityFilterOpNotIn, V: []any{float64(456.789)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpNotIn, V: []any{float64(456.788), float64(456.789)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpNotIn, V: []any{}, R: true},
			// like
			{F: "propA" + suf, O: EntityFilterOpLike, V: []any{"pa%"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpLike, V: []any{"pb%"}, R: false},
			// not like
			{F: "propA" + suf, O: EntityFilterOpNotLike, V: []any{"pa%"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpNotLike, V: []any{"pb%"}, R: true},
		}...)
	}

	// about array field
	for gx := 3; gx <= 4; gx++ {
		suf := fmt.Sprintf("%d", gx)
		testcases = append(testcases, []Testcase{
			// eq
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{[]string{"pa1", "pa2"}}, R: true},
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{[]string{"pa1", "pa3"}}, R: false},
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{[]string{"pa2", "pa1"}}, R: false},
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{[]string{}}, R: false},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{[]string{"pb1", "pb2"}}, R: true},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{[]string{"pb1", "pb3"}}, R: false},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{[]string{"pb2", "pb1"}}, R: false},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{[]string{}}, R: false},
			{F: "propC" + suf, O: EntityFilterOpEq, V: []any{[]bool{true, false}}, R: true},
			{F: "propC" + suf, O: EntityFilterOpEq, V: []any{[]bool{false, true}}, R: false},
			{F: "propC" + suf, O: EntityFilterOpEq, V: []any{[]bool{false}}, R: false},
			{F: "propD" + suf, O: EntityFilterOpEq, V: []any{[]int32{1, 23, 456}}, R: true},
			{F: "propD" + suf, O: EntityFilterOpEq, V: []any{[]int32{1, 456}}, R: false},
			{F: "propE" + suf, O: EntityFilterOpEq, V: []any{[]*big.Int{big.NewInt(456)}}, R: true},
			{F: "propE" + suf, O: EntityFilterOpEq, V: []any{[]*big.Int{big.NewInt(123)}}, R: false},
			{F: "propE" + suf, O: EntityFilterOpEq, V: []any{[]*big.Int{big.NewInt(123), big.NewInt(456)}}, R: false},
			{F: "propF" + suf, O: EntityFilterOpEq, V: []any{[]decimal.Decimal{decimal.NewFromFloat(123.456)}}, R: true},
			{F: "propF" + suf, O: EntityFilterOpEq, V: []any{[]decimal.Decimal{decimal.NewFromInt(1)}}, R: false},
			{F: "propF" + suf, O: EntityFilterOpEq, V: []any{[]decimal.Decimal{}}, R: false},
			{F: "propG" + suf, O: EntityFilterOpEq, V: []any{[]string{"AAA", "BBB"}}, R: true},
			{F: "propG" + suf, O: EntityFilterOpEq, V: []any{[]string{"AAA", "CCC"}}, R: false},
			{F: "propG" + suf, O: EntityFilterOpEq, V: []any{[]string{"AAA"}}, R: false},
			{F: "propG" + suf, O: EntityFilterOpEq, V: []any{[]string{"BBB"}}, R: false},
			{F: "propH" + suf, O: EntityFilterOpEq, V: []any{[]int64{1, 23, 456}}, R: false},
			{F: "propH" + suf, O: EntityFilterOpEq, V: []any{[]int64{123}}, R: true},
			{F: "propH" + suf, O: EntityFilterOpEq, V: []any{[]int64{}}, R: false},
			{F: "propI" + suf, O: EntityFilterOpEq, V: []any{[]float64{1, 23}}, R: false},
			{F: "propI" + suf, O: EntityFilterOpEq, V: []any{[]float64{456.789}}, R: true},
			{F: "propI" + suf, O: EntityFilterOpEq, V: []any{[]float64{456.7891}}, R: false},
			// ne
			{F: "propA" + suf, O: EntityFilterOpNe, V: []any{[]string{"pa1", "pa2"}}, R: false},
			{F: "propA" + suf, O: EntityFilterOpNe, V: []any{[]string{"pa1", "pa3"}}, R: true},
			{F: "propA" + suf, O: EntityFilterOpNe, V: []any{[]string{"pa2", "pa1"}}, R: true},
			{F: "propA" + suf, O: EntityFilterOpNe, V: []any{[]string{}}, R: true},
			{F: "propB" + suf, O: EntityFilterOpNe, V: []any{[]string{"pb1", "pb2"}}, R: false},
			{F: "propB" + suf, O: EntityFilterOpNe, V: []any{[]string{"pb1", "pb3"}}, R: true},
			{F: "propB" + suf, O: EntityFilterOpNe, V: []any{[]string{"pb2", "pb1"}}, R: true},
			{F: "propB" + suf, O: EntityFilterOpNe, V: []any{[]string{}}, R: true},
			{F: "propC" + suf, O: EntityFilterOpNe, V: []any{[]bool{true, false}}, R: false},
			{F: "propC" + suf, O: EntityFilterOpNe, V: []any{[]bool{false, true}}, R: true},
			{F: "propC" + suf, O: EntityFilterOpNe, V: []any{[]bool{false}}, R: true},
			{F: "propD" + suf, O: EntityFilterOpNe, V: []any{[]int32{1, 23, 456}}, R: false},
			{F: "propD" + suf, O: EntityFilterOpNe, V: []any{[]int32{1, 456}}, R: true},
			{F: "propE" + suf, O: EntityFilterOpNe, V: []any{[]*big.Int{big.NewInt(456)}}, R: false},
			{F: "propE" + suf, O: EntityFilterOpNe, V: []any{[]*big.Int{big.NewInt(123)}}, R: true},
			{F: "propE" + suf, O: EntityFilterOpNe, V: []any{[]*big.Int{big.NewInt(123), big.NewInt(456)}}, R: true},
			{F: "propF" + suf, O: EntityFilterOpNe, V: []any{[]decimal.Decimal{decimal.NewFromFloat(123.456)}}, R: false},
			{F: "propF" + suf, O: EntityFilterOpNe, V: []any{[]decimal.Decimal{decimal.NewFromInt(1)}}, R: true},
			{F: "propF" + suf, O: EntityFilterOpNe, V: []any{[]decimal.Decimal{}}, R: true},
			{F: "propG" + suf, O: EntityFilterOpNe, V: []any{[]string{"AAA", "BBB"}}, R: false},
			{F: "propG" + suf, O: EntityFilterOpNe, V: []any{[]string{"AAA", "CCC"}}, R: true},
			{F: "propG" + suf, O: EntityFilterOpNe, V: []any{[]string{"AAA"}}, R: true},
			{F: "propG" + suf, O: EntityFilterOpNe, V: []any{[]string{"BBB"}}, R: true},
			{F: "propH" + suf, O: EntityFilterOpNe, V: []any{[]int64{1, 23, 456}}, R: true},
			{F: "propH" + suf, O: EntityFilterOpNe, V: []any{[]int64{123}}, R: false},
			{F: "propH" + suf, O: EntityFilterOpNe, V: []any{[]int64{}}, R: true},
			{F: "propI" + suf, O: EntityFilterOpNe, V: []any{[]float64{1, 23}}, R: true},
			{F: "propI" + suf, O: EntityFilterOpNe, V: []any{[]float64{456.789}}, R: false},
			{F: "propI" + suf, O: EntityFilterOpNe, V: []any{[]float64{456.7891}}, R: true},
			// hasAll
			{F: "propA" + suf, O: EntityFilterOpHasAll, V: []any{"pa1", "pa2"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpHasAll, V: []any{"pa1", "pa3"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpHasAll, V: []any{"pa2", "pa1"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpHasAll, V: []any{}, R: true},
			{F: "propB" + suf, O: EntityFilterOpHasAll, V: []any{"pb1", "pb2"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpHasAll, V: []any{"pb1", "pb3"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpHasAll, V: []any{"pb2", "pb1"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpHasAll, V: []any{}, R: true},
			{F: "propC" + suf, O: EntityFilterOpHasAll, V: []any{true, false}, R: true},
			{F: "propC" + suf, O: EntityFilterOpHasAll, V: []any{false, true}, R: true},
			{F: "propC" + suf, O: EntityFilterOpHasAll, V: []any{false}, R: true},
			{F: "propD" + suf, O: EntityFilterOpHasAll, V: []any{int32(1), int32(23), int32(456)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpHasAll, V: []any{int32(1), int32(456)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpHasAll, V: []any{big.NewInt(456)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpHasAll, V: []any{big.NewInt(123)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpHasAll, V: []any{big.NewInt(123), big.NewInt(456)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpHasAll, V: []any{decimal.NewFromFloat(123.456)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpHasAll, V: []any{decimal.NewFromInt(1)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpHasAll, V: []any{}, R: true},
			{F: "propG" + suf, O: EntityFilterOpHasAll, V: []any{"AAA", "BBB"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpHasAll, V: []any{"AAA", "CCC"}, R: false},
			{F: "propG" + suf, O: EntityFilterOpHasAll, V: []any{"AAA"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpHasAll, V: []any{"BBB"}, R: true},
			{F: "propH" + suf, O: EntityFilterOpHasAll, V: []any{int64(1), int64(23), int64(456)}, R: false},
			{F: "propH" + suf, O: EntityFilterOpHasAll, V: []any{int64(123)}, R: true},
			{F: "propH" + suf, O: EntityFilterOpHasAll, V: []any{}, R: true},
			{F: "propI" + suf, O: EntityFilterOpHasAll, V: []any{float64(1), float64(23)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpHasAll, V: []any{456.789}, R: true},
			{F: "propI" + suf, O: EntityFilterOpHasAll, V: []any{456.7891}, R: false},
			// hasAny
			{F: "propA" + suf, O: EntityFilterOpHasAny, V: []any{"pa1", "pa2"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpHasAny, V: []any{"pa1", "pa3"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpHasAny, V: []any{"pa2", "pa1"}, R: true},
			{F: "propA" + suf, O: EntityFilterOpHasAny, V: []any{"pa3", "pa4"}, R: false},
			{F: "propA" + suf, O: EntityFilterOpHasAny, V: []any{}, R: false},
			{F: "propB" + suf, O: EntityFilterOpHasAny, V: []any{"pb1", "pb2"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpHasAny, V: []any{"pb1", "pb3"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpHasAny, V: []any{"pb2", "pb1"}, R: true},
			{F: "propB" + suf, O: EntityFilterOpHasAny, V: []any{"pb3", "pb4"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpHasAny, V: []any{}, R: false},
			{F: "propC" + suf, O: EntityFilterOpHasAny, V: []any{true, false}, R: true},
			{F: "propC" + suf, O: EntityFilterOpHasAny, V: []any{false, true}, R: true},
			{F: "propC" + suf, O: EntityFilterOpHasAny, V: []any{false}, R: true},
			{F: "propC" + suf, O: EntityFilterOpHasAny, V: []any{}, R: false},
			{F: "propD" + suf, O: EntityFilterOpHasAny, V: []any{int32(1), int32(23), int32(456)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpHasAny, V: []any{int32(1), int32(456)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpHasAny, V: []any{int32(2), int32(456)}, R: true},
			{F: "propD" + suf, O: EntityFilterOpHasAny, V: []any{int32(2), int32(4567)}, R: false},
			{F: "propD" + suf, O: EntityFilterOpHasAny, V: []any{}, R: false},
			{F: "propE" + suf, O: EntityFilterOpHasAny, V: []any{big.NewInt(456)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpHasAny, V: []any{big.NewInt(123)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpHasAny, V: []any{big.NewInt(123), big.NewInt(456)}, R: true},
			{F: "propE" + suf, O: EntityFilterOpHasAny, V: []any{}, R: false},
			{F: "propF" + suf, O: EntityFilterOpHasAny, V: []any{decimal.NewFromFloat(123.456)}, R: true},
			{F: "propF" + suf, O: EntityFilterOpHasAny, V: []any{decimal.NewFromInt(1)}, R: false},
			{F: "propF" + suf, O: EntityFilterOpHasAny, V: []any{}, R: false},
			{F: "propG" + suf, O: EntityFilterOpHasAny, V: []any{"AAA", "BBB"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpHasAny, V: []any{"AAA", "CCC"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpHasAny, V: []any{"AAA"}, R: true},
			{F: "propG" + suf, O: EntityFilterOpHasAny, V: []any{"CCC"}, R: false},
			{F: "propH" + suf, O: EntityFilterOpHasAny, V: []any{int64(1), int64(23), int64(456)}, R: false},
			{F: "propH" + suf, O: EntityFilterOpHasAny, V: []any{int64(1), int64(123), int64(456)}, R: true},
			{F: "propH" + suf, O: EntityFilterOpHasAny, V: []any{int64(123)}, R: true},
			{F: "propH" + suf, O: EntityFilterOpHasAny, V: []any{}, R: false},
			{F: "propI" + suf, O: EntityFilterOpHasAny, V: []any{float64(1), float64(23)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpHasAny, V: []any{456.789}, R: true},
			{F: "propI" + suf, O: EntityFilterOpHasAny, V: []any{456.789, 456.7891}, R: true},
			{F: "propI" + suf, O: EntityFilterOpHasAny, V: []any{456.7891}, R: false},
		}...)
	}

	for i, tc := range testcases {
		msg := fmt.Sprintf("testcase #%d %#v", i, tc)
		filter := EntityFilter{Field: entityType.GetFieldByName(tc.F), Op: tc.O, Value: tc.V}
		cr, err = checkFilter(filter, box)
		assert.Equal(t, tc.R, cr, msg)
		assert.NoError(t, err, msg)
	}
}

func Test_checkFilterWithNullValue(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	entityType := sch.GetEntity("EntityD")

	var cr bool
	var bigNum big.Int
	bigNum.SetInt64(456)
	var bigDec decimal.Decimal
	bigDec, _ = decimal.NewFromString("123.456")
	box := EntityBox{
		Data: map[string]any{
			"id": "id",

			"propA1": "pa",
			"propB1": "pb",
			"propC1": true,
			"propD1": int32(123),
			"propE1": &bigNum,
			"propF1": bigDec,
			"propG1": "AAA",
			"propH1": int64(123),
			"propI1": float64(456.789),

			"propA2": (*string)(nil),
			"propB2": (*string)(nil),
			"propC2": (*bool)(nil),
			"propD2": (*int32)(nil),
			"propE2": (*big.Int)(nil),
			"propF2": (*decimal.Decimal)(nil),
			"propG2": (*string)(nil),
			"propH2": (*int64)(nil),
			"propI2": (*float64)(nil),

			"propA3": []string{"pa1", "pa2"},
			"propB3": []string{"pb1", "pb2"},
			"propC3": []bool{true, false},
			"propD3": []int32{1, 23, 456},
			"propE3": []*big.Int{&bigNum},
			"propF3": []decimal.Decimal{bigDec},
			"propG3": []string{"AAA", "BBB"},
			"propH3": []int64{123},
			"propI3": []float64{456.789},

			"propA4": ([]string)(nil),
			"propB4": ([]string)(nil),
			"propC4": ([]bool)(nil),
			"propD4": ([]int32)(nil),
			"propE4": ([]*big.Int)(nil),
			"propF4": ([]decimal.Decimal)(nil),
			"propG4": ([]string)(nil),
			"propH4": ([]int64)(nil),
			"propI4": ([]float64)(nil),
		},
	}

	type Testcase struct {
		F string
		O EntityFilterOp
		V []any
		R bool
	}
	var testcases []Testcase

	// about scalar type
	for gx := 1; gx <= 2; gx++ {
		suf := fmt.Sprintf("%d", gx)
		testcases = append(testcases, []Testcase{
			// eq
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 2},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 2},
			{F: "propC" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 2},
			{F: "propD" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 2},
			{F: "propE" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 2},
			{F: "propF" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 2},
			{F: "propG" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 2},
			{F: "propH" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 2},
			{F: "propI" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 2},
			// ---
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{"pa"}, R: gx == 1},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{"pb"}, R: gx == 1},
			{F: "propC" + suf, O: EntityFilterOpEq, V: []any{true}, R: gx == 1},
			{F: "propD" + suf, O: EntityFilterOpEq, V: []any{int32(123)}, R: gx == 1},
			{F: "propE" + suf, O: EntityFilterOpEq, V: []any{&bigNum}, R: gx == 1},
			{F: "propF" + suf, O: EntityFilterOpEq, V: []any{bigDec}, R: gx == 1},
			{F: "propG" + suf, O: EntityFilterOpEq, V: []any{"AAA"}, R: gx == 1},
			{F: "propH" + suf, O: EntityFilterOpEq, V: []any{int64(123)}, R: gx == 1},
			{F: "propI" + suf, O: EntityFilterOpEq, V: []any{float64(456.789)}, R: gx == 1},
			// ne
			{F: "propA" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 1},
			{F: "propB" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 1},
			{F: "propC" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 1},
			{F: "propD" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 1},
			{F: "propE" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 1},
			{F: "propF" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 1},
			{F: "propG" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 1},
			{F: "propH" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 1},
			{F: "propI" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 1},
			// ---
			{F: "propA" + suf, O: EntityFilterOpNe, V: []any{"pa"}, R: false},
			{F: "propB" + suf, O: EntityFilterOpNe, V: []any{"pb"}, R: false},
			{F: "propC" + suf, O: EntityFilterOpNe, V: []any{true}, R: false},
			{F: "propD" + suf, O: EntityFilterOpNe, V: []any{int32(123)}, R: false},
			{F: "propE" + suf, O: EntityFilterOpNe, V: []any{&bigNum}, R: false},
			{F: "propF" + suf, O: EntityFilterOpNe, V: []any{bigDec}, R: false},
			{F: "propG" + suf, O: EntityFilterOpNe, V: []any{"AAA"}, R: false},
			{F: "propH" + suf, O: EntityFilterOpNe, V: []any{int64(123)}, R: false},
			{F: "propI" + suf, O: EntityFilterOpNe, V: []any{float64(456.789)}, R: false},
		}...)
		// gt & lt & ge & le
		for _, op := range []EntityFilterOp{EntityFilterOpGt, EntityFilterOpLt, EntityFilterOpGe, EntityFilterOpLe} {
			for p := 'A'; p <= 'I'; p++ {
				// always false because has null value
				testcases = append(testcases, Testcase{F: "prop" + string([]rune{p}) + suf, O: op, V: []any{nil}, R: false})
			}
		}
		// like
		testcases = append(testcases, Testcase{F: "propA" + suf, O: EntityFilterOpLike, V: []any{nil}, R: false})
		testcases = append(testcases, Testcase{F: "propA" + suf, O: EntityFilterOpLike, V: []any{"%"}, R: gx == 1})
		// !like
		testcases = append(testcases, Testcase{F: "propA" + suf, O: EntityFilterOpNotLike, V: []any{nil}, R: false})
		testcases = append(testcases, Testcase{F: "propA" + suf, O: EntityFilterOpNotLike, V: []any{"%"}, R: false})
	}

	// about array field
	for gx := 3; gx <= 4; gx++ {
		suf := fmt.Sprintf("%d", gx)
		testcases = append(testcases, []Testcase{
			// eq
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 4},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 4},
			{F: "propC" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 4},
			{F: "propD" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 4},
			{F: "propE" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 4},
			{F: "propF" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 4},
			{F: "propG" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 4},
			{F: "propH" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 4},
			{F: "propI" + suf, O: EntityFilterOpEq, V: []any{nil}, R: gx == 4},
			// ---
			{F: "propA" + suf, O: EntityFilterOpEq, V: []any{[]string{"pa1", "pa2"}}, R: gx == 3},
			{F: "propB" + suf, O: EntityFilterOpEq, V: []any{[]string{"pb1", "pb2"}}, R: gx == 3},
			{F: "propC" + suf, O: EntityFilterOpEq, V: []any{[]bool{true, false}}, R: gx == 3},
			{F: "propD" + suf, O: EntityFilterOpEq, V: []any{[]int32{1, 23, 456}}, R: gx == 3},
			{F: "propE" + suf, O: EntityFilterOpEq, V: []any{[]*big.Int{&bigNum}}, R: gx == 3},
			{F: "propF" + suf, O: EntityFilterOpEq, V: []any{[]decimal.Decimal{bigDec}}, R: gx == 3},
			{F: "propG" + suf, O: EntityFilterOpEq, V: []any{[]string{"AAA", "BBB"}}, R: gx == 3},
			{F: "propH" + suf, O: EntityFilterOpEq, V: []any{[]int64{123}}, R: gx == 3},
			{F: "propI" + suf, O: EntityFilterOpEq, V: []any{[]float64{456.789}}, R: gx == 3},
			// ne
			{F: "propA" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 3},
			{F: "propB" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 3},
			{F: "propC" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 3},
			{F: "propD" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 3},
			{F: "propE" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 3},
			{F: "propF" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 3},
			{F: "propG" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 3},
			{F: "propH" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 3},
			{F: "propI" + suf, O: EntityFilterOpNe, V: []any{nil}, R: gx == 3},
			// ---
			{F: "propA" + suf, O: EntityFilterOpNe, V: []any{[]string{"pa1", "pa2"}}, R: false},
			{F: "propB" + suf, O: EntityFilterOpNe, V: []any{[]string{"pb1", "pb2"}}, R: false},
			{F: "propC" + suf, O: EntityFilterOpNe, V: []any{[]bool{true, false}}, R: false},
			{F: "propD" + suf, O: EntityFilterOpNe, V: []any{[]int32{1, 23, 456}}, R: false},
			{F: "propE" + suf, O: EntityFilterOpNe, V: []any{[]*big.Int{&bigNum}}, R: false},
			{F: "propF" + suf, O: EntityFilterOpNe, V: []any{[]decimal.Decimal{bigDec}}, R: false},
			{F: "propG" + suf, O: EntityFilterOpNe, V: []any{[]string{"AAA", "BBB"}}, R: false},
			{F: "propH" + suf, O: EntityFilterOpNe, V: []any{[]int64{123}}, R: false},
			{F: "propI" + suf, O: EntityFilterOpNe, V: []any{[]float64{456.789}}, R: false},
		}...)
	}

	for i, tc := range testcases {
		msg := fmt.Sprintf("testcase #%d %#v", i, tc)
		filter := EntityFilter{Field: entityType.GetFieldByName(tc.F), Op: tc.O, Value: tc.V}
		cr, err = checkFilter(filter, box)
		assert.Equal(t, tc.R, cr, msg)
		assert.NoError(t, err, msg)
	}
}

func Test_checkFilters(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	entityType := sch.GetEntity("EntityD")

	var cr bool

	box := EntityBox{
		Data: map[string]any{
			"id":     "id",
			"propA1": "pa",
			"propB1": "pb",
		},
	}

	// propA1 == "pa" && propB1 == "pb"
	cr, err = checkFilters([]EntityFilter{
		{
			Field: entityType.GetFieldByName("propA1"),
			Op:    EntityFilterOpEq,
			Value: []any{"pa"},
		},
		{
			Field: entityType.GetFieldByName("propB1"),
			Op:    EntityFilterOpEq,
			Value: []any{"pb"},
		},
	}, box)
	assert.True(t, cr)
	assert.NoError(t, err)
	// propA1 == "pa" && propB1 != "pb"
	cr, err = checkFilters([]EntityFilter{
		{
			Field: entityType.GetFieldByName("propA1"),
			Op:    EntityFilterOpEq,
			Value: []any{"pa"},
		},
		{
			Field: entityType.GetFieldByName("propB1"),
			Op:    EntityFilterOpNe,
			Value: []any{"pb"},
		},
	}, box)
	assert.False(t, cr)
	assert.NoError(t, err)
	// propA1 != "pa" && propB1 != "pb"
	cr, err = checkFilters([]EntityFilter{
		{
			Field: entityType.GetFieldByName("propA1"),
			Op:    EntityFilterOpNe,
			Value: []any{"pa"},
		},
		{
			Field: entityType.GetFieldByName("propB1"),
			Op:    EntityFilterOpNe,
			Value: []any{"pb"},
		},
	}, box)
	assert.False(t, cr)
	assert.NoError(t, err)
	// propA1 != "pa" && propB1 == "pb"
	cr, err = checkFilters([]EntityFilter{
		{
			Field: entityType.GetFieldByName("propA1"),
			Op:    EntityFilterOpNe,
			Value: []any{"pa"},
		},
		{
			Field: entityType.GetFieldByName("propB1"),
			Op:    EntityFilterOpEq,
			Value: []any{"pb"},
		},
	}, box)
	assert.False(t, cr)
	assert.NoError(t, err)
}
