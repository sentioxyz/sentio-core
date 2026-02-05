package clickhouse

import (
	"bytes"
	"context"
	"fmt"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/cmstr"
	"sentioxyz/sentio-core/common/format"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	"sentioxyz/sentio-core/driver/entity/schema/interval"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go/types"
)

// type mapping table
// |-------------|------------|-------------------|-----------------|------------------------------------------|
// | schema type | js type    | wasm type         | go type         | clickhouse type                          |
// |-------------|------------|-------------------|-----------------|------------------------------------------|
// | ID          | String     | wasm.String       | string          | String                                   |
// | Bytes       | Bytes      | wasm.ByteArray    | string          | String (hex)                             |
// | String      | String     | wasm.String       | string          | String                                   |
// | Boolean     | boolean    | wasm.Bool         | boolean         | Bool                                     |
// | Int         | i32        | wasm.I32          | int32           | Int32                                    |
// | Int8        | i64        | wasm.I64          | int64           | Int64                                    |
// | Timestamp   | i64        | wasm.I64          | int64           | Int64 (TimestampMicro) || DateTime64(6)  |
// | Float       |            |                   | float64         | Float64                                  |
// | BigInt      | BigInt     | common.BigInt     | big.Int         | Tuple(Bool,Int8,UInt256) || Int256       |
// | BigDecimal  | BigDecimal | common.BigDecimal | decimal.Decimal | Decimal256(30) || String                 |
// | Enum        | String     | wasm.String       | string          | Enum                                     |
// | List        | Array      | wasm.ObjectArray  | []any           | String || Array                          |
// |-------------|------------|-------------------|-----------------|------------------------------------------|
// for foreign key field, List always use Array as the db field type
//
// see: https://thegraph.com/docs/en/subgraphs/developing/creating/ql-schema/#built-in-scalar-types
//      https://clickhouse.com/docs/en/sql-reference/data-types/decimal

const (
	genBlockNumberFieldName     = "__genBlockNumber__"
	genBlockNumberFieldType     = "UInt64"
	genBlockNumberViewFieldName = "meta.block_number"

	genBlockTimeFieldName     = "__genBlockTime__"
	genBlockTimeFieldType     = "DateTime64(6, 'UTC')"
	genBlockTimeViewFieldName = "meta.block_time"

	genBlockHashFieldName     = "__genBlockHash__"
	genBlockHashFieldType     = "String"
	genBlockHashViewFieldName = "meta.block_hash"

	genBlockChainFieldName     = "__genBlockChain__"
	genBlockChainFieldType     = "String"
	genBlockChainViewFieldName = "meta.chain"

	deletedFieldName     = "__deleted__"
	deletedFieldType     = "Bool"
	deletedViewFieldName = "meta.deleted"

	signFieldName = "__sign__"
	signFieldType = "Int8"

	versionFieldName = "__version__"
	versionFieldType = "UInt64"

	timestampFieldName     = "__timestamp__"
	timestampFieldType     = "DateTime64(3, 'UTC')"
	timestampViewFieldName = "meta.timestamp"

	implEntityFieldName     = "__implEntity__"
	implEntityFieldType     = "String"
	implEntityViewFieldName = "meta.impl_entity"

	aggIntervalFieldName     = "__interval__"
	aggIntervalViewFieldName = "meta.aggregation_interval"
)

func (s *Store) loadExists(ctx context.Context, categories []string) (map[string]map[string]chx.TableOrView, error) {
	loaded, err := s.ctrl.Load(ctx, s.database, s.processorID+"\\_%")
	if err != nil {
		return nil, err
	}
	valid := set.New(categories...)
	tvs := make(map[string]map[string]chx.TableOrView)
	for tableName, tv := range loaded {
		category, entityName, _ := strings.Cut(strings.TrimPrefix(tableName, s.processorID+"_"), "_")
		if valid.Contains(category) {
			utils.PutIntoK2Map(tvs, entityName, tableName, tv)
		}
	}
	return tvs, nil
}

