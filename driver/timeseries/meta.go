package timeseries

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/service/analytic/common/schema"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/encoder"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

type MetaComment struct {
	Hash    string
	Encoded string
}

type MetaType string

const (
	MetaTypeGauge   = "gauge"
	MetaTypeCounter = "counter"
	MetaTypeEvent   = "event"
)

func IsValidMetaType(t MetaType) bool {
	return t == MetaTypeGauge || t == MetaTypeCounter || t == MetaTypeEvent
}

type Meta struct {
	Name   string
	Type   MetaType
	Fields map[string]Field

	Aggregation *Aggregation `json:"omitempty"`
	HashData    string       `json:"-"`
}

var _ schema.Meta = (*Meta)(nil)

func (m Meta) Dump() []byte {
	data, err := sonic.Marshal(m)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		panic(err)
	}
	if err := gw.Close(); err != nil {
		panic(err)
	}
	mc := MetaComment{
		Hash:    m.CalculateHash(),
		Encoded: base64.StdEncoding.EncodeToString(buf.Bytes()),
	}

	encoded, err := sonic.Marshal(mc)
	if err != nil {
		panic(err)
	}
	return encoded
}

func LoadMeta(comment string) (Meta, error) {
	var (
		meta Meta
		mc   MetaComment
	)
	if err := sonic.Unmarshal([]byte(comment), &mc); err != nil {
		return meta, err
	}

	data, err := base64.StdEncoding.DecodeString(mc.Encoded)
	if err != nil {
		return meta, err
	}
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return meta, err
	}
	defer gr.Close()
	uncompressed, err := io.ReadAll(gr)
	if err != nil {
		return meta, err
	}

	if err := sonic.Unmarshal(uncompressed, &meta); err != nil {
		return meta, err
	}
	meta.HashData = mc.Hash
	return meta, nil
}

func (m Meta) Load(data []byte) error {
	return fmt.Errorf("not implemented, use LoadMeta instead")
}

func (m Meta) GetFullName() string {
	return fmt.Sprintf("%s.%s", m.Type, m.Name)
}

func (m Meta) GetTableSuffix() string {
	return fmt.Sprintf("%s_%s", m.Type, m.Name)
}

func (m Meta) DiffFields(other Meta) FieldsDiff {
	return CalcFieldsDiff(m.Fields, other.Fields)
}

func (m Meta) Merge(target Meta) Meta {
	r := Meta{
		Name:        m.Name,
		Type:        m.Type,
		Fields:      make(map[string]Field),
		Aggregation: target.Aggregation,
	}
	utils.PutAll(r.Fields, m.Fields)
	for _, field := range target.Fields {
		exists, has := r.Fields[field.Name]
		if has {
			r.Fields[field.Name], _ = exists.Merge(field)
		} else {
			r.Fields[field.Name] = field
		}
	}
	return r
}

func (m Meta) GetChainIDField() Field {
	fields := m.GetFieldsByRole(FieldRoleChainID)
	if len(fields) == 0 {
		panic(fmt.Errorf("no ChainID field for %s", m.GetFullName()))
	}
	return fields[0]
}

func (m Meta) GetTimestampField() Field {
	fields := m.GetFieldsByRole(FieldRoleTimestamp)
	if len(fields) == 0 {
		panic(fmt.Errorf("no Timestamp field for %s", m.GetFullName()))
	}
	return fields[0]
}

func (m Meta) GetSlotNumberField() Field {
	fields := m.GetFieldsByRole(FieldRoleSlotNumber)
	if len(fields) == 0 {
		panic(fmt.Errorf("no SlotNumber field for %s", m.GetFullName()))
	}
	return fields[0]
}

func (m Meta) GetAggIntervalField() Field {
	fields := m.GetFieldsByRole(FieldRoleAggInterval)
	if len(fields) == 0 {
		panic(fmt.Errorf("no AggInterval field for %s", m.GetFullName()))
	}
	return fields[0]
}

func (m Meta) GetFieldsByRole(role FieldRole) []Field {
	var result []Field
	for _, field := range utils.GetMapValuesOrderByKey(m.Fields) {
		if field.Role == role {
			result = append(result, field)
		}
	}
	return result
}

