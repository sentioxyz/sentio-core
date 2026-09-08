package persistent

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	rsh "sentioxyz/sentio-core/common/richstructhelper"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	entityProtos "sentioxyz/sentio-core/processor/protos"
	"sentioxyz/sentio-core/service/common/protos"
)

func mustBigInt(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("invalid big int " + s)
	}
	return n
}

// dRow is a fully populated EntityD row used as the previous version in evaluation tests.
func dRow() expRow {
	return expRow{exists: true, data: map[string]any{
		"id":       "d0",
		"propA1":   "abc",
		"propA2":   (*string)(nil),
		"propB1":   "0x0a",
		"propC1":   true,
		"propC2":   utils.WrapPointer(false),
		"propD1":   int32(10),
		"propD2":   (*int32)(nil),
		"propE1":   big.NewInt(1000),
		"propE2":   (*big.Int)(nil),
		"propF1":   decimal.RequireFromString("1.5"),
		"propF2":   utils.WrapPointer(decimal.RequireFromString("2.5")),
		"propG1":   "BBB",
		"propH1":   int64(1700000000000000),
		"propI1":   float64(0.25),
		"propJ2":   utils.WrapPointer(int64(7)),
		"foreign1": "0x0a00",
		"foreign2": utils.WrapPointer("0x0a01"),
	}}
}

func TestCompileUpdateExp_errors(t *testing.T) {
	e := numEntity(t)
	cases := []struct {
		field string
		exp   string
		err   string
	}{
		{"propD1", "", "empty expression"},
		{"propD1", "propX", "unknown field"},
		{"propD1", "propA3 + 1", "list type"},
		{"propD1", "propA1 + 1", "argument #1 is string, expect number"},
		{"propD1", "propA1", "expression result is string but field EntityD.propD1 is number"},
		{"propA1", "propD1 * 2", "expression result is number but field EntityD.propA1 is string"},
		{"propC1", "propD1", "expression result is number but field EntityD.propC1 is boolean"},
		{"propD1", "foo(1)", "unsupported operator 'foo'"},
		{"propD1", "if(propC1, 1)", "unsupported operator 'if' at expression[0..1] with 2 arguments"},
		{"propD1", "if(propD1, 1, 2)", "argument #1 is number, expect boolean"},
		{"propD1", "if(propC1, 1, 'a')", "argument #2 is number but argument #3 is string"},
		{"propC1", "propC1 > propC2", "boolean is not comparable"},
		{"propC1", "propD1 = propA1", "argument #1 is number but argument #2 is string"},
		{"propC1", "propD1 and propC1", "argument #1 is number, expect boolean"},
		{"propC1", "not propD1", "argument #1 is number, expect boolean"},
		{"propC1", "exist(propD1)", "unsupported operator 'exist' at expression[0..4] with 1 arguments"},
		{"propD1", "coalesce()", "unsupported operator 'coalesce' at expression[0..7] with 0 arguments"},
		{"propD1", "coalesce(propD1, 'a')", "argument #2 is string but argument #1 is number"},
		{"propD1", "isNull()", "with 0 arguments"},
		{"propA3", "propA1", "does not support expression"},
		{"propD1", "propD1 +", "empty expression"},
		{"propD1", "propD1 / 0", "division by zero"},
		{"propD1", "propD1 / (0.0)", "division by zero"},
		{"propD1", "1 / -0", "division by zero"},
	}
	for i, c := range cases {
		field := e.GetFieldByName(c.field)
		_, err := compileUpdateExp(e, field, c.exp)
		assert.ErrorContainsf(t, err, c.err, "case #%d: %s = %s", i, c.field, c.exp)
	}

	// derived fields are not stored, so they cannot be referenced
	sch, err := schema.ParseAndVerifySchema(testSchema)
	assert.NoError(t, err)
	ea := sch.GetEntity("EntityA")
	_, err = compileUpdateExp(ea, ea.GetFieldByName("foreignA"), "foreignF")
	assert.ErrorContains(t, err, "is derived and cannot be referenced")
}