func (s *Store) syncTablesAndViews(ctx context.Context, viewOnly bool) (err error) {
	startAt := time.Now()
	_, logger := log.FromContext(ctx, "processorID", s.processorID, "viewOnly", viewOnly)
	logger.Info("will sync tables and views from subgraph schema")
	defer func() {
		logger = logger.With("used", time.Since(startAt).String())
		if err != nil {
			logger.Errorfe(err, "sync tables and views from subgraph schema failed")
		} else {
			logger.Infof("all tables and views for the subgraph schema are ready now")
		}
	}()

	// load exists
	var categories = []string{
		"versionedEntity",
		"versionedLatestEntity",
		"versionedLatestEntityMV",
		"entity",
		"interface",
		"aggregation",
		"view",
		"latestView",
	}
	if viewOnly {
		categories = []string{"view", "latestView"}
	}
	var exists map[string]map[string]chx.TableOrView
	if exists, err = s.loadExists(ctx, categories); err != nil {
		return err
	}

	// sync
	expects := s.buildTablesAndViews(viewOnly)
	for _, item := range s.sch.ListEntitiesAndInterfacesAndAggregations(false) {
		for _, tv := range expects[item.GetName()] {
			pre, has := utils.GetFromK2Map(exists, item.GetName(), tv.GetFullName().Name)
			if !has {
				if err = s.ctrl.Create(ctx, tv); err != nil {
					return err
				}
			} else {
				var kvs cmstr.KVS
				_ = kvs.Load(pre.GetComment())
				if sh, _ := kvs.Get("SCHEMA_HASH"); sh != s.schHash {
					// schema changed, try to sync
					if err = s.ctrl.Sync(ctx, pre, tv); err != nil {
						return err
					}
				}
				utils.DelFromK2Map(exists, item.GetName(), tv.GetFullName().Name)
			}
		}
	}
	return utils.TravelK2Map(exists, func(entityName string, tableName string, tv chx.TableOrView) error {
		return s.ctrl.Drop(ctx, tv)
	})
}

func (s *Store) InitEntitySchema(ctx context.Context) error {
	return s.syncTablesAndViews(ctx, false)
}

func (s *Store) CreateViews(ctx context.Context) error {
	return s.syncTablesAndViews(ctx, true)
}

func (s *Store) getViewFields(item schema.EntityOrInterface, latestView bool) []ViewField {
	fields := s.getViewClickhouseFields(item)
	buildSystemViewField := func(tableFieldName, viewFieldName, fieldType string) []ViewField {
		return []ViewField{{
			Field:     chx.Field{Name: viewFieldName, Type: fieldType},
			SelectSQL: fmt.Sprintf("`%s` AS `%s`", tableFieldName, viewFieldName),
		}, {
			Field:     chx.Field{Name: tableFieldName, Type: fieldType},
			SelectSQL: fmt.Sprintf("`%s`", tableFieldName),
		}}
	}
	if latestView {
		fields = append(fields,
			buildSystemViewField(genBlockChainFieldName, genBlockChainViewFieldName, genBlockChainFieldType)...,
		)
	} else {
		fields = utils.MergeArr(fields,
			buildSystemViewField(genBlockNumberFieldName, genBlockNumberViewFieldName, genBlockNumberFieldType),
			buildSystemViewField(genBlockTimeFieldName, genBlockTimeViewFieldName, genBlockTimeFieldType),
			buildSystemViewField(genBlockHashFieldName, genBlockHashViewFieldName, genBlockHashFieldType),
			buildSystemViewField(genBlockChainFieldName, genBlockChainViewFieldName, genBlockChainFieldType),
			buildSystemViewField(deletedFieldName, deletedViewFieldName, deletedFieldType),
			buildSystemViewField(timestampFieldName, timestampViewFieldName, timestampFieldType),
		)
	}
	if _, is := item.(*schema.Interface); is {
		fields = append(fields,
			buildSystemViewField(implEntityFieldName, implEntityViewFieldName, implEntityFieldType)...)
	}
	if aggType, is := item.(*schema.Aggregation); is {
		fields = append(fields,
			ViewField{
				Field: SimpleField{BaseField: NewBaseField(nil, &types.FieldDefinition{
					Name: aggIntervalViewFieldName,
					Type: interval.BuildEnumType(aggType.Name, aggType.GetIntervals()),
				})}.GetClickhouseFields()[0],
				SelectSQL: fmt.Sprintf("`%s` AS `%s`", aggIntervalFieldName, aggIntervalViewFieldName),
			},
			ViewField{
				Field: SimpleField{BaseField: NewBaseField(nil, &types.FieldDefinition{
					Name: aggIntervalFieldName,
					Type: interval.BuildEnumType(aggType.Name, aggType.GetIntervals()),
				})}.GetClickhouseFields()[0],
				SelectSQL: fmt.Sprintf("`%s`", aggIntervalFieldName),
			})
	}
	return fields
}

