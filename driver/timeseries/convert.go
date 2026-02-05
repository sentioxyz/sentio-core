package timeseries

import (
	"strconv"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/period"
	rsh "sentioxyz/sentio-core/common/richstructhelper"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/processor/protos"
	commonProtos "sentioxyz/sentio-core/service/common/protos"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
)

var (
	metricTypeMapping = map[protos.MetricType]MetaType{
		protos.MetricType_COUNTER: MetaTypeCounter,
		protos.MetricType_GAUGE:   MetaTypeGauge,
	}

	eventAllowOverwriteField = map[string]struct{}{
		SystemUserID:                   {},
		SystemFieldPrefix + "severity": {},
	}
)

type MetricConfigSet map[MetaType]map[string]*protos.MetricConfig

func BuildMetricConfigs(configs []*protos.MetricConfig) MetricConfigSet {
	set := make(MetricConfigSet)
	for _, mc := range configs {
		utils.PutIntoK2Map(set, metricTypeMapping[mc.GetType()], mc.GetName(), mc)
	}
	return set
}

var typeMapping = map[protos.TimeseriesResult_TimeseriesType]MetaType{
	protos.TimeseriesResult_EVENT:   MetaTypeEvent,
	protos.TimeseriesResult_GAUGE:   MetaTypeGauge,
	protos.TimeseriesResult_COUNTER: MetaTypeCounter,
}

const (
	SystemFieldPrefix          = "meta."
	MetricValueFieldName       = "value"
	MetricAggIntervalFieldName = SystemFieldPrefix + "aggregation_interval"
	SystemUserID               = "distinctId"
	SystemTimestamp            = "timestamp"
)

type enumConverter func(value string) string

var (
	reservedMetricNameSuffix = utils.MapSliceNoError(
		utils.GetMapValuesOrderByKey(protos.AggregationType_name),
		func(aggType string) string {
			return "_" + strings.ToLower(aggType)
		})

	eventLogsEnumConverters = map[string]enumConverter{
		SystemFieldPrefix + "severity": func(value string) string {
			enum, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return value
			}
			return protos.LogLevel_name[int32(enum)]
		},
	}
)

func containsReservedMetricNameSuffix(name string) bool {
	return utils.HasAny(reservedMetricNameSuffix, func(suf string) bool {
		return strings.HasSuffix(name, suf)
	})
}

func UpdateEvents(data *commonProtos.RichStruct, row *Row, meta *Meta, blockTime time.Time) error {
	for fn, val := range data.GetFields() {
		// rename field name
		// distinctEntityId is the user field distinctId, it was renamed by sdk,
		// severity is the field added by sdk.
		var (
			isBuiltIn      = false
			allowOverwrite = false
		)
		switch fn {
		case "severity":
			fn = SystemFieldPrefix + "severity"
			isBuiltIn = true
		case "distinctEntityId":
			fn = SystemUserID
			isBuiltIn = true
		case SystemUserID:
			isBuiltIn = true
		default:
			if !eventlogsCustomFieldNameExpr.MatchString(fn) {
				return errors.Wrapf(ErrInvalidMeta,
					"field name %q for %s is invalid, the legal regular expression is %q",
					fn, meta.GetFullName(), eventlogsCustomFieldNameRawExpr)
			}
		}
		_, allowOverwrite = eventAllowOverwriteField[fn]
		if _, has := (*row)[fn]; has && !allowOverwrite {
			return errors.Wrapf(ErrInvalidMeta, "field %s.%s is reserved", meta.GetFullName(), fn)
		}
		var (
			fieldType          FieldType
			nestedStructSchema = make(map[string]FieldType)
		)
		switch v := val.GetValue().(type) {
		case *commonProtos.RichValue_NullValue_:
			continue
		case *commonProtos.RichValue_StringValue:
			fieldType = FieldTypeString
			if converter, has := eventLogsEnumConverters[fn]; has {
				(*row)[fn] = converter(v.StringValue)
			} else {
				(*row)[fn] = v.StringValue
			}
		case *commonProtos.RichValue_BytesValue:
			fieldType, (*row)[fn] = FieldTypeString, hexutil.Encode(v.BytesValue)
		case *commonProtos.RichValue_BoolValue:
			fieldType, (*row)[fn] = FieldTypeBool, v.BoolValue
		case *commonProtos.RichValue_IntValue:
			fieldType, (*row)[fn] = FieldTypeInt, int64(v.IntValue)
		case *commonProtos.RichValue_Int64Value:
			fieldType, (*row)[fn] = FieldTypeInt, v.Int64Value
		case *commonProtos.RichValue_TimestampValue:
			fieldType, (*row)[fn] = FieldTypeTime, v.TimestampValue.AsTime()
		case *commonProtos.RichValue_FloatValue:
			fieldType, (*row)[fn] = FieldTypeFloat, v.FloatValue
		case *commonProtos.RichValue_BigintValue:
			bigIntVal, _ := rsh.GetBigInt(val)
			fieldType, (*row)[fn] = FieldTypeBigInt, bigIntVal
		case *commonProtos.RichValue_BigdecimalValue:
			bigFloatVal, _ := rsh.GetBigDecimal(val)
			fieldType, (*row)[fn] = FieldTypeBigFloat, bigFloatVal
		case *commonProtos.RichValue_ListValue, *commonProtos.RichValue_TokenValue:
			const wrappedKey = "_wrapped_value_"
			nestedRow := &NestedRow{
				Row:          make(map[string]any),
				StructSchema: make(map[string]FieldType),
			}
			if err := nestedRow.Update("", &commonProtos.RichStruct{
				Fields: map[string]*commonProtos.RichValue{
					wrappedKey: val,
				},
			}, blockTime); err != nil {
				return err
			}
			switch val.GetValue().(type) {
			case *commonProtos.RichValue_ListValue:
				fieldType, (*row)[fn] = FieldTypeArray, nestedRow.DataByKey(wrappedKey)
			case *commonProtos.RichValue_TokenValue:
				fieldType, (*row)[fn] = FieldTypeToken, nestedRow.DataByKey(wrappedKey)
			}
		case *commonProtos.RichValue_StructValue:
			nestedRow := &NestedRow{
				Row:          make(map[string]any),
				StructSchema: make(map[string]FieldType),
			}
			if err := nestedRow.Update("", v.StructValue, blockTime); err != nil {
				return err
			}
			fieldType, (*row)[fn] = FieldTypeJSON, nestedRow.Data()
			nestedStructSchema = nestedRow.StructSchema
		default:
			return errors.Wrapf(ErrInvalidMeta, "%s.%s has invalid type %T", meta.GetFullName(), fn, val.GetValue())
		}
		field := Field{Name: fn, Type: fieldType, BuiltIn: isBuiltIn, NestedStructSchema: nestedStructSchema, NestedIndex: make(map[string]FieldType)}
		exist, hasField := meta.Fields[fn]
		if hasField {
			if !exist.Compatible(field) {
				diff := exist.CompatibleDiff(field)
				return errors.Wrapf(ErrInvalidMetaDiff, "types of fields %s.%s are not uniform, including %s and %s",
					meta.GetFullName(), diff.Before.Name, diff.Before.Type, diff.After.Type)
			}
			field, _ = exist.Merge(field)
		}
		meta.Fields[fn] = field
	}
	return nil
}