func TestCompileUpdateExp_kinds(t *testing.T) {
	e := numEntity(t)
	cases := []struct {
		field  string
		exp    string
		kind   expKind
		fields []string
	}{
		{"propD1", "1", kindNumber, nil},
		{"propD1", "null", kindNull, nil},
		{"propD1", "propD1 + propD2 * 2", kindNumber, []string{"propD1", "propD2"}},
		{"propD1", "if(exist(), propD1, null)", kindNumber, []string{"propD1"}},
		{"propD1", "if(true, null, null)", kindNull, nil},
		{"propD1", "coalesce(null, propJ2, 0)", kindNumber, []string{"propJ2"}},
		{"propA1", "coalesce(propA2, 'x')", kindString, []string{"propA2"}},
		{"propA1", "foreign1", kindString, []string{"foreign1"}},
		{"propA1", "propG1", kindString, []string{"propG1"}},
		{
			"propC1", "propH1 > 0 and not isNull(propF2) or propA1 = 'abc'",
			kindBool, []string{"propH1", "propF2", "propA1"},
		},
		{"propC1", "'a' <= 'b'", kindBool, nil},
		{"propC1", "null = 1", kindBool, nil},
		{"propC1", "ISNULL(PROPD1)", kindBool, nil}, // functions are case-insensitive, field names are not
	}
	for i, c := range cases {
		field := e.GetFieldByName(c.field)
		if c.exp == "ISNULL(PROPD1)" {
			_, err := compileUpdateExp(e, field, c.exp)
			assert.ErrorContainsf(t, err, "unknown field 'PROPD1'", "case #%d", i)
			continue
		}
		compiled, err := compileUpdateExp(e, field, c.exp)
		assert.NoErrorf(t, err, "case #%d: %s = %s", i, c.field, c.exp)
		assert.Equalf(t, c.kind, compiled.kind, "case #%d: %s = %s", i, c.field, c.exp)
		assert.ElementsMatchf(t, c.fields, compiled.fields, "case #%d: %s = %s", i, c.field, c.exp)
		assert.Equal(t, c.exp, compiled.String())
	}
}

