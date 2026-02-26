package chx

import (
	"bytes"
	"fmt"
	"sentioxyz/sentio-core/common/utils"
	"strconv"
	"strings"
)

type Field struct {
	Name             string
	Type             FieldType
	CompressionCodec string // for example 'CODEC(ZSTD(1))'
	DefaultExpr      string
	Comment          string
}

func (f Field) CreateSQL() string {
	var sql bytes.Buffer
	sql.WriteString(fmt.Sprintf("`%s` %s", f.Name, f.Type))
	if f.DefaultExpr != "" {
		sql.WriteString(fmt.Sprintf(" DEFAULT %s", f.DefaultExpr))
	}
	if f.Comment != "" {
		sql.WriteString(fmt.Sprintf(" COMMENT '%s'", f.Comment))
	}
	if f.CompressionCodec != "" {
		sql.WriteString(" ")
		sql.WriteString(f.CompressionCodec)
	}
	return sql.String()
}

type Fields []Field

func (f Fields) FindByName(name string) (Field, int) {
	for i, field := range f {
		if field.Name == name {
			return field, i
		}
	}
	return Field{}, -1
}

func (f Fields) Names() []string {
	return utils.MapSliceNoError(f, func(fd Field) string {
		return fd.Name
	})
}

type FieldType interface {
	String() string
	CheckModify(FieldType) bool
	SameAs(FieldType) bool
}

type FieldTypeNormal string

func (t FieldTypeNormal) String() string {
	return string(t)
}

func (t FieldTypeNormal) CheckModify(a FieldType) bool {
	return false
}

func (t FieldTypeNormal) SameAs(a FieldType) bool {
	x, is := a.(FieldTypeNormal)
	if !is {
		return false
	}
	// remove all space and ignore case
	return strings.EqualFold(
		strings.ReplaceAll(string(t), " ", ""),
		strings.ReplaceAll(string(x), " ", ""))
}

const (
	FieldTypeString = FieldTypeNormal("String")
	FieldTypeBool   = FieldTypeNormal("Bool")

	FieldTypeInt8   = FieldTypeNormal("Int8")
	FieldTypeInt32  = FieldTypeNormal("Int32")
	FieldTypeInt64  = FieldTypeNormal("Int64")
	FieldTypeInt256 = FieldTypeNormal("Int256")

	FieldTypeUInt8   = FieldTypeNormal("UInt8")
	FieldTypeUInt32  = FieldTypeNormal("UInt32")
	FieldTypeUInt64  = FieldTypeNormal("UInt64")
	FieldTypeUInt256 = FieldTypeNormal("UInt256")

	FieldTypeFloat32 = FieldTypeNormal("Float32")
	FieldTypeFloat64 = FieldTypeNormal("Float64")
)

type FieldTypeEnum []string

func (t FieldTypeEnum) String() string {
	return fmt.Sprintf("Enum('%s')", strings.Join(t, "', '"))
}

func (t FieldTypeEnum) CheckModify(a FieldType) bool {
	_, is := a.(FieldTypeEnum)
	return is
}

func (t FieldTypeEnum) SameAs(a FieldType) bool {
	x, is := a.(FieldTypeEnum)
	if !is {
		return false
	}
	if len(t) != len(x) {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] != x[i] {
			return false
		}
	}
	return true
}

type FieldTypeDecimal struct {
	Precision uint8
	Scale     uint8
}

func (t FieldTypeDecimal) String() string {
	return fmt.Sprintf("Decimal(%d, %d)", t.Precision, t.Scale)
}

func (t FieldTypeDecimal) CheckModify(a FieldType) bool {
	_, is := a.(FieldTypeDecimal)
	return is
}

func (t FieldTypeDecimal) SameAs(a FieldType) bool {
	x, is := a.(FieldTypeDecimal)
	if !is {
		return false
	}
	return t.Precision == x.Precision && t.Scale == x.Scale
}

type FieldTypeDateTime64 struct {
	Precision uint8
	Timezone  string
}

func (t FieldTypeDateTime64) String() string {
	return fmt.Sprintf("DateTime64(%d, '%s')", t.Precision, t.Timezone)
}

func (t FieldTypeDateTime64) CheckModify(a FieldType) bool {
	_, is := a.(FieldTypeDateTime64)
	return is
}

func (t FieldTypeDateTime64) SameAs(a FieldType) bool {
	x, is := a.(FieldTypeDateTime64)
	if !is {
		return false
	}
	return t.Precision == x.Precision && t.Timezone == x.Timezone
}

type FieldTypeNullable struct {
	Inner FieldType
}

func (t FieldTypeNullable) String() string {
	return fmt.Sprintf("Nullable(%s)", t.Inner)
}

func (t FieldTypeNullable) CheckModify(a FieldType) bool {
	x, is := a.(FieldTypeNullable)
	if !is {
		return false
	}
	return t.Inner.CheckModify(x.Inner)
}

func (t FieldTypeNullable) SameAs(a FieldType) bool {
	x, is := a.(FieldTypeNullable)
	if !is {
		return false
	}
	return t.Inner.SameAs(x.Inner)
}

type FieldTypeArray struct {
	Inner FieldType
}

func (t FieldTypeArray) String() string {
	return fmt.Sprintf("Array(%s)", t.Inner)
}

func (t FieldTypeArray) CheckModify(a FieldType) bool {
	x, is := a.(FieldTypeArray)
	if !is {
		return false
	}
	return t.Inner.CheckModify(x.Inner)
}

func (t FieldTypeArray) SameAs(a FieldType) bool {
	x, is := a.(FieldTypeArray)
	if !is {
		return false
	}
	return t.Inner.SameAs(x.Inner)
}

func getInnerPart(raw string, left, right byte) string {
	raw = raw[strings.IndexByte(raw, left)+1:]
	return raw[:strings.LastIndexByte(raw, right)]
}

func BuildFieldType(raw string) FieldType {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "Nullable") {
		return FieldTypeNullable{
			Inner: BuildFieldType(getInnerPart(raw, '(', ')')),
		}
	}
	if strings.HasPrefix(raw, "Array") {
		return FieldTypeArray{
			Inner: BuildFieldType(getInnerPart(raw, '(', ')')),
		}
	}
	if strings.HasPrefix(raw, "Enum") {
		// Enum8('AAA' = 1, 'BBB' = 2, 'CCC' = 3, 'DDD' = 4)
		var enumValues FieldTypeEnum
		for _, vs := range strings.Split(getInnerPart(raw, '(', ')'), ",") {
			// 'AAA' = 1
			enumValues = append(enumValues, getInnerPart(vs, '\'', '\''))
		}
		return enumValues
	}
	if strings.HasPrefix(raw, "Decimal") {
		// Decimal(76, 30)
		parts := strings.Split(getInnerPart(raw, '(', ')'), ",")
		if len(parts) == 2 {
			p, pe := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
			s, se := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			if pe == nil && se == nil {
				return FieldTypeDecimal{
					Precision: uint8(p),
					Scale:     uint8(s),
				}
			}
		}
	}
	if strings.HasPrefix(raw, "DateTime64") {
		// DateTime64(6, 'UTC')
		parts := strings.Split(getInnerPart(raw, '(', ')'), ",")
		if len(parts) == 2 {
			p, pe := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
			z := getInnerPart(parts[1], '\'', '\'')
			if pe == nil {
				return FieldTypeDateTime64{Precision: uint8(p), Timezone: z}
			}
		}
	}
	return FieldTypeNormal(raw)
}
