package persistent

import (
	"fmt"
	"github.com/graph-gophers/graphql-go/types"
	"github.com/shopspring/decimal"
	"math/big"
	rsh "sentioxyz/sentio-core/common/richstructhelper"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	"sentioxyz/sentio-core/service/common/protos"
)

// operatorNumCalc newValue = preValue * Multi + Add
type operatorNumCalc struct {
	Multi *protos.RichValue
	Add   *protos.RichValue
}

func (o operatorNumCalc) Calc(origin decimal.Decimal) decimal.Decimal {
	multi, _ := rsh.GetBigDecimal(o.Multi)
	add, _ := rsh.GetBigDecimal(o.Add)
	return origin.Mul(multi).Add(add)
}

func (o operatorNumCalc) String() string {
	return fmt.Sprintf("x*%s+%s", richValueText(o.Multi), richValueText(o.Add))
}

// operatorSet replaces the value of a field.
type operatorSet struct {
	Value any
}

// Operator is one correction of a field, derived from the previous version of the entity. Exactly
// one of the three is set: Set replaces the value, NumCalc applies an affine calculation to the
// previous value of the field, Exp evaluates an expression against the whole previous entity (it
// may reference other fields). An Operator with none set keeps the latest value.
type Operator struct {
	Set     *operatorSet
	NumCalc *operatorNumCalc
	Exp     *compiledExp
}

func richValueText(v *protos.RichValue) string {
	text, _ := rsh.GetString(v)
	return text
}

func (o Operator) RemainLatest() bool {
	return o.Set == nil && o.NumCalc == nil && o.Exp == nil
}

func (o Operator) String() string {
	switch {
	case o.Set != nil:
		return fmt.Sprintf("%v", o.Set.Value)
	case o.NumCalc != nil:
		return o.NumCalc.String()
	case o.Exp != nil:
		return o.Exp.String()
	default:
		return "x"
	}
}

func checkNumCalcValueTypeMatch(typ types.Type, val *protos.RichValue) error {
	typeChain := schema.BreakType(typ)
	if typeChain.CountListLayer() > 0 {
		return fmt.Errorf("type %s is not support NumCalc operator", typ.String())
	}
	innerType := typeChain.InnerType()
	scalarType, is := innerType.(*types.ScalarTypeDefinition)
	if !is {
		return fmt.Errorf("type %s is not support NumCalc operator", typ.String())
	}
	switch scalarType.Name {
	case "Int", "Int8", "BigInt", "Float", "BigDecimal":
		if _, is = rsh.GetBigDecimal(val); is {
			return nil
		}
	default:
		return fmt.Errorf("type %s is not support NumCalc operator", typ.String())
	}
	v, _ := rsh.GetValue(val)
	return fmt.Errorf("type %s is not support NumCalc operator with value %T %s", typ.String(), val, v)
}

// mergeOperator composes two corrections of the same field in the same round: op2(op1(x)).
// Neither may be an expression: an expression reads the previous version of the whole entity, so
// a round with expressions is never merged with another one (see UncommittedEntityBox.Merge).
func mergeOperator(typ types.Type, op1, op2 Operator) Operator {
	if op1.Exp != nil || op2.Exp != nil {
		panic(fmt.Errorf("unreachable: merge expression operators %s and %s", op1, op2))
	}
	switch {
	case op2.RemainLatest():
		return op1
	case op1.RemainLatest(), op2.Set != nil:
		return op2
	case op1.Set != nil:
		// the input is concrete, so is the result
		return Operator{Set: &operatorSet{Value: calcNumCalc(typ, op1.Set.Value, op2.NumCalc)}}
	default:
		return Operator{NumCalc: mergeNumCalc(typ, op1.NumCalc, op2.NumCalc)}
	}
}