func TestCompiledExp_eval(t *testing.T) {
	e := numEntity(t)
	row := dRow()
	none := expRow{exists: false}
	num := func(s string) expValue { return expNumber(decimal.RequireFromString(s)) }
	cases := []struct {
		field string
		exp   string
		row   expRow
		want  expValue
	}{
		// arithmetic
		{"propD1", "propD1 + 5", row, num("15")},
		{"propD1", "propD1 - 5 * 2", row, num("0")},
		{"propD1", "(propD1 - 5) * 2", row, num("10")},
		{"propD1", "propD1 / 4", row, num("2.5")},
		{"propD1", "-1 * propD1", row, num("-10")},
		{"propF1", "propF1 + propF2 + propI1", row, num("4.25")},
		{"propE1", "propE1 * 1e18", row, num("1000000000000000000000")},
		{"propH1", "propH1 + 1", row, num("1700000000000001")},
		{"propJ2", "propJ2 * 3", row, num("21")},
		// null propagation
		{"propD1", "propD2 + 1", row, expNull},
		{"propD1", "propD1 + null", row, expNull},
		{"propD1", "propD1 + 1", none, expNull},
		{"propC1", "propD2 = 0", row, expNull},
		{"propC1", "propD1 > propD2", row, expNull},
		{"propC1", "not propD2 = 0", row, expNull},
		// isNull / exist / coalesce / if
		{"propC1", "isNull(propD2)", row, expBool(true)},
		{"propC1", "isNull(propD1)", row, expBool(false)},
		{"propC1", "isNull(propD1)", none, expBool(true)},
		{"propC1", "isNull(0)", none, expBool(false)},
		{"propC1", "isNull(propD2 + 1)", row, expBool(true)},
		{"propC1", "exist()", row, expBool(true)},
		{"propC1", "exists()", none, expBool(false)},
		{"propD1", "coalesce(propD2, propD1, 0)", row, num("10")},
		{"propD1", "coalesce(propD2, null)", row, expNull},
		{"propD1", "coalesce(propD1, 0) + 1", none, num("1")},
		{"propD1", "if(exist(), propD1 + 1, 100)", row, num("11")},
		{"propD1", "if(exist(), propD1 + 1, 100)", none, num("100")},
		{"propD1", "if(propD2 = 1, 1, 2)", row, num("2")},     // null condition takes the false branch
		{"propD1", "if(true, 1, 1 / propD2)", none, num("1")}, // the unused branch is not evaluated
		{"propD1", "coalesce(1, 1 / (propD1 - 10))", row, num("1")},
		// comparisons
		{"propC1", "propD1 > 9", row, expBool(true)},
		{"propC1", "propD1 >= 10", row, expBool(true)},
		{"propC1", "propD1 < 10", row, expBool(false)},
		{"propC1", "propD1 <= 10", row, expBool(true)},
		{"propC1", "propD1 = 10.0", row, expBool(true)},
		{"propC1", "propD1 != 10", row, expBool(false)},
		{"propC1", "propA1 = 'abc'", row, expBool(true)},
		{"propC1", "propA1 > 'abb'", row, expBool(true)},
		{"propC1", "propG1 = 'BBB'", row, expBool(true)},
		{"propC1", "foreign1 = '0x0a00'", row, expBool(true)},
		{"propC1", "foreign2 = '0x0a01'", row, expBool(true)},
		{"propC1", "propB1 = '0x0a'", row, expBool(true)},
		{"propC1", "propC1 = true", row, expBool(true)},
		{"propC1", "propC2 = propC1", row, expBool(false)},
		{"propC1", "propE1 = 1000", row, expBool(true)},
		{"propC1", "propE2 = 0", row, expNull},
		// three-valued logic
		{"propC1", "propC1 and propC2", row, expBool(false)},
		{"propC1", "propC1 or propC2", row, expBool(true)},
		{"propC1", "propC1 and null", row, expNull},
		{"propC1", "propC2 and null", row, expBool(false)},
		{"propC1", "propC1 or null", row, expBool(true)},
		{"propC1", "propC2 or null", row, expNull},
		{"propC1", "not propC2", row, expBool(true)},
		{"propC1", "not null", row, expNull},
		{"propC1", "propC1 and not propC2 or false", row, expBool(true)},
		// strings
		{"propA1", "if(propD1 > 5, 'big', 'small')", row, expString("big")},
		{"propA1", "coalesce(propA2, propA1)", row, expString("abc")},
		{"propA1", "'it\\'s'", row, expString("it's")},
		{"propA1", "propA2", row, expNull},
		{"propA1", "null", row, expNull},
	}
	for i, c := range cases {
		field := e.GetFieldByName(c.field)
		compiled, err := compileUpdateExp(e, field, c.exp)
		if !assert.NoErrorf(t, err, "case #%d: %s = %s", i, c.field, c.exp) {
			continue
		}
		got, err := compiled.eval(c.row)
		assert.NoErrorf(t, err, "case #%d: %s = %s", i, c.field, c.exp)
		if c.want.kind == kindNumber {
			assert.Equalf(t, kindNumber, got.kind, "case #%d: %s = %s", i, c.field, c.exp)
			assert.Truef(t, c.want.n.Equal(got.n), "case #%d: %s = %s, want %s got %s", i, c.field, c.exp, c.want, got)
		} else {
			assert.Equalf(t, c.want, got, "case #%d: %s = %s", i, c.field, c.exp)
		}
	}

	// runtime errors
	compiled, err := compileUpdateExp(e, e.GetFieldByName("propD1"), "propD1 / (propD1 - 10)")
	assert.NoError(t, err)
	_, err = compiled.eval(row)
	assert.ErrorContains(t, err, "division by zero")
	// a null divisor is null, not an error
	compiled, err = compileUpdateExp(e, e.GetFieldByName("propD1"), "propD1 / propD2")
	assert.NoError(t, err)
	got, err := compiled.eval(row)
	assert.NoError(t, err)
	assert.True(t, got.isNull())
}

