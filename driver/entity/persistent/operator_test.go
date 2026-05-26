package persistent

import (
	"math/big"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	rsh "sentioxyz/sentio-core/common/richstructhelper"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// numEntity returns the EntityD entity from testSchema, which contains all
// numeric scalar types in both nullable and non-nullable forms.
func numEntity(t *testing.T) *schema.Entity {
	t.Helper()
	sch, err := schema.ParseAndVerifySchema(testSchema)
	assert.NoError(t, err)
	return sch.GetEntity("EntityD")
}

// intOp builds f(x) = x * multi + add using integer RichValues.
// Suitable for Int and Int8 field operators.
func intOp(multi, add int64) Operator {
	return Operator{NumCalc: &OperatorNumCalc{
		Multi: rsh.NewIntValue(int32(multi)),
		Add:   rsh.NewIntValue(int32(add)),
	}}
}

// bigIntOp builds f(x) = x * multi + add using BigInt RichValues.
// Suitable for BigInt field operators.
func bigIntOp(multi, add int64) Operator {
	return Operator{NumCalc: &OperatorNumCalc{
		Multi: rsh.NewBigIntValue(big.NewInt(multi)),
		Add:   rsh.NewBigIntValue(big.NewInt(add)),
	}}
}

// decOp builds f(x) = x * multi + add using BigDecimal RichValues.
// Suitable for Float and BigDecimal field operators.
func decOp(multi, add float64) Operator {
	return Operator{NumCalc: &OperatorNumCalc{
		Multi: rsh.NewBigDecimalValue(decimal.NewFromFloat(multi)),
		Add:   rsh.NewBigDecimalValue(decimal.NewFromFloat(add)),
	}}
}

// ─── OperatorNumCalc.Calc ────────────────────────────────────────────────────

// TestOperatorNumCalc_Calc verifies the basic arithmetic: f(x) = x * Multi + Add.
func TestOperatorNumCalc_Calc(t *testing.T) {
	cases := []struct {
		origin, multi, add, want int64
	}{
		{5, 2, 3, 13},  // 5*2+3=13
		{0, 2, 3, 3},   // zero origin: 0*2+3=3
		{7, 1, 0, 7},   // identity
		{99, 0, 5, 5},  // zero multiplier
		{4, 3, -1, 11}, // negative add: 4*3-1=11
	}
	for _, tc := range cases {
		op := OperatorNumCalc{
			Multi: rsh.NewIntValue(int32(tc.multi)),
			Add:   rsh.NewIntValue(int32(tc.add)),
		}
		got := op.Calc(decimal.NewFromInt(tc.origin))
		assert.Equal(t, decimal.NewFromInt(tc.want), got,
			"Calc(%d) with multi=%d add=%d", tc.origin, tc.multi, tc.add)
	}
}

// ─── mergeOperator ───────────────────────────────────────────────────────────

// TestMergeOperator verifies algebraic composition:
//
//	g(f(x)) = x*(m1*m2) + (a1*m2 + a2)
//
// Example: f(x)=2x+3, g(x)=4x+5 → merged(x)=8x+17, so merged(10)=97.
func TestMergeOperator(t *testing.T) {
	e := numEntity(t)

	t.Run("RemainLatest_passthrough", func(t *testing.T) {
		field := e.GetFieldByName("propD1") // Int!
		op := intOp(2, 3)
		remain := Operator{} // NumCalc == nil → RemainLatest

		// remain ∘ op → op unchanged
		m1 := mergeOperator(field.Type, remain, op)
		assert.Equal(t, decimal.NewFromInt(13), m1.NumCalc.Calc(decimal.NewFromInt(5)))

		// op ∘ remain → op unchanged
		m2 := mergeOperator(field.Type, op, remain)
		assert.Equal(t, decimal.NewFromInt(13), m2.NumCalc.Calc(decimal.NewFromInt(5)))

		// remain ∘ remain → remain
		assert.True(t, mergeOperator(field.Type, remain, remain).RemainLatest())
	})

	// Int and Int8 use big.Int arithmetic internally.
	for _, tc := range []struct {
		fieldName string
		op        func(int64, int64) Operator
	}{
		{"propD1", intOp}, // Int!
		{"propJ1", intOp}, // Int8!
	} {
		tc := tc
		t.Run("Int_like/"+tc.fieldName, func(t *testing.T) {
			field := e.GetFieldByName(tc.fieldName)
			// f(x)=2x+3, g(x)=4x+5 → g(f(10))=(10*2+3)*4+5=97
			merged := mergeOperator(field.Type, tc.op(2, 3), tc.op(4, 5))
			assert.Equal(t, decimal.NewFromInt(97), merged.NumCalc.Calc(decimal.NewFromInt(10)))
		})
	}

	t.Run("BigInt", func(t *testing.T) {
		field := e.GetFieldByName("propE1") // BigInt!
		merged := mergeOperator(field.Type, bigIntOp(2, 3), bigIntOp(4, 5))
		assert.Equal(t, decimal.NewFromInt(97), merged.NumCalc.Calc(decimal.NewFromInt(10)))
	})

	// Float and BigDecimal use decimal arithmetic internally.
	for _, fieldName := range []string{"propI1", "propF1"} {
		fieldName := fieldName
		t.Run("Float_like/"+fieldName, func(t *testing.T) {
			field := e.GetFieldByName(fieldName)
			// f(x)=2.5x+1, g(x)=2x+0.5 → g(f(4))=(4*2.5+1)*2+0.5=22.5
			merged := mergeOperator(field.Type, decOp(2.5, 1.0), decOp(2.0, 0.5))
			want, _ := decimal.NewFromString("22.5")
			assert.Equal(t, want, merged.NumCalc.Calc(decimal.NewFromInt(4)))
		})
	}
}

// ─── calcOperator ────────────────────────────────────────────────────────────

// TestCalcOperator covers every supported numeric scalar type, both nullable
// and non-nullable, with nil and concrete origins.
//
// Field naming in EntityD (from testSchema):
//
//	propD1/propD2 → Int!/Int
//	propJ1/propJ2 → Int8!/Int8
//	propE1/propE2 → BigInt!/BigInt
//	propI1/propI2 → Float!/Float
//	propF1/propF2 → BigDecimal!/BigDecimal
func TestCalcOperator(t *testing.T) {
	e := numEntity(t)
	op := intOp(2, 3) // f(x) = 2x + 3

	t.Run("RemainLatest_returns_origin_unchanged", func(t *testing.T) {
		field := e.GetFieldByName("propD1")
		remain := Operator{}
		assert.Equal(t, int32(42), calcOperator(field.Type, int32(42), remain))
		assert.Nil(t, calcOperator(field.Type, nil, remain))
	})

	t.Run("Int_non_null", func(t *testing.T) {
		field := e.GetFieldByName("propD1")
		assert.Equal(t, int32(13), calcOperator(field.Type, int32(5), op))     // 5*2+3=13
		assert.Equal(t, int32(3), calcOperator(field.Type, nil, op))            // zero origin
		assert.Equal(t, int32(3), calcOperator(field.Type, (*int32)(nil), op)) // nil ptr → 0
	})

	t.Run("Int_nullable", func(t *testing.T) {
		field := e.GetFieldByName("propD2")
		v13, v3 := int32(13), int32(3)
		assert.Equal(t, &v13, calcOperator(field.Type, int32(5), op))
		assert.Equal(t, &v3, calcOperator(field.Type, nil, op))
	})

	t.Run("Int8_non_null", func(t *testing.T) {
		field := e.GetFieldByName("propJ1")
		assert.Equal(t, int64(13), calcOperator(field.Type, int64(5), op))
		assert.Equal(t, int64(3), calcOperator(field.Type, nil, op))
		assert.Equal(t, int64(3), calcOperator(field.Type, (*int64)(nil), op))
	})

	t.Run("Int8_nullable", func(t *testing.T) {
		field := e.GetFieldByName("propJ2")
		v13, v3 := int64(13), int64(3)
		assert.Equal(t, &v13, calcOperator(field.Type, int64(5), op))
		assert.Equal(t, &v3, calcOperator(field.Type, nil, op))
	})

	// BigInt is special: always returns *big.Int regardless of nullable/non-null.
	t.Run("BigInt_always_returns_ptr", func(t *testing.T) {
		bigOp := bigIntOp(2, 3)
		for _, fieldName := range []string{"propE1", "propE2"} {
			field := e.GetFieldByName(fieldName)

			// *big.Int origin
			got := calcOperator(field.Type, big.NewInt(5), bigOp)
			result, ok := got.(*big.Int)
			assert.True(t, ok, "%s: expected *big.Int, got %T", fieldName, got)
			assert.Equal(t, big.NewInt(13), result)

			// big.Int (value, not pointer) origin
			val := *big.NewInt(5)
			got2 := calcOperator(field.Type, val, bigOp)
			result2, ok2 := got2.(*big.Int)
			assert.True(t, ok2, "%s: expected *big.Int for value origin, got %T", fieldName, got2)
			assert.Equal(t, big.NewInt(13), result2)

			// nil origin → treat as 0
			got3 := calcOperator(field.Type, nil, bigOp)
			assert.Equal(t, big.NewInt(3), got3)
		}
	})

	t.Run("Float_non_null", func(t *testing.T) {
		field := e.GetFieldByName("propI1")
		assert.Equal(t, float64(13), calcOperator(field.Type, float64(5), op))
		assert.Equal(t, float64(3), calcOperator(field.Type, nil, op))
		assert.Equal(t, float64(3), calcOperator(field.Type, (*float64)(nil), op))
	})

	t.Run("Float_nullable", func(t *testing.T) {
		field := e.GetFieldByName("propI2")
		got := calcOperator(field.Type, float64(5), op)
		p, ok := got.(*float64)
		assert.True(t, ok)
		assert.Equal(t, float64(13), *p)
		got2 := calcOperator(field.Type, nil, op)
		p2, ok2 := got2.(*float64)
		assert.True(t, ok2)
		assert.Equal(t, float64(3), *p2)
	})

	t.Run("BigDecimal_non_null", func(t *testing.T) {
		field := e.GetFieldByName("propF1")
		assert.Equal(t, decimal.NewFromInt(13), calcOperator(field.Type, decimal.NewFromInt(5), op))
		assert.Equal(t, decimal.NewFromInt(3), calcOperator(field.Type, nil, op))
		// *decimal.Decimal origin
		d := decimal.NewFromInt(5)
		assert.Equal(t, decimal.NewFromInt(13), calcOperator(field.Type, &d, op))
	})

	t.Run("BigDecimal_nullable", func(t *testing.T) {
		field := e.GetFieldByName("propF2")
		got := calcOperator(field.Type, decimal.NewFromInt(5), op)
		p, ok := got.(*decimal.Decimal)
		assert.True(t, ok)
		assert.Equal(t, decimal.NewFromInt(13), *p)
		got2 := calcOperator(field.Type, nil, op)
		p2, ok2 := got2.(*decimal.Decimal)
		assert.True(t, ok2)
		assert.Equal(t, decimal.NewFromInt(3), *p2)
	})
}
