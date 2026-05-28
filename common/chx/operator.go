package chx

import (
	"bytes"
	"context"
	"fmt"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
)

type EqualOrLike struct {
	Name     *string
	NameLike *string
}

func (c Controller) loadCondition(name EqualOrLike) (operator string, arg string) {
	if name.Name != nil {
		return "=", c.tableNamePrefix + *name.Name
	}
	tableNamePrefix := strings.ReplaceAll(c.tableNamePrefix, "_", "\\_")
	tableNamePrefix = strings.ReplaceAll(tableNamePrefix, "%", "\\%")
	if name.NameLike != nil {
		return "LIKE", tableNamePrefix + *name.NameLike
	}
	return "LIKE", tableNamePrefix + "%"
}

// result only contains Name, Comment
func (c Controller) loadSimple(ctx context.Context, name EqualOrLike) ([]TableOrView, error) {
	var tableOrViews []TableOrView
	operator, nameArg := c.loadCondition(name)
	sql := fmt.Sprintf("SELECT name, engine, comment "+
		"FROM system.tables "+
		"WHERE database = ? AND name %s ? AND is_temporary = 0", operator)
	err := c.Query(ctx, func(rows driver.Rows) error {
		var name, engine, comment string
		scanErr := rows.Scan(&name, &engine, &comment)
		if scanErr != nil {
			return scanErr
		}
		name = strings.TrimPrefix(name, c.tableNamePrefix)
		switch engine {
		case "View":
			tableOrViews = append(tableOrViews, View{
				Name:    name,
				Comment: comment,
			})
		case "MaterializedView":
			tableOrViews = append(tableOrViews, MaterializedView{
				View: View{
					Name:    name,
					Comment: comment,
				},
			})
		default:
			// TODO There might also be other elements (like View MaterializedView) that indicate this is not a regular table engine.
			tableOrViews = append(tableOrViews, Table{
				Name:    name,
				Comment: comment,
			})
		}
		return nil
	}, sql, c.database, nameArg)
	if err != nil {
		return nil, errors.Wrapf(err, "list table failed")
	}
	return tableOrViews, nil
}