func TestExpValueToField(t *testing.T) {
	e := numEntity(t)
	num := func(s string) expValue { return expNumber(decimal.RequireFromString(s)) }
	cases := []struct {
		field string
		val   expValue
		want  any
	}{
		{"propD1", num("2.5"), int32(3)},
		{"propD1", num("2.4"), int32(2)},
		{"propD2", num("3"), utils.WrapPointer(int32(3))},
		{"propD1", expNull, nil},
		{"propD2", expNull, (*int32)(nil)},
		{"propJ1", num("7"), int64(7)},
		{"propJ2", num("7"), utils.WrapPointer(int64(7))},
		{"propJ2", expNull, (*int64)(nil)},
		{"propH1", num("1700000000000000"), int64(1700000000000000)},
		{"propE1", num("123456789012345678901234567890"), mustBigInt("123456789012345678901234567890")},
		{"propE2", num("1.6"), big.NewInt(2)},
		{"propE1", expNull, nil},
		{"propE2", expNull, (*big.Int)(nil)},
		{"propI1", num("0.25"), float64(0.25)},
		{"propI2", num("0.25"), utils.WrapPointer(float64(0.25))},
		{"propF1", num("1.5"), decimal.RequireFromString("1.5")},
		{"propF2", num("1.5"), utils.WrapPointer(decimal.RequireFromString("1.5"))},
		{"propF2", expNull, (*decimal.Decimal)(nil)},
		{"propC1", expBool(true), true},
		{"propC2", expBool(false), utils.WrapPointer(false)},
		{"propC2", expNull, (*bool)(nil)},
		{"propA1", expString("x"), "x"},
		{"propA2", expString("x"), utils.WrapPointer("x")},
		{"propA2", expNull, (*string)(nil)},
		{"propB1", expString("0x0a"), "0x0a"},
		{"propG1", expString("CCC"), "CCC"},
		{"propG2", expString("CCC"), utils.WrapPointer("CCC")},
		{"foreign1", expString("0x0a00"), "0x0a00"},
		{"foreign2", expString("0x0a00"), utils.WrapPointer("0x0a00")},
	}
	for i, c := range cases {
		field := e.GetFieldByName(c.field)
		got, err := expValueToField(field.Type, c.val)
		assert.NoErrorf(t, err, "case #%d: %s = %s", i, c.field, c.val)
		assert.Equalf(t, c.want, got, "case #%d: %s = %s", i, c.field, c.val)
	}

	_, err := expValueToField(e.GetFieldByName("propD1").Type, expString("x"))
	assert.ErrorContains(t, err, "is string, expect number")
	_, err = expValueToField(e.GetFieldByName("propA3").Type, expString("x"))
	assert.ErrorContains(t, err, "list type")
}

func TestExpValueFromField(t *testing.T) {
	for i, c := range []struct {
		in   any
		want expValue
	}{
		{nil, expNull},
		{(*int32)(nil), expNull},
		{(*big.Int)(nil), expNull},
		{int32(1), expNumber(decimal.NewFromInt(1))},
		{utils.WrapPointer(int32(1)), expNumber(decimal.NewFromInt(1))},
		{int64(2), expNumber(decimal.NewFromInt(2))},
		{big.NewInt(3), expNumber(decimal.NewFromInt(3))},
		{*big.NewInt(3), expNumber(decimal.NewFromInt(3))},
		{decimal.NewFromInt(4), expNumber(decimal.NewFromInt(4))},
		{utils.WrapPointer(decimal.NewFromInt(4)), expNumber(decimal.NewFromInt(4))},
		{float64(0.5), expNumber(decimal.NewFromFloat(0.5))},
		{"s", expString("s")},
		{utils.WrapPointer("s"), expString("s")},
		{[]byte{0x0a}, expString("0x0a")},
		{true, expBool(true)},
		{utils.WrapPointer(false), expBool(false)},
	} {
		got, err := expValueFromField(c.in)
		assert.NoErrorf(t, err, "case #%d: %v", i, c.in)
		assert.Equalf(t, c.want, got, "case #%d: %v", i, c.in)
	}
	_, err := expValueFromField([]string{"a"})
	assert.ErrorContains(t, err, "unsupported field value")
}

// ─── FromEntityUpdateData ────────────────────────────────────────────────────

