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

// LoadTables load tables only contains FullName, Engine, Comment
func (c Controller) LoadTables(ctx context.Context, database string, tableNameLike string) ([]TableOrView, error) {
	var tableOrViews []TableOrView
	sql := "SELECT name, engine, comment FROM system.tables WHERE database = ? AND name LIKE ?"
	err := c.Query(ctx, func(rows driver.Rows) error {
		var name, engine, comment string
		scanErr := rows.Scan(&name, &engine, &comment)
		if scanErr != nil {
			return scanErr
		}
		fn := FullName{
			Database: database,
			Name:     name,
		}
		switch engine {
		case "View":
			tableOrViews = append(tableOrViews, View{
				FullName: fn,
				Comment:  comment,
			})
		case "MaterializedView":
			tableOrViews = append(tableOrViews, MaterializedView{
				View: View{
					FullName: fn,
					Comment:  comment,
				},
			})
		default:
			tableOrViews = append(tableOrViews, Table{
				FullName: fn,
				Comment:  comment,
			})
		}
		return nil
	}, sql, database, tableNameLike)
	if err != nil {
		return nil, errors.Wrapf(err, "list table failed")
	}
	return tableOrViews, nil
}

func (c Controller) Load(
	ctx context.Context,
	database string,
	tableNameLike string,
) (tables map[string]TableOrView, err error) {
	_, logger := log.FromContext(ctx)
	defer func() {
		if err != nil {
			logger.With("database", database, "tableNameLike", tableNameLike).Errorfe(err, "load tables failed")
		}
	}()
	tables = make(map[string]TableOrView)

	// show tables
	sql := "SELECT name, engine, engine_full, create_table_query, as_select, partition_key, sorting_key, comment " +
		"FROM system.tables " +
		"WHERE database = ? AND name LIKE ?"
	err = c.Query(ctx, func(rows driver.Rows) error {
		var name, engine, engineFull, createTableQuery, asSelect, partitionKey, sortingKey, comment string
		scanErr := rows.Scan(&name, &engine, &engineFull, &createTableQuery, &asSelect, &partitionKey, &sortingKey, &comment)
		if scanErr != nil {
			return scanErr
		}
		fn := FullName{
			Database: database,
			Name:     name,
		}
		switch engine {
		case "View":
			tables[name] = View{
				FullName: fn,
				Select:   asSelect,
				Comment:  comment,
			}
		case "MaterializedView":
			view := MaterializedView{
				View: View{
					FullName: fn,
					Select:   asSelect,
					Comment:  comment,
				},
			}
			var sector string
			for sector != "TO" && createTableQuery != "" {
				sector, createTableQuery, _ = cutBySpace(createTableQuery)
			}
			if sector == "TO" {
				raw, _, _ := cutBySpace(createTableQuery)
				p := findNotIn(raw, '.', '`')
				view.To.Database = strings.Trim(raw[:p], "`")
				view.To.Name = strings.Trim(raw[p+1:], "`")
			}
			tables[name] = view
		default:
			table := Table{
				FullName: fn,
				Comment:  comment,
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
				logger.Warnfe(err, "load engine of table %s failed, will be ignored", name)
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
	}, sql, database, tableNameLike)
	if err != nil {
		return nil, errors.Wrapf(err, "list table failed")
	}

	// get table fields
	sql = "SELECT table, name, type, default_expression, comment, compression_codec " +
		"FROM system.columns " +
		"WHERE database = ? AND table LIKE ? " +
		"ORDER BY table, position"
	err = c.Query(ctx, func(rows driver.Rows) error {
		var tableName string
		var field Field
		scanErr := rows.Scan(
			&tableName,
			&field.Name,
			&field.Type,
			&field.DefaultExpr,
			&field.Comment,
			&field.CompressionCodec,
		)
		if scanErr != nil {
			return scanErr
		}
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
	}, sql, database, tableNameLike)
	if err != nil {
		return nil, errors.Wrapf(err, "list table fields failed")
	}

	// get index
	sql = "SELECT table, name, type_full, expr, granularity " +
		"FROM system.data_skipping_indices " +
		"WHERE database = ? AND table like ? " +
		"ORDER BY table, name"
	err = c.Query(ctx, func(rows driver.Rows) error {
		var tableName string
		var index Index
		if scanErr := rows.Scan(&tableName, &index.Name, &index.Type, &index.Expr, &index.Granularity); scanErr != nil {
			return scanErr
		}
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
	}, sql, database, tableNameLike)
	if err != nil {
		return nil, errors.Wrapf(err, "list indices failed")
	}

	// get projection
	sql = "SELECT table, name, query " +
		"FROM system.projections " +
		"WHERE database = ? AND table like ? " +
		"ORDER BY table, name"
	err = c.Query(ctx, func(rows driver.Rows) error {
		var tableName string
		var projection Projection
		if scanErr := rows.Scan(&tableName, &projection.Name, &projection.Query); scanErr != nil {
			return scanErr
		}
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
	}, sql, database, tableNameLike)

	return tables, nil
}

func (c Controller) LoadOne(ctx context.Context, fullName FullName) (TableOrView, bool, error) {
	r, err := c.Load(ctx, fullName.Database, fullName.Name)
	if err != nil {
		return nil, false, err
	}
	tv, has := r[fullName.Name]
	return tv, has, nil
}

func (c Controller) buildCreateTableSQL(table Table) string {
	var sql bytes.Buffer
	sql.WriteString(fmt.Sprintf("CREATE TABLE %s (", c.FullNameWithOnCluster(table.FullName)))
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
	sql.WriteString(c.FullNameWithOnCluster(view.FullName))
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
	sql.WriteString(fmt.Sprintf("CREATE MATERIALIZED VIEW %s TO `%s`.`%s`",
		c.FullNameWithOnCluster(view.FullName), view.To.Database, view.To.Name))
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
	_, logger := log.FromContext(ctx, "name", tableOrView.GetFullName().String())
	if err = c.Exec(ctx, c.BuildCreateSQL(tableOrView)); err != nil {
		logger.Errorfe(err, "create %s failed", tableOrView.GetKind())
		return errors.Wrapf(err, "create %s %s failed", tableOrView.GetKind(), tableOrView.GetFullName())
	}
	switch tableOrView.(type) {
	case View, MaterializedView:
		err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s MODIFY COMMENT '%s'",
			c.FullNameWithOnCluster(tableOrView.GetFullName()), tableOrView.GetComment()))
		if err != nil {
			return errors.Wrapf(err, "create %s %s failed: set comment failed",
				tableOrView.GetKind(), tableOrView.GetFullName())
		}
	}
	logger.Infof("create %s succeed", tableOrView.GetKind())
	return nil
}