func (c Controller) load(ctx context.Context, name EqualOrLike) (result []TableOrView, err error) {
	_, logger := log.FromContext(ctx)
	operator, nameArg := c.loadCondition(name)
	defer func() {
		if err != nil {
			logger.Errorfe(err, "load tables failed")
		}
	}()
	tables := make(map[string]TableOrView)

	// show tables
	sql := fmt.Sprintf(
		"SELECT name, engine, engine_full, create_table_query, as_select, partition_key, sorting_key, comment "+
			"FROM system.tables "+
			"WHERE database = ? AND name %s ? AND is_temporary = 0", operator)
	err = c.Query(ctx, func(rows driver.Rows) error {
		var rawName, engine, engineFull, createTableQuery, asSelect, partitionKey, sortingKey, comment string
		scanErr := rows.Scan(
			&rawName,
			&engine,
			&engineFull,
			&createTableQuery,
			&asSelect,
			&partitionKey,
			&sortingKey,
			&comment,
		)
		if scanErr != nil {
			return scanErr
		}
		name := strings.TrimPrefix(rawName, c.tableNamePrefix)
		switch engine {
		case "View":
			tables[name] = View{
				Name:    name,
				Select:  asSelect,
				Comment: comment,
			}
		case "MaterializedView":
			view := MaterializedView{
				View: View{
					Name:    name,
					Select:  asSelect,
					Comment: comment,
				},
			}
			var sector string
			for sector != "TO" && createTableQuery != "" {
				sector, createTableQuery, _ = cutBySpace(createTableQuery)
			}
			if sector == "TO" {
				// raw is FullLogicName, not FullName
				raw, _, _ := cutBySpace(createTableQuery)
				p := findNotIn(raw, '.', '`')
				db, table := strings.Trim(raw[:p], "`"), strings.Trim(raw[p+1:], "`")
				logicDatabase, logicTableNamePrefix := c.getLogicDatabase(), c.getLogicTableNamePrefix()
				if db != logicDatabase {
					return errors.Errorf("invalid To database %s for MaterializedView %s, should be %s",
						db, view.Name, logicDatabase)
				}
				if !strings.HasPrefix(table, logicTableNamePrefix) {
					return errors.Errorf("invalid To table %q for MaterializedView %s, should start with %q",
						table, view.Name, logicTableNamePrefix)
				}
				view.To = strings.TrimPrefix(table, logicTableNamePrefix)
			}
			tables[name] = view
		default:
			// TODO There might also be other elements (like View MaterializedView) that indicate this is not a regular table engine.
			table := Table{
				Name:    name,
				Comment: comment,
			}
			// PartitionBy
			table.Config.PartitionBy = partitionKey
			// OrderBy
			for _, key := range strings.Split(sortingKey, ",") {
				table.Config.OrderBy = append(table.Config.OrderBy, strings.Trim(strings.TrimSpace(key), "`"))
			}
			// Engine
			var engineWithArgs string
			engineWithArgs, engineFull, _ = cutBySpace(engineFull)
			table.Config.Engine, err = buildEngineFromString(engineWithArgs)
			if err != nil {
				logger.Warnfe(err, "build engine of table %s failed, will be ignored", rawName)
				return nil
			}
			// Settings
			var sector string
			for sector != "SETTINGS" && engineFull != "" {
				sector, engineFull, _ = cutBySpace(engineFull)
			}
			table.Config.Settings = make(map[string]string)
			if engineFull != "" {
				for _, kv := range strings.Split(engineFull, ",") {
					k, v, _ := strings.Cut(kv, "=")
					table.Config.Settings[strings.TrimSpace(k)] = strings.TrimSpace(v)
				}
			}
			// got a table
			tables[name] = table
		}
		return nil
	}, sql, c.database, nameArg)
	if err != nil {
		return nil, errors.Wrapf(err, "list table failed")
	}

	// get table fields
	sql = fmt.Sprintf("SELECT table, name, type, default_expression, comment, compression_codec "+
		"FROM system.columns "+
		"WHERE database = ? AND table %s ? "+
		"ORDER BY table, position", operator)
	err = c.Query(ctx, func(rows driver.Rows) error {
		var rawTableName string
		var field Field
		var rawFieldType string
		scanErr := rows.Scan(
			&rawTableName,
			&field.Name,
			&rawFieldType,
			&field.DefaultExpr,
			&field.Comment,
			&field.CompressionCodec,
		)
		if scanErr != nil {
			return scanErr
		}
		tableName := strings.TrimPrefix(rawTableName, c.tableNamePrefix)
		field.Type = BuildFieldType(rawFieldType)
		tableOrView, has := tables[tableName]
		if !has {
			// this is a field of a ignored table, just ignore it
			return nil
		}
		switch tv := tableOrView.(type) {
		case Table:
			tv.Fields = append(tv.Fields, field)
			tables[tableName] = tv
		case View:
			tv.Fields = append(tv.Fields, field)
			tables[tableName] = tv
		case MaterializedView:
			tv.Fields = append(tv.Fields, field)
			tables[tableName] = tv
		}
		return nil
	}, sql, c.database, nameArg)
	if err != nil {
		return nil, errors.Wrapf(err, "list table fields failed")
	}

	// get index
	sql = fmt.Sprintf("SELECT table, name, type_full, expr, granularity "+
		"FROM system.data_skipping_indices "+
		"WHERE database = ? AND table %s ? "+
		"ORDER BY table, name", operator)
	err = c.Query(ctx, func(rows driver.Rows) error {
		var rawTableName string
		var index Index
		if scanErr := rows.Scan(&rawTableName, &index.Name, &index.Type, &index.Expr, &index.Granularity); scanErr != nil {
			return scanErr
		}
		tableName := strings.TrimPrefix(rawTableName, c.tableNamePrefix)
		tableOrView, has := tables[tableName]
		if !has {
			// this is a index of a ignored table, just ignore it
			return nil
		}
		table, is := tableOrView.(Table)
		if !is {
			// this is a index of a view or some other kind of table, not a normal table, just ignore it
			return nil
		}
		table.Indexes = append(table.Indexes, index)
		tables[tableName] = table
		return nil
	}, sql, c.database, nameArg)
	if err != nil {
		return nil, errors.Wrapf(err, "list indices failed")
	}

	// get projection
	sql = fmt.Sprintf("SELECT table, name, query "+
		"FROM system.projections "+
		"WHERE database = ? AND table %s ? "+
		"ORDER BY table, name", operator)
	err = c.Query(ctx, func(rows driver.Rows) error {
		var rawTableName string
		var projection Projection
		if scanErr := rows.Scan(&rawTableName, &projection.Name, &projection.Query); scanErr != nil {
			return scanErr
		}
		tableName := strings.TrimPrefix(rawTableName, c.tableNamePrefix)
		tableOrView, has := tables[tableName]
		if !has {
			// this is a projection of a ignored table, just ignore it
			return nil
		}
		table, is := tableOrView.(Table)
		if !is {
			// this is a projection of a view or some other kind of table, not a normal table, just ignore it
			return nil
		}
		table.Projections = append(table.Projections, projection)
		tables[tableName] = table
		return nil
	}, sql, c.database, nameArg)

	return utils.GetMapValuesOrderByKey(tables), nil
}

