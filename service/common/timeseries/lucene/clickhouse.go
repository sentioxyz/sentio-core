package lucene

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
	clickhouselib "sentioxyz/sentio-core/driver/timeseries/clickhouse"
	"sentioxyz/sentio-core/service/common/timeseries/compatible"

	"github.com/blevesearch/bleve/search/query"
	"github.com/pkg/errors"
)

type clickhouse struct {
	prefix       string
	presetColumn map[string]struct{}
	fieldTypes   map[string]timeseries.FieldType
}

func NewClickhouse(attributesPrefix string,
	presetColumn map[string]struct{}, meta timeseries.Metaset) Driver {
	return &clickhouse{
		prefix:       attributesPrefix,
		presetColumn: presetColumn,
		fieldTypes:   meta.FieldTypes(true, false),
	}
}

func (c *clickhouse) Render(q query.Query) (s string, err error) {
	switch qt := q.(type) {
	case *query.MatchPhraseQuery:
		return c.matchPhraseQuery(*qt)
	case *query.NumericRangeQuery:
		return c.numericRangeQuery(*qt)
	case *query.MatchNoneQuery:
		return "(false)", nil
	case *query.WildcardQuery:
		return c.wildcardQuery(*qt)
	case *query.RegexpQuery:
		return c.regexpQuery(*qt)
	case *query.TermRangeQuery:
		return c.termRangeQuery(*qt)
	case *query.DateRangeQuery:
		return c.dateRangeQuery(*qt)
	case *query.ConjunctionQuery:
		return c.conjunctionQuery(*qt)
	case *query.DisjunctionQuery:
		return c.disjunctionQuery(*qt)
	case *query.BooleanQuery:
		return c.booleanQuery(*qt)
	default:
		return "", errors.Errorf("unsupported query type %T", q)
	}
}

func (c *clickhouse) field(fieldName string) (string, timeseries.FieldType) {
	transformed, ok := compatible.FieldNameTransform[fieldName]
	if ok {
		fieldName = transformed
	}
	fieldType, ok := c.fieldTypes[fieldName]
	if !ok {
		fieldType = timeseries.FieldTypeString
	}
	if _, ok := c.presetColumn[fieldName]; ok {
		return clickhouselib.DbTypeCasting(timeseries.EscapeEventlogFieldName(fieldName), fieldType), fieldType
	}
	return clickhouselib.DbTypeCasting(timeseries.EscapeEventlogFieldName(c.prefix+"."+fieldName), fieldType), fieldType
}

func (c *clickhouse) fulltextQuery(template string) (s string, err error) {
	var fieldConds []string

	for name, _ := range c.presetColumn {
		fieldName, fieldType := c.field(name)
		switch fieldType {
		case timeseries.FieldTypeString, timeseries.FieldTypeJSON:
			if hasTokenSeparator(template) {
				fieldConds = append(fieldConds,
					fmt.Sprintf("countSubstrings(lowerUTF8(%s), '%s')",
						fieldName,
						strings.ToLower(template)))
			} else {
				fieldConds = append(fieldConds,
					fmt.Sprintf("hasToken(lowerUTF8(%s), '%s')",
						fieldName,
						strings.ToLower(template)))
			}
		case timeseries.FieldTypeInt, timeseries.FieldTypeFloat, timeseries.FieldTypeBigInt, timeseries.FieldTypeBigFloat:
			if f, err := strconv.ParseFloat(template, 64); err == nil {
				fieldConds = append(fieldConds,
					fmt.Sprintf("abs(minus(%s, %f))<%f",
						fieldName,
						f, threshold))
			}
		case timeseries.FieldTypeTime:
			if t, err := time.Parse(time.DateTime, template); err == nil {
				fieldConds = append(fieldConds,
					fmt.Sprintf("equals(%s, toDateTime('%s'))",
						fieldName,
						t.Format(time.DateTime)))
			}
		case timeseries.FieldTypeBool:
			if template == "true" {
				fieldConds = append(fieldConds,
					fmt.Sprintf("equals(%s, true)",
						fieldName))
			} else if template == "false" {
				fieldConds = append(fieldConds,
					fmt.Sprintf("equals(%s, false)",
						fieldName))
			}
		}
	}
	fieldConds = append(fieldConds, fmt.Sprintf("hasToken(lowerUTF8(%s::String), lower('%s'))", c.prefix, strings.ToLower(template)))
	if len(fieldConds) == 0 {
		return "(true)", nil
	}
	return "(" + strings.Join(fieldConds, " OR ") + ")", nil
}

