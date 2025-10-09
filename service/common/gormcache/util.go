package gormcache

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/pkg/errors"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type WhereClause[T any] struct {
	Column    string
	Operator  string
	Value     T
	ValueList []T // Only used for "IN" operator
	Table     string
}

func ParseWhereClause(sql string) ([]WhereClause[string], error) {

	// Parse WHERE clause
	var re = regexp.MustCompile(`(?mi)([\w.()]+)\s*([=<>]+|!=|\s+in)\s*(\?|\$?[\d+-.]+|'[^']+'|\(.+\))`)
	matches := re.FindAllStringSubmatch(strings.TrimSpace(sql), -1)
	if matches == nil {
		return nil, fmt.Errorf("invalid WHERE clause: %s", sql)
	}

	// Create list of WhereClause objects
	whereClauses := make([]WhereClause[string], len(matches))
	for i, match := range matches {
		var value string
		var valueList []string

		if match[3] == "?" {
			value = "?"
		} else if strings.HasPrefix(match[3], "'") {
			value = strings.Trim(match[3], "'")
		} else if strings.HasPrefix(match[3], "(") {
			values := strings.Split(match[3][1:len(match[3])-1], ", ")
			for _, v := range values {
				if v == "?" {
					valueList = append(valueList, "?")
				} else {
					valueList = append(valueList, strings.Trim(v, "'"))
				}
			}
		} else {
			value = match[3]
		}

		whereClauses[i] = WhereClause[string]{
			Column:    match[1],
			Operator:  strings.ToLower(strings.TrimSpace(match[2])),
			Value:     value,
			ValueList: valueList,
		}
	}

	return whereClauses, nil
}

func FillWhereValues(where []WhereClause[string], args []any) ([]WhereClause[any], error) {
	var results = make([]WhereClause[any], len(where))
	argIdx := 0
	getArg := func(v string) (any, error) {
		if v == "?" {
			if argIdx >= len(args) {
				return nil, errors.New("not enough arguments")
			}
			value := args[argIdx]
			argIdx++
			return value, nil
		}
		if strings.HasPrefix(v, "$") {
			// positional argument
			idx, err := strconv.Atoi(v[1:])
			if err != nil {
				return nil, errors.Wrapf(err, "invalid positional argument %s", v)
			}
			if idx > len(args) {
				return nil, fmt.Errorf("not enough arguments")
			}
			// args are 0-based, but $1 is the first argument
			return args[idx-1], nil
		}
		return v, nil
	}

	for i, clause := range where {
		if clause.Operator == "in" {
			valueList, err := utils.MapSlice(clause.ValueList, getArg)
			if err != nil {
				return nil, err
			}
			results[i] = WhereClause[any]{
				Column:    clause.Column,
				Operator:  clause.Operator,
				ValueList: valueList,
			}
		}
		if clause.Operator == "=" {
			value, err := getArg(clause.Value)
			if err != nil {
				return nil, err
			}
			results[i] = WhereClause[any]{
				Column:   clause.Column,
				Operator: clause.Operator,
				Value:    value,
			}
		}
	}
	return results, nil
}

func getValueByField(ctx context.Context, obj any, field *schema.Field) ([]any, error) {
	reflectValue := reflect.Indirect(reflect.ValueOf(obj))
	var results []any
	switch reflectValue.Kind() {
	case reflect.Struct:
		v, zero := field.ValueOf(ctx, reflectValue)
		if !zero {
			results = append(results, v)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < reflectValue.Len(); i++ {
			elem := reflectValue.Index(i)
			elemKey := elem.Interface()
			if elem.Kind() != reflect.Ptr {
				elemKey = elem.Addr().Interface()
			}
			if elemKey == nil {
				continue
			}
			v, zero := field.ValueOf(ctx, reflect.ValueOf(elemKey))
			//rv := field.ReflectValueOf(ctx, reflect.ValueOf(elem))
			//v := rv.Interface()
			if !zero {
				results = append(results, v)
			}
		}
	}
	return results, nil
}

func GetFromWhere(db *gorm.DB) []WhereClause[any] {
	var results []WhereClause[any]

	if where, ok := db.Statement.Clauses["WHERE"]; ok {
		traverse(where, func(expr clause.Expression) {
			switch e := expr.(type) {
			case clause.IN:
				if col, ok := e.Column.(clause.Column); ok {
					results = append(results, WhereClause[any]{
						Column:    col.Name,
						Table:     col.Table,
						Operator:  "in",
						ValueList: e.Values,
					})
				}
			case clause.Eq:
				if col, ok := e.Column.(clause.Column); ok {
					results = append(results, WhereClause[any]{
						Column:   col.Name,
						Table:    col.Table,
						Operator: "=",
						Value:    e.Value,
					})
				}
			}
		})
	}
	return results
}

func traverse(expr clause.Expression, callback func(e clause.Expression)) {
	switch e := expr.(type) {
	case clause.Where:
		for _, ex := range e.Exprs {
			traverse(ex, callback)
			callback(ex)
		}
		callback(e)
	case clause.Clause:
		traverse(e.Expression, callback)
	case clause.From:
		for _, join := range e.Joins {
			traverse(join, callback)
			callback(join)
		}
	}
}

func getJoinRelations(db *gorm.DB) []Relation {
	var result []Relation
	if from, ok := db.Statement.Clauses["FROM"]; ok {
		traverse(from, func(expr clause.Expression) {
			switch e := expr.(type) {
			case clause.Join:
				joinTable := e.Table.Name
				// for simplicity, listen to all changes on the joined table
				if joinTable != "" {
					result = append(result, Relation{
						TableName: joinTable,
						Column:    "*",
					})
				}
			}
		})
	}
	return result
}

func recoverPanic() {
	if r := recover(); r != nil {
		err, ok := r.(error)
		if !ok {
			log.Errorf("panic during cache invalidation: %v", err)
		} else {
			log.Errore(err, "panic during cache invalidation")
		}
	}
}