func (c Controller) SyncTable(ctx context.Context, pre, cur Table) (err error) {
	// === check diff
	if pre.Database != cur.Database {
		return errors.Errorf("try to change database from %q to %q", pre.Database, cur.Database)
	}
	if pre.Name != cur.Name {
		return errors.Errorf("try to change table name from %q to %q", pre.Name, cur.Name)
	}
	_, logger := log.FromContext(ctx, "table", pre.GetFullName())
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
	for _, field := range cur.Fields {
		pf, has := preFields[field.Name]
		if !has {
			addFields = append(addFields, field)
		} else {
			if !pf.HasSameType(field) {
				// Enum, Decimal256 and some other types will be concretized (like Enum8, Enum16, Decimal(76,30), ...)
				// after the field created, it will be different from the type specified in the declaration
				logger.Warnf("type of field %q changed from %q to %q, will be ignored", field.Name, pf.Type, field.Type)
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
			delete(preFields, field.Name)
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
			err = errors.Wrapf(err, "sync table %s failed", cur.GetFullName())
		} else {
			logger.Info("sync table succeed")
		}
	}()
	// drop fields
	for _, fn := range delFields {
		err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s DROP COLUMN `%s`", c.FullNameWithOnCluster(cur.FullName), fn))
		if err != nil {
			return errors.Wrapf(err, "drop column %q failed", fn)
		}
		logger.Infof("dropped column %s", fn)
	}
	// add fields
	for _, field := range addFields {
		err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s",
			c.FullNameWithOnCluster(cur.FullName), field.CreateSQL()))
		if err != nil {
			return errors.Wrapf(err, "add column %q failed", field.Name)
		}
		logger.With("field", field).Infof("added column %s", field.Name)
	}
	// update field default
	for _, field := range defaultChangedFields {
		var sql string
		if field.DefaultExpr == "" {
			sql = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN `%s` REMOVE DEFAULT",
				c.FullNameWithOnCluster(cur.FullName), field.Name)
		} else {
			sql = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN `%s` %s DEFAULT %s",
				c.FullNameWithOnCluster(cur.FullName), field.Name, field.Type, field.DefaultExpr)
		}
		if err = c.Exec(ctx, sql); err != nil {
			return errors.Wrapf(err, "modify default of column %q failed", field.Name)
		}
		logger.With("defaultExpr", field.DefaultExpr).Infof("updated default of column %s", field.Name)
	}
	// update field comment
	for _, field := range commentChangedFields {
		err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s COMMENT COLUMN `%s` '%s'",
			c.FullNameWithOnCluster(cur.FullName), field.Name, field.Comment))
		if err != nil {
			return errors.Wrapf(err, "comment column %q failed", field.Name)
		}
		logger.With("comment", field.Comment).Infof("updated comment of column %s", field.Name)
	}
	// drop indexes
	for _, index := range delIndexes {
		err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s DROP INDEX `%s`",
			c.FullNameWithOnCluster(cur.FullName), index.Name))
		if err != nil {
			return errors.Wrapf(err, "drop index %q failed", index.Name)
		}
		logger.With("index", index).Infof("dropped index %s", index.Name)
	}
	// add indexes
	for _, index := range addIndexes {
		err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s ADD INDEX `%s` %s TYPE %s GRANULARITY %d",
			c.FullNameWithOnCluster(cur.FullName), index.Name, index.Expr, index.Type, index.Granularity))
		if err != nil {
			return errors.Wrapf(err, "add index %q failed", index.Name)
		}
		logger.With("index", index).Infof("added index %s", index.Name)
	}
	// drop projections
	for _, projection := range delProjections {
		err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s DROP PROJECTION `%s`",
			c.FullNameWithOnCluster(cur.FullName), projection.Name))
		if err != nil {
			return errors.Wrapf(err, "drop projection %q failed", projection.Name)
		}
		logger.With("projection", projection).Infof("dropped projection %s", projection.Name)
	}
	// add projections
	for _, projection := range addProjections {
		err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s ADD PROJECTION `%s` (%s)",
			c.FullNameWithOnCluster(cur.FullName), projection.Name, projection.Query))
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
		err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s MODIFY SETTING %s",
			c.FullNameWithOnCluster(cur.FullName), strings.Join(updateSettings, ",")))
		if err != nil {
			return errors.Wrapf(err, "modify settings failed")
		}
		logger.Infow("modified table settings",
			"pre", pre.Config.Settings, "cur", cur.Config.Settings, "update", updateSettings)
	}
	// update table comment
	if pre.Comment != cur.Comment {
		err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s MODIFY COMMENT '%s'",
			c.FullNameWithOnCluster(cur.FullName), cur.Comment))
		if err != nil {
			return errors.Wrapf(err, "modify table comment failed")
		}
		logger.Infow("modified table comment", "pre", pre.Comment, "cur", cur.Comment)
	}
	return nil
}