func TestFromEntityUpdateData_expression(t *testing.T) {
	e := numEntity(t)
	var box UncommittedEntityBox
	err := box.FromEntityUpdateData(e, &entityProtos.EntityUpdateData{
		Fields: map[string]*entityProtos.EntityUpdateData_FieldValue{
			"propD1": {Op: entityProtos.EntityUpdateData_EXPRESSION, Expression: "coalesce(propD1, 0) + propJ1"},
			"propA1": {Op: entityProtos.EntityUpdateData_SET, Value: rsh.NewStringValue("x")},
			"propJ1": {Op: entityProtos.EntityUpdateData_ADD, Value: rsh.NewIntValue(1)},
		},
	})
	assert.NoError(t, err)
	assert.Empty(t, box.Data)
	assert.Equal(t, Operator{Set: &operatorSet{Value: "x"}}, box.Operator[0]["propA1"])
	assert.True(t, box.HasExpression())
	assert.Equal(t, "coalesce(propD1, 0) + propJ1", box.Operator[0]["propD1"].Exp.String())
	assert.Nil(t, box.Operator[0]["propD1"].NumCalc)
	assert.Equal(t, "x*1+1", box.Operator[0]["propJ1"].String())
	// every other field keeps its latest value
	assert.True(t, box.Operator[0]["propD2"].RemainLatest())
	assert.Equal(t, "x", box.Operator[0]["propD2"].String())

	err = box.FromEntityUpdateData(e, &entityProtos.EntityUpdateData{
		Fields: map[string]*entityProtos.EntityUpdateData_FieldValue{
			"propD1": {Op: entityProtos.EntityUpdateData_EXPRESSION, Expression: "propA1 + 1"},
		},
	})
	assert.ErrorContains(t, err, `invalid expression "propA1 + 1" for EntityD.propD1`)
	assert.False(t, box.HasExpression())
}

// ─── Merge ───────────────────────────────────────────────────────────────────

func exprOp(t *testing.T, e *schema.Entity, field, text string) Operator {
	t.Helper()
	compiled, err := compileUpdateExp(e, e.GetFieldByName(field), text)
	assert.NoError(t, err)
	return Operator{Exp: compiled}
}