// Load result only contains name and comment if simple is true
func (c Controller) Load(ctx context.Context, name EqualOrLike, simple bool) (result []TableOrView, err error) {
	if simple {
		return c.loadSimple(ctx, name)
	}
	return c.load(ctx, name)
}

// LoadAll result only contains name and comment if simple is true
func (c Controller) LoadAll(ctx context.Context, simple bool) ([]TableOrView, error) {
	return c.Load(ctx, EqualOrLike{}, simple)
}

// LoadOne result only contains name and comment if simple is true
func (c Controller) LoadOne(ctx context.Context, name string, simple bool) (TableOrView, bool, error) {
	r, err := c.Load(ctx, EqualOrLike{Name: &name}, simple)
	if err != nil || len(r) == 0 {
		return nil, false, err
	}
	return r[0], true, nil
}

func (c Controller) buildCreateTableSQL(table Table) string {
	var sql bytes.Buffer
	sql.WriteString("CREATE ")
	if table.IsTemporary {
		sql.WriteString("TEMPORARY ")
	}
	sql.WriteString("TABLE ")
	if table.IsTemporary {
		sql.WriteString(c.LogicName(table.Name))
	} else {
		sql.WriteString(c.FullLogicNameWithOnCluster(table.Name))
	}
	sql.WriteString(" (")
	for i, field := range table.Fields {
		if i > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString(field.CreateSQL())
	}
	for _, index := range table.Indexes {
		sql.WriteString(", ")
		sql.WriteString(index.CreateSQL())
	}
	for _, projection := range table.Projections {
		sql.WriteString(", ")
		sql.WriteString(projection.CreateSQL())
	}
	sql.WriteString(fmt.Sprintf(") ENGINE = %s", table.Config.Engine.Full()))
	if table.Config.PartitionBy != "" {
		sql.WriteString(fmt.Sprintf(" PARTITION BY %s", table.Config.PartitionBy))
	}
	if len(table.Config.OrderBy) > 0 {
		sql.WriteString(fmt.Sprintf(" ORDER BY (`%s`)", strings.Join(table.Config.OrderBy, "`,`")))
	}
	if len(table.Config.Settings) > 0 {
		sql.WriteString(" SETTINGS ")
		var i int
		for _, k := range utils.GetOrderedMapKeys(table.Config.Settings) {
			if i > 0 {
				sql.WriteString(",")
			}
			sql.WriteString(fmt.Sprintf("%s=%s", k, table.Config.Settings[k]))
			i++
		}
	}
	if table.Comment != "" {
		sql.WriteString(fmt.Sprintf(" COMMENT '%s'", table.Comment))
	}
	return sql.String()
}

