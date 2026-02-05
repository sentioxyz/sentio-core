package clickhouse

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	processormodels "sentioxyz/sentio-core/service/processor/models"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/bytedance/sonic"
	"github.com/jinzhu/copier"
	"github.com/samber/lo"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
)

type storeMeta struct {
	Metas map[timeseries.MetaType]map[string]timeseries.Meta // map[<Type>][<Name>]

	mutex sync.Mutex `copier:"-"`
}

func (s *storeMeta) GetHash() string {
	h := sha256.New()

	var mType []string
	for t := range s.Metas {
		mType = append(mType, string(t))
	}
	sort.Strings(mType)
	for _, t := range mType {
		for _, n := range utils.GetOrderedMapKeys(s.Metas[timeseries.MetaType(t)]) {
			m := s.Metas[timeseries.MetaType(t)][n]
			h.Write([]byte(m.Hash()))
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (s *storeMeta) GetAllMeta() map[timeseries.MetaType]map[string]timeseries.Meta {
	s.Lock()
	defer s.Unlock()

	return s.Metas
}

func (s *storeMeta) MetaTypes() []timeseries.MetaType {
	s.Lock()
	defer s.Unlock()

	return lo.Keys(s.Metas)
}

func (s *storeMeta) MetaNames(t timeseries.MetaType) []string {
	s.Lock()
	defer s.Unlock()

	if m, ok := s.Metas[t]; ok {
		return lo.Keys(m)
	}
	return nil
}

func (s *storeMeta) Meta(t timeseries.MetaType, name string) (timeseries.Meta, bool) {
	s.Lock()
	defer s.Unlock()

	if m, ok := s.Metas[t]; ok {
		if m, ok := m[name]; ok {
			return m, true
		}
	}
	return timeseries.Meta{}, false
}

func (s *storeMeta) MetaByType(t timeseries.MetaType) map[string]timeseries.Meta {
	s.Lock()
	defer s.Unlock()

	if m, ok := s.Metas[t]; ok {
		return m
	}
	return nil
}

func (s *storeMeta) MustMeta(t timeseries.MetaType, name string) timeseries.Meta {
	s.Lock()
	defer s.Unlock()

	if m, ok := s.Metas[t]; ok {
		if m, ok := m[name]; ok {
			return m
		}
	}
	return timeseries.Meta{}
}

func (s *storeMeta) Lock() {
	s.mutex.Lock()
}

func (s *storeMeta) Unlock() {
	s.mutex.Unlock()
}

func (s *storeMeta) Different(other timeseries.StoreMeta) bool {
	return s.GetHash() != other.GetHash()
}

func (s *storeMeta) String() string {
	meta, _ := sonic.Marshal(s.Metas)
	return string(meta)
}

func (s *Store) sqlOnClusterPart() string {
	if s.cluster == "" {
		return ""
	}
	return fmt.Sprintf("ON CLUSTER '%s'", s.cluster)
}

func (s *Store) sqlEnginePart() string {
	if s.cluster == "" {
		return "ENGINE MergeTree()"
	}
	return "ENGINE ReplicatedMergeTree('/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}','{replica}')"
}

func (s *Store) sqlTableSettingsPart() string {
	if s.tableSettings == "" {
		return "SETTINGS index_granularity = 8192, enable_block_number_column = 1, enable_block_offset_column = 1"
	}
	return "SETTINGS " + s.tableSettings
}

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

func GetDBTypeMapping(ttype timeseries.FieldType) string {
	dbType, has := dbTypeMapping[ttype]
	if !has {
		panic(fmt.Errorf("unsupported field type %s", ttype))
	}
	return dbType
}

func (s *Store) buildField(fieldName, fieldType, fieldComment, indexName string) (timeseries.Field, error) {
	field := timeseries.Field{
		Name:               fieldName,
		Index:              indexName != "",
		Role:               timeseries.FieldRole(fieldComment),
		NestedIndex:        make(map[string]timeseries.FieldType),
		NestedStructSchema: make(map[string]timeseries.FieldType),
	}
	for ft, dbType := range dbTypeMapping {
		if dbType == fieldType {
			field.Type = ft
		}
	}
	if field.Type == "" {
		return field, fmt.Errorf("invalid field type %q for field %q with comment %q", fieldType, fieldName, fieldComment)
	}
	return field, nil
}

func (s *Store) buildFieldDBTypeAndComment(field timeseries.Field) (string, string) {
	dbType, has := dbTypeMapping[field.Type]
	if !has {
		panic(fmt.Errorf("unsupported field type %s", field.Type))
	}
	return dbType, string(field.Role)
}

func (s *Store) buildFieldSQL(field timeseries.Field) string {
	dbType, comment := s.buildFieldDBTypeAndComment(field)
	sql := fmt.Sprintf("`%s` %s", field.Name, dbType)
	if comment != "" {
		sql += fmt.Sprintf(" COMMENT '%s'", comment)
	}
	return sql
}

func (s *Store) buildFieldIndexName(field timeseries.Field) string {
	return fmt.Sprintf("`idx_%s`", field.Name)
}

func (s *Store) buildFieldIndexNestedName(field timeseries.Field, nestedName string) string {
	return fmt.Sprintf("`idx_%s_%s`", field.Name, nestedName)
}

var (
	staticFieldIndexTypeMapping = map[timeseries.FieldType]string{
		timeseries.FieldTypeString:   "bloom_filter GRANULARITY 1",
		timeseries.FieldTypeBool:     "minmax GRANULARITY 1",
		timeseries.FieldTypeTime:     "minmax GRANULARITY 1",
		timeseries.FieldTypeInt:      "minmax GRANULARITY 1",
		timeseries.FieldTypeBigInt:   "minmax GRANULARITY 1",
		timeseries.FieldTypeFloat:    "minmax GRANULARITY 1",
		timeseries.FieldTypeBigFloat: "minmax GRANULARITY 1",
	}

	dynamicFieldIndexType = map[timeseries.FieldType]bool{
		timeseries.FieldTypeJSON:  true,
		timeseries.FieldTypeArray: false,
	}
)

func GetStaticFieldIndexType(ttype timeseries.FieldType) string {
	indexType, has := staticFieldIndexTypeMapping[ttype]
	if !has {
		panic(fmt.Errorf("unsupported field type %s", ttype))
	}
	return indexType
}

func (s *Store) buildStaticFieldIndexType(field timeseries.Field) string {
	return GetStaticFieldIndexType(field.Type)
}

func (s *Store) buildStaticFieldIndexSQL(field timeseries.Field) string {
	return fmt.Sprintf("INDEX %s %s TYPE %s",
		s.buildFieldIndexName(field), s.buildFieldName(field), s.buildStaticFieldIndexType(field))
}

func (s *Store) buildDynamicFieldIndexSQL(field timeseries.Field) []string {
	var sqls []string
	for _, k := range utils.GetOrderedMapKeys(field.NestedIndex) {
		sqls = append(sqls, fmt.Sprintf("INDEX %s %s TYPE %s",
			s.buildFieldIndexNestedName(field, k), s.buildFieldNestedCast(field, k, field.NestedIndex[k]), GetStaticFieldIndexType(field.NestedIndex[k])))
	}
	return sqls
}

func (s *Store) buildFieldIndexSQL(field timeseries.Field) []string {
	switch {
	case dynamicFieldIndexType[field.Type]:
		return s.buildDynamicFieldIndexSQL(field)
	case staticFieldIndexTypeMapping[field.Type] != "":
		return []string{s.buildStaticFieldIndexSQL(field)}
	default:
		panic(fmt.Errorf("unsupported field type %s", field.Type))
	}
}

func (s *Store) buildAddIndexSQL(sqlPrefix string, field timeseries.Field) []string {
	switch {
	case dynamicFieldIndexType[field.Type]:
		var sqls []string
		for _, k := range utils.GetOrderedMapKeys(field.NestedIndex) {
			sqls = append(sqls, fmt.Sprintf("%s ADD INDEX IF NOT EXISTS %s %s TYPE %s",
				sqlPrefix,
				s.buildFieldIndexNestedName(field, k), s.buildFieldNestedCast(field, k, field.NestedIndex[k]), GetStaticFieldIndexType(field.NestedIndex[k])))
		}
		return sqls
	case staticFieldIndexTypeMapping[field.Type] != "":
		return []string{fmt.Sprintf("%s ADD INDEX IF NOT EXISTS %s %s TYPE %s",
			sqlPrefix, s.buildFieldIndexName(field), s.buildFieldName(field), s.buildStaticFieldIndexType(field))}
	default:
		panic(fmt.Errorf("unsupported field type %s", field.Type))
	}
}

func (s *Store) buildTableName(meta timeseries.Meta) string {
	return fmt.Sprintf("`%s`.`%s_%s`", s.database, s.processorID, meta.GetTableSuffix())
}

func (s *Store) BuildTableNameWithoutDatabase(meta timeseries.Meta) string {
	return fmt.Sprintf("%s_%s", s.processorID, meta.GetTableSuffix())
}

func (s *Store) MetaTable(meta timeseries.Meta) string {
	return s.BuildTableNameWithoutDatabase(meta)
}

func (s *Store) cutTableName(tableName string) (timeseries.MetaType, string) {
	typ, name, _ := strings.Cut(strings.TrimPrefix(tableName, s.processorID+"_"), "_")
	return timeseries.MetaType(typ), name
}

func (s *Store) buildFieldName(field timeseries.Field) string {
	return fmt.Sprintf("`%s`", field.Name)
}

func (s *Store) buildFieldNestedCast(field timeseries.Field, nestedName string, nestedType timeseries.FieldType) string {
	return fmt.Sprintf("CAST(`%s.%s`, '%s')", field.Name, nestedName, nestedType)
}

func (s *Store) buildTableNameLike() string {
	return fmt.Sprintf("%s\\_%%", s.processorID)
}

func (s *Store) buildCreateTableSQL(meta timeseries.Meta) string {
	var buf bytes.Buffer

	// header part
	buf.WriteString(fmt.Sprintf("CREATE TABLE %s %s (", s.buildTableName(meta), s.sqlOnClusterPart()))

	// fields and index part
	var fieldSQLList []string
	var indexSQLList []string
	for _, fn := range utils.GetOrderedMapKeys(meta.Fields) {
		field := meta.Fields[fn]
		fieldSQLList = append(fieldSQLList, s.buildFieldSQL(field))
		if field.Index {
			indexSQLList = append(indexSQLList, s.buildFieldIndexSQL(field)...)
		}
	}
	buf.WriteString(strings.Join(utils.MergeArr(fieldSQLList, indexSQLList), ", "))

	// footer part
	chainIDField := meta.GetChainIDField()
	timestampField := meta.GetTimestampField()
	buf.WriteString(fmt.Sprintf(") %s PARTITION BY %s ORDER BY %s %s",
		s.sqlEnginePart(), s.buildFieldName(chainIDField), s.buildFieldName(timestampField), s.sqlTableSettingsPart()))

	buf.WriteString(fmt.Sprintf(" COMMENT '%s'", string(meta.Dump())))

	return buf.String()
}

func (s *Store) CreateTable(ctx context.Context, meta timeseries.Meta) error {
	return s.client.Exec(ctx, s.buildCreateTableSQL(meta))
}

func (s *Store) addFieldsForTable(ctx context.Context, tableName string, fields []timeseries.Field) error {
	for _, field := range fields {
		sqlPrefix := fmt.Sprintf("ALTER TABLE %s %s ", tableName, s.sqlOnClusterPart())
		dbType, comment := s.buildFieldDBTypeAndComment(field)
		sql := fmt.Sprintf("%s ADD COLUMN IF NOT EXISTS %s %s", sqlPrefix, s.buildFieldName(field), dbType)
		if err := s.client.Exec(ctx, sql); err != nil {
			return fmt.Errorf("add field %s with sql %q failed: %w", field.Name, sql, err)
		}
		if comment != "" {
			sql = fmt.Sprintf("%s COMMENT COLUMN IF EXISTS %s '%s'", sqlPrefix, s.buildFieldName(field), comment)
			if err := s.client.Exec(ctx, sql); err != nil {
				return fmt.Errorf("comment field %s with sql %q failed: %w", field.Name, sql, err)
			}
		}
		if field.Index {
			for _, sql := range s.buildAddIndexSQL(sqlPrefix, field) {
				if err := s.client.Exec(ctx, sql); err != nil {
					return fmt.Errorf("add index for field %s with sql %q failed: %w", field.Name, sql, err)
				}
			}
		}
	}
	return nil
}

func (s *Store) alterCommentForTable(ctx context.Context, tableName string, meta timeseries.Meta) error {
	sqlPrefix := fmt.Sprintf("ALTER TABLE %s %s ", tableName, s.sqlOnClusterPart())
	sql := fmt.Sprintf("%s MODIFY COMMENT '%s'", sqlPrefix, string(meta.Dump()))
	return s.client.Exec(ctx, sql)
}

func (s *Store) saveMeta(ctx context.Context) error {
	startTime := time.Now()
	_, logger := log.FromContext(ctx, "processorID", s.processorID)

	for _, metaType := range s.meta.Metas {
		for _, meta := range metaType {
			if err := s.alterCommentForTable(ctx, s.buildTableName(meta), meta); err != nil {
				logger.Warnf("alter comment for table %s failed: %v", meta.GetFullName(), err)
			}
		}
	}
	logger.Infow("save meta",
		"used", time.Since(startTime).String())
	return nil
}

func (s *Store) loadMeta(ctx context.Context) (timeseries.StoreMeta, error) {
	startTime := time.Now()
	_, logger := log.FromContext(ctx, "processorID", s.processorID)
	var (
		storeMeta = &storeMeta{
			Metas: make(map[timeseries.MetaType]map[string]timeseries.Meta),
		}
		tables []string
	)
	if processormodels.IsMockProcessorID(s.processorID) {
		// if it is a mocked processor, return empty meta
		return storeMeta, nil
	}

	sql := "SELECT name, comment FROM system.tables WHERE database = ? AND name LIKE ?"
	err := queryAndScan(ctx, s.client, func(rows driver.Rows) error {
		for rows.Next() {
			var tableName string
			var comment string
			if scanErr := rows.Scan(&tableName, &comment); scanErr != nil {
				return scanErr
			}
			var meta timeseries.Meta
			meta.Type, meta.Name = s.cutTableName(tableName)
			if len(meta.Type) == 0 || len(meta.Name) == 0 {
				logger.Warnf("got invalid table name %q", tableName)
				continue
			}
			if !timeseries.IsValidMetaType(meta.Type) {
				continue
			}
			if comment != "" {
				metaFromComment, err := timeseries.LoadMeta(comment)
				if err != nil {
					logger.Warnf("invalid comment for table %q: %v", tableName, err)
					return err
				}
				utils.PutIntoK2Map(storeMeta.Metas, meta.Type, meta.Name, metaFromComment)
				tables = append(tables, tableName)
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	}, sql, s.database, s.buildTableNameLike())
	if err != nil {
		logger.Errore(err, "list table failed")
		return nil, fmt.Errorf("list table failed: %w", err)
	}

	sort.Strings(tables)
	logger.InfoIfF(time.Since(startTime) > time.Millisecond*500,
		"load all meta, tables: %v, hash: %s, used: %s", tables, func() string {
			return storeMeta.GetHash()
		}, time.Since(startTime).String())
	logger.DebugEveryN(100, "load meta debug, hash: %s, meta: %s",
		storeMeta.GetHash(), storeMeta.String())
	return storeMeta, nil
}

// fetchMetas is a heavy operation, only used in initialization or under ReloadMeta()
func (s *Store) fetchMetas(ctx context.Context, overWriteMeta bool) error {
	startTime := time.Now()
	_, logger := log.FromContext(ctx, "processorID", s.processorID)
	metas := make(map[string]timeseries.Meta) // key is tableName
	comments := make(map[string]string)       // key is meta FullName

	// show tables
	sql := "SELECT name, comment FROM system.tables WHERE database = ? AND name LIKE ?"
	err := queryAndScan(ctx, s.client, func(rows driver.Rows) error {
		for rows.Next() {
			var tableName string
			var comment string
			if scanErr := rows.Scan(&tableName, &comment); scanErr != nil {
				return scanErr
			}
			var meta timeseries.Meta
			meta.Type, meta.Name = s.cutTableName(tableName)
			meta.Fields = make(map[string]timeseries.Field)
			if len(meta.Type) == 0 || len(meta.Name) == 0 {
				logger.Warnf("got invalid table name %q", tableName)
				continue
			}
			if !timeseries.IsValidMetaType(meta.Type) {
				continue
			}
			if comment != "" {
				metaFromComment, err := timeseries.LoadMeta(comment)
				if err != nil {
					logger.Warnf("invalid comment for table %q: %v", tableName, err)
					continue
				}
				comments[meta.GetFullName()] = comment
				meta.Aggregation = metaFromComment.Aggregation
				meta.Fields = metaFromComment.Fields
			}
			metas[tableName] = meta
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	}, sql, s.database, s.buildTableNameLike())
	if err != nil {
		logger.Errore(err, "list table failed")
		return fmt.Errorf("list table failed: %w", err)
	}

	// get index
	indices := make(map[string]map[string]string) // map[tableName][fieldName] = indexName
	sql = "SELECT table, name, expr FROM system.data_skipping_indices WHERE database = ? AND table like ?"
	err = queryAndScan(ctx, s.client, func(rows driver.Rows) error {
		for rows.Next() {
			var tableName, indexName, indexExpr string
			if scanErr := rows.Scan(&tableName, &indexName, &indexExpr); scanErr != nil {
				return scanErr
			}
			utils.PutIntoK2Map(indices, tableName, indexExpr, indexName)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	}, sql, s.database, s.buildTableNameLike())
	if err != nil {
		logger.Errore(err, "list indices failed")
		return fmt.Errorf("list indices failed: %w", err)
	}

	// get table fields
	sql = "SELECT table, name, type, comment FROM system.columns WHERE database = ? AND table LIKE ?"
	err = queryAndScan(ctx, s.client, func(rows driver.Rows) error {
		for rows.Next() {
			var tableName, fieldName, fieldType, fieldComment string
			if scanErr := rows.Scan(&tableName, &fieldName, &fieldType, &fieldComment); scanErr != nil {
				return scanErr
			}
			if _, has := metas[tableName]; has {
				indexName, _ := utils.GetFromK2Map(indices, tableName, fieldName)
				field, buildFieldErr := s.buildField(fieldName, fieldType, fieldComment, indexName)
				if buildFieldErr != nil {
					return buildFieldErr
				}
				loadedField, ok := metas[tableName].Fields[field.Name]
				if ok {
					if newField, changed := field.Merge(loadedField); changed {
						metas[tableName].Fields[loadedField.Name] = newField
					}
				} else {
					metas[tableName].Fields[field.Name] = field
				}
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	}, sql, s.database, s.buildTableNameLike())
	if err != nil {
		logger.Errore(err, "list table fields failed")
		return fmt.Errorf("list table fields failed: %w", err)
	}

	// update s.metas
	s.meta.Metas = make(map[timeseries.MetaType]map[string]timeseries.Meta)
	for _, meta := range metas {
		meta.HashData = meta.CalculateHash()
		utils.PutIntoK2Map(s.meta.Metas, meta.Type, meta.Name, meta)
		if overWriteMeta && string(meta.Dump()) != comments[meta.GetFullName()] {
			if err := s.alterCommentForTable(ctx, s.buildTableName(meta), meta); err != nil {
				logger.Warnw("alter comment for table failed", "meta", meta.GetFullName(), "err", err)
			}
		}
	}

	logger.InfoIfF(time.Since(startTime) > time.Millisecond*500,
		"fetch all meta, tables: %v, hash: %s, used: %s", utils.GetOrderedMapKeys(metas), func() string {
			return s.meta.GetHash()
		}, time.Since(startTime).String())
	logger.DebugEveryN(100, "fetch meta debug, hash: %s, meta: %s", s.meta.GetHash(), s.meta.String())
	return nil
}

func (s *Store) syncMeta(ctx context.Context, data timeseries.Dataset) error {
	meta := data.Meta
	_, logger := log.FromContext(ctx, "processorID", s.processorID, "meta", meta.GetFullName())
	startTime := time.Now()
	if err := meta.Verify(); err != nil {
		logger.Errore(err, "invalid meta")
		return err
	}

	preMeta, has := utils.GetFromK2Map(s.meta.Metas, meta.Type, meta.Name)
	if !has {
		logger.Debug("will create table")
		err := s.CreateTable(ctx, meta)
		logger = logger.With("used", time.Since(startTime).String())
		if err != nil {
			logger.Errore(err, "create table failed")
			return fmt.Errorf("create table for %s failed: %w", meta.GetFullName(), err)
		}
		logger.Info("table created")
		utils.PutIntoK2Map(s.meta.Metas, meta.Type, meta.Name, meta)
		return nil
	}

	if !preMeta.Aggregation.IsSame(meta.Aggregation) {
		logger.Errorw("unacceptable aggregation change", "before", preMeta.Aggregation, "after", meta.Aggregation)
		return fmt.Errorf("%w: unacceptable aggregation change for table %s",
			timeseries.ErrInvalidMetaDiff, meta.GetFullName())
	}

	diff := preMeta.DiffFields(meta)
	var addFields, alterSchema bool
	switch {
	case len(diff.UpdFields) > 0:
		logger.Errorw("unacceptable table fields change", "diff", diff.UpdFields)
		var sampleText string
		if len(data.Rows) > 0 {
			sampleText = fmt.Sprintf("\nsample data: %v", data.Rows[0])
		}
		return fmt.Errorf("%w: unacceptable field update for table %s:\n%s%s", timeseries.ErrInvalidMetaDiff,
			meta.GetFullName(),
			timeseries.GetFieldsDiffSummary(diff.UpdFields, "\t", "\n"),
			sampleText)
	case len(diff.AddFields) > 0:
		addFields = true
		alterSchema = true
	case len(diff.UpdSchema) > 0:
		alterSchema = true
	case len(diff.DelFields) > 0:
		// do nothing for now
	}
	if addFields {
		logger = logger.With("newFields", diff.AddFields, "hash", preMeta.Hash())
		logger.Debug("table will add fields")
		err := s.addFieldsForTable(ctx, s.buildTableName(meta), diff.AddFields)
		logger = logger.With("used", time.Since(startTime).String())
		if err != nil {
			logger.Errore(err, "add fields for table failed")
			return fmt.Errorf("add fields for table %s failed: %w", meta.GetFullName(), err)
		}
		logger.Infow("table added fields")
	}
	if alterSchema {
		logger = logger.With("updateSchema", diff.UpdSchema, "hash", preMeta.Hash())
		metaMergeStart := time.Now()
		meta = preMeta.Merge(meta)
		meta.HashData = meta.CalculateHash()
		if err := s.alterCommentForTable(ctx, s.buildTableName(meta), meta); err != nil {
			logger.Errore(err, "alter comment for table failed")
			return fmt.Errorf("alter comment for table %s failed: %w", meta.GetFullName(), err)
		}
		logger = logger.With("used", time.Since(metaMergeStart).String(), "newHash", meta.Hash())
		logger.Info("table updated schema")
		utils.PutIntoK2Map(s.meta.Metas, meta.Type, meta.Name, meta)
	}
	return nil
}

func (s *Store) syncMetas(ctx context.Context, data []timeseries.Dataset) error {
	for _, set := range data {
		if err := s.syncMeta(ctx, set); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CleanAll(ctx context.Context) error {
	s.meta.Lock()
	defer s.meta.Unlock()

	for _, metas := range s.meta.Metas {
		for _, meta := range metas {
			sql := fmt.Sprintf("DROP TABLE IF EXISTS %s %s", s.buildTableName(meta), s.sqlOnClusterPart())
			start := time.Now()
			err := s.client.Exec(ctx, sql)
			_, logger := log.FromContext(ctx,
				"processorID", s.processorID,
				"meta", meta.GetFullName(),
				"sql", sql,
				"used", time.Since(start).String())
			if err != nil {
				logger.Errore(err, "drop table failed")
				return err
			}
			logger.Info("table dropped")
		}
	}
	s.meta.Metas = make(map[timeseries.MetaType]map[string]timeseries.Meta)
	return nil
}

func (s *Store) Meta() timeseries.StoreMeta {
	var meta storeMeta
	if err := copier.CopyWithOption(&meta, s.meta, copier.Option{
		DeepCopy: true,
	}); err != nil {
		log.With("processorID", s.processorID).Errore(err, "copy meta failed")
	}
	return &meta
}

// ReloadMeta used to reload meta from storage
// if force is true, it will ignore the hash compare, force to fetch all meta.
// or it will use hash to compare, if the meta is changed, it will start to fetch.
func (s *Store) ReloadMeta(ctx context.Context, force bool) error {
	_, logger := log.FromContext(ctx, "processorID", s.processorID, "hash", s.meta.GetHash())
	var (
		loadErr error
		newMeta timeseries.StoreMeta
	)
	if !force {
		newMeta, loadErr = s.loadMeta(ctx)
		switch {
		case loadErr != nil:
			logger.Warnw("load meta failed", "err", loadErr)
		case newMeta == nil:
			logger.InfoEveryNw(10, "no meta found")
		case !s.meta.Different(newMeta):
			logger.InfoEveryNw(10, "meta not changed")
			return nil
		default:
			logger = logger.With("newHash", newMeta.GetHash())
			logger.Infow("meta changed")
		}
	}

	s.meta.Lock()
	defer s.meta.Unlock()
	if err := s.fetchMetas(ctx, loadErr != nil); err != nil {
		logger.Warnf("fetch metas in reloading, err: %v", err)
		return err
	}
	logger.Debugw("meta reloaded")
	return nil
}