func (s *Store) GetViewFields(item schema.EntityOrInterface) []ViewField {
	return s.getViewFields(item, false)
}

func (s *Store) GetLatestViewFields(item schema.EntityOrInterface) []ViewField {
	return s.getViewFields(item, true)
}

// each entity type may have 10 table or view:
// - <ProcessorID>_versionedEntity_<EntityName>
// - <ProcessorID>_versionedLatestEntity_<EntityName>
// - <ProcessorID>_versionedLatestEntityMV_<EntityName>
// - <ProcessorID>_entity_<EntityName>
// - <ProcessorID>_interface_<EntityName>
// - <ProcessorID>_view_<EntityName>
// - <ProcessorID>_latestView_<EntityName>
// - <ProcessorID>_aggregation_<AggregationName>
// - <ProcessorID>_view_<AggregationName>
// - <ProcessorID>_latestView_<AggregationName>
//
// the meaning of them are:
// - 'versionedEntity'         has all changes, and all history changes has opposite row
// - 'versionedLatestEntity'   data format is as same as 'versionedEntity' but history changes may be collapsed
// - 'versionedLatestEntityMV' sync all insert from 'versionedEntity' to 'versionedLatestEntity'
// - 'entity'|'aggregation'    has all changes
// - 'interface'               is the union of 'entity'
// - 'view'                    has all changes, but fields will be transformed
// - 'latestView'              has only latest changes for each entity id, and fields will be transformed.
//                             if target is a interface, it is the union of 'latestView' of implemented entities,
//                             or it is a view of 'entity' or 'versionedLatestEntity' that ignored all history changes
//
// disable VersionedCollapsing:
// - 'versionedEntity' and 'versionedLatestEntity' and 'versionedLatestEntityMV' will not exist,
// - 'entity' will be a table
// - others will be view or union
// enable VersionedCollapsing:
// - 'versionedEntity' and 'versionedLatestEntity' will be tables
// - others will be view or union
//
//
// entity disabled VersionedCollapsing:
//
//    entity ---> view ---> latestView
//
// entity enable VersionedCollapsing:
//
//    versionedEntity --(versionedLatestEntityMV)--> versionedLatestEntity --> latestView
//                   \
//                    +--> entity --> view
//
// interface:
//
//    [entity] --------------> [latestView]
//            \                            \
//             +--> interface --> view      +--> latestView
//            /                            /
//    [entity] --------------> [latestView]
//
// aggregation:
//
//    aggregation ---> view ---> latestView
//

func (s *Store) UseVersionedCollapsingTable(item schema.EntityOrInterface) bool {
	entityType, is := item.(*schema.Entity)
	return s.feaOpt.VersionedCollapsing && is && !entityType.IsImmutable()
}