func (c Controller) SyncView(ctx context.Context, pre, cur View) (err error) {
	if pre.Database != cur.Database {
		return errors.Errorf("try to change database from %q to %q", pre.Database, cur.Database)
	}
	if pre.Name != cur.Name {
		return errors.Errorf("try to change table name from %q to %q", pre.Name, cur.Name)
	}
	_, logger := log.FromContext(ctx, "name", cur.GetFullName())
	logger.Info("will sync view")
	if err = c.Exec(ctx, c.buildCreateViewSQL(cur, true)); err != nil {
		logger.Errorfe(err, "replace view failed")
		return errors.Wrapf(err, "sync view %s failed, replace view failed", cur.GetFullName())
	}
	err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s MODIFY COMMENT '%s'",
		c.FullNameWithOnCluster(cur.FullName), cur.Comment))
	if err != nil {
		logger.Errorfe(err, "set comment failed")
		return errors.Wrapf(err, "sync view %s failed, set comment failed", cur.GetFullName())
	}
	logger.Info("sync view succeed")
	return nil
}

func (c Controller) SyncMaterializedView(ctx context.Context, pre, cur MaterializedView) (err error) {
	if pre.Database != cur.Database {
		return errors.Errorf("try to change database from %q to %q", pre.Database, cur.Database)
	}
	if pre.Name != cur.Name {
		return errors.Errorf("try to change table name from %q to %q", pre.Name, cur.Name)
	}
	_, logger := log.FromContext(ctx, "name", cur.GetFullName())
	logger.Info("will sync materialized view")
	if err = c.Exec(ctx, fmt.Sprintf("DROP VIEW %s", c.FullNameWithOnCluster(pre.FullName))); err != nil {
		logger.Errorfe(err, "drop old one failed")
		return errors.Wrapf(err, "sync materialized view %s failed: drop old one failed", cur.GetFullName())
	}
	if err = c.Exec(ctx, c.buildCreateMaterializedViewSQL(cur)); err != nil {
		logger.Errorfe(err, "create new one failed")
		return errors.Wrapf(err, "sync materialized view %s failed: create new one failed", cur.GetFullName())
	}
	err = c.Exec(ctx, fmt.Sprintf("ALTER TABLE %s MODIFY COMMENT '%s'",
		c.FullNameWithOnCluster(cur.FullName), cur.Comment))
	if err != nil {
		logger.Errorfe(err, "set comment failed")
		return errors.Wrapf(err, "sync materialized view %s failed: set comment failed", cur.GetFullName())
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
	_, logger := log.FromContext(ctx, "name", tableOrView.GetFullName().String())
	var sql string
	switch tv := tableOrView.(type) {
	case Table:
		sql = fmt.Sprintf("DROP TABLE %s", c.FullNameWithOnCluster(tv.FullName))
	case View:
		sql = fmt.Sprintf("DROP VIEW %s", c.FullNameWithOnCluster(tv.FullName))
	case MaterializedView:
		sql = fmt.Sprintf("DROP VIEW %s", c.FullNameWithOnCluster(tv.FullName))
	default:
		panic(fmt.Sprintf("unknown type %T", tableOrView))
	}
	if err := c.Exec(ctx, sql); err != nil {
		logger.Errorfe(err, "drop %s failed", tableOrView.GetKind())
		return errors.Wrapf(err, "drop %s %s failed", tableOrView.GetKind(), tableOrView.GetFullName())
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

func (c Controller) DropAll(ctx context.Context, database, tableNameLike string) error {
	tvs, err := c.LoadTables(ctx, database, tableNameLike)
	if err != nil {
		return err
	}
	return c.Drop(ctx, tvs...)
}