func Convert(
	chainID string,
	blockNumber uint64,
	blockHash string,
	blockTime time.Time,
	metricConfigs MetricConfigSet,
	data []*protos.TimeseriesResult,
) ([]Dataset, error) {
	var datasets = make(map[string]*Dataset)
	for _, r := range data {
		// check name and field name
		if !metaNameExpr.MatchString(r.Metadata.Name) {
			return nil, errors.Wrapf(ErrInvalidMeta, "%s name %q is invalid, the legal regular expression is %q",
				r.GetType(), r.Metadata.Name, metaNameRawExpr)
		}
		for fn := range r.Data.GetFields() {
			if !fieldNameExpr.MatchString(fn) {
				return nil, errors.Wrapf(ErrInvalidMeta,
					"field name %q for %s.%s is invalid, the legal regular expression is %q",
					fn, r.GetType(), r.Metadata.Name, fieldNameRawExpr)
			}
		}
		if r.Metadata.BlockNumber != blockNumber {
			panic(errors.Errorf("block number is %d, expected is %d", r.Metadata.BlockNumber, blockNumber))
		}
		if _, has := typeMapping[r.GetType()]; !has {
			// unknown type, just ignore it
			continue
		}
		// build initial meta
		meta := Meta{Type: typeMapping[r.GetType()], Name: r.Metadata.Name}
		metaFullName := meta.GetFullName()
		ds, has := datasets[metaFullName]
		if !has {
			meta.Fields = BuildFields(
				Field{Name: SystemTimestamp, Type: FieldTypeTime, Role: FieldRoleTimestamp, BuiltIn: true},
				Field{Name: SystemUserID, Type: FieldTypeString, Role: FieldRoleNone, BuiltIn: true},
				Field{Name: SystemFieldPrefix + "chain", Type: FieldTypeString, Role: FieldRoleChainID, BuiltIn: true},
				Field{Name: SystemFieldPrefix + "block_number", Type: FieldTypeInt, Role: FieldRoleSlotNumber, BuiltIn: true},
				Field{Name: SystemFieldPrefix + "block_hash", Type: FieldTypeString, BuiltIn: true},
				Field{Name: SystemFieldPrefix + "transaction_hash", Type: FieldTypeString, BuiltIn: true},
				Field{Name: SystemFieldPrefix + "transaction_index", Type: FieldTypeInt, BuiltIn: true},
				Field{Name: SystemFieldPrefix + "log_index", Type: FieldTypeInt, BuiltIn: true},
				Field{Name: SystemFieldPrefix + "address", Type: FieldTypeString, Role: FieldRoleSeriesLabel, BuiltIn: true},
				Field{Name: SystemFieldPrefix + "contract", Type: FieldTypeString, Role: FieldRoleSeriesLabel, BuiltIn: true},
				Field{Name: SystemFieldPrefix + "severity", Type: FieldTypeString, Role: FieldRoleNone, BuiltIn: true},
			)
			ds = &Dataset{Meta: meta}
			datasets[metaFullName] = ds
		}
		// build row and complete meta
		row := Row{
			SystemTimestamp:                         blockTime,
			SystemUserID:                            "",
			SystemFieldPrefix + "chain":             chainID,
			SystemFieldPrefix + "block_number":      int64(blockNumber),
			SystemFieldPrefix + "block_hash":        blockHash,
			SystemFieldPrefix + "transaction_hash":  r.Metadata.TransactionHash,
			SystemFieldPrefix + "transaction_index": int64(r.Metadata.TransactionIndex),
			SystemFieldPrefix + "log_index":         int64(r.Metadata.LogIndex),
			SystemFieldPrefix + "address":           r.Metadata.Address,
			SystemFieldPrefix + "contract":          r.Metadata.ContractName,
			SystemFieldPrefix + "severity":          "",
		}
		if r.GetType() == protos.TimeseriesResult_EVENT {
			if err := UpdateEvents(r.Data, &row, &datasets[metaFullName].Meta, blockTime); err != nil {
				return nil, err
			}
		} else {
			if containsReservedMetricNameSuffix(ds.Name) {
				return nil, errors.Wrapf(ErrInvalidMeta,
					"%s name %q is invalid, cannot use %v as the suffix", r.GetType(), r.Metadata.Name, reservedMetricNameSuffix)
			}
			ds.Meta.Fields[MetricValueFieldName] = Field{
				Name: MetricValueFieldName,
				Type: FieldTypeFloat,
				Role: FieldRoleSeriesValue,
			}
			row[MetricValueFieldName] = float64(0) // default use zero value
			for fn, val := range r.Data.GetFields() {
				if fn == MetricValueFieldName {
					row[MetricValueFieldName], _ = rsh.GetFloat(val)
					continue
				}
				if _, has = row[fn]; has {
					return nil, errors.Wrapf(ErrInvalidMeta, "field %s.%s is reserved", metaFullName, fn)
				}
				if _, is := val.GetValue().(*commonProtos.RichValue_NullValue_); !is {
					row[fn], _ = rsh.GetString(val)
					ds.Meta.Fields[fn] = Field{
						Name: fn,
						Type: FieldTypeString,
						Role: FieldRoleSeriesLabel,
					}
				}
			}
		}
		ds.Rows = append(ds.Rows, row)
	}

	// append aggregation meta
	var dss []Dataset
	for _, ds := range datasets {
		dss = append(dss, *ds)

		// find metric config
		metricConfig, has := utils.GetFromK2Map(metricConfigs, ds.Type, ds.Name)
		if !has || metricConfig == nil || len(metricConfig.GetAggregationConfig().GetIntervalInMinutes()) == 0 {
			continue
		}

		var intervals []period.Period
		for _, interval := range metricConfig.GetAggregationConfig().GetIntervalInMinutes() {
			intervals = append(intervals, period.Minute.Multi(uint64(interval)))
		}
		var fields = append([]Field{
			{Name: SystemTimestamp, Type: FieldTypeTime, Role: FieldRoleTimestamp},
			{Name: SystemFieldPrefix + "chain", Type: FieldTypeString, Role: FieldRoleChainID},
			{Name: SystemFieldPrefix + "block_number", Type: FieldTypeInt, Role: FieldRoleSlotNumber},
			{Name: MetricAggIntervalFieldName, Type: FieldTypeString, Role: FieldRoleAggInterval},
			{Name: MetricValueFieldName, Type: FieldTypeFloat, Role: FieldRoleSeriesValue},
		}, ds.Meta.GetFieldsByRole(FieldRoleSeriesLabel)...)

		for _, aggType := range metricConfig.GetAggregationConfig().GetTypes() {
			aggMeta := Meta{
				Name:   metricConfig.Name + "_" + strings.ToLower(aggType.String()),
				Type:   ds.Meta.Type,
				Fields: BuildFields(fields...),
				Aggregation: &Aggregation{
					Source:    ds.Meta.Name,
					Intervals: intervals,
					Fields: map[string]AggregationField{
						MetricValueFieldName: {
							Name:       MetricValueFieldName,
							Function:   strings.ToLower(aggType.String()),
							Expression: MetricValueFieldName,
						},
					},
				},
			}
			dss = append(dss, Dataset{Meta: aggMeta})
		}
	}
	return dss, nil
}
