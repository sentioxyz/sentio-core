package adaptor_eventlogs

import (
	"context"
	"testing"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/protos"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func createMockMeta() timeseries.Meta {
	return timeseries.Meta{
		Fields: map[string]timeseries.Field{
			"string_field": {
				Name: "string_field",
				Type: timeseries.FieldTypeString,
			},
			"numeric_field": {
				Name: "numeric_field",
				Type: timeseries.FieldTypeInt,
			},
			"time_field": {
				Name: "time_field",
				Type: timeseries.FieldTypeTime,
			},
			"array_field": {
				Name: "array_field",
				Type: timeseries.FieldTypeArray,
			},
			"nested_field": {
				Name: "nested_field",
				Type: timeseries.FieldTypeJSON,
				NestedStructSchema: map[string]timeseries.FieldType{
					"nested_field":                      timeseries.FieldTypeJSON,
					"nested_field.string_field":         timeseries.FieldTypeString,
					"nested_field.json.x.numeric_field": timeseries.FieldTypeBigFloat,
					"nested_field.json":                 timeseries.FieldTypeJSON,
					"nested_field.json.x":               timeseries.FieldTypeJSON,
					"nested_field.amount":               timeseries.FieldTypeToken,
				},
			},
			timeseries.SystemFieldPrefix + "chain": {
				Name:    timeseries.SystemFieldPrefix + "chain",
				Type:    timeseries.FieldTypeString,
				BuiltIn: true,
			},
			timeseries.SystemFieldPrefix + "block_number": {
				Name:    timeseries.SystemFieldPrefix + "block_number",
				Type:    timeseries.FieldTypeInt,
				BuiltIn: true,
			},
			timeseries.SystemFieldPrefix + "block_hash": {
				Name:    timeseries.SystemFieldPrefix + "block_hash",
				Type:    timeseries.FieldTypeString,
				BuiltIn: true,
			},
			timeseries.SystemFieldPrefix + "transaction_hash": {
				Name:    timeseries.SystemFieldPrefix + "transaction_hash",
				Type:    timeseries.FieldTypeString,
				BuiltIn: true,
			},
			timeseries.SystemFieldPrefix + "transaction_index": {
				Name:    timeseries.SystemFieldPrefix + "transaction_index",
				Type:    timeseries.FieldTypeInt,
				BuiltIn: true,
			},
			timeseries.SystemFieldPrefix + "log_index": {
				Name:    timeseries.SystemFieldPrefix + "log_index",
				Type:    timeseries.FieldTypeInt,
				BuiltIn: true,
			},
			timeseries.SystemUserID: {
				Name: timeseries.SystemUserID,
				Type: timeseries.FieldTypeString,
			},
			timeseries.SystemTimestamp: {
				Name:    timeseries.SystemTimestamp,
				Type:    timeseries.FieldTypeTime,
				BuiltIn: true,
			},
		},
	}
}

func createAnyValue(value interface{}) *protos.Any {
	switch v := value.(type) {
	case string:
		return &protos.Any{AnyValue: &protos.Any_StringValue{StringValue: v}}
	case int32:
		return &protos.Any{AnyValue: &protos.Any_IntValue{IntValue: v}}
	case float64:
		return &protos.Any{AnyValue: &protos.Any_DoubleValue{DoubleValue: v}}
	case bool:
		return &protos.Any{AnyValue: &protos.Any_BoolValue{BoolValue: v}}
	case []string:
		return &protos.Any{
			AnyValue: &protos.Any_ListValue{
				ListValue: &protos.StringList{
					Values: v,
				},
			},
		}
	default:
		return &protos.Any{AnyValue: &protos.Any_StringValue{StringValue: "default"}}
	}
}

func TestNewSelectorExpression(t *testing.T) {
	ctx := context.Background()
	meta := createMockMeta()

	selectorExpr := &protos.SelectorExpr{}

	selector := NewSelectorExpression2(ctx, selectorExpr, meta)

	assert.NotNil(t, selector)
	assert.Equal(t, nilSelector, selector.String())
	assert.Nil(t, selector.Cond())
	assert.NoError(t, selector.Error())
}