func TestUncommittedEntityBox_Merge_expression(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchema)
	assert.NoError(t, err)
	e := sch.GetEntity("EntityE1")

	t.Run("expression reads the state before the same update covers other fields", func(t *testing.T) {
		box := &UncommittedEntityBox{EntityBox: EntityBox{
			Entity: "EntityE1", ID: "e", GenBlockNumber: 3,
			Data: map[string]any{"id": "e", "propA": "a", "propB": int32(1)},
		}}
		err := box.Merge(e, &UncommittedEntityBox{
			EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3, Data: map[string]any{"propA": "b"}},
			Operator: []map[string]Operator{{
				"propB": exprOp(t, e, "propB", "if(propA = 'a', propB + 10, -1)"),
				"id":    {},
			}},
		})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"id": "e", "propA": "b", "propB": int32(11)}, box.Data)
		assert.Empty(t, box.Operator)
	})

	t.Run("update after delete in the same block sees no previous version", func(t *testing.T) {
		box := &UncommittedEntityBox{EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3}}
		err := box.Merge(e, &UncommittedEntityBox{
			EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3, Data: map[string]any{}},
			Operator: []map[string]Operator{{
				"propB": exprOp(t, e, "propB", "if(exist(), propB + 10, coalesce(propB, 5))"),
				"propA": exprOp(t, e, "propA", "coalesce(propA, 'new')"),
				"id":    {},
			}},
		})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"id": "", "propA": "new", "propB": int32(5)}, box.Data)
	})

	t.Run("rounds with expressions are never merged with other rounds", func(t *testing.T) {
		box := &UncommittedEntityBox{
			EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3, Data: map[string]any{}},
			Operator:  []map[string]Operator{{"propA": {Set: &operatorSet{Value: "a"}}, "propB": intOp(1, 3), "id": {}}},
		}
		// round 2: an expression always starts a new round
		assert.NoError(t, box.Merge(e, &UncommittedEntityBox{
			EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3, Data: map[string]any{}},
			Operator:  []map[string]Operator{{"propB": exprOp(t, e, "propB", "propB * 2"), "propA": {}, "id": {}}},
		}))
		// round 3: a plain write does not fold into a round with expressions either
		assert.NoError(t, box.Merge(e, &UncommittedEntityBox{
			EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3, Data: map[string]any{}},
			Operator:  []map[string]Operator{{"propA": {Set: &operatorSet{Value: "b"}}, "propB": intOp(1, 100), "id": {}}},
		}))
		// still round 3: plain writes compose into a plain last round (SET replaces, ADD stacks)
		assert.NoError(t, box.Merge(e, &UncommittedEntityBox{
			EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3, Data: map[string]any{}},
			Operator:  []map[string]Operator{{"propA": {Set: &operatorSet{Value: "c"}}, "propB": intOp(2, 1), "id": {}}},
		}))
		assert.Empty(t, box.Data)
		assert.Len(t, box.Operator, 3)
		assert.Equal(t, "x*1+3", box.Operator[0]["propB"].String())
		assert.Equal(t, "propB * 2", box.Operator[1]["propB"].String())
		assert.Equal(t, "x*2+201", box.Operator[2]["propB"].String())
		assert.Equal(t, Operator{Set: &operatorSet{Value: "c"}}, box.Operator[2]["propA"])
		assert.True(t, box.Operator[2]["id"].RemainLatest())
		assert.False(t, box.Resolved())

		// resolve against a previous version: propB = ((5 + 3) * 2) * 2 + 201
		row := expRow{exists: true, data: map[string]any{"id": "e", "propA": "z", "propB": int32(5)}}
		for _, round := range box.Operator {
			next := utils.CopyMap(row.data)
			assert.NoError(t, resolveRound(e, round, row, next))
			row = expRow{exists: true, data: next}
		}
		assert.Equal(t, map[string]any{"id": "e", "propA": "c", "propB": int32(233)}, row.data)
	})

	t.Run("an upsert on a pending box travels as a round", func(t *testing.T) {
		box := &UncommittedEntityBox{
			EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3, Data: map[string]any{}},
			Operator: []map[string]Operator{
				{"propB": intOp(1, 3), "propA": {}, "id": {}},
				{"propB": exprOp(t, e, "propB", "propB * 2")},
			},
		}
		assert.NoError(t, box.Merge(e, &UncommittedEntityBox{EntityBox: EntityBox{
			Entity: "EntityE1", ID: "e", GenBlockNumber: 3,
			Data: map[string]any{"id": "e", "propA": "u", "propB": int32(7)},
		}}))
		assert.Len(t, box.Operator, 3)
		assert.Equal(t, "u", box.Operator[2]["propA"].Set.Value)
		assert.Equal(t, int32(7), box.Operator[2]["propB"].Set.Value)
	})

	t.Run("a write with several rounds is rejected", func(t *testing.T) {
		box := &UncommittedEntityBox{
			EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3, Data: map[string]any{}},
		}
		err := box.Merge(e, &UncommittedEntityBox{
			EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3, Data: map[string]any{}},
			Operator:  []map[string]Operator{{"propB": intOp(1, 3)}, {"propB": intOp(1, 4)}},
		})
		assert.ErrorContains(t, err, "merge entity with 2 pending rounds, expect at most one")
	})

	t.Run("delete drops every pending round", func(t *testing.T) {
		box := &UncommittedEntityBox{
			EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3, Data: map[string]any{}},
			Operator: []map[string]Operator{
				{"propB": intOp(1, 3), "propA": {}, "id": {}},
				{"propB": exprOp(t, e, "propB", "propB * 2")},
			},
		}
		deleted := &UncommittedEntityBox{EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3}}
		assert.NoError(t, box.Merge(e, deleted))
		assert.Nil(t, box.Data)
		assert.True(t, box.Resolved())
	})

	t.Run("evaluation error is reported", func(t *testing.T) {
		box := &UncommittedEntityBox{EntityBox: EntityBox{
			Entity: "EntityE1", ID: "e", GenBlockNumber: 3,
			Data: map[string]any{"id": "e", "propA": "a", "propB": int32(0)},
		}}
		err := box.Merge(e, &UncommittedEntityBox{
			EntityBox: EntityBox{Entity: "EntityE1", ID: "e", GenBlockNumber: 3, Data: map[string]any{}},
			Operator:  []map[string]Operator{{"propB": exprOp(t, e, "propB", "1 / propB")}},
		})
		assert.ErrorContains(t, err, "division by zero")
	})
}

