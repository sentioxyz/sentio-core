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

// OperatorNumCalc newValue = preValue * Multi + Add
type OperatorNumCalc struct {
	Multi *protos.RichValue
	Add   *protos.RichValue
}

func (o OperatorNumCalc) Calc(origin decimal.Decimal) decimal.Decimal {
	multi, _ := rsh.GetBigDecimal(o.Multi)
	add, _ := rsh.GetBigDecimal(o.Add)
	return origin.Mul(multi).Add(add)
}

// Operator describes how the new value of a field derives from the previous version of the entity:
//
//	newValue = NumCalc(Exp(previousEntity))
//
// Exp, when set, is evaluated first against the whole previous entity (it may reference other
// fields); NumCalc, when set, is then applied to that result (or to the previous value of the
// field itself when Exp is nil). An Operator with neither keeps the latest value of the field.
type Operator struct {
	Exp     *compiledExp
	NumCalc *OperatorNumCalc
}

func richValueText(v *protos.RichValue) string {
	text, _ := rsh.GetString(v)
	return text
}

func (o Operator) RemainLatest() bool {
	return o.Exp == nil && o.NumCalc == nil
}

func (o Operator) String() string {
	switch {
	case o.Exp != nil && o.NumCalc != nil:
		return fmt.Sprintf("(%s)*%s+%s", o.Exp, richValueText(o.NumCalc.Multi), richValueText(o.NumCalc.Add))
	case o.Exp != nil:
		return o.Exp.String()
	case o.NumCalc != nil:
		return fmt.Sprintf("x*%s+%s", richValueText(o.NumCalc.Multi), richValueText(o.NumCalc.Add))
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

// mergeOperator composes two operators applied one after the other: op2(op1(x)).
//
// op2 must not carry an expression: an expression reads the previous version of the whole
// entity, which is only known once op1 has been resolved, so the caller has to resolve pending
// operators before pushing an expression on top of them (see Controller.SetEntity).
func mergeOperator(typ types.Type, op1, op2 Operator) (Operator, error) {
	if op1.RemainLatest() {
		return op2, nil
	}
	if op2.RemainLatest() {
		return op1, nil
	}
	if op2.Exp != nil {
		return Operator{}, fmt.Errorf("cannot merge expression %s on top of unresolved operator %s", op2, op1)
	}
	numCalc, err := mergeNumCalc(typ, op1.NumCalc, op2.NumCalc)
	if err != nil {
		return Operator{}, err
	}
	return Operator{Exp: op1.Exp, NumCalc: numCalc}, nil
}

// mergeNumCalc composes two affine calculations: (x * m1 + a1) * m2 + a2 = x * (m1 * m2) + (a1 * m2 + a2)
func mergeNumCalc(typ types.Type, c1, c2 *OperatorNumCalc) (*OperatorNumCalc, error) {
	if c1 == nil {
		return c2, nil
	}
	if c2 == nil {
		return c1, nil
	}
	typeChain := schema.BreakType(typ)
	if typeChain.CountListLayer() > 0 {
		return nil, fmt.Errorf("type %s is not support NumCalc operator", typ.String())
	}
	innerType := typeChain.InnerType()
	scalarType, is := innerType.(*types.ScalarTypeDefinition)
	if !is {
		return nil, fmt.Errorf("type %s is not support NumCalc operator", typ.String())
	}
	switch scalarType.Name {
	case "Int", "Int8", "BigInt":
		m1, _ := rsh.GetBigInt(c1.Multi)
		a1, _ := rsh.GetBigInt(c1.Add)
		m2, _ := rsh.GetBigInt(c2.Multi)
		a2, _ := rsh.GetBigInt(c2.Add)
		return &OperatorNumCalc{
			Multi: rsh.NewBigIntValue(new(big.Int).Mul(m1, m2)),
			Add:   rsh.NewBigIntValue(new(big.Int).Add(new(big.Int).Mul(a1, m2), a2)),
		}, nil
	case "Float", "BigDecimal":
		m1, _ := rsh.GetBigDecimal(c1.Multi)
		a1, _ := rsh.GetBigDecimal(c1.Add)
		m2, _ := rsh.GetBigDecimal(c2.Multi)
		a2, _ := rsh.GetBigDecimal(c2.Add)
		return &OperatorNumCalc{
			Multi: rsh.NewBigDecimalValue(m1.Mul(m2)),
			Add:   rsh.NewBigDecimalValue(a1.Mul(m2).Add(a2)),
		}, nil
	default:
		return nil, fmt.Errorf("type %s is not support NumCalc operator", typ.String())
	}
}

// calcOperator resolves an operator: originVal is the previous value of the field itself and row
// is the previous version of the whole entity, which expressions read their references from.
func calcOperator(typ types.Type, originVal any, operator Operator, row expRow) (any, error) {
	if operator.RemainLatest() {
		// just use origin value
		return originVal, nil
	}
	if operator.Exp != nil {
		result, err := operator.Exp.eval(row)
		if err != nil {
			return nil, err
		}
		if originVal, err = expValueToField(typ, result); err != nil {
			return nil, err
		}
		if operator.NumCalc == nil {
			return originVal, nil
		}
	}
	return calcNumCalc(typ, originVal, operator.NumCalc), nil
}

func calcNumCalc(typ types.Type, originVal any, numCalc *OperatorNumCalc) any {
	operator := Operator{NumCalc: numCalc}
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
		result := int32(operator.NumCalc.Calc(decimal.NewFromInt32(origin)).Round(0).IntPart())
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
		result := operator.NumCalc.Calc(decimal.NewFromInt(origin)).Round(0).IntPart()
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
		result := operator.NumCalc.Calc(decimal.NewFromBigInt(origin, 0)).Round(0).BigInt()
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
		result, _ := operator.NumCalc.Calc(decimal.NewFromFloat(origin)).Float64()
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
		result := operator.NumCalc.Calc(origin)
		if nullable {
			return &result
		}
		return result
	default:
		panic(fmt.Errorf("type %s is not support operator", typ.String()))
	}
}