func TestExpression_verifySelector_NilSelector(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	key, err := expr.verifySelector(nil)
	assert.Equal(t, nilKey, key)
	assert.Equal(t, ErrNilSelector, err)
}

func TestExpression_verifySelector_IgnoreUnknownField(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "unknown_field",
		Operator: protos.Selector_EQ,
		Value:    []*protos.Any{createAnyValue("test")},
	}

	key, err := expr.verifySelector(selector)
	assert.Equal(t, ignoreSelector, key)
	assert.NoError(t, err)
}

func TestExpression_verifySelector_ExistsOperator(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_EXISTS,
	}

	key, err := expr.verifySelector(selector)
	assert.Equal(t, "CAST(`string_field`, 'String')", key)
	assert.NoError(t, err)
}

func TestExpression_verifySelector_InOperator_Success(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_IN,
		Value:    []*protos.Any{createAnyValue("test1"), createAnyValue("test2")},
	}

	key, err := expr.verifySelector(selector)
	assert.Equal(t, "CAST(`string_field`, 'String')", key)
	assert.NoError(t, err)
}

func TestExpression_verifySelector_InOperator_KeyTypeMismatch(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "array_field", // not an array field
		Operator: protos.Selector_IN,
		Value:    []*protos.Any{createAnyValue("test1"), createAnyValue("test2")},
	}

	key, err := expr.verifySelector(selector)
	assert.Equal(t, nilKey, key)
	assert.Equal(t, ErrSelectorKeyOperatorTypeNotMatch, err)
}

func TestExpression_verifySelector_InOperator_NilValue(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_IN,
		Value:    []*protos.Any{},
	}

	key, err := expr.verifySelector(selector)
	assert.Equal(t, nilKey, key)
	assert.Equal(t, ErrSelectorNilValue, err)
}

func TestExpression_verifySelector_ComparisonOperators_Success(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	testCases := []struct {
		name     string
		operator protos.Selector_OperatorType
		field    string
		key      string
	}{
		{"GT_numeric", protos.Selector_GT, "numeric_field", "CAST(`numeric_field`, 'Int64')"},
		{"GTE_numeric", protos.Selector_GTE, "numeric_field", "CAST(`numeric_field`, 'Int64')"},
		{"LT_string", protos.Selector_LT, "string_field", "CAST(`string_field`, 'String')"},
		{"LTE_time", protos.Selector_LTE, "time_field", "`time_field`::DateTime64(6, 'UTC')"},
		{"GT_nested_numeric", protos.Selector_GT, "nested_field.nested_field.json.x.numeric_field", "CAST(`nested_field`.`nested_field`.`json`.`x`.`numeric_field`, 'Decimal(76, 30)')"},
		{"LT_nested_string", protos.Selector_LT, "nested_field.nested_field.string_field", "CAST(`nested_field`.`nested_field`.`string_field`, 'String')"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selector := &protos.Selector{
				Key:      tc.field,
				Operator: tc.operator,
				Value:    []*protos.Any{createAnyValue(100.0)},
			}

			key, err := expr.verifySelector(selector)
			assert.Equal(t, tc.key, key)
			assert.NoError(t, err)
		})
	}
}

func TestExpression_verifySelector_BetweenOperator_Success(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "numeric_field",
		Operator: protos.Selector_BETWEEN,
		Value:    []*protos.Any{createAnyValue(10.0), createAnyValue(20.0)},
	}

	key, err := expr.verifySelector(selector)
	assert.Equal(t, "CAST(`numeric_field`, 'Int64')", key)
	assert.NoError(t, err)
}

func TestExpression_verifySelector_BetweenOperator_WrongValueCount(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "numeric_field",
		Operator: protos.Selector_BETWEEN,
		Value:    []*protos.Any{createAnyValue(10.0)}, // should be 2 values
	}

	key, err := expr.verifySelector(selector)
	assert.Equal(t, nilKey, key)
	assert.Equal(t, ErrSelectorOperatorValueTypeNotMatch, err)
}