func (m Meta) CalculateHash() string {
	h := sha256.New()

	io.WriteString(h, string(m.Type))
	io.WriteString(h, "\n")
	io.WriteString(h, m.Name)
	io.WriteString(h, "\n")

	// Sort field names.
	fieldNames := make([]string, 0, len(m.Fields))
	for fn := range m.Fields {
		fieldNames = append(fieldNames, fn)
	}
	sort.Strings(fieldNames)

	for _, fn := range fieldNames {
		f := m.Fields[fn]
		io.WriteString(h, "F:")
		io.WriteString(h, fn)
		io.WriteString(h, "=")
		io.WriteString(h, f.String()) // assumes Field has String()
		io.WriteString(h, "\n")
	}

	if m.Aggregation != nil {
		// Use JSON for aggregation (encoding/json sorts map keys).
		if b, err := encoder.Encode(m.Aggregation, encoder.SortMapKeys); err == nil {
			io.WriteString(h, "AGG:")
			h.Write(b)
			io.WriteString(h, "\n")
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (m Meta) Hash() string {
	return m.HashData
}

func (m Meta) IsNumeric(fieldName string) bool {
	if field, ok := m.Fields[fieldName]; ok {
		return field.IsNumeric()
	}
	parts := SplitNestedFieldName(fieldName)
	if len(parts) > 1 {
		if field, ok := m.Fields[parts[0]]; ok {
			return field.NestedTypeChecker(fieldName, isNumericType)
		}
	}
	return false
}

func (m Meta) IsString(fieldName string) bool {
	if field, ok := m.Fields[fieldName]; ok {
		return field.IsString()
	}
	parts := SplitNestedFieldName(fieldName)
	if len(parts) > 1 {
		if field, ok := m.Fields[parts[0]]; ok {
			return field.NestedTypeChecker(fieldName, isStringType)
		}
	}
	return false
}

func (m Meta) IsBool(fieldName string) bool {
	if field, ok := m.Fields[fieldName]; ok {
		return field.IsBool()
	}
	parts := SplitNestedFieldName(fieldName)
	if len(parts) > 1 {
		if field, ok := m.Fields[parts[0]]; ok {
			return field.NestedTypeChecker(fieldName, isBoolType)
		}
	}
	return false
}

func (m Meta) IsTime(fieldName string) bool {
	if field, ok := m.Fields[fieldName]; ok {
		return field.IsTime()
	}
	parts := SplitNestedFieldName(fieldName)
	if len(parts) > 1 {
		if field, ok := m.Fields[parts[0]]; ok {
			return field.NestedTypeChecker(fieldName, isTimeType)
		}
	}
	return false
}

func (m Meta) IsJSON(fieldName string) bool {
	if field, ok := m.Fields[fieldName]; ok {
		return field.IsJSON()
	}
	parts := SplitNestedFieldName(fieldName)
	if len(parts) > 1 {
		if field, ok := m.Fields[parts[0]]; ok {
			return field.NestedTypeChecker(fieldName, isJSONType)
		}
	}
	return false
}

func (m Meta) IsToken(fieldName string) bool {
	if field, ok := m.Fields[fieldName]; ok {
		return field.IsToken()
	}
	parts := SplitNestedFieldName(fieldName)
	if len(parts) > 1 {
		if field, ok := m.Fields[parts[0]]; ok {
			return field.NestedTypeChecker(fieldName, isTokenType)
		}
	}
	return false
}

func (m Meta) IsArray(fieldName string) bool {
	if field, ok := m.Fields[fieldName]; ok {
		return field.IsArray()
	}
	parts := SplitNestedFieldName(fieldName)
	if len(parts) > 1 {
		if field, ok := m.Fields[parts[0]]; ok {
			return field.NestedTypeChecker(fieldName, isArrayType)
		}
	}
	return false
}

func (m Meta) GetFields() map[string]schema.Field {
	var result = make(map[string]schema.Field)
	for _, field := range utils.GetOrderedMapKeys(m.Fields) {
		result[m.Fields[field].Name] = lo.ToPtr(m.Fields[field])
	}
	return result
}

func (m Meta) GetReservedFields() []schema.Field {
	var result []schema.Field
	for _, field := range m.Fields {
		if field.BuiltIn {
			result = append(result, &field)
		}
	}
	return result
}

func (m Meta) GetFieldTypeValue(name string) any {
	if m.Fields == nil {
		return nil
	}
	var fieldType, _ = m.GetFieldType(name)
	if fieldType == "" {
		return nil
	}
	switch fieldType {
	case FieldTypeString:
		return ""
	case FieldTypeInt:
		return int64(0)
	case FieldTypeFloat:
		return float64(0)
	case FieldTypeBool:
		return false
	case FieldTypeTime:
		return time.Time{}
	case FieldTypeBigFloat:
		return decimal.Decimal{}
	case FieldTypeJSON:
		return json.RawMessage{}
	case FieldTypeToken:
		return make(map[string]any)
	case FieldTypeArray:
		return []any{}
	default:
		return nil
	}
}

func EscapeEventlogFieldName(name string) string {
	name = strings.ReplaceAll(name, "`", "")
	if strings.HasPrefix(name, SystemFieldPrefix) {
		return "`" + name + "`"
	}
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = fmt.Sprintf("`%s`", part)
	}
	return strings.Join(parts, ".")
}

func EscapeMetricsFieldName(name string) string {
	name = strings.ReplaceAll(name, "`", "")
	return fmt.Sprintf("`%s`", name)
}

func UnescapeFieldName(name string) string {
	return strings.ReplaceAll(name, "`", "")
}

func SplitNestedFieldName(name string) []string {
	return strings.Split(UnescapeFieldName(name), ".")
}

func (m Meta) GetFieldType(name string) (FieldType, bool) {
	if m.Fields == nil {
		return "", false
	}
	var fieldType FieldType
	field, ok := m.Fields[name]
	if ok {
		fieldType = field.Type
	} else {
		parts := SplitNestedFieldName(name)
		if len(parts) > 1 {
			if field, ok := m.Fields[parts[0]]; ok {
				nestedName := parts[1:]
				fieldType = field.NestedStructSchema[UnescapeFieldName(strings.Join(nestedName, "."))]
			}
		}
	}
	return fieldType, fieldType != ""
}

func (m Meta) GetFieldTypes(includeNested, includeComplexType bool) map[string]FieldType {
	set := make(map[string]Meta)
	set[m.Name] = m
	return Metaset(set).FieldTypes(includeNested, includeComplexType)
}

var (
	metaNameRawExpr                 = `^[a-zA-Z_]([a-zA-Z0-9_\- ]*[a-zA-Z0-9_])?$`
	metaNameExpr                    = regexp.MustCompile(metaNameRawExpr)
	fieldNameRawExpr                = `^[a-zA-Z_]([a-zA-Z0-9_.]*[a-zA-Z0-9_])?$`
	fieldNameExpr                   = regexp.MustCompile(fieldNameRawExpr)
	eventlogsCustomFieldNameRawExpr = `^[a-zA-Z_]([a-zA-Z0-9_]*[a-zA-Z0-9_])?$`
	eventlogsCustomFieldNameExpr    = regexp.MustCompile(eventlogsCustomFieldNameRawExpr)

	validAggFunctions = []string{"count", "sum", "avg", "max", "min", "last", "first"}
)

func (m Meta) Verify() error {
	// meta type should be valid
	if !IsValidMetaType(m.Type) {
		return errors.Wrapf(ErrInvalidMeta, "type %q is invalid", m.Type)
	}
	// meta name should be valid
	if !metaNameExpr.MatchString(m.Name) {
		return errors.Wrapf(ErrInvalidMeta, "%s name %q is invalid, the legal regular expression is %q",
			m.Type, m.Name, metaNameRawExpr)
	}
	// field name should be valid
	for _, field := range m.Fields {
		if !fieldNameExpr.MatchString(field.Name) {
			return errors.Wrapf(ErrInvalidMeta, "field name %q of %s is invalid, the legal regular expression is %q",
				field.Name, m.GetFullName(), fieldNameRawExpr)
		}
	}
	// a unique aggregate interval field is required for aggregation
	// a unique ChainID field, and a unique SlotNumber field, and a unique Timestamp field, are required
	uniqFieldType := map[FieldRole]FieldType{
		FieldRoleTimestamp:  FieldTypeTime,
		FieldRoleSlotNumber: FieldTypeInt,
		FieldRoleChainID:    FieldTypeString,
	}
	if m.Aggregation != nil {
		uniqFieldType[FieldRoleAggInterval] = FieldTypeString
	}
	for role, typ := range uniqFieldType {
		switch fields := m.GetFieldsByRole(role); len(fields) {
		case 0:
			return errors.Wrapf(ErrInvalidMeta, "%s miss %s field", m.GetFullName(), role)
		case 1:
			if ft := fields[0].Type; ft != typ {
				return errors.Wrapf(ErrInvalidMeta, "type of %s field of %s is %s, should be %s",
					role, m.GetFullName(), ft, typ)
			}
		default:
			fieldNames := utils.MapSliceNoError(fields, func(f Field) string {
				return f.Name
			})
			return errors.Wrapf(ErrInvalidMeta, "%s has more than one %s fields %v", m.GetFullName(), role, fieldNames)
		}
	}
	// counter and gauge need series value
	if m.Type == MetaTypeCounter || m.Type == MetaTypeGauge {
		if fields := m.GetFieldsByRole(FieldRoleSeriesValue); len(fields) == 0 {
			return errors.Wrapf(ErrInvalidMeta, "%s miss %s field", m.GetFullName(), FieldRoleSeriesValue)
		}
	}
	// all the series value fields of aggregation should have aggregate config
	if m.Aggregation != nil {
		if fields := m.GetFieldsByRole(FieldRoleNone); len(fields) > 0 {
			return errors.Wrapf(ErrInvalidMeta, "fields %s of %s without any role",
				strings.Join(GetFieldNames(fields), ","), m.GetFullName())
		}
		for _, field := range m.GetFieldsByRole(FieldRoleSeriesValue) {
			if aggField, has := m.Aggregation.Fields[field.Name]; !has {
				return errors.Wrapf(ErrInvalidMeta, "%s miss aggregate config for %s field %s",
					m.GetFullName(), FieldRoleSeriesValue, field.Name)
			} else if utils.IndexOf(validAggFunctions, aggField.Function) < 0 {
				return errors.Wrapf(ErrInvalidMeta, "%s has invalid aggregate function %q for %s field %s",
					m.GetFullName(), aggField.Function, FieldRoleSeriesValue, field.Name)
			}
		}
	}
	return nil
}

type FieldDiff struct {
	Before Field
	After  Field
}

type FieldsDiff struct {
	AddFields []Field
	DelFields []Field
	UpdFields []FieldDiff
	UpdSchema []FieldDiff
}

func CalcFieldsDiff(origin, other map[string]Field) FieldsDiff {
	diff := FieldsDiff{}
	for fn, after := range other {
		if before, has := origin[fn]; !has {
			diff.AddFields = append(diff.AddFields, after)
		} else if !before.Compatible(after) {
			diff.UpdFields = append(diff.UpdFields, FieldDiff{
				Before: before,
				After:  after,
			})
		}
	}
	for fn, before := range origin {
		if _, has := other[fn]; !has {
			diff.DelFields = append(diff.DelFields, before)
		} else {
			after, changed := before.Merge(other[fn])
			if changed {
				diff.UpdSchema = append(diff.UpdSchema, FieldDiff{
					Before: before,
					After:  after,
				})
			}
		}
	}
	return diff
}

func SameFields(a, b []Field) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Compatible(b[i]) {
			return false
		}
	}
	return true
}