func (s *Store) buildTablesAndViews(viewOnly bool) (result map[string][]chx.TableOrView) {
	result = make(map[string][]chx.TableOrView)
	for _, item := range s.sch.ListEntitiesAndInterfacesAndAggregations(false) {
		if !viewOnly {
			// origin data
			switch itemType := item.(type) {
			case *schema.Entity:
				if itemType.IsCache() {
					continue
				}
				if s.UseVersionedCollapsingTable(itemType) {
					result[item.GetName()] = append(result[item.GetName()],
						// [TABLE] versionedEntity
						s.buildVersionedEntityTable(itemType),
						// [TABLE] versionedLatestEntity
						s.buildVersionedLatestEntityTable(itemType),
						// [MV] versionedLatestEntityMV
						//    versionedEntity --(versionedLatestEntityMV)--> versionedLatestEntity
						s.buildVersionedLatestEntityMaterializedView(itemType),
						// [VIEW]  versionedEntity --> entity
						s.buildEntityView(itemType),
					)
				} else {
					// [TABLE] entity
					result[item.GetName()] = append(result[item.GetName()], s.buildEntityTable(itemType))
				}
			case *schema.Interface:
				// [VIEW] interface
				//    [entity]
				//            \
				//             +--> interface
				//            /
				//    [entity]
				result[item.GetName()] = append(result[item.GetName()], s.buildInterfaceView(itemType))
			case *schema.Aggregation:
				// [TABLE] aggregation
				result[item.GetName()] = append(result[item.GetName()], s.buildEntityTable(itemType))
			}
		}
		// [VIEW] view
		//    entity|interface|aggregation --> view
		result[item.GetName()] = append(result[item.GetName()], s.buildView(item))
		switch itemType := item.(type) {
		case *schema.Entity:
			// [VIEW] latestView
			//    versionedLatestEntity|view --> latestView
			result[item.GetName()] = append(result[item.GetName()], s.buildLatestView(itemType))
		case *schema.Interface:
			// [view] latestView
			//    [latestView]
			//                \
			//                 +--> latestView
			//                /
			//    [latestView]
			result[item.GetName()] = append(result[item.GetName()], s.buildInterfaceLatestView(itemType))
		case *schema.Aggregation:
			// [view] latestView
			//    view --> latestView
			result[item.GetName()] = append(result[item.GetName()], s.buildAggregationLatestView(itemType))
		}
	}
	return
}

func (s *Store) getClickhouseIndexes(entityType schema.EntityOrInterface) []chx.Index {
	return utils.MapAndMergeNoError(s.NewEntity(entityType).Fields, func(f Field) []chx.Index {
		return f.GetClickhouseIndexes()
	})
}

func (s *Store) getClickhouseFields(item schema.EntityOrInterface) []chx.Field {
	return utils.MapAndMergeNoError(s.NewEntity(item).Fields, func(f Field) []chx.Field {
		return f.GetClickhouseFields()
	})
}

func (s *Store) getViewClickhouseFields(item schema.EntityOrInterface) (fields []ViewField) {
	for _, f := range s.NewEntity(item).Fields {
		fields = append(fields, f.GetViewClickhouseFields()...)
	}
	return fields
}

func wrapAnyAs(fields []string) []string {
	return utils.MapSliceNoError(fields, func(f string) string {
		return fmt.Sprintf("any_respect_nulls(%s) AS %s", quote(f), quote(f))
	})
}

func (s *Store) TableOrViewComment(entityType schema.EntityOrInterface) string {
	var comments cmstr.KVS
	switch typ := entityType.(type) {
	case *schema.Entity:
		for _, iface := range typ.GetInterfaces() {
			comments.Add("IMPL", iface.Name)
		}
	case *schema.Aggregation:
		comments.Add("SRC", typ.GetSource())
	}
	comments.Add("SCHEMA_HASH", s.schHash)
	return comments.String()
}

