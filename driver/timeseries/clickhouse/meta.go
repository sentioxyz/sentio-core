package clickhouse

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
)

var (
	timeseriesMetaLogPrintEveryN = flag.Int("timeseries-meta-log-print-debug-every-n", 10, "")
)

var (
	dbTypeMapping = map[timeseries.FieldType]string{
		timeseries.FieldTypeString:   "String",
		timeseries.FieldTypeBool:     "Bool",
		timeseries.FieldTypeTime:     "DateTime64(6, 'UTC')",
		timeseries.FieldTypeInt:      "Int64",
		timeseries.FieldTypeBigInt:   "Int256",
		timeseries.FieldTypeFloat:    "Float64",
		timeseries.FieldTypeBigFloat: "Decimal(76, 30)",
		timeseries.FieldTypeJSON:     "JSON",
		timeseries.FieldTypeArray:    "Array(Dynamic)",
		timeseries.FieldTypeToken:    "Tuple(symbol String, chain String, address String, amount Decimal(76, 30), timestamp DateTime64(6, 'UTC'))",
	}

	escapeSQLString = func(s string) string {
		return strings.ReplaceAll(s, "'", "''")
	}

	DbValueCasting = func(strValue string, t timeseries.FieldType) string {
		clickhouseType, ok := dbTypeMapping[t]
		if !ok {
			panic(fmt.Errorf("unsupported field type %s", t))
		}
		escapedValue := escapeSQLString(strValue)
		switch t {
		case timeseries.FieldTypeTime, timeseries.FieldTypeToken:
			return "'" + escapedValue + "'::" + clickhouseType
		default:
			return fmt.Sprintf("CAST('%s', '%s')", escapedValue, clickhouseType)
		}
	}

	DbTypeCasting = func(name string, t timeseries.FieldType) string {
		clickhouseType, ok := dbTypeMapping[t]
		if !ok {
			panic(fmt.Errorf("unsupported field type %s", t))
		}
		switch t {
		case timeseries.FieldTypeTime, timeseries.FieldTypeToken:
			return name + "::" + clickhouseType
		default:
			return fmt.Sprintf("CAST(%s, '%s')", name, clickhouseType)
		}
	}

	DbNullableTypeCasting = func(name string, t timeseries.FieldType) string {
		return fmt.Sprintf("toNullable(%s)", DbTypeCasting(name, t))
	}
)

var (
	staticFieldIndexTypeMapping = map[timeseries.FieldType]string{
		timeseries.FieldTypeString:   "bloom_filter",
		timeseries.FieldTypeBool:     "minmax",
		timeseries.FieldTypeTime:     "minmax",
		timeseries.FieldTypeInt:      "minmax",
		timeseries.FieldTypeBigInt:   "minmax",
		timeseries.FieldTypeFloat:    "minmax",
		timeseries.FieldTypeBigFloat: "minmax",
	}

	dynamicFieldIndexType = map[timeseries.FieldType]bool{
		timeseries.FieldTypeJSON:  true,
		timeseries.FieldTypeArray: false,
	}
)

func indexName(fieldName, nestedName string) string {
	idxName := "idx_" + fieldName
	if nestedName != "" {
		idxName += "_" + nestedName
	}
	return idxName
}

