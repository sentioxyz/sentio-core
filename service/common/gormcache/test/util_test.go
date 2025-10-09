package test

import (
	"sentioxyz/sentio-core/service/common/gormcache"
	"testing"
)

func TestParseWhereClause(t *testing.T) {
	sql := "name = 'John' AND age >= ? AND city IN (?, ?, ?) OR state = 'NY'"
	expected := []gormcache.WhereClause[string]{
		{Column: "name", Operator: "=", Value: "John"},
		{Column: "age", Operator: ">=", Value: "?"},
		{Column: "city", Operator: "in", ValueList: []string{"?", "?", "?"}},
		{Column: "state", Operator: "=", Value: "NY"},
	}

	actual, err := gormcache.ParseWhereClause(sql)

	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}

	if len(expected) != len(actual) {
		t.Errorf("Expected %d clauses, but got %d", len(expected), len(actual))
	}

	for i, exp := range expected {
		act := actual[i]
		if exp.Column != act.Column {
			t.Errorf("Expected column '%s', but got '%s'", exp.Column, act.Column)
		}
		if exp.Operator != act.Operator {
			t.Errorf("Expected operator '%s', but got '%s'", exp.Operator, act.Operator)
		}
		if exp.Value != act.Value {
			t.Errorf("Expected value '%s', but got '%s'", exp.Value, act.Value)
		}
		if len(exp.ValueList) != len(act.ValueList) {
			t.Errorf("Expected %d value list items, but got %d", len(exp.ValueList), len(act.ValueList))
		} else {
			for j, expVal := range exp.ValueList {
				actVal := act.ValueList[j]
				if expVal != actVal {
					t.Errorf("Expected value '%s', but got '%s'", expVal, actVal)
				}
			}
		}
	}
}