// <ProcessorID>_versionedEntity_<EntityName>
func (s *Store) buildVersionedEntityTable(entityType *schema.Entity) chx.Table {
	return chx.Table{
		FullName: chx.FullName{
			Database: s.database,
			Name:     s.VersionedTableName(entityType),
		},
		Config: chx.TableConfig{
			Engine:      chx.NewDefaultMergeTreeEngine(s.ctrl.GetCluster() != ""),
			PartitionBy: genBlockChainFieldName,
			OrderBy: []string{
				genBlockChainFieldName,
				schema.EntityPrimaryFieldName,
				versionFieldName,
				genBlockNumberFieldName,
			},
			Settings: s.tableOpt.TableSettings,
		},
		Comment: s.TableOrViewComment(entityType),
		Fields: append(
			s.getClickhouseFields(entityType),
			chx.Field{Name: genBlockNumberFieldName, Type: genBlockNumberFieldType},
			chx.Field{Name: genBlockTimeFieldName, Type: genBlockTimeFieldType},
			chx.Field{Name: genBlockHashFieldName, Type: genBlockHashFieldType},
			chx.Field{Name: genBlockChainFieldName, Type: genBlockChainFieldType},
			chx.Field{Name: deletedFieldName, Type: deletedFieldType},
			chx.Field{Name: timestampFieldName, Type: timestampFieldType, DefaultExpr: "now()"},
			chx.Field{Name: signFieldName, Type: signFieldType},
			chx.Field{Name: versionFieldName, Type: versionFieldType},
		),
		Indexes: s.getClickhouseIndexes(entityType),
	}
}

// <ProcessorID>_versionedLatestEntity_<EntityName>
func (s *Store) buildVersionedLatestEntityTable(entityType *schema.Entity) chx.Table {
	return chx.Table{
		FullName: chx.FullName{
			Database: s.database,
			Name:     s.VersionedLatestTableName(entityType),
		},
		Config: chx.TableConfig{
			Engine: chx.NewDefaultVersionedCollapsingMergeTreeEngine(
				s.ctrl.GetCluster() != "",
				signFieldName,
				versionFieldName,
			),
			PartitionBy: genBlockChainFieldName,
			OrderBy:     []string{genBlockChainFieldName, schema.EntityPrimaryFieldName},
			Settings:    s.tableOpt.TableSettings,
		},
		Comment: s.TableOrViewComment(entityType),
		Fields: append(
			s.getClickhouseFields(entityType),
			chx.Field{Name: genBlockNumberFieldName, Type: genBlockNumberFieldType},
			chx.Field{Name: genBlockTimeFieldName, Type: genBlockTimeFieldType},
			chx.Field{Name: genBlockHashFieldName, Type: genBlockHashFieldType},
			chx.Field{Name: genBlockChainFieldName, Type: genBlockChainFieldType},
			chx.Field{Name: deletedFieldName, Type: deletedFieldType},
			chx.Field{Name: timestampFieldName, Type: timestampFieldType, DefaultExpr: "now()"},
			chx.Field{Name: signFieldName, Type: signFieldType},
			chx.Field{Name: versionFieldName, Type: versionFieldType},
		),
		Indexes: s.getClickhouseIndexes(entityType),
	}
}

// <ProcessorID>_versionedLatestEntityMV_<EntityName>
// used to sync all insert from versionedEntity to versionedLatestEntity
func (s *Store) buildVersionedLatestEntityMaterializedView(entityType *schema.Entity) chx.MaterializedView {
	return chx.MaterializedView{
		View: chx.View{
			FullName: chx.FullName{
				Database: s.database,
				Name:     s.VersionedLatestTableMaterializedViewName(entityType),
			},
			Fields: append(s.getClickhouseFields(entityType),
				chx.Field{Name: genBlockNumberFieldName, Type: genBlockNumberFieldType},
				chx.Field{Name: genBlockTimeFieldName, Type: genBlockTimeFieldType},
				chx.Field{Name: genBlockHashFieldName, Type: genBlockHashFieldType},
				chx.Field{Name: genBlockChainFieldName, Type: genBlockChainFieldType},
				chx.Field{Name: deletedFieldName, Type: deletedFieldType},
				chx.Field{Name: timestampFieldName, Type: timestampFieldType},
				chx.Field{Name: signFieldName, Type: signFieldType},
				chx.Field{Name: versionFieldName, Type: versionFieldType},
			),
			Select:  "SELECT * FROM " + s.fullName(s.VersionedTableName(entityType)),
			Comment: s.TableOrViewComment(entityType),
		},
		To: chx.FullName{
			Database: s.database,
			Name:     s.VersionedLatestTableName(entityType),
		},
	}
}

