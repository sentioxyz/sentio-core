package timeseries

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/service/analytic/common/schema"

	"github.com/jinzhu/copier"
	"github.com/shopspring/decimal"
)

type FieldType string

type FieldTypes []FieldType

func (t FieldTypes) Compatible() bool {
	if len(t) == 0 {
		return true
	}
	var count = make(map[string]int)
	for _, ft := range t {
		switch {
		case isNumericType(ft):
			if isBigIntType(ft) {
				count["bigint"]++
			} else {
				count["decimal"]++
			}
		case isJSONType(ft):
			count["json"]++
		case isTokenType(ft):
			count["token"]++
		case isTimeType(ft):
			count["time"]++
		case isBoolType(ft):
			count["bool"]++
		case isArrayType(ft):
			count["array"]++
		case isStringType(ft):
			count["string"]++
		}
	}

	countKeyLen := len(count)
	switch {
	case count["token"] != 0 && countKeyLen > 1:
		// token can only be used in one field
		return false
	case count["array"] != 0 && countKeyLen > 1:
		// array can only be used in one field
		return false
	case count["json"] != 0 && countKeyLen > 1:
		// json can only be used in one field
		return false
	case count["bigint"] != 0 && count["decimal"] != 0:
		// bigint and decimal can not be used together, because of the precision loss
		return false
	default:
		return true
	}
}

func (t FieldTypes) SimplyGCD() FieldType {
	if len(t) == 0 {
		return FieldTypeString
	}

	var count = make(map[string]int)
	for _, ft := range t {
		switch {
		case isNumericType(ft):
			if isBigIntType(ft) {
				count["bigint"]++
			} else {
				count["decimal"]++
			}
		case isTimeType(ft):
			count["time"]++
		case isBoolType(ft):
			count["bool"]++
		case isStringType(ft):
			count["string"]++
		}
	}
	if len(count) == 0 {
		return t[0]
	}

	switch {
	case count["time"] > 0:
		// time is the highest priority
		return FieldTypeTime
	case count["bigint"] > 0:
		// if bigint is used, then always trans to BigInt
		return FieldTypeBigInt
	case count["decimal"] > 0:
		// always trans to Decimal256
		return FieldTypeBigFloat
	case count["string"] > 0:
		return FieldTypeString
	default:
		return FieldTypeBool
	}
}

func (t FieldTypes) ComplexGCD() FieldType {
	if len(t) == 0 {
		return FieldTypeString
	}

	var count = make(map[string]int)
	for _, ft := range t {
		switch {
		case isNumericType(ft):
			if isBigIntType(ft) {
				count["bigint"]++
			} else {
				count["decimal"]++
			}
		case isJSONType(ft):
			count["json"]++
		case isTokenType(ft):
			count["token"]++
		case isTimeType(ft):
			count["time"]++
		case isBoolType(ft):
			count["bool"]++
		case isArrayType(ft):
			count["array"]++
		case isStringType(ft):
			count["string"]++
		}
	}

	switch {
	case count["json"] > 0:
		return FieldTypeJSON
	case count["array"] > 0:
		return FieldTypeArray
	case count["time"] > 0:
		return FieldTypeTime
	case count["bigint"] > 0:
		// if bigint is used, then always trans to BigInt
		return FieldTypeBigInt
	case count["decimal"] > 0:
		// always trans to Decimal256
		return FieldTypeBigFloat
	case count["string"] > 0:
		return FieldTypeString
	default:
		return FieldTypeBool
	}
}

const (
	FieldTypeString   FieldType = "String"
	FieldTypeBool     FieldType = "Bool"
	FieldTypeTime     FieldType = "Time"
	FieldTypeInt      FieldType = "Int"
	FieldTypeBigInt   FieldType = "BigInt"
	FieldTypeFloat    FieldType = "Float"
	FieldTypeBigFloat FieldType = "BigFloat"
	FieldTypeJSON     FieldType = "JSON"
	FieldTypeArray    FieldType = "Array"
	FieldTypeToken    FieldType = "Token"
)

type fieldTypeFunc = func(t FieldType) bool

func isNumericType(t FieldType) bool {
	switch t {
	case FieldTypeInt, FieldTypeBigInt, FieldTypeFloat, FieldTypeBigFloat:
		return true
	default:
		return false
	}
}