func TestExpression_verifySelector_ContainsOperator_Success(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_CONTAINS,
		Value:    []*protos.Any{createAnyValue("test")},
	}

	key, err := expr.verifySelector(selector)
	assert.Equal(t, "CAST(`string_field`, 'String')", key)
	assert.NoError(t, err)
}

func TestExpression_verifySelector_ContainsOperator_NonStringField(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "numeric_field", // not a string field
		Operator: protos.Selector_CONTAINS,
		Value:    []*protos.Any{createAnyValue("test")},
	}

	key, err := expr.verifySelector(selector)
	assert.Equal(t, nilKey, key)
	assert.Equal(t, ErrSelectorKeyOperatorTypeNotMatch, err)
}

func TestExpression_buildSelector_SimpleSelector(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_EQ,
		Value:    []*protos.Any{createAnyValue("test_value")},
	}

	result := expr.buildSelector(selector)
	expected := "equals(CAST(`string_field`, 'String'), 'test_value')"
	assert.Equal(t, expected, result)
	assert.NoError(t, expr.Error())
}

func TestExpression_buildSelector_UnsupportedOperator(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "string_field",
		Operator: 999, // unsupported operator
		Value:    []*protos.Any{createAnyValue("test")},
	}

	result := expr.buildSelector(selector)
	assert.Equal(t, nilSelector, result)
	assert.Error(t, expr.Error())
}

func TestExpression_String_NilSelectorExpr(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:          ctx,
		logger:       logger,
		selectorExpr: nil,
		meta:         meta,
	}

	result := expr.String()
	assert.Equal(t, nilSelector, result)
}

func TestExpression_String_SingleSelector(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	selector := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_EQ,
		Value:    []*protos.Any{createAnyValue("test")},
	}

	selectorExpr := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_Selector{Selector: selector},
	}

	expr := &Expression{
		ctx:          ctx,
		logger:       logger,
		selectorExpr: selectorExpr,
		meta:         meta,
	}

	result := expr.String()
	expected := "equals(CAST(`string_field`, 'String'), 'test')"
	assert.Equal(t, expected, result)
}

func TestExpression_String_LogicExprAND(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	selector1 := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_EQ,
		Value:    []*protos.Any{createAnyValue("test1")},
	}

	selector2 := &protos.Selector{
		Key:      "numeric_field",
		Operator: protos.Selector_GT,
		Value:    []*protos.Any{createAnyValue(10.0)},
	}

	selectorExpr1 := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_Selector{Selector: selector1},
	}

	selectorExpr2 := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_Selector{Selector: selector2},
	}

	logicExpr := &protos.SelectorExpr_LogicExpr{
		Operator:    protos.JoinOperator_AND,
		Expressions: []*protos.SelectorExpr{selectorExpr1, selectorExpr2},
	}

	selectorExpr := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_LogicExpr_{LogicExpr: logicExpr},
	}

	expr := &Expression{
		ctx:          ctx,
		logger:       logger,
		selectorExpr: selectorExpr,
		meta:         meta,
	}

	result := expr.String()
	assert.Contains(t, result, "AND")
	assert.Contains(t, result, "equals(CAST(`string_field`, 'String'), 'test1')")
	assert.Contains(t, result, "greater(CAST(`numeric_field`, 'Int64'), toDecimal256OrZero('10.000000', 30))")
}

func TestExpression_String_LogicExprOR(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	selector1 := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_EQ,
		Value:    []*protos.Any{createAnyValue("test1")},
	}

	selector2 := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_EQ,
		Value:    []*protos.Any{createAnyValue("test2")},
	}

	selectorExpr1 := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_Selector{Selector: selector1},
	}

	selectorExpr2 := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_Selector{Selector: selector2},
	}

	logicExpr := &protos.SelectorExpr_LogicExpr{
		Operator:    protos.JoinOperator_OR,
		Expressions: []*protos.SelectorExpr{selectorExpr1, selectorExpr2},
	}

	selectorExpr := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_LogicExpr_{LogicExpr: logicExpr},
	}

	expr := &Expression{
		ctx:          ctx,
		logger:       logger,
		selectorExpr: selectorExpr,
		meta:         meta,
	}

	result := expr.String()
	assert.Contains(t, result, "OR")
	assert.Contains(t, result, "equals(CAST(`string_field`, 'String'), 'test1')")
	assert.Contains(t, result, "equals(CAST(`string_field`, 'String'), 'test2')")
}