// <ProcessorID>_entity_<EntityName>
// view of <ProcessorID>_versionedEntity_<EntityName> that ignored all opposite rows
func (s *Store) buildEntityView(entityType *schema.Entity) chx.View {
	return chx.View{
		FullName: chx.FullName{
			Database: s.database,
			Name:     s.TableName(entityType),
		},
		Fields: append(s.getClickhouseFields(entityType),
			chx.Field{Name: genBlockNumberFieldName, Type: genBlockNumberFieldType},
			chx.Field{Name: genBlockTimeFieldName, Type: genBlockTimeFieldType},
			chx.Field{Name: genBlockHashFieldName, Type: genBlockHashFieldType},
			chx.Field{Name: genBlockChainFieldName, Type: genBlockChainFieldType},
			chx.Field{Name: deletedFieldName, Type: deletedFieldType},
			chx.Field{Name: timestampFieldName, Type: timestampFieldType},
		),
		Select: fmt.Sprintf("SELECT %s FROM %s WHERE %s > 0",
			joinWithQuote(append(s.NewEntity(entityType).fieldNames(false),
				genBlockNumberFieldName,
				genBlockTimeFieldName,
				genBlockHashFieldName,
				genBlockChainFieldName,
				deletedFieldName,
				timestampFieldName), ", "),
			s.fullName(s.VersionedTableName(entityType)),
			signFieldName,
		),
		Comment: s.TableOrViewComment(entityType),
	}
}

// <ProcessorID>_entity_<EntityName>
func (s *Store) buildEntityTable(entityType schema.EntityOrInterface) chx.Table {
	systemFields := []chx.Field{
		{Name: genBlockNumberFieldName, Type: genBlockNumberFieldType},
		{Name: genBlockTimeFieldName, Type: genBlockTimeFieldType},
		{Name: genBlockHashFieldName, Type: genBlockHashFieldType},
		{Name: genBlockChainFieldName, Type: genBlockChainFieldType},
		{Name: deletedFieldName, Type: deletedFieldType},
		{Name: timestampFieldName, Type: timestampFieldType, DefaultExpr: "now()"},
	}
	orderBy := []string{genBlockChainFieldName, schema.EntityPrimaryFieldName, genBlockNumberFieldName}
	if aggType, is := entityType.(*schema.Aggregation); is {
		field := SimpleField{BaseField: NewBaseField(nil, &types.FieldDefinition{
			Name: aggIntervalFieldName,
			Type: interval.BuildEnumType(aggType.Name, aggType.GetIntervals()),
		})}
		systemFields = append(systemFields, field.GetClickhouseFields()...)
		orderBy = []string{genBlockChainFieldName, aggIntervalFieldName, schema.EntityTimestampFieldName}
	}
	return chx.Table{
		FullName: chx.FullName{
			Database: s.database,
			Name:     s.TableName(entityType),
		},
		Config: chx.TableConfig{
			Engine:      chx.NewDefaultMergeTreeEngine(s.ctrl.GetCluster() != ""),
			PartitionBy: genBlockChainFieldName,
			OrderBy:     orderBy,
			Settings:    s.tableOpt.TableSettings,
		},
		Comment: s.TableOrViewComment(entityType),
		Fields:  append(s.getClickhouseFields(entityType), systemFields...),
		Indexes: s.getClickhouseIndexes(entityType),
	}
}