func (c Controller) buildCreateViewSQL(view View, replace bool) string {
	var sql bytes.Buffer
	sql.WriteString(utils.Select(replace, "CREATE OR REPLACE VIEW ", "CREATE VIEW "))
	sql.WriteString(c.FullLogicNameWithOnCluster(view.Name))
	if len(view.Fields) > 0 {
		sql.WriteString(" (")
		for i, field := range view.Fields {
			if i > 0 {
				sql.WriteString(", ")
			}
			sql.WriteString(field.CreateSQL())
		}
		sql.WriteString(")")
	}
	sql.WriteString(" AS ")
	sql.WriteString(view.Select)
	return sql.String()
}

func (c Controller) buildCreateMaterializedViewSQL(view MaterializedView) string {
	var sql bytes.Buffer
	sql.WriteString("CREATE MATERIALIZED VIEW ")
	sql.WriteString(fmt.Sprintf("%s TO %s", c.FullLogicNameWithOnCluster(view.Name), c.FullLogicName(view.To)))
	if len(view.Fields) > 0 {
		sql.WriteString(" (")
		for i, field := range view.Fields {
			if i > 0 {
				sql.WriteString(", ")
			}
			sql.WriteString(field.CreateSQL())
		}
		sql.WriteString(")")
	}
	sql.WriteString(" AS ")
	sql.WriteString(view.Select)
	return sql.String()
}

func (c Controller) BuildCreateSQL(tableOrView TableOrView) string {
	switch tv := tableOrView.(type) {
	case Table:
		return c.buildCreateTableSQL(tv)
	case View:
		return c.buildCreateViewSQL(tv, false)
	case MaterializedView:
		return c.buildCreateMaterializedViewSQL(tv)
	default:
		panic(fmt.Sprintf("unknown type %T", tableOrView))
	}
}

func (c Controller) Create(ctx context.Context, tableOrView TableOrView) (err error) {
	_, logger := log.FromContext(ctx, "name", tableOrView.GetName())
	if err = c.Exec(ctx, c.BuildCreateSQL(tableOrView)); err != nil {
		logger.Errorfe(err, "create %s failed", tableOrView.GetKind())
		return errors.Wrapf(err, "create %s %s failed", tableOrView.GetKind(), tableOrView.GetName())
	}
	switch tableOrView.(type) {
	case View, MaterializedView:
		err = c.AlterTable(ctx, tableOrView.GetName(), fmt.Sprintf("MODIFY COMMENT '%s'", tableOrView.GetComment()))
		if err != nil {
			return errors.Wrapf(err, "create %s %s failed: set comment failed",
				tableOrView.GetKind(), tableOrView.GetName())
		}
	}
	logger.Infof("create %s succeed", tableOrView.GetKind())
	return nil
}