func TestExpression_newCond_AllOperators(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	testCases := []struct {
		name     string
		operator protos.Selector_OperatorType
		field    string
		values   []*protos.Any
	}{
		{"EQ", protos.Selector_EQ, "string_field", []*protos.Any{createAnyValue("test")}},
		{"NEQ", protos.Selector_NEQ, "string_field", []*protos.Any{createAnyValue("test")}},
		{"EXISTS", protos.Selector_EXISTS, "string_field", nil},
		{"NOT_EXISTS", protos.Selector_NOT_EXISTS, "string_field", nil},
		{"GT", protos.Selector_GT, "numeric_field", []*protos.Any{createAnyValue(10.0)}},
		{"GTE", protos.Selector_GTE, "numeric_field", []*protos.Any{createAnyValue(10.0)}},
		{"LT", protos.Selector_LT, "numeric_field", []*protos.Any{createAnyValue(10.0)}},
		{"LTE", protos.Selector_LTE, "numeric_field", []*protos.Any{createAnyValue(10.0)}},
		{"BETWEEN", protos.Selector_BETWEEN, "numeric_field", []*protos.Any{createAnyValue(5.0), createAnyValue(15.0)}},
		{"NOT_BETWEEN", protos.Selector_NOT_BETWEEN, "numeric_field", []*protos.Any{createAnyValue(5.0), createAnyValue(15.0)}},
		{"CONTAINS", protos.Selector_CONTAINS, "string_field", []*protos.Any{createAnyValue("test")}},
		{"NOT_CONTAINS", protos.Selector_NOT_CONTAINS, "string_field", []*protos.Any{createAnyValue("test")}},
		{"IN", protos.Selector_IN, "string_field", []*protos.Any{createAnyValue([]string{"test1", "test2"})}},
		{"NOT_IN", protos.Selector_NOT_IN, "string_field", []*protos.Any{createAnyValue([]string{"test1", "test2"})}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selector := &protos.Selector{
				Key:      tc.field,
				Operator: tc.operator,
				Value:    tc.values,
			}

			cond := expr.newCond(selector)
			if tc.operator == protos.Selector_EXISTS || tc.operator == protos.Selector_NOT_EXISTS {
				if tc.field != "unknown_field" {
					assert.NotNil(t, cond, "condition should not be nil for %s", tc.name)
				}
			} else {
				assert.NotNil(t, cond, "condition should not be nil for %s", tc.name)
			}
			assert.NoError(t, expr.Error())
		})
	}
}

func TestExpression_newCond_UnsupportedOperator(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
	}

	selector := &protos.Selector{
		Key:      "string_field",
		Operator: 999, // unsupported operator
		Value:    []*protos.Any{createAnyValue("test")},
	}

	cond := expr.newCond(selector)
	assert.Nil(t, cond)
	assert.Error(t, expr.Error())
}

func TestExpression_Cond_NilSelectorExpr(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:          ctx,
		logger:       logger,
		selectorExpr: nil,
		meta:         meta,
	}

	cond := expr.Cond()
	assert.Nil(t, cond)
}

func TestExpression_Cond_SingleSelector(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	selector := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_EQ,
		Value:    []*protos.Any{createAnyValue("test")},
	}

	selectorExpr := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_Selector{Selector: selector},
	}

	expr := &Expression{
		ctx:          ctx,
		logger:       logger,
		selectorExpr: selectorExpr,
		meta:         meta,
	}

	cond := expr.Cond()
	assert.NotNil(t, cond)
}

