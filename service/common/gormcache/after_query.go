package gormcache

import (
	"context"
	"sentioxyz/sentio-core/common/log"
	"strings"

	"github.com/pkg/errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func (p *Plugin) AfterQuery() func(*gorm.DB) {
	return func(db *gorm.DB) {
		tableName := ""
		if db.Statement.Schema != nil {
			tableName = db.Statement.Schema.Table
		} else {
			tableName = db.Statement.Table
		}
		ctx := db.Statement.Context

		if noCache, ok := db.Get(NoCacheKey); ok && noCache.(bool) {
			return
		}

		//log.Debugw("after query", "table", tableName, "sql", db.Statement.SQL.String(), "values", db.Statement.Vars)
		if db.Error == nil || errors.Is(db.Error, gorm.ErrRecordNotFound) {
			p.cacheData(ctx, db, tableName)
		}
		if db.Error == ErrCacheHit {
			// cache hit, data loaded from cache
			db.Error = nil
			return
		}
	}
}

func (p *Plugin) cacheData(ctx context.Context, db *gorm.DB, tableName string) {
	defer recoverPanic()

	cacheKey := p.cache.GetCacheKey(tableName, db.Statement.SQL.String(), db.Statement.Vars, db.Statement.Preloads)
	var parentCacheKey string
	var ok bool
	if parentCacheKey, ok = db.Statement.Context.Value(ParentCacheKey).(string); !ok {
		parentCacheKey = cacheKey
	}

	var err error

	// query relations
	relations := getRelationsFromWhere(db)
	if len(relations) == 0 {
		// related to the whole table
		relations = append(relations, Relation{
			TableName: tableName,
			Column:    "*",
		})
	}
	// preload relations
	relations = append(relations, getPreloadRelations(db)...)
	// join relations
	relations = append(relations, getJoinRelations(db)...)

	if parentCacheKey != "" {
		for _, rel := range relations {
			// relate the parent query to nested relations
			err = p.cache.AddRelation(ctx, parentCacheKey, &rel)
			if err != nil {
				log.Errore(err, "cache add relation error")
			}
		}
	}

	var data []byte
	var cacheResult CachedResult
	if errors.Is(db.Error, gorm.ErrRecordNotFound) {
		cacheResult = CachedResult{
			AffectRows: 0,
			SQL:        db.Statement.SQL.String(),
			ErrString:  db.Error.Error(),
			IsNotFound: true,
		}
	} else {
		if db.Statement.Dest != nil {
			data, err = p.cache.Encode(ctx, db.Statement.Dest)
			if err != nil {
				log.Errore(err, "cache encode error")
				return
			}
		}
		cacheResult = CachedResult{
			AffectRows: db.Statement.RowsAffected,
			Data:       data,
			SQL:        db.Statement.SQL.String(),
		}
	}

	if cacheKey != parentCacheKey {
		// no need to cache the nested query result
		return
	}
	err = p.cache.AddQuery(ctx, cacheKey, &cacheResult)
	if err != nil {
		log.Errore(err, "failed to cache result")
	}
}

func getRelationsFromWhere(db *gorm.DB) []Relation {
	var relations []Relation

	if whereClause, ok := db.Statement.Clauses["WHERE"]; ok {
		if where, ok := whereClause.Expression.(clause.Where); ok {
			if db.Statement.Schema == nil {
				return nil
			}
			uniqueKeys := GetKeyFields(db)

			for _, expr := range where.Exprs {
				eqExpr, ok := expr.(clause.Eq)
				if ok {
					column := getColNameFromColumn(eqExpr.Column)
					if _, ok := uniqueKeys[column]; ok {
						relations = append(relations, Relation{
							TableName: db.Statement.Schema.Table,
							Column:    column,
							Values:    []any{eqExpr.Value},
						})
					}
					continue
				}
				inExpr, ok := expr.(clause.IN)
				if ok {
					column := getColNameFromColumn(inExpr.Column)
					if _, ok := uniqueKeys[column]; ok {
						relations = append(relations, Relation{
							TableName: db.Statement.Schema.Table,
							Column:    column,
							Values:    inExpr.Values,
						})
					}
				}
				exprStruct, ok := expr.(clause.Expr)
				if ok {
					columnAndValues := extractExprStruct(exprStruct)
					for column, values := range columnAndValues {
						if _, ok := uniqueKeys[column]; ok {
							relations = append(relations, Relation{
								TableName: db.Statement.Schema.Table,
								Column:    column,
								Values:    values,
							})
						}
					}
				}
			}

		}
	}

	return relations
}

// GetKeyFields get important fields from current model
func GetKeyFields(db *gorm.DB) map[string]*schema.Field {
	fields := map[string]*schema.Field{}
	for _, field := range db.Statement.Schema.Fields {
		if field.PrimaryKey {
			fields[field.DBName] = field
			continue
		}
		if tag, ok := field.TagSettings["INDEX"]; ok && strings.Contains(strings.ToLower(tag), "unique") {
			fields[field.DBName] = field
		}
	}
	return fields
}

