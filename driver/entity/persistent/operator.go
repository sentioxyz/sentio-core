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

type Operator struct {
	NumCalc *OperatorNumCalc
}

func (o Operator) RemainLatest() bool {
	return o.NumCalc == nil
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

func mergeOperator(typ types.Type, op1, op2 Operator) Operator {
	if op1.RemainLatest() {
		return op2
	}
	if op2.RemainLatest() {
		return op1
	}
	typeChain := schema.BreakType(typ)
	if typeChain.CountListLayer() > 0 {
		panic(fmt.Errorf("type %s is not support NumCalc operator", typ.String()))
	}
	innerType := typeChain.InnerType()
	scalarType, is := innerType.(*types.ScalarTypeDefinition)
	if !is {
		panic(fmt.Errorf("type %s is not support NumCalc operator", typ.String()))
	}
	// op1 and op2 are both NumCalc operator,
	// (x * m1 + a1) * m2 + a2 = x * (m1 * m2) + (a1 * m2 + a2)
	switch scalarType.Name {
	case "Int", "Int8", "BigInt":
		m1, _ := rsh.GetBigInt(op1.NumCalc.Multi)
		a1, _ := rsh.GetBigInt(op1.NumCalc.Add)
		m2, _ := rsh.GetBigInt(op2.NumCalc.Multi)
		a2, _ := rsh.GetBigInt(op2.NumCalc.Add)
		return Operator{
			NumCalc: &OperatorNumCalc{
				Multi: rsh.NewBigIntValue(new(big.Int).Mul(m1, m2)),
				Add:   rsh.NewBigIntValue(new(big.Int).Add(new(big.Int).Mul(a1, m2), a2)),
			},
		}
	case "Float", "BigDecimal":
		m1, _ := rsh.GetBigDecimal(op1.NumCalc.Multi)
		a1, _ := rsh.GetBigDecimal(op1.NumCalc.Add)
		m2, _ := rsh.GetBigDecimal(op2.NumCalc.Multi)
		a2, _ := rsh.GetBigDecimal(op2.NumCalc.Add)
		return Operator{
			NumCalc: &OperatorNumCalc{
				Multi: rsh.NewBigDecimalValue(m1.Mul(m2)),
				Add:   rsh.NewBigDecimalValue(a1.Mul(m2).Add(a2)),
			},
		}
	default:
		panic(fmt.Errorf("type %s is not support NumCalc operator", typ.String()))
	}
}

func calcOperator(typ types.Type, originVal any, operator Operator) any {
	if operator.RemainLatest() {
		// just use origin value
		return originVal
	}
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
		if nullable {
			return result
		}
		return &result
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