func isBigIntType(t FieldType) bool {
	switch t {
	case FieldTypeBigInt:
		return true
	default:
		return false
	}
}

func isStringType(t FieldType) bool {
	switch t {
	case FieldTypeString:
		return true
	}
	return false
}

func isTimeType(t FieldType) bool {
	switch t {
	case FieldTypeTime:
		return true
	}
	return false
}

func isBoolType(t FieldType) bool {
	switch t {
	case FieldTypeBool:
		return true
	}
	return false
}

func isArrayType(t FieldType) bool {
	switch t {
	case FieldTypeArray:
		return true
	}
	return false
}

func isJSONType(t FieldType) bool {
	switch t {
	case FieldTypeJSON:
		return true
	}
	return false
}

func isTokenType(t FieldType) bool {
	switch t {
	case FieldTypeToken:
		return true
	}
	return false
}

type FieldRole string

const (
	FieldRoleNone        FieldRole = ""
	FieldRoleChainID     FieldRole = "@ChainID"
	FieldRoleTimestamp   FieldRole = "@Timestamp"
	FieldRoleSlotNumber  FieldRole = "@SlotNumber"
	FieldRoleAggInterval FieldRole = "@AggInterval"
	FieldRoleSeriesLabel FieldRole = "@SeriesLabel"
	FieldRoleSeriesValue FieldRole = "@SeriesValue"
)

type Field struct {
	Name string
	Type FieldType
	Role FieldRole

	// Eventlog field property
	NestedStructSchema map[string]FieldType
	BuiltIn            bool

	// Index info
	Index       bool
	NestedIndex map[string]FieldType
}

var _ schema.Field = (*Field)(nil)

func (f *Field) Copy() Field {
	n := Field{}
	_ = copier.CopyWithOption(&n, f, copier.Option{
		DeepCopy: true,
	})
	return n
}

func (f *Field) Compatible(other Field) bool {
	equal := f.Name == other.Name &&
		f.Type == other.Type &&
		f.Role == other.Role
	if !equal {
		return false
	}
	for _, k := range utils.GetOrderedMapKeys(f.NestedStructSchema) {
		otherFieldType, has := other.NestedStructSchema[k]
		if has && otherFieldType != f.NestedStructSchema[k] {
			return false
		}
	}
	return true
}

func (f *Field) CompatibleDiff(other Field) FieldDiff {
	if f.Compatible(other) {
		return FieldDiff{}
	}

	if f.Type != other.Type {
		return FieldDiff{
			Before: Field{
				Name: f.Name,
				Type: f.Type,
			},
			After: Field{
				Name: f.Name,
				Type: other.Type,
			},
		}
	}

	for _, k := range utils.GetOrderedMapKeys(f.NestedStructSchema) {
		otherFieldType, has := other.NestedStructSchema[k]
		if has && otherFieldType != f.NestedStructSchema[k] {
			return FieldDiff{
				Before: Field{
					Name: fmt.Sprintf("%s.%s", f.Name, k),
					Type: f.NestedStructSchema[k],
				},
				After: Field{
					Name: fmt.Sprintf("%s.%s", f.Name, k),
					Type: otherFieldType,
				},
			}
		}
	}
	return FieldDiff{}
}

func (f *Field) Merge(other Field) (newField Field, changed bool) {
	if !f.Compatible(other) {
		return f.Copy(), false
	}
	changed = false

	newField = f.Copy()
	if !f.BuiltIn && other.BuiltIn {
		newField.BuiltIn = true
		changed = true
	}
	if newField.NestedStructSchema == nil {
		newField.NestedStructSchema = make(map[string]FieldType)
	}
	for _, k := range utils.GetOrderedMapKeys(other.NestedStructSchema) {
		_, has := f.NestedStructSchema[k]
		if !has {
			changed = true
			newField.NestedStructSchema[k] = other.NestedStructSchema[k]
		}
	}
	return
}