func TestExpression_Cond_LogicExprAND(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	selector1 := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_EQ,
		Value:    []*protos.Any{createAnyValue("test1")},
	}

	selector2 := &protos.Selector{
		Key:      "numeric_field",
		Operator: protos.Selector_GT,
		Value:    []*protos.Any{createAnyValue(10.0)},
	}

	selectorExpr1 := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_Selector{Selector: selector1},
	}

	selectorExpr2 := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_Selector{Selector: selector2},
	}

	logicExpr := &protos.SelectorExpr_LogicExpr{
		Operator:    protos.JoinOperator_AND,
		Expressions: []*protos.SelectorExpr{selectorExpr1, selectorExpr2},
	}

	selectorExpr := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_LogicExpr_{LogicExpr: logicExpr},
	}

	expr := &Expression{
		ctx:          ctx,
		logger:       logger,
		selectorExpr: selectorExpr,
		meta:         meta,
	}

	cond := expr.Cond()
	assert.NotNil(t, cond)
}

func TestExpression_Cond_LogicExprOR(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	selector1 := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_EQ,
		Value:    []*protos.Any{createAnyValue("test1")},
	}

	selector2 := &protos.Selector{
		Key:      "string_field",
		Operator: protos.Selector_EQ,
		Value:    []*protos.Any{createAnyValue("test2")},
	}

	selectorExpr1 := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_Selector{Selector: selector1},
	}

	selectorExpr2 := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_Selector{Selector: selector2},
	}

	logicExpr := &protos.SelectorExpr_LogicExpr{
		Operator:    protos.JoinOperator_OR,
		Expressions: []*protos.SelectorExpr{selectorExpr1, selectorExpr2},
	}

	selectorExpr := &protos.SelectorExpr{
		Expr: &protos.SelectorExpr_LogicExpr_{LogicExpr: logicExpr},
	}

	expr := &Expression{
		ctx:          ctx,
		logger:       logger,
		selectorExpr: selectorExpr,
		meta:         meta,
	}

	cond := expr.Cond()
	assert.NotNil(t, cond)
}

func TestExpression_Error(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	meta := createMockMeta()

	expr := &Expression{
		ctx:    ctx,
		logger: logger,
		meta:   meta,
		err:    errors.New("test error"),
	}

	assert.Error(t, expr.Error())
	assert.Equal(t, "test error", expr.Error().Error())
}

func TestOperatorMap_AllOperators(t *testing.T) {
	expectedOperators := map[protos.Selector_OperatorType]string{
		protos.Selector_EQ:           integerEqualTpl,
		protos.Selector_NEQ:          integerNotEqualTpl,
		protos.Selector_EXISTS:       existsTpl,
		protos.Selector_NOT_EXISTS:   notExistsTpl,
		protos.Selector_GT:           greaterThanTpl,
		protos.Selector_GTE:          greaterEqualThanTpl,
		protos.Selector_LT:           lessThanTpl,
		protos.Selector_LTE:          lessEqualThanTpl,
		protos.Selector_BETWEEN:      betweenTpl,
		protos.Selector_NOT_BETWEEN:  notBetweenTpl,
		protos.Selector_CONTAINS:     containsTpl,
		protos.Selector_NOT_CONTAINS: notContainsTpl,
		protos.Selector_IN:           inTpl,
		protos.Selector_NOT_IN:       notInTpl,
	}

	for op, template := range expectedOperators {
		actualTemplate, exists := operatorMap[op]
		assert.True(t, exists, "operator %s should exist in operatorMap", op.String())
		assert.Equal(t, template, actualTemplate, "template for operator %s should match", op.String())
	}
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "1", nilSelector)
	assert.Equal(t, "<nil>", nilKey)
	assert.Equal(t, "ignore", ignoreSelector)
}

func TestErrors(t *testing.T) {
	assert.NotNil(t, ErrNilSelector)
	assert.NotNil(t, ErrSelectorKeyOperatorTypeNotMatch)
	assert.NotNil(t, ErrSelectorOperatorValueTypeNotMatch)
	assert.NotNil(t, ErrSelectorNilValue)
	assert.NotNil(t, ErrSelectorArrayValueNotSupport)
}