// ─── Controller ──────────────────────────────────────────────────────────────

type fieldValues = map[string]*entityProtos.EntityUpdateData_FieldValue

func updateReq(fields fieldValues) *entityProtos.EntityUpdateData {
	return &entityProtos.EntityUpdateData{Fields: fields}
}

func exprField(text string) *entityProtos.EntityUpdateData_FieldValue {
	return &entityProtos.EntityUpdateData_FieldValue{Op: entityProtos.EntityUpdateData_EXPRESSION, Expression: text}
}

func addField(v int32) *entityProtos.EntityUpdateData_FieldValue {
	return &entityProtos.EntityUpdateData_FieldValue{Op: entityProtos.EntityUpdateData_ADD, Value: rsh.NewIntValue(v)}
}

func setField(v *protos.RichValue) *entityProtos.EntityUpdateData_FieldValue {
	return &entityProtos.EntityUpdateData_FieldValue{Op: entityProtos.EntityUpdateData_SET, Value: v}
}

func TestController_UpdateWithExpression(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchema)
	assert.NoError(t, err)
	e := sch.GetEntity("EntityE1")
	ctx := context.Background()

	newBox := func(id string, block uint64, data *entityProtos.EntityUpdateData) UncommittedEntityBox {
		box := UncommittedEntityBox{EntityBox: EntityBox{ID: id, GenBlockNumber: block, GenBlockHash: "0x1234"}}
		assert.NoError(t, box.FromEntityUpdateData(e, data))
		return box
	}
	seed := func(ps *mockChainStore, id string, propA string, propB int32) {
		utils.PutIntoK2Map(ps.data, "EntityE1", id, &EntityBox{
			Entity: "EntityE1", ID: id, GenBlockNumber: 10, GenBlockHash: "0x1234",
			Data: map[string]any{"id": id, "propA": propA, "propB": propB},
		})
	}
	getData := func(ctrl *Controller, id string, block uint64) map[string]any {
		box, err := ctrl.GetEntity(ctx, e, id, block)
		assert.NoError(t, err)
		if box == nil {
			return nil
		}
		return box.Data
	}

	t.Run("expression against the persistent version", func(t *testing.T) {
		ps, s := newTestStore(sch, "mainnet")
		seed(ps, "e0", "a", 10)
		ctrl, _ := newCtrl(s)
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", 11, updateReq(fieldValues{
			"propB": exprField("if(propA = 'a', propB * 2, 0)"),
			"propA": exprField("if(propB > 5, 'big', 'small')"),
		}))))
		// both expressions read the version at block 10
		assert.Equal(t, map[string]any{"id": "e0", "propA": "big", "propB": int32(20)}, getData(ctrl, "e0", 11))
		assert.Equal(t, map[string]any{"id": "e0", "propA": "a", "propB": int32(10)}, getData(ctrl, "e0", 10))

		_, _, err := ctrl.Commit(ctx, 11, time.Now())
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"id": "e0", "propA": "big", "propB": int32(20)}, ps.data["EntityE1"]["e0"].Data)
	})

	t.Run("expression for a new entity", func(t *testing.T) {
		ps, s := newTestStore(sch, "mainnet")
		ctrl, _ := newCtrl(s)
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e1", 11, updateReq(fieldValues{
			"propB": exprField("coalesce(propB, 0) + 1"),
			"propA": exprField("if(exist(), 'old', 'new')"),
		}))))
		assert.Equal(t, map[string]any{"id": "", "propA": "new", "propB": int32(1)}, getData(ctrl, "e1", 11))

		// a null result for a non-null field is left as nil for CheckValue to reject
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e2", 11, updateReq(fieldValues{
			"propB": exprField("propB + 1"),
		}))))
		assert.Equal(t, map[string]any{"id": "", "propA": "", "propB": nil}, getData(ctrl, "e2", 11))
		_ = ps
	})

	t.Run("same block: ADD then expression resolves round by round", func(t *testing.T) {
		ps, s := newTestStore(sch, "mainnet")
		seed(ps, "e0", "a", 10)
		ctrl, _ := newCtrl(s)
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", 11, updateReq(fieldValues{
			"propB": addField(5),
		}))))
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", 11, updateReq(fieldValues{
			"propB": exprField("propB * 2"),
			"propA": exprField("if(propB > 12, 'big', 'small')"),
		}))))
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", 11, updateReq(fieldValues{
			"propB": addField(1),
		}))))
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", 11, updateReq(fieldValues{
			"propA": setField(rsh.NewStringValue("set")),
			"propB": exprField("if(propA = 'big', propB + 1000, propB)"),
		}))))
		history, _ := utils.GetFromK2Map(ctrl.changes, "EntityE1", "e0")
		assert.Len(t, history, 1)
		assert.Len(t, history[0].Operator, 4)
		// (10 + 5) * 2 + 1 + 1000, propA was "big" when the last expression read it
		assert.Equal(t, map[string]any{"id": "e0", "propA": "set", "propB": int32(1031)}, getData(ctrl, "e0", 11))
		assert.True(t, history[0].Resolved())

		_, _, err := ctrl.Commit(ctx, 11, time.Now())
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"id": "e0", "propA": "set", "propB": int32(1031)}, ps.data["EntityE1"]["e0"].Data)
	})

	t.Run("same block: expression then ADD stays as two rounds", func(t *testing.T) {
		ps, s := newTestStore(sch, "mainnet")
		seed(ps, "e0", "a", 10)
		ctrl, _ := newCtrl(s)
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", 11, updateReq(fieldValues{
			"propB": exprField("propB * 2"),
		}))))
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", 11, updateReq(fieldValues{
			"propB": addField(5),
		}))))
		history, _ := utils.GetFromK2Map(ctrl.changes, "EntityE1", "e0")
		assert.Len(t, history, 1)
		assert.Len(t, history[0].Operator, 2)
		assert.Equal(t, "propB * 2", history[0].Operator[0]["propB"].String())
		assert.Equal(t, "x*1+5", history[0].Operator[1]["propB"].String())
		assert.Equal(t, map[string]any{"id": "e0", "propA": "a", "propB": int32(25)}, getData(ctrl, "e0", 11))
	})

	t.Run("same block: SET then expression, expression then expression", func(t *testing.T) {
		ps, s := newTestStore(sch, "mainnet")
		seed(ps, "e0", "a", 10)
		ctrl, _ := newCtrl(s)
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", 11, updateReq(fieldValues{
			"propA": setField(rsh.NewStringValue("b")),
		}))))
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", 11, updateReq(fieldValues{
			"propB": exprField("if(propA = 'b', propB + 1, propB - 1)"),
		}))))
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", 11, updateReq(fieldValues{
			"propB": exprField("propB * 10"),
		}))))
		assert.Equal(t, map[string]any{"id": "e0", "propA": "b", "propB": int32(110)}, getData(ctrl, "e0", 11))
	})

	t.Run("expressions across blocks resolve in order at commit", func(t *testing.T) {
		ps, s := newTestStore(sch, "mainnet")
		seed(ps, "e0", "a", 1)
		ctrl, _ := newCtrl(s)
		for block := uint64(11); block <= 13; block++ {
			assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", block, updateReq(fieldValues{
				"propB": exprField("propB * 2"),
			}))))
		}
		_, _, err := ctrl.Commit(ctx, 13, time.Now())
		assert.NoError(t, err)
		assert.Equal(t, int32(8), ps.data["EntityE1"]["e0"].Data["propB"])
	})

	t.Run("evaluation error surfaces as ErrInvalidFieldValue", func(t *testing.T) {
		ps, s := newTestStore(sch, "mainnet")
		seed(ps, "e0", "a", 0)
		ctrl, _ := newCtrl(s)
		assert.NoError(t, ctrl.SetEntity(ctx, e, newBox("e0", 11, updateReq(fieldValues{
			"propB": exprField("1 / propB"),
		}))))
		_, err := ctrl.GetEntity(ctx, e, "e0", 11)
		assert.ErrorIs(t, err, ErrInvalidFieldValue)
		assert.ErrorContains(t, err, "division by zero")
	})
}