// <ProcessorID>_interface_<EntityName>
// union of <ProcessorID>_entity_<EntityName> of each implemented entity type
func (s *Store) buildInterfaceView(ifaceType *schema.Interface) chx.View {
	selectFields := append(s.NewEntity(ifaceType).fieldNames(false),
		genBlockNumberFieldName,
		genBlockTimeFieldName,
		genBlockHashFieldName,
		genBlockChainFieldName,
		deletedFieldName,
		timestampFieldName)
	var entitySelects []string
	for _, entityType := range ifaceType.ListEntities() {
		entitySelects = append(entitySelects, fmt.Sprintf("SELECT %s, %s FROM %s",
			joinWithQuote(selectFields, ", "),
			fmt.Sprintf("'%s' AS %s", entityType.Name, quote(implEntityFieldName)),
			s.fullName(s.TableName(entityType)),
		))
	}
	return chx.View{
		FullName: chx.FullName{
			Database: s.database,
			Name:     s.TableName(ifaceType),
		},
		Fields: append(s.getClickhouseFields(ifaceType),
			chx.Field{Name: genBlockNumberFieldName, Type: genBlockNumberFieldType},
			chx.Field{Name: genBlockTimeFieldName, Type: genBlockTimeFieldType},
			chx.Field{Name: genBlockHashFieldName, Type: genBlockHashFieldType},
			chx.Field{Name: genBlockChainFieldName, Type: genBlockChainFieldType},
			chx.Field{Name: deletedFieldName, Type: deletedFieldType},
			chx.Field{Name: timestampFieldName, Type: timestampFieldType},
			chx.Field{Name: implEntityFieldName, Type: implEntityFieldType},
		),
		Select:  strings.Join(entitySelects, " UNION ALL "),
		Comment: s.TableOrViewComment(ifaceType),
	}
}

// <ProcessorID>_view_<EntityName>
// include all change history and transformed the fields
func (s *Store) buildView(item schema.EntityOrInterface) chx.View {
	viewFields := s.GetViewFields(item)
	selects := utils.MapSliceNoError(viewFields, ViewField.GetSelectSQL)
	return chx.View{
		FullName: chx.FullName{
			Database: s.database,
			Name:     s.ViewName(item),
		},
		Fields:  utils.MapSliceNoError(viewFields, ViewField.GetField),
		Select:  fmt.Sprintf("SELECT %s FROM %s", strings.Join(selects, ", "), s.fullName(s.TableName(item))),
		Comment: s.TableOrViewComment(item),
	}
}

// <ProcessorID>_latestView_<EntityName>
func (s *Store) buildLatestView(entityType *schema.Entity) chx.View {
	viewFields := s.GetLatestViewFields(entityType)
	view := chx.View{
		FullName: chx.FullName{
			Database: s.database,
			Name:     s.LatestViewName(entityType),
		},
		Fields:  utils.MapSliceNoError(viewFields, ViewField.GetField),
		Comment: s.TableOrViewComment(entityType),
	}
	if s.UseVersionedCollapsingTable(entityType) {
		// Build Select SQL
		// ----------------------------------------
		// SELECT id, __genBlockChain__, any_respect_nulls(propA) AS propA
		// FROM versionedLatestEntity
		// WHERE NOT __deleted__
		// GROUP by id, __genBlockChain__, __version__
		// HAVING SUM(__sign__) > 0
		var selectFields []string
		for _, field := range s.NewEntity(entityType).Fields {
			if field.Name() != schema.EntityPrimaryFieldName {
				selectFields = append(selectFields, field.FieldNames()...)
			}
		}
		selectFields = wrapAnyAs(selectFields)
		sql := format.Format("SELECT %pk#s, %gbc#s, %selectFields#s "+
			"FROM %vtable#s "+
			"WHERE NOT %deleted#s "+
			"GROUP BY %pk#s, %gbc#s, %version#s "+
			"HAVING SUM(%sign#s) > 0",
			map[string]any{
				"pk":           quote(schema.EntityPrimaryFieldName),
				"gbc":          quote(genBlockChainFieldName),
				"deleted":      quote(deletedFieldName),
				"sign":         quote(signFieldName),
				"version":      quote(versionFieldName),
				"vtable":       s.fullName(s.VersionedLatestTableName(entityType)),
				"selectFields": strings.Join(selectFields, ", "),
			})
		// should transform field type
		view.Select = fmt.Sprintf("SELECT %s FROM (%s)",
			strings.Join(utils.MapSliceNoError(viewFields, ViewField.GetSelectSQL), ", "),
			sql)
	} else if entityType.IsImmutable() {
		// Build Select SQL
		// ----------------------------------------
		// SELECT id, propA, __genBlockChain__ FROM entity
		view.Select = fmt.Sprintf("SELECT %s FROM %s",
			joinWithQuote(utils.MapSliceNoError(viewFields, ViewField.GetFieldName), ", "),
			s.fullName(s.ViewName(entityType)))
	} else {
		// Build Select SQL
		// ----------------------------------------
		// SELECT id, __genBlockChain__, __last__.3 AS propA
		// FROM (
		//   SELECT id, __genBlockChain__, MAX((__genBlockNumber__, __deleted__, propA)) as __last__
		//   FROM entity
		//   GROUP by id, __genBlockChain__
		// )
		// WHERE NOT __last__.2
		var lastAs bytes.Buffer
		lastFields := []string{genBlockNumberViewFieldName, deletedViewFieldName}
		for _, field := range s.NewEntity(entityType).Fields {
			if field.Name() == schema.EntityPrimaryFieldName {
				continue
			}
			for _, fieldName := range field.FieldNames() {
				lastFields = append(lastFields, fieldName)
				lastAs.WriteString(fmt.Sprintf(", __last__.%d AS %s", len(lastFields), quote(fieldName)))
			}
		}
		view.Select = format.Format("SELECT %pk#s, %gbc#s, %gbc#s AS %gbcRaw#s%lastAs#s "+
			"FROM ("+
			"SELECT %pk#s, %gbc#s, MAX((%last#s)) AS __last__ "+
			"FROM %ft#s "+
			"GROUP BY %pk#s, %gbc#s"+
			") "+
			"WHERE NOT __last__.2",
			map[string]any{
				"pk":     quote(schema.EntityPrimaryFieldName),
				"gbc":    quote(genBlockChainViewFieldName),
				"gbcRaw": quote(genBlockChainFieldName),
				"ft":     s.fullName(s.ViewName(entityType)),
				"last":   joinWithQuote(lastFields, ", "),
				"lastAs": lastAs.String(), // ignore last.1 and last.2
			})
	}
	return view
}