func (c Controller) SyncTable(ctx context.Context, pre, cur Table) (err error) {
	// === check diff
	if pre.Name != cur.Name {
		return errors.Errorf("try to change table name from %q to %q", pre.Name, cur.Name)
	}
	_, logger := log.FromContext(ctx, "table", pre.Name)
	logger.Info("will sync table")
	if pe, ce := pre.Config.Engine.Name(), cur.Config.Engine.Name(); pe != ce {
		logger.Warnf("engine changed from %q to %q, will be ignored", pe, ce)
	}
	if pp, cp := pre.Config.PartitionBy, cur.Config.PartitionBy; pp != cp {
		logger.Warnf("partition by changed from %q to %q, will be ignored", pp, cp)
	}
	if po, co := pre.Config.OrderBy, cur.Config.OrderBy; !utils.ArrEqual(po, co) {
		// VersionedCollapsingMergeTree will auto add version field to tail of the order by list
		logger.Warnf("order by changed from %v to %v, will be ignored", po, co)
	}
	// check field
	preFields := make(map[string]Field)
	for _, field := range pre.Fields {
		preFields[field.Name] = field
	}
	var addFields []Field
	var commentChangedFields []Field
	var defaultChangedFields []Field
	var typeChangedFields []Field
	for _, field := range cur.Fields {
		pf, has := preFields[field.Name]
		if !has {
			addFields = append(addFields, field)
			continue
		}
		delete(preFields, field.Name)
		if !pf.Type.SameAs(field.Type) {
			if !pf.Type.CheckModify(field.Type) {
				logger.Warnf("cannot modify type from %s to %s for column %s, will be ignored", pf.Type, field.Type, field.Name)
			} else {
				typeChangedFields = append(typeChangedFields, field)
				if pf.DefaultExpr != "" && field.DefaultExpr == "" {
					defaultChangedFields = append(defaultChangedFields, field)
				}
				continue
			}
		}
		if !strings.EqualFold(pf.CompressionCodec, field.CompressionCodec) {
			logger.Warnf("compression codec of field %q changed from %q to %q, will be ignored",
				field.Name, pf.CompressionCodec, field.CompressionCodec)
		}
		if pf.Comment != field.Comment {
			commentChangedFields = append(commentChangedFields, field)
		}
		if pf.DefaultExpr != field.DefaultExpr {
			defaultChangedFields = append(defaultChangedFields, field)
		}
	}
	delFields := utils.GetMapKeys(preFields)
	// check index
	preIndex := make(map[string]Index)
	for _, index := range pre.Indexes {
		preIndex[index.Name] = index
	}
	var addIndexes []Index
	var delIndexes []Index
	for _, index := range cur.Indexes {
		pi, has := preIndex[index.Name]
		if !has {
			addIndexes = append(addIndexes, index)
		} else if !pi.Equal(index) {
			delIndexes = append(delIndexes, pi)
			addIndexes = append(addIndexes, index)
		}
		delete(preIndex, index.Name)
	}
	delIndexes = append(delIndexes, utils.GetMapValuesOrderByKey(preIndex)...)
	// check projection
	preProjection := make(map[string]Projection)
	for _, projection := range pre.Projections {
		preProjection[projection.Name] = projection
	}
	var addProjections []Projection
	var delProjections []Projection
	for _, projection := range cur.Projections {
		pp, has := preProjection[projection.Name]
		if !has {
			addProjections = append(addProjections, projection)
		} else if !pp.Equal(projection) {
			delProjections = append(delProjections, pp)
			addProjections = append(addProjections, projection)
		}
		delete(preProjection, projection.Name)
	}
	delProjections = append(delProjections, utils.GetMapValuesOrderByKey(preProjection)...)
	// === execute update schema
	startAt := time.Now()
	defer func() {
		logger = logger.With("used", time.Since(startAt).String())
		if err != nil {
			logger.Errorfe(err, "sync table failed")
			err = errors.Wrapf(err, "sync table %s failed", cur.Name)
		} else {
			logger.Info("sync table succeed")
		}
	}()
	// drop fields
	for _, fn := range delFields {
		err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("DROP COLUMN `%s`", fn))
		if err != nil {
			return errors.Wrapf(err, "drop column %q failed", fn)
		}
		logger.Infof("dropped column %s", fn)
	}
	// add fields
	for _, field := range addFields {
		err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("ADD COLUMN %s", field.CreateSQL()))
		if err != nil {
			return errors.Wrapf(err, "add column %q failed", field.Name)
		}
		logger.With("field", field).Infof("added column %s", field.Name)
	}
	// update field default
	for _, field := range defaultChangedFields {
		var sql string
		if field.DefaultExpr == "" {
			sql = fmt.Sprintf("MODIFY COLUMN `%s` REMOVE DEFAULT", field.Name)
		} else {
			sql = fmt.Sprintf("MODIFY COLUMN `%s` %s DEFAULT %s", field.Name, field.Type, field.DefaultExpr)
		}
		if err = c.AlterTable(ctx, cur.Name, sql); err != nil {
			return errors.Wrapf(err, "modify default of column %q failed", field.Name)
		}
		logger.With("defaultExpr", field.DefaultExpr).Infof("updated default of column %s", field.Name)
	}
	// modify field, should behind update field default because may be need to remove default first
	for _, field := range typeChangedFields {
		err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("MODIFY COLUMN %s", field.CreateSQL()))
		if err != nil {
			return errors.Wrapf(err, "modify type to %s for column %q failed", field.Type, field.Name)
		}
		logger.With("field", field).Infof("modified column %s", field.Name)
	}
	// update field comment
	for _, field := range commentChangedFields {
		err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("COMMENT COLUMN `%s` '%s'", field.Name, field.Comment))
		if err != nil {
			return errors.Wrapf(err, "comment column %q failed", field.Name)
		}
		logger.With("comment", field.Comment).Infof("updated comment of column %s", field.Name)
	}
	// drop indexes
	for _, index := range delIndexes {
		err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("DROP INDEX `%s`", index.Name))
		if err != nil {
			return errors.Wrapf(err, "drop index %q failed", index.Name)
		}
		logger.With("index", index).Infof("dropped index %s", index.Name)
	}
	// add indexes
	for _, index := range addIndexes {
		err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("ADD %s", index.CreateSQL()))
		if err != nil {
			return errors.Wrapf(err, "add index %q failed", index.Name)
		}
		logger.With("index", index).Infof("added index %s", index.Name)
	}
	// drop projections
	for _, projection := range delProjections {
		err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("DROP PROJECTION `%s`", projection.Name))
		if err != nil {
			return errors.Wrapf(err, "drop projection %q failed", projection.Name)
		}
		logger.With("projection", projection).Infof("dropped projection %s", projection.Name)
	}
	// add projections
	for _, projection := range addProjections {
		err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("ADD PROJECTION `%s` (%s)", projection.Name, projection.Query))
		if err != nil {
			return errors.Wrapf(err, "add projection %q failed", projection.Name)
		}
		logger.With("projection", projection).Infof("added projection %s", projection.Name)
	}
	// update table settings
	var updateSettings []string
	for k, v := range cur.Config.Settings {
		pv, has := pre.Config.Settings[k]
		if !has || pv != v {
			updateSettings = append(updateSettings, fmt.Sprintf("%s = %s", k, v))
		}
	}
	if len(updateSettings) > 0 {
		err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("MODIFY SETTING %s", strings.Join(updateSettings, ",")))
		if err != nil {
			return errors.Wrapf(err, "modify settings failed")
		}
		logger.Infow("modified table settings",
			"pre", pre.Config.Settings, "cur", cur.Config.Settings, "update", updateSettings)
	}
	// update table comment
	if pre.Comment != cur.Comment {
		err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("MODIFY COMMENT '%s'", cur.Comment))
		if err != nil {
			return errors.Wrapf(err, "modify table comment failed")
		}
		logger.Infow("modified table comment", "pre", pre.Comment, "cur", cur.Comment)
	}
	return nil
}

