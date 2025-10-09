package gormcache

import (
	"context"
	"reflect"
	"sentioxyz/sentio-core/common/log"

	"gorm.io/gorm/schema"

	"gorm.io/gorm"
)

func (p *Plugin) AfterUpdate() func(*gorm.DB) {
	return func(db *gorm.DB) {
		//tableName := ""

		//if db.Statement.Schema != nil {
		//	tableName = db.Statement.Schema.Table
		//} else {
		//	tableName = db.Statement.Table
		//}
		ctx := db.Statement.Context
		//log.Debugw("after update", "table", tableName, "sql", db.Statement.SQL.String(), "values", db.Statement.Vars)
		if db.Error == nil {
			if db.Statement.Dest != nil {
				p.recoverableInvalidateObject(ctx, db)
			}
		}
	}
}
func (p *Plugin) AfterCreate() func(*gorm.DB) {
	return func(db *gorm.DB) {
		//tableName := ""

		//if db.Statement.Schema != nil {
		//	tableName = db.Statement.Schema.Table
		//} else {
		//	tableName = db.Statement.Table
		//}
		ctx := db.Statement.Context
		//log.Debugw("after create", "table", tableName, "sql", db.Statement.SQL.String(), "values", db.Statement.Vars)

		if db.Error == nil {
			if db.Statement.Dest != nil {
				p.recoverableInvalidateObject(ctx, db)
			}
		}
	}
}

func (p *Plugin) AfterDelete() func(*gorm.DB) {
	return func(db *gorm.DB) {
		//tableName := ""
		//
		//if db.Statement.Schema != nil {
		//	tableName = db.Statement.Schema.Table
		//} else {
		//	tableName = db.Statement.Table
		//}
		ctx := db.Statement.Context
		//log.Debugw("after delete", "table", tableName, "sql", db.Statement.SQL.String(), "values", db.Statement.Vars)

		if db.Error == nil {
			if db.Statement.Dest != nil {
				p.recoverableInvalidateObject(ctx, db)
			}
		}
	}
}

func (p *Plugin) recoverableInvalidateObject(ctx context.Context, db *gorm.DB) {
	defer recoverPanic()
	p.invalidateObject(ctx, db)
}

func (p *Plugin) invalidateObject(ctx context.Context, db *gorm.DB) {
	tableName := ""

	if db.Statement.Schema != nil {
		tableName = db.Statement.Schema.Table
	} else {
		tableName = db.Statement.Table
	}

	if db.Statement.RowsAffected == 0 {
		return
	}

	objects := make([]reflect.Value, 0)

	destValue := db.Statement.ReflectValue
	switch destValue.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < destValue.Len(); i++ {
			elem := destValue.Index(i)
			objects = append(objects, elem)
		}
	case reflect.Struct:
		objects = append(objects, destValue)
	}

	var relations []Relation

	for _, field := range GetKeyFields(db) {
		for _, elemValue := range objects {
			value, isZero := field.ValueOf(context.Background(), elemValue)
			if !isZero {
				relations = append(relations, Relation{
					TableName: tableName,
					Column:    field.DBName,
					Values:    []any{value},
				})
			}
		}
	}
	whereWithValues := GetFromWhere(db)
	whereClause, _ := ParseWhereClause(db.Statement.SQL.String())
	if len(whereClause) > 0 {
		filled, _ := FillWhereValues(whereClause, db.Statement.Vars)
		whereWithValues = append(whereWithValues, filled...)
	}

	// handle relations
	for _, rel := range GetRelationFields(db) {
		for _, ref := range rel.References {
			var values []any
			var ownField, foreignField *schema.Field
			// other table references current table
			if ref.OwnPrimaryKey {
				ownField = ref.PrimaryKey
				foreignField = ref.ForeignKey
			} else {
				ownField = ref.ForeignKey
				foreignField = ref.PrimaryKey
			}
			for _, elemValue := range objects {
				value, isZero := ownField.ValueOf(context.Background(), elemValue)
				if !isZero {
					values = append(values, value)
				}
			}
			if len(values) == 0 {
				for _, where := range whereWithValues {
					if where.Column == ownField.DBName {
						if where.Operator == "in" {
							values = where.ValueList
							break
						}
						if where.Operator == "=" {
							values = []any{where.Value}
							break
						}
					}
				}
			}
			if len(values) > 0 {
				relations = append(relations, Relation{
					TableName: ownField.Schema.Table,
					Column:    ownField.DBName,
					Values:    values,
				})
				if foreignField != nil {
					relations = append(relations, Relation{
						TableName: foreignField.Schema.Table,
						Column:    foreignField.DBName,
						Values:    values,
					})
				}
			}
		}
	}

	// cache hints
	if m, ok := db.Statement.Model.(ModelWithCacheHints); ok {
		relations = append(relations, m.CacheHints()...)
	}

	// invalidate table level queries
	relations = append(relations, Relation{
		TableName: tableName,
		Column:    "*",
	})

	for _, rel := range relations {
		err := p.cache.InvalidateQuery(ctx, &rel)
		if err != nil {
			log.Errorw(
				"error invalidating cache",
				"table",
				rel.TableName,
				"column",
				rel.Column,
				"values",
				rel.Values,
				"error",
				err,
			)
		}
	}
}