func (c *clickhouse) matchPhraseQuery(qt query.MatchPhraseQuery) (s string, err error) {
	name := qt.Field()
	if name == "" {
		return c.fulltextQuery(qt.MatchPhrase)
	}
	fieldName, fieldType := c.field(name)
	switch fieldType {
	case timeseries.FieldTypeString, timeseries.FieldTypeJSON:
		return fmt.Sprintf("equals(lowerUTF8(%s), '%s')",
			fieldName, strings.ToLower(qt.MatchPhrase)), nil
	default:
		return fmt.Sprintf("equals(%s, %s)",
			fieldName, qt.MatchPhrase), nil
	}
}

func (c *clickhouse) wildcardQuery(qt query.WildcardQuery) (s string, err error) {
	name := qt.Field()
	if name == "" {
		return c.fulltextQuery(qt.Wildcard)
	}
	fieldName, fieldType := c.field(name)
	switch fieldType {
	case timeseries.FieldTypeString, timeseries.FieldTypeJSON:
		if hasTokenSeparator(qt.Wildcard) {
			return fmt.Sprintf("countSubstrings(lowerUTF8(%s), '%s')",
				fieldName, strings.ToLower(qt.Wildcard)), nil
		} else {
			return fmt.Sprintf("hasToken(lowerUTF8(%s), '%s')",
				fieldName, strings.ToLower(qt.Wildcard)), nil
		}
	default:
		return "", errors.Errorf("wildcard query is not supported for non-string field %s", qt.Field())
	}
}

func (c *clickhouse) regexpQuery(qt query.RegexpQuery) (s string, err error) {
	name := qt.Field()
	if name == "" {
		return c.fulltextQuery(qt.Regexp)
	}
	fieldName, fieldType := c.field(name)
	switch fieldType {
	case timeseries.FieldTypeString, timeseries.FieldTypeJSON:
		return fmt.Sprintf("match(lowerUTF8(%s), '%s')",
			fieldName, strings.ToLower(qt.Regexp)), nil
	default:
		return "", errors.Errorf("regexp query is not supported for non-string field %s", qt.Field())
	}
}

func (c *clickhouse) conjunctionQuery(qt query.ConjunctionQuery) (s string, err error) {
	if len(qt.Conjuncts) == 1 {
		return c.Render(qt.Conjuncts[0])
	}
	conjuncts, err := utils.MapSlice(qt.Conjuncts, func(q query.Query) (string, error) {
		return c.Render(q)
	})
	if err != nil {
		return "", err
	}
	return "(" + strings.Join(conjuncts, " AND ") + ")", nil
}

func (c *clickhouse) disjunctionQuery(qt query.DisjunctionQuery) (s string, err error) {
	if len(qt.Disjuncts) == 1 {
		return c.Render(qt.Disjuncts[0])
	}
	disjuncts, err := utils.MapSlice(qt.Disjuncts, func(q query.Query) (string, error) {
		return c.Render(q)
	})
	if err != nil {
		return "", err
	}
	return "(" + strings.Join(disjuncts, " OR ") + ")", nil
}

func (c *clickhouse) booleanQuery(qt query.BooleanQuery) (s string, err error) {
	var conds []string
	if qt.Must != nil {
		must, err := c.Render(qt.Must)
		if err != nil {
			return "", err
		}
		conds = append(conds, must)
	}
	if qt.MustNot != nil {
		mustNot, err := c.Render(qt.MustNot)
		if err != nil {
			return "", err
		}
		conds = append(conds, "NOT("+mustNot+")")
	}
	if qt.Should != nil {
		should, err := c.Render(qt.Should)
		if err != nil {
			return "", err
		}
		conds = append(conds, should)
	}
	if len(conds) == 0 {
		return "(true)", nil
	}
	return "(" + strings.Join(conds, " AND ") + ")", nil
}