func (c Controller) SyncView(ctx context.Context, pre, cur View) (err error) {
	if pre.Name != cur.Name {
		return errors.Errorf("try to change table name from %q to %q", pre.Name, cur.Name)
	}
	_, logger := log.FromContext(ctx, "name", cur.Name)
	logger.Info("will sync view")
	if err = c.Exec(ctx, c.buildCreateViewSQL(cur, true)); err != nil {
		logger.Errorfe(err, "replace view failed")
		return errors.Wrapf(err, "sync view %s failed, replace view failed", cur.Name)
	}
	err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("MODIFY COMMENT '%s'", cur.Comment))
	if err != nil {
		logger.Errorfe(err, "set comment failed")
		return errors.Wrapf(err, "sync view %s failed, set comment failed", cur.Name)
	}
	logger.Info("sync view succeed")
	return nil
}

func (c Controller) SyncMaterializedView(ctx context.Context, pre, cur MaterializedView) (err error) {
	if pre.Name != cur.Name {
		return errors.Errorf("try to change table name from %q to %q", pre.Name, cur.Name)
	}
	_, logger := log.FromContext(ctx, "name", cur.Name)
	logger.Info("will sync materialized view")
	if err = c.Exec(ctx, fmt.Sprintf("DROP VIEW %s", c.FullLogicNameWithOnCluster(pre.Name))); err != nil {
		logger.Errorfe(err, "drop old one failed")
		return errors.Wrapf(err, "sync materialized view %s failed: drop old one failed", cur.Name)
	}
	if err = c.Exec(ctx, c.buildCreateMaterializedViewSQL(cur)); err != nil {
		logger.Errorfe(err, "create new one failed")
		return errors.Wrapf(err, "sync materialized view %s failed: create new one failed", cur.Name)
	}
	err = c.AlterTable(ctx, cur.Name, fmt.Sprintf("MODIFY COMMENT '%s'", cur.Comment))
	if err != nil {
		logger.Errorfe(err, "set comment failed")
		return errors.Wrapf(err, "sync materialized view %s failed: set comment failed", cur.Name)
	}
	logger.Info("sync materialized view succeed")
	return nil
}