type RelationField struct {
	Field      *schema.Field
	References []*schema.Reference
}

func GetRelationFields(db *gorm.DB) []RelationField {
	var fields []RelationField
	for _, rel := range db.Statement.Schema.Relationships.Relations {
		fields = append(fields, RelationField{
			References: rel.References,
			Field:      rel.Field,
		})
	}
	return fields
}

// match  a = ?
//
//	b in (?, ?)
//	c = "xxx"
//var re = regexp.MustCompile(`(?m)(\S+)\s*(=|in)\s*(\?|[-=\d.]+|".+"|\(.*\))`)

func extractExprStruct(exprStruct clause.Expr) map[string][]any {
	results := map[string][]any{}
	whereClauses, err := ParseWhereClause(exprStruct.SQL)
	if err != nil {
		return nil
	}
	whereWithValues, err := FillWhereValues(whereClauses, exprStruct.Vars)
	if err != nil {
		return nil
	}

	for _, w := range whereWithValues {
		field := w.Column
		switch w.Operator {
		case "=":
			results[field] = []any{w.Value}
		case "in":
			results[field] = w.ValueList
		}
	}

	return results
}

func getColNameFromColumn(col interface{}) string {
	switch v := col.(type) {
	case string:
		return v
	case clause.Column:
		return v.Name
	default:
		return ""
	}
}

func getPreloadRelations(db *gorm.DB) []Relation {
	var relations []Relation
	for name := range db.Statement.Preloads {
		// TODO: need to handle nested preloads

		if rel, ok := db.Statement.Schema.Relationships.Relations[name]; ok {
			if rel.JoinTable != nil {
				relations = append(relations, handleJoinTable(db, rel)...)
				continue
			}

			for _, ref := range rel.References {
				if ref.PrimaryValue != "" {
					continue
				}
				var ownField, foreignField *schema.Field
				if ref.OwnPrimaryKey {
					ownField = ref.PrimaryKey
					foreignField = ref.ForeignKey
				} else {
					ownField = ref.ForeignKey
					foreignField = ref.PrimaryKey
				}

				values, _ := getValueByField(
					db.Statement.Context,
					db.Statement.Dest,
					ownField,
				)
				if len(values) == 0 {
					// can't find any value, fallback to listen all table changes
					ownRelation := Relation{
						TableName: ownField.Schema.Table,
						Column:    "*",
					}
					foreignRelation := Relation{
						TableName: foreignField.Schema.Table,
						Column:    "*",
					}
					relations = append(relations, ownRelation, foreignRelation)
				} else {
					ownRelation := Relation{
						TableName: ownField.Schema.Table,
						Column:    ownField.DBName,
						Values:    values,
					}
					foreignRelation := Relation{
						TableName: foreignField.Schema.Table,
						Column:    foreignField.DBName,
						Values:    values,
					}
					relations = append(relations, ownRelation, foreignRelation)
				}
			}
		}
	}

	return relations
}

func handleJoinTable(db *gorm.DB, rel *schema.Relationship) []Relation {
	var relations []Relation
	for _, ref := range rel.References {
		if ref.PrimaryValue != "" {
			continue
		}
		if ref.PrimaryKey.Schema.Name == rel.Schema.Name {
			values, err := getValueByField(db.Statement.Context, db.Statement.Dest, ref.PrimaryKey)
			if err != nil {
				continue
			}
			relations = append(relations, Relation{
				TableName: ref.PrimaryKey.Schema.Table,
				Column:    ref.PrimaryKey.DBName,
				Values:    values,
			})
			relations = append(relations, Relation{
				TableName: ref.ForeignKey.Schema.Table,
				Column:    ref.ForeignKey.DBName,
				Values:    values,
			})
		}
		if ref.PrimaryKey.Schema.Name == rel.FieldSchema.Name {
			relValues := schema.GetRelationsValues(
				db.Statement.Context,
				db.Statement.ReflectValue,
				[]*schema.Relationship{rel},
			)
			_, fieldValues := schema.GetIdentityFieldValuesMap(
				db.Statement.Context,
				relValues,
				rel.FieldSchema.PrimaryFields,
			)
			if len(fieldValues) > 0 {
				var values []any
				for _, v := range fieldValues {
					values = append(values, v...)
				}
				relations = append(relations, Relation{
					TableName: ref.PrimaryKey.Schema.Table,
					Column:    ref.PrimaryKey.DBName,
					Values:    values,
				})
				relations = append(relations, Relation{
					TableName: ref.ForeignKey.Schema.Table,
					Column:    ref.ForeignKey.DBName,
					Values:    values,
				})
			}
		}
	}
	return relations
}