// mergeNumCalc composes two affine calculations: (x * m1 + a1) * m2 + a2 = x * (m1 * m2) + (a1 * m2 + a2)
func mergeNumCalc(typ types.Type, c1, c2 *operatorNumCalc) *operatorNumCalc {
	typeChain := schema.BreakType(typ)
	if typeChain.CountListLayer() > 0 {
		panic(fmt.Errorf("type %s is not support NumCalc operator", typ.String()))
	}
	innerType := typeChain.InnerType()
	scalarType, is := innerType.(*types.ScalarTypeDefinition)
	if !is {
		panic(fmt.Errorf("type %s is not support NumCalc operator", typ.String()))
	}
	switch scalarType.Name {
	case "Int", "Int8", "BigInt":
		m1, _ := rsh.GetBigInt(c1.Multi)
		a1, _ := rsh.GetBigInt(c1.Add)
		m2, _ := rsh.GetBigInt(c2.Multi)
		a2, _ := rsh.GetBigInt(c2.Add)
		return &operatorNumCalc{
			Multi: rsh.NewBigIntValue(new(big.Int).Mul(m1, m2)),
			Add:   rsh.NewBigIntValue(new(big.Int).Add(new(big.Int).Mul(a1, m2), a2)),
		}
	case "Float", "BigDecimal":
		m1, _ := rsh.GetBigDecimal(c1.Multi)
		a1, _ := rsh.GetBigDecimal(c1.Add)
		m2, _ := rsh.GetBigDecimal(c2.Multi)
		a2, _ := rsh.GetBigDecimal(c2.Add)
		return &operatorNumCalc{
			Multi: rsh.NewBigDecimalValue(m1.Mul(m2)),
			Add:   rsh.NewBigDecimalValue(a1.Mul(m2).Add(a2)),
		}
	default:
		panic(fmt.Errorf("type %s is not support NumCalc operator", typ.String()))
	}
}

// calcOperator resolves an operator: originVal is the previous value of the field itself and row
// is the previous version of the whole entity, which expressions read their references from.
func calcOperator(typ types.Type, originVal any, operator Operator, row expRow) (any, error) {
	switch {
	case operator.Set != nil:
		return operator.Set.Value, nil
	case operator.NumCalc != nil:
		return calcNumCalc(typ, originVal, operator.NumCalc), nil
	case operator.Exp != nil:
		result, err := operator.Exp.eval(row)
		if err != nil {
			return nil, err
		}
		return expValueToField(typ, result)
	default:
		// just use origin value
		return originVal, nil
	}
}

func calcNumCalc(typ types.Type, originVal any, numCalc *operatorNumCalc) any {
	typeChain := schema.BreakType(typ)
	if typeChain.CountListLayer() > 0 {
		panic(fmt.Errorf("type %s is not support operator", typ.String()))
	}
	nullable := typeChain.InnerTypeNullable()
	innerType := typeChain.InnerType()
	scalarType, is := innerType.(*types.ScalarTypeDefinition)
	if !is {
		panic(fmt.Errorf("type %s is not support operator", typ.String()))
	}
	switch scalarType.Name {
	case "Int":
		var origin int32
		if !utils.IsNil(originVal) {
			switch ov := originVal.(type) {
			case int32:
				origin = ov
			case *int32:
				origin = *ov
			}
		}
		result := int32(numCalc.Calc(decimal.NewFromInt32(origin)).Round(0).IntPart())
		if nullable {
			return &result
		}
		return result
	case "Int8":
		var origin int64
		if !utils.IsNil(originVal) {
			switch ov := originVal.(type) {
			case int64:
				origin = ov
			case *int64:
				origin = *ov
			}
		}
		result := numCalc.Calc(decimal.NewFromInt(origin)).Round(0).IntPart()
		if nullable {
			return &result
		}
		return result
	case "BigInt":
		origin := big.NewInt(0)
		if !utils.IsNil(originVal) {
			switch ov := originVal.(type) {
			case big.Int:
				origin = &ov
			case *big.Int:
				origin = ov
			}
		}
		result := numCalc.Calc(decimal.NewFromBigInt(origin, 0)).Round(0).BigInt()
		// BigInt is special, always use *big.Int regardless of nonNull declaration
		return result
	case "Float":
		var origin float64
		if !utils.IsNil(originVal) {
			switch ov := originVal.(type) {
			case float64:
				origin = ov
			case *float64:
				origin = *ov
			}
		}
		result, _ := numCalc.Calc(decimal.NewFromFloat(origin)).Float64()
		if nullable {
			return &result
		}
		return result
	case "BigDecimal":
		origin := decimal.Zero
		if !utils.IsNil(originVal) {
			switch ov := originVal.(type) {
			case decimal.Decimal:
				origin = ov
			case *decimal.Decimal:
				origin = *ov
			}
		}
		result := numCalc.Calc(origin)
		if nullable {
			return &result
		}
		return result
	default:
		panic(fmt.Errorf("type %s is not support operator", typ.String()))
	}
}
