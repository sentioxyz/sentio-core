package clickhouse

import (
	"fmt"
	"math/big"
	"reflect"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"

	"github.com/graph-gophers/graphql-go/types"
	"github.com/shopspring/decimal"
)

// BigInt/BigDecimal bounds
var (
	// Int256 range: [-2^255, 2^255-1]
	int256Min = new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 255))
	int256Max = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(1))

	// Decimal256(30) range: |val| <= (10^76-1)/10^30
	decimal256_30_max = decimal.NewFromBigInt(
		new(big.Int).Sub(new(big.Int).Exp(big.NewInt(10), big.NewInt(76), nil), big.NewInt(1)), -30)
)

func (s *Store) CheckValue(entityType *schema.Entity, data map[string]any) error {
	for fieldName, val := range data {
		field := entityType.GetFieldByName(fieldName)
		if field == nil {
			return fmt.Errorf("%s.%s is not exist", entityType.Name, fieldName)
		}
		if err := s.checkFieldValue(entityType.Name, fieldName, field.Type, val); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) checkFieldValue(entityName, fieldPath string, typ types.Type, val any) error {
	nonNullType, nonNull := typ.(*types.NonNull)
	if nonNull {
		if utils.IsNil(val) {
			return fmt.Errorf("%s.%s cannot be null", entityName, fieldPath)
		}
		typ = nonNullType.OfType
	} else {
		if utils.IsNil(val) {
			return nil
		}
		rv := reflect.ValueOf(val)
		if rv.Kind() == reflect.Pointer {
			if _, is := val.(*big.Int); !is {
				val = rv.Elem().Interface()
			}
		}
	}
	switch wrapType := typ.(type) {
	case *types.List:
		rv := reflect.ValueOf(val)
		if rv.Kind() != reflect.Slice {
			return fmt.Errorf("%s.%s must be slice", entityName, fieldPath)
		}
		for i := 0; i < rv.Len(); i++ {
			elemPath := fmt.Sprintf("%s[%d]", fieldPath, i)
			elem := rv.Index(i).Interface()
			if err := s.checkFieldValue(entityName, elemPath, wrapType.OfType, elem); err != nil {
				return err
			}
		}
	case *types.ScalarTypeDefinition:
		switch wrapType.Name {
		case "BigInt":
			return s.checkBigIntBounds(entityName, fieldPath, val)
		case "BigDecimal":
			return s.checkBigDecimalBounds(entityName, fieldPath, val)
		}
	case *types.EnumTypeDefinition:
		v, is := val.(string)
		if !is {
			return fmt.Errorf("%s.%s must be string", entityName, fieldPath)
		}
		ok := utils.HasAny(wrapType.EnumValuesDefinition, func(ev *types.EnumValueDefinition) bool {
			return ev.EnumValue == v
		})
		if !ok {
			return fmt.Errorf("%s.%s has unexpected enum value %s for %s", entityName, fieldPath, v, wrapType)
		}
	case *types.ObjectTypeDefinition, *types.InterfaceTypeDefinition:
		// TODO check value type, may be string or int64
	}
	return nil
}

func (s *Store) checkBigIntBounds(entityName, fieldName string, val any) error {
	var v *big.Int
	switch bv := val.(type) {
	case *big.Int:
		v = bv
	case big.Int:
		v = &bv
	default:
		return nil
	}
	if v.Cmp(int256Min) < 0 || v.Cmp(int256Max) > 0 {
		return fmt.Errorf(
			"field %s.%s has BigInt value %s out of Int256 range [-2^255, 2^255-1]",
			entityName, fieldName, v.String())
	}
	return nil
}

func (s *Store) checkBigDecimalBounds(entityName, fieldName string, val any) error {
	if s.feaOpt.BigDecimalUseString {
		return nil
	}
	var v decimal.Decimal
	switch dv := val.(type) {
	case decimal.Decimal:
		v = dv
	case *decimal.Decimal:
		v = *dv
	default:
		return nil
	}
	if s.feaOpt.BigDecimalUseDecimal512 {
		scaled := v.Round(int32(decimal512Scale))
		totalDigits := len(scaled.Coefficient().String())
		if totalDigits > decimal512Precision {
			return fmt.Errorf(
				"field %s.%s has BigDecimal value %s exceeding Decimal512 precision:"+
					" total digits %d > %d (scale %d)",
				entityName, fieldName, v.String(), totalDigits, decimal512Precision, decimal512Scale)
		}
		return nil
	}
	// Default: Decimal256(30)
	if v.Abs().GreaterThan(decimal256_30_max) {
		return fmt.Errorf(
			"field %s.%s has BigDecimal value %s out of Decimal256(30) range",
			entityName, fieldName, v.String())
	}
	return nil
}
