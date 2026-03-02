package cascade_function

import (
	"fmt"
	"strings"

	"sentioxyz/sentio-core/common/gonanoid"
	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/driver/timeseries"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"

	"github.com/samber/lo"
)

type Functions interface {
	Add(f prebuilt.Function)
	AddCustomAlias(f prebuilt.Function, resultAlias string)

	Generate() (string, error)
	Snippets() (map[string]string, error)
	LastTableAlias() string
	LastValueAlias() string
}

type cascadeFunctions struct {
	functions          []prebuilt.Function
	functionTableAlias []string
}

func NewCascadeFunctions() Functions {
	return &cascadeFunctions{}
}

func (f *cascadeFunctions) Add(current prebuilt.Function) {
	var lastResultAlias, tableName string
	if len(f.functions) > 0 {
		lastResultAlias = f.functions[len(f.functions)-1].GetResultAlias()
		tableName = f.functionTableAlias[len(f.functionTableAlias)-1]
	} else {
		lastResultAlias = timeseries.MetricValueFieldName
	}

	current.WithValueField(lastResultAlias)
	if tableName != "" {
		current.WithTable(tableName)
	}
	f.functions = append(f.functions, current)
	f.functionTableAlias = append(f.functionTableAlias, "query_"+gonanoid.Must(5))
}

func (f *cascadeFunctions) AddCustomAlias(current prebuilt.Function, resultAlias string) {
	if len(f.functions) > 0 {
		current.WithTable(f.functionTableAlias[len(f.functionTableAlias)-1])
	}
	current.WithValueField(resultAlias)
	f.functions = append(f.functions, current)
	f.functionTableAlias = append(f.functionTableAlias, "query_"+gonanoid.Must(5))
}

func (f *cascadeFunctions) Snippets() (map[string]string, error) {
	if len(f.functions) == 0 {
		return nil, fmt.Errorf("no functions to generate")
	}
	var cteQueries = make(map[string]string)
	for i := 0; i < len(f.functions); i++ {
		current := f.functions[i]
		cteQuery, err := current.Generate()
		if err != nil {
			return nil, err
		}
		cteQueries[f.functionTableAlias[i]] = cteQuery
	}
	return cteQueries, nil
}

func (f *cascadeFunctions) LastTableAlias() string {
	if len(f.functionTableAlias) == 0 {
		return ""
	}
	return f.functionTableAlias[len(f.functionTableAlias)-1]
}

func (f *cascadeFunctions) Generate() (string, error) {
	cteQueries, err := f.Snippets()
	if err != nil {
		return "", err
	}
	cteQuerySlice := lo.MapToSlice(cteQueries, func(k, v string) string {
		return k + " AS (" + v + ")"
	})

	const tpl = "WITH {cte_query} SELECT * FROM {last_cte_alias}"
	return builder.FormatSQLTemplate(tpl, map[string]any{
		"cte_query":      strings.Join(cteQuerySlice, ","),
		"last_cte_alias": f.LastTableAlias(),
	}), nil
}

func (f *cascadeFunctions) LastValueAlias() string {
	if len(f.functions) == 0 {
		return timeseries.MetricValueFieldName
	}
	return f.functions[len(f.functions)-1].GetResultAlias()
}