func tableToMeta(
	ctx context.Context,
	table chx.Table,
	ignoreInvalidTableCommentErr bool,
) (meta timeseries.Meta, err error) {
	_, logger := log.FromContext(ctx)

	meta.Type, meta.Name, err = timeseries.CutTableName(table.Name)
	if err != nil {
		logger.Warnfe(err, "invalid table name %q", table.Name)
		return meta, errors.Wrapf(err, "invalid table name %q", table.Name)
	}

	var metaFromComment timeseries.Meta
	metaFromComment, err = timeseries.LoadMeta(table.Comment)
	if err != nil {
		logger.Warnfe(err, "invalid comment for table %q", table.Name)
		if !ignoreInvalidTableCommentErr {
			return meta, errors.Wrapf(err, "invalid comment for table %q", table.Name)
		}
	} else {
		meta.Fields = metaFromComment.Fields
		meta.Aggregation = metaFromComment.Aggregation
		meta.HashData = metaFromComment.HashData
		logger.DebugEveryN(*timeseriesMetaLogPrintEveryN, "loaded meta from comment, table: %s, fields: %v",
			table.Name, metaFromComment.Fields)
	}
	if meta.Fields == nil {
		meta.Fields = make(map[string]timeseries.Field)
	}

	for _, tf := range table.Fields {
		field := timeseries.Field{
			Name:               tf.Name,
			Role:               timeseries.FieldRole(tf.Comment),
			NestedStructSchema: make(map[string]timeseries.FieldType),
			NestedIndex:        make(map[string]timeseries.FieldType),
		}
		fieldType := tf.Type.String()
		for ft, dbType := range dbTypeMapping {
			if dbType == fieldType {
				field.Type = ft
			}
		}
		if field.Type == "" {
			return meta, errors.Errorf("invalid type %q for field %s.%s", fieldType, table.Name, tf.Name)
		}
		if exist, has := meta.Fields[field.Name]; has {
			field, _ = exist.Merge(field)
		}
		meta.Fields[field.Name] = field
	}

	indexes := set.New[string]()
	for _, index := range table.Indexes {
		indexes.Add(index.Name)
	}
	for fieldName, field := range meta.Fields {
		if indexes.Contains(indexName(field.Name, "")) {
			field.Index = true
		}
		if field.NestedIndex == nil {
			field.NestedIndex = make(map[string]timeseries.FieldType)
		}
		for nestedName, nestedType := range field.NestedStructSchema {
			if indexes.Contains(indexName(field.Name, nestedName)) {
				field.NestedIndex[nestedName] = nestedType
			}
		}
		meta.Fields[fieldName] = field
	}

	meta.HashData = meta.CalculateHash()
	return meta, nil
}

func tablesToMetes(
	ctx context.Context,
	tvs []chx.TableOrView,
	ignoreInvalidTableCommentErr bool,
) (sm storeMeta, err error) {
	var items []metaAndTable
	for _, tv := range tvs {
		table, is := tv.(chx.Table)
		if !is {
			continue
		}

		var meta timeseries.Meta
		meta, err = tableToMeta(ctx, table, ignoreInvalidTableCommentErr)
		if err != nil {
			return nil, err
		}
		items = append(items, metaAndTable{meta: meta, table: table})
	}
	return newStoreMeta(items), nil
}

func (s *Store) metaToTable(ctx context.Context, meta timeseries.Meta) chx.Table {
	_, logger := log.FromContext(ctx)
	table := chx.Table{
		Name: meta.GetTableName(),
		Config: chx.TableConfig{
			Engine:      s.ctrl.NewDefaultMergeTreeEngine(),
			PartitionBy: meta.GetChainIDField().Name,
			OrderBy:     []string{meta.GetTimestampField().Name},
			Settings:    map[string]string{"index_granularity": "8192"},
		},
		Comment: string(meta.Dump()),
	}
	chx.WithLightDeleteTableSettings(table.Config.Settings)
	for _, field := range utils.GetMapValuesOrderByKey(meta.Fields) {
		table.Fields = append(table.Fields, chx.Field{
			Name:    field.Name,
			Type:    chx.BuildFieldType(dbTypeMapping[field.Type]),
			Comment: string(field.Role),
		})
		if field.Index {
			switch {
			case dynamicFieldIndexType[field.Type]:
				for _, nestedName := range utils.GetOrderedMapKeys(field.NestedIndex) {
					nestedType := field.NestedIndex[nestedName]
					table.Indexes = append(table.Indexes, chx.Index{
						Name:        indexName(field.Name, nestedName),
						Type:        staticFieldIndexTypeMapping[nestedType],
						Expr:        fmt.Sprintf("CAST(`%s.%s`, '%s')", field.Name, nestedName, nestedType),
						Granularity: 1,
					})
				}
			case staticFieldIndexTypeMapping[field.Type] != "":
				table.Indexes = append(table.Indexes, chx.Index{
					Name:        indexName(field.Name, ""),
					Type:        staticFieldIndexTypeMapping[field.Type],
					Expr:        field.Name,
					Granularity: 1,
				})
			default:
				logger.Warnf("unsupported field type %s for %s.%s to build index", field.Type, meta.GetFullName(), field.Name)
			}
		}
	}
	return table
}