// <ProcessorID>_latestView_<EntityName>
// union of latestView of each implemented entity type
func (s *Store) buildInterfaceLatestView(ifaceType *schema.Interface) chx.View {
	selectFields := utils.MapSliceNoError(s.getViewClickhouseFields(ifaceType), ViewField.GetFieldName)
	selectFields = append(selectFields, genBlockChainViewFieldName, genBlockChainFieldName)
	var entitySelects []string
	for _, entityType := range ifaceType.ListEntities() {
		entitySelects = append(entitySelects, fmt.Sprintf("SELECT %s, %s, %s FROM %s",
			joinWithQuote(selectFields, ", "),
			fmt.Sprintf("'%s' AS %s", entityType.Name, quote(implEntityViewFieldName)),
			fmt.Sprintf("'%s' AS %s", entityType.Name, quote(implEntityFieldName)),
			s.fullName(s.LatestViewName(entityType)),
		))
	}
	return chx.View{
		FullName: chx.FullName{
			Database: s.database,
			Name:     s.LatestViewName(ifaceType),
		},
		Fields:  utils.MapSliceNoError(s.GetLatestViewFields(ifaceType), ViewField.GetField),
		Select:  strings.Join(entitySelects, " UNION ALL "),
		Comment: s.TableOrViewComment(ifaceType),
	}
}

// <ProcessorID>_latestView_<EntityName>
// has __interval__ field
func (s *Store) buildAggregationLatestView(agg *schema.Aggregation) chx.View {
	viewFields := s.GetLatestViewFields(agg)
	selects := utils.MapSliceNoError(viewFields, ViewField.GetFieldName)
	return chx.View{
		FullName: chx.FullName{
			Database: s.database,
			Name:     s.LatestViewName(agg),
		},
		Fields:  utils.MapSliceNoError(viewFields, ViewField.GetField),
		Select:  fmt.Sprintf("SELECT %s FROM %s", joinWithQuote(selects, ", "), s.fullName(s.ViewName(agg))),
		Comment: s.TableOrViewComment(agg),
	}
}