func (c Controller) Sync(ctx context.Context, pre, cur TableOrView) error {
	if pk, ck := pre.GetKind(), cur.GetKind(); pk != ck {
		return errors.Errorf("try to change a %s to a %s", pk, ck)
	}
	switch cc := cur.(type) {
	case Table:
		return c.SyncTable(ctx, pre.(Table), cc)
	case View:
		return c.SyncView(ctx, pre.(View), cc)
	case MaterializedView:
		return c.SyncMaterializedView(ctx, pre.(MaterializedView), cc)
	default:
		panic(errors.Errorf("unreachable, %T is not supported", cur))
	}
}

func (c Controller) drop(ctx context.Context, tableOrView TableOrView) error {
	_, logger := log.FromContext(ctx, "name", tableOrView.GetName())
	var sql string
	switch tv := tableOrView.(type) {
	case Table:
		name := utils.Select(tv.IsTemporary, c.FullLogicName(tv.Name), c.FullLogicNameWithOnCluster(tv.Name))
		sql = fmt.Sprintf("DROP TABLE %s", name)
	case View:
		sql = fmt.Sprintf("DROP VIEW %s", c.FullLogicNameWithOnCluster(tv.Name))
	case MaterializedView:
		sql = fmt.Sprintf("DROP VIEW %s", c.FullLogicNameWithOnCluster(tv.Name))
	default:
		panic(fmt.Sprintf("unknown type %T", tableOrView))
	}
	if err := c.Exec(ctx, sql); err != nil {
		logger.Errorfe(err, "drop %s failed", tableOrView.GetKind())
		return errors.Wrapf(err, "drop %s %s failed", tableOrView.GetKind(), tableOrView.GetName())
	}
	logger.Infof("drop %s succeed", tableOrView.GetKind())
	return nil
	// TODO The replicas still need to be cleaned up.
	//      If replicas are used when building the table, there will still be residues in zk after dropping the table.
	//      SQL:
	//       - select * from system.zookeeper where path = '/clickhouse/tables/{database}/shard0'
	//       - select * from system.replicas
}

func (c Controller) Drop(ctx context.Context, tablesOrViews ...TableOrView) error {
	for _, tv := range tablesOrViews {
		if err := c.drop(ctx, tv); err != nil {
			return err
		}
	}
	return nil
}

func (c Controller) DropAll(ctx context.Context) error {
	tvs, err := c.loadSimple(ctx, EqualOrLike{})
	if err != nil {
		return err
	}
	return c.Drop(ctx, tvs...)
}