func (s *Store) fetchMetas(ctx context.Context, fullLoad bool) error {
	startAt := time.Now()
	_, logger := log.FromContext(ctx, "fullLoad", fullLoad)
	tvs, err := s.ctrl.LoadAll(ctx, fullLoad)
	if err != nil {
		return err
	}
	var sm storeMeta
	if sm, err = tablesToMetes(ctx, tvs, true); err != nil {
		return err
	}
	s.meta = sm
	used := time.Since(startAt)

	logger.InfoIfF(
		used > time.Millisecond*500,
		"fetch all meta, tables: %v, hash: %s, used: %s",
		func() []string {
			return sm.GetTableNames()
		},
		func() string {
			return sm.GetHash()
		},
		used.String(),
	)
	logger.DebugEveryN(*timeseriesMetaLogPrintEveryN, "fetch meta debug, hash: %s, meta: %s",
		sm.GetHash(), sm.String())
	return nil
}

func (s *Store) syncMeta(ctx context.Context, data timeseries.Dataset) error {
	startAt := time.Now()
	meta := data.Meta
	_, logger := log.FromContext(ctx, "meta", meta.GetFullName())
	if err := meta.Verify(); err != nil {
		logger.Errore(err, "invalid meta")
		return err
	}

	preMeta, preTable, has := s.meta.find(meta.Type, meta.Name)
	if !has {
		logger.Debug("will create table")
		table := s.metaToTable(ctx, meta)
		if err := s.probe.PreCreateTable(ctx, table); err != nil {
			return err
		}
		if err := s.ctrl.Create(ctx, table); err != nil {
			return err
		}
		s.meta = s.meta.add(meta, table)
		return nil
	}

	if !preMeta.Aggregation.IsSame(meta.Aggregation) {
		logger.Errorw("unacceptable aggregation change", "before", preMeta.Aggregation, "after", meta.Aggregation)
		return errors.Wrapf(timeseries.ErrInvalidMetaDiff, "unacceptable aggregation change for %s", meta.GetFullName())
	}
	if diff := preMeta.DiffFields(meta); len(diff.UpdFields) > 0 {
		logger.Errorw("unacceptable table fields change", "diff", diff.UpdFields)
		var sampleText string
		if len(data.Rows) > 0 {
			sampleText = fmt.Sprintf("\nsample data: %v", data.Rows[0])
		}
		return errors.Wrapf(timeseries.ErrInvalidMetaDiff, "unacceptable field update for %s:\n%s%s",
			meta.GetFullName(),
			timeseries.GetFieldsDiffSummary(diff.UpdFields, "\t", "\n"),
			sampleText,
		)
	}

	meta = preMeta.Merge(meta)
	table := s.metaToTable(ctx, meta)
	if err := s.ctrl.SyncTable(ctx, preTable, table); err != nil {
		return err
	}
	s.meta.set(meta, table)
	used := time.Since(startAt)
	logger.InfoIfF(used > time.Second, "sync meta succeed, hash: %s, used: %s", meta.Hash(), used.String())
	return nil
}

func (s *Store) syncMetas(ctx context.Context, data []timeseries.Dataset) error {
	for _, ds := range data {
		if err := s.syncMeta(ctx, ds); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CleanAll(ctx context.Context) error {
	s.metaLock.Lock()
	defer s.metaLock.Unlock()
	for _, item := range s.meta {
		if err := s.ctrl.Drop(ctx, item.table); err != nil {
			return err
		}
	}
	s.meta = nil
	return nil
}

func (s *Store) Meta() timeseries.StoreMeta {
	s.metaLock.Lock()
	defer s.metaLock.Unlock()
	return s.meta
}

func (s *Store) ReloadMeta(ctx context.Context) error {
	s.metaLock.Lock()
	defer s.metaLock.Unlock()
	return s.fetchMetas(ctx, false)
}

func GetDBTypeMapping(ttype timeseries.FieldType) string {
	return dbTypeMapping[ttype]
}