func (c *clickhouse) numericRangeQuery(qt query.NumericRangeQuery) (s string, err error) {
	var min, max = qt.Min, qt.Max
	if qt.Field() == "" {
		return "", errors.Errorf("field name is required for range query")
	}
	fieldName, fieldType := c.field(qt.Field())
	if fieldType != timeseries.FieldTypeInt && fieldType != timeseries.FieldTypeFloat && fieldType != timeseries.FieldTypeBigInt && fieldType != timeseries.FieldTypeBigFloat {
		return "", errors.Errorf("field %s is not numeric type", qt.Field())
	}
	var fieldConds []string
	if min != nil {
		if qt.InclusiveMin != nil && !*qt.InclusiveMin {
			fieldConds = append(fieldConds, fmt.Sprintf("greater(%s, %f)",
				fieldName, *min))
		} else {
			fieldConds = append(fieldConds, fmt.Sprintf("greaterOrEquals(%s, %f)",
				fieldName, *min))
		}
	}
	if max != nil {
		if qt.InclusiveMax != nil && !*qt.InclusiveMax {
			fieldConds = append(fieldConds, fmt.Sprintf("less(%s, %f)",
				fieldName, *max))
		} else {
			fieldConds = append(fieldConds, fmt.Sprintf("lessOrEquals(%s, %f)",
				fieldName, *max))
		}
	}
	if len(fieldConds) == 0 {
		return "(true)", nil
	}
	return "(" + strings.Join(fieldConds, " AND ") + ")", nil
}

func (c *clickhouse) termRangeQuery(qt query.TermRangeQuery) (s string, err error) {
	var min, max = qt.Min, qt.Max
	if qt.Field() == "" {
		return "", errors.Errorf("field name is required for range query")
	}
	var fieldConds []string
	fieldName, fieldType := c.field(qt.Field())
	if fieldType != timeseries.FieldTypeString && fieldType != timeseries.FieldTypeJSON {
		return "", errors.Errorf("field %s is not string type", qt.Field())
	}
	if min != "" {
		if qt.InclusiveMin != nil && !*qt.InclusiveMin {
			fieldConds = append(fieldConds, fmt.Sprintf("greater(%s, '%s')",
				fieldName, min))
		} else {
			fieldConds = append(fieldConds, fmt.Sprintf("greaterOrEquals(%s, '%s')",
				fieldName, min))
		}
	}
	if max != "" {
		if qt.InclusiveMax != nil && !*qt.InclusiveMax {
			fieldConds = append(fieldConds, fmt.Sprintf("less(%s, '%s')",
				fieldName, max))
		} else {
			fieldConds = append(fieldConds, fmt.Sprintf("lessOrEquals(%s, '%s')",
				fieldName, max))
		}
	}
	if len(fieldConds) == 0 {
		return "(true)", nil
	}
	return "(" + strings.Join(fieldConds, " AND ") + ")", nil
}

func (c *clickhouse) dateRangeQuery(qt query.DateRangeQuery) (s string, err error) {
	var min, max = qt.Start, qt.End
	if qt.Field() == "" {
		return "", errors.Errorf("field name is required for range query")
	}
	var fieldConds []string
	fieldName, fieldType := c.field(qt.Field())
	if fieldType != timeseries.FieldTypeTime {
		return "", errors.Errorf("field %s is not time type", qt.Field())
	}
	if !min.IsZero() {
		if qt.InclusiveStart != nil && !*qt.InclusiveStart {
			fieldConds = append(fieldConds,
				fmt.Sprintf("greater(%s, toDateTime('%s'))",
					fieldName,
					min.Format(time.DateTime)))
		} else {
			fieldConds = append(fieldConds,
				fmt.Sprintf("greaterOrEquals(%s, toDateTime('%s'))",
					fieldName,
					min.Format(time.DateTime)))
		}
	}
	if !max.IsZero() {
		if qt.InclusiveEnd != nil && !*qt.InclusiveEnd {
			fieldConds = append(fieldConds,
				fmt.Sprintf("less(%s, toDateTime('%s'))",
					fieldName,
					max.Format(time.DateTime)))
		} else {
			fieldConds = append(fieldConds,
				fmt.Sprintf("lessOrEquals(%s, toDateTime('%s'))",
					fieldName,
					max.Format(time.DateTime)))
		}
	}
	if len(fieldConds) == 0 {
		return "(true)", nil
	}
	return "(" + strings.Join(fieldConds, " AND ") + ")", nil
}