func GetFieldsDiffSummary(fields []FieldDiff, indent, seq string) string {
	var sectors []string
	for _, field := range fields {
		sectors = append(sectors, fmt.Sprintf("%s%s: before:%s, after:%s",
			indent, field.Before.Name, field.Before.String(), field.After.String()))
	}
	return strings.Join(sectors, seq)
}

func (m Meta) GetInvertedIndex() map[string]map[string]map[string]bool {
	panic("not implemented")
}

type Metaset map[string]Meta

func (m Metaset) FieldTypes(includeNested, includeComplexType bool) map[string]FieldType {
	var (
		candidates = make(map[string][]FieldType)
		results    = make(map[string]FieldType)
	)
	for _, meta := range m {
		for _, field := range meta.Fields {
			switch field.Type {
			case FieldTypeArray, FieldTypeToken:
				if includeComplexType {
					candidates[field.Name] = append(candidates[field.Name], field.Type)
				}
			case FieldTypeJSON:
				if includeComplexType {
					candidates[field.Name] = append(candidates[field.Name], field.Type)
				}
				if includeNested {
					for nestedField, nestedType := range field.NestedStructSchema {
						nestedFieldName := field.Name + "." + nestedField
						switch nestedType {
						case FieldTypeArray, FieldTypeToken, FieldTypeJSON:
							if includeComplexType {
								candidates[nestedFieldName] = append(candidates[nestedFieldName], nestedType)
							}
						default:
							candidates[nestedFieldName] = append(candidates[nestedFieldName], nestedType)
						}
					}
				}
			default:
				candidates[field.Name] = append(candidates[field.Name], field.Type)
			}
		}
	}
	for name, types := range candidates {
		fieldTypes := FieldTypes(types)
		results[name] = fieldTypes.ComplexGCD()
	}
	return results
}
