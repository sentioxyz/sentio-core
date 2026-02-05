package timeseries

import (
	"bytes"
	"fmt"
	"math/big"
	"time"

	rsh "sentioxyz/sentio-core/common/richstructhelper"
	"sentioxyz/sentio-core/common/utils"
	commonProtos "sentioxyz/sentio-core/service/common/protos"

	"github.com/bytedance/sonic"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/shopspring/decimal"
)

type Row map[string]any
type Dataset struct {
	Meta
	Rows []Row
}

func GetDatasetsSummary(dss []Dataset) string {
	sum := make(map[string]int)
	for _, ds := range dss {
		sum[ds.Meta.GetFullName()] += len(ds.Rows)
	}
	var buf bytes.Buffer
	for i, key := range utils.GetOrderedMapKeys(sum) {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(fmt.Sprintf("%s/%d", key, sum[key]))
	}
	return buf.String()
}

func Statistic(dss []Dataset, stat map[MetaType]map[string]int) {
	for _, ds := range dss {
		utils.IncrK2Map(stat, ds.Type, ds.Name, len(ds.Rows))
	}
}

// NestedRow is a row with nested struct.
type NestedRow struct {
	Row
	StructSchema map[string]FieldType
}

func richValueToAny(v *commonProtos.RichValue, defaultTimestamp time.Time) (any, FieldType, error) {
	switch v.Value.(type) {
	case *commonProtos.RichValue_IntValue:
		return int64(v.GetIntValue()), FieldTypeInt, nil
	case *commonProtos.RichValue_Int64Value:
		return v.GetInt64Value(), FieldTypeInt, nil
	case *commonProtos.RichValue_FloatValue:
		return v.GetFloatValue(), FieldTypeFloat, nil
	case *commonProtos.RichValue_BytesValue:
		return hexutil.Encode(v.GetBytesValue()), FieldTypeString, nil
	case *commonProtos.RichValue_BoolValue:
		return v.GetBoolValue(), FieldTypeBool, nil
	case *commonProtos.RichValue_StringValue:
		return v.GetStringValue(), FieldTypeString, nil
	case *commonProtos.RichValue_TimestampValue:
		return v.GetTimestampValue().AsTime(), FieldTypeTime, nil
	case *commonProtos.RichValue_BigintValue:
		bigIntVal, ok := rsh.GetBigInt(v)
		if ok {
			return bigIntVal, FieldTypeBigInt, nil
		}
		return big.NewInt(0), FieldTypeBigInt, fmt.Errorf("failed to parse big int value %v", v)
	case *commonProtos.RichValue_BigdecimalValue:
		bigFloatVal, ok := rsh.GetBigDecimal(v)
		if ok {
			return bigFloatVal, FieldTypeBigFloat, nil
		}
		return decimal.NewFromBigInt(big.NewInt(0), 0), FieldTypeBigFloat, fmt.Errorf("failed to parse big decimal value %v", v)
	case *commonProtos.RichValue_ListValue:
		var list []any
		for _, v := range v.GetListValue().GetValues() {
			v, _, err := richValueToAny(v, defaultTimestamp)
			if err != nil {
				return nil, FieldTypeArray, err
			}
			list = append(list, v)
		}
		return list, FieldTypeArray, nil
	case *commonProtos.RichValue_StructValue:
		var m = make(map[string]any)
		for k, v := range v.GetStructValue().GetFields() {
			v, _, err := richValueToAny(v, defaultTimestamp)
			if err != nil {
				return nil, FieldTypeJSON, err
			}
			m[k] = v
		}
		return m, FieldTypeJSON, nil
	case *commonProtos.RichValue_TokenValue:
		token, ok := rsh.GetTokenPrice(v, defaultTimestamp)
		if ok {
			return token, FieldTypeToken, nil
		}
		return nil, FieldTypeToken, fmt.Errorf("failed to parse token value %v", v)
	default:
		return nil, FieldTypeString, fmt.Errorf("unknown RichValue type: %T", v.Value)
	}
}

func (r *NestedRow) Update(prefix string, structValue *commonProtos.RichStruct, timestamp time.Time) error {
	if structValue.Fields == nil {
		return nil
	}
	var (
		field  = structValue.GetFields()
		update = prefix == ""
	)
	for _, k := range utils.GetOrderedMapKeys[string, *commonProtos.RichValue](field) {
		var name = k
		if prefix != "" {
			name = fmt.Sprintf("%s.%s", prefix, k)
		}
		v := field[k]
		switch v.Value.(type) {
		case *commonProtos.RichValue_NullValue_:
			continue
		case *commonProtos.RichValue_IntValue,
			*commonProtos.RichValue_Int64Value,
			*commonProtos.RichValue_FloatValue,
			*commonProtos.RichValue_StringValue,
			*commonProtos.RichValue_BytesValue,
			*commonProtos.RichValue_TimestampValue,
			*commonProtos.RichValue_BoolValue:
			var value any
			value, r.StructSchema[name], _ = richValueToAny(field[k], timestamp)
			if update {
				r.Row[name] = value
			}
		case *commonProtos.RichValue_BigintValue,
			*commonProtos.RichValue_BigdecimalValue,
			*commonProtos.RichValue_ListValue,
			*commonProtos.RichValue_TokenValue:
			var (
				value any
				err   error
			)
			value, r.StructSchema[name], err = richValueToAny(field[k], timestamp)
			if err != nil {
				return err
			}
			if update {
				r.Row[name] = value
			}
		case *commonProtos.RichValue_StructValue:
			var (
				value any
				err   error
			)
			value, r.StructSchema[name], err = richValueToAny(field[k], timestamp)
			if err != nil {
				return err
			}
			if update {
				r.Row[name] = value
			}
			if err := r.Update(name, v.GetStructValue(), timestamp); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown RichValue type: %T", v.Value)
		}
	}
	return nil
}

func (r *NestedRow) Data() string {
	data, _ := sonic.Marshal(r.Row)
	return string(data)
}

func (r *NestedRow) DataByKey(key string) any {
	if r.Row[key] == nil {
		return ""
	}
	return r.Row[key]
}