func (f *Field) String() string {
	var buf bytes.Buffer
	buf.WriteString(f.Name)
	buf.WriteString("/")
	buf.WriteString(string(f.Type))
	buf.WriteString("/")
	buf.WriteString(string(f.Role))
	if f.BuiltIn {
		buf.WriteString("#BuiltIn")
	}
	if f.NestedStructSchema != nil {
		buf.WriteString("#NestedSchema{")
		for _, k := range utils.GetOrderedMapKeys(f.NestedStructSchema) {
			buf.WriteString("(")
			buf.WriteString(k)
			buf.WriteString(":")
			buf.WriteString(string(f.NestedStructSchema[k]))
			buf.WriteString(")")
		}
		buf.WriteString("}")
	}
	if f.Index {
		buf.WriteString("#Index{")
		if f.NestedIndex != nil {
			for _, k := range utils.GetOrderedMapKeys(f.NestedIndex) {
				buf.WriteString("(" + k)
				buf.WriteString(":")
				buf.WriteString(string(f.NestedIndex[k]))
				buf.WriteString(")")
			}
		}
		buf.WriteString("}")
	}
	return buf.String()
}

func (f *Field) IsBuiltIn() bool {
	return f.BuiltIn
}

func (f *Field) Equal(other schema.Field) (bool, string) {
	return f.GetName() == other.GetName() && f.GetType() == other.GetType(), ""
}

func (f *Field) GetDisplayName() string {
	return f.Name
}

func (f *Field) GetName() string {
	return f.Name
}

func (f *Field) GetType() string {
	return string(f.Type)
}

func (f *Field) GetExtend() map[string]schema.Field {
	return nil
}

func (f *Field) IsJSON() bool {
	return isJSONType(f.Type)
}

func (f *Field) IsToken() bool {
	return isTokenType(f.Type)
}

func (f *Field) IsTime() bool {
	return isTimeType(f.Type)
}

func (f *Field) IsBool() bool {
	return isBoolType(f.Type)
}

func (f *Field) IsNumeric() bool {
	return isNumericType(f.Type)
}

func (f *Field) IsString() bool {
	return isStringType(f.Type)
}

func (f *Field) IsArray() bool {
	return isArrayType(f.Type)
}

func (f *Field) NestedTypeChecker(name string, checker fieldTypeFunc) bool {
	if !f.IsJSON() {
		return false
	}
	if f.NestedStructSchema == nil {
		return false
	}
	fieldType, has := f.NestedStructSchema[strings.ReplaceAll(strings.Join(strings.Split(name, ".")[1:], "."), "`", "")]
	if !has {
		return false
	}
	return checker(fieldType)
}

func (f *Field) TypeMatch(value any) (any, bool) {
	switch {
	case f.IsString():
		switch v := value.(type) {
		case string:
			return v, true
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return fmt.Sprintf("%d", v), true
		case float32, float64:
			return fmt.Sprintf("%f", v), true
		case time.Time:
			return v.Format(time.RFC3339), true
		default:
			return nil, false
		}
	case f.IsNumeric():
		switch v := value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return v, true
		case float32, float64:
			return v, true
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return i, true
			}
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f, true
			}
		case decimal.Decimal:
			return v, true
		default:
			return nil, false
		}
	case f.IsTime():
		switch v := value.(type) {
		case time.Time:
			return v, true
		case string:
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t, true
			}
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				return t, true
			}
			return nil, false
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			switch i := v.(type) {
			case int:
				return time.Unix(int64(i), 0), true
			case int8:
				return time.Unix(int64(i), 0), true
			case int16:
				return time.Unix(int64(i), 0), true
			case int32:
				return time.Unix(int64(i), 0), true
			case int64:
				return time.Unix(i, 0), true
			case uint:
				return time.Unix(int64(i), 0), true
			case uint8:
				return time.Unix(int64(i), 0), true
			case uint16:
				return time.Unix(int64(i), 0), true
			case uint32:
				return time.Unix(int64(i), 0), true
			case uint64:
				return time.Unix(int64(i), 0), true
			default:
				return nil, false
			}
		default:
			return nil, false
		}
	}
	return nil, false
}

func BuildFields(fields ...Field) map[string]Field {
	m := make(map[string]Field)
	for _, field := range fields {
		m[field.Name] = field
	}
	return m
}

func GetFieldNames(fields []Field) []string {
	return utils.MapSliceNoError(fields, func(f Field) string {
		return f.Name
	})
}
