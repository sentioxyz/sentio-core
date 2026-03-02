package adaptor_eventlogs

import (
	"context"
	"fmt"
	"strings"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/sqlbuilder/condition"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/driver/timeseries/clickhouse"
	"sentioxyz/sentio-core/service/common/protos"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

type Selector interface {
	String() string
	Cond() *condition.Cond
	Error() error
}

const (
	nilSelector    = "1"
	nilKey         = "<nil>"
	ignoreSelector = "ignore"
)

const (
	integerEqualTpl     = "equals(%s, %s)"
	integerNotEqualTpl  = "notEquals(%s, %s)"
	existsTpl           = "%s IS NOT NULL"
	notExistsTpl        = "%s IS NULL"
	greaterThanTpl      = "greater(%s, %s)"
	greaterEqualThanTpl = "greaterOrEquals(%s, %s)"
	lessThanTpl         = "less(%s, %s)"
	lessEqualThanTpl    = "lessOrEquals(%s, %s)"
	betweenTpl          = "%s BETWEEN %s AND %s"
	notBetweenTpl       = "%s NOT BETWEEN %s AND %s"
	containsTpl         = "like(%s, concat('%%',%s,'%%'))"
	notContainsTpl      = "notLike(%s, concat('%%',%s,'%%'))"
	inTpl               = "%s IN (%s)"
	notInTpl            = "%s NOT IN (%s)"
)

var operatorMap = map[protos.Selector_OperatorType]string{
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

var (
	ErrNilSelector                       = errors.Errorf("clickhouse builder: nil selector")
	ErrSelectorKeyOperatorTypeNotMatch   = errors.Errorf("clickhouse builder: selector key operator type not match")
	ErrSelectorOperatorValueTypeNotMatch = errors.Errorf("clickhouse builder: selector operator value type not match")
	ErrSelectorNilValue                  = errors.Errorf("clickhouse builder: selector value is nil")
	ErrSelectorArrayValueNotSupport      = errors.Errorf("clickhouse builder: selector array value is not supported")
)

func migrateSelector(old *protos.SegmentationQuery_SelectorExpr, new *protos.SelectorExpr) error {
	data, err := proto.Marshal(old)
	if err != nil {
		return err
	}
	return proto.Unmarshal(data, new)
}

type Expression struct {
	ctx          context.Context
	logger       *log.SentioLogger
	selectorExpr *protos.SelectorExpr
	meta         timeseries.Meta
	err          error
}

func NewSelectorExpression(ctx context.Context,
	selectorExpr *protos.SegmentationQuery_SelectorExpr,
	meta timeseries.Meta) Selector {
	ctx, logger := log.FromContext(ctx)
	var newSelectorExpr = new(protos.SelectorExpr)
	if err := migrateSelector(selectorExpr, newSelectorExpr); err != nil {
		panic(err)
	}
	return &Expression{
		ctx:          ctx,
		logger:       logger,
		selectorExpr: newSelectorExpr,
		meta:         meta,
		err:          nil,
	}
}

func NewSelectorExpression2(ctx context.Context,
	selectorExpr *protos.SelectorExpr,
	meta timeseries.Meta) Selector {
	ctx, logger := log.FromContext(ctx)
	return &Expression{
		ctx:          ctx,
		logger:       logger,
		selectorExpr: selectorExpr,
		meta:         meta,
	}
}

func (s *Expression) verifySelector(selector *protos.Selector) (string, error) {
	if selector == nil {
		return nilKey, ErrNilSelector
	}
	fieldType, ok := s.meta.GetFieldType(selector.GetKey())
	if !ok {
		return ignoreSelector, nil
	}
	switch selector.GetOperator() {
	case protos.Selector_EXISTS, protos.Selector_NOT_EXISTS:
		s.logger.Debugf("ignore selector value for operator: %s", selector.GetOperator().String())
	case protos.Selector_IN, protos.Selector_NOT_IN:
		switch {
		case s.meta.IsToken(selector.GetKey()), s.meta.IsArray(selector.GetKey()):
			s.logger.Warnf("selector operator is not matched with key type"+
				", key: %s, type: %s, operator: %s",
				selector.GetKey(), fieldType, selector.GetOperator().String())
			return nilKey, ErrSelectorKeyOperatorTypeNotMatch
		}
		if len(selector.GetValue()) == 0 {
			return nilKey, ErrSelectorNilValue
		}
	case protos.Selector_GT,
		protos.Selector_GTE,
		protos.Selector_LT,
		protos.Selector_LTE,
		protos.Selector_BETWEEN,
		protos.Selector_NOT_BETWEEN:
		if len(selector.GetValue()) == 0 {
			return nilKey, ErrSelectorNilValue
		}
		switch {
		case s.meta.IsNumeric(selector.GetKey()), s.meta.IsTime(selector.GetKey()), s.meta.IsString(selector.GetKey()):
			// do nothing
		default:
			s.logger.Warnf("selector operator is not matched with key type"+
				", key: %s, type: %s, operator: %s",
				selector.GetKey(), fieldType, selector.GetOperator().String())
			return nilKey, ErrSelectorKeyOperatorTypeNotMatch
		}
		if op := selector.GetOperator(); op == protos.Selector_BETWEEN ||
			op == protos.Selector_NOT_BETWEEN {
			if len(selector.GetValue()) != 2 {
				log.Warnf("selector operator is not matched with value count"+
					", key: %s, operator: %s, value count: %d",
					selector.GetKey(), selector.GetOperator().String(), len(selector.GetValue()))
				return nilKey, ErrSelectorOperatorValueTypeNotMatch
			}
		}
	case protos.Selector_CONTAINS,
		protos.Selector_NOT_CONTAINS:
		if len(selector.GetValue()) == 0 {
			return nilKey, ErrSelectorNilValue
		}
		if !s.meta.IsString(selector.GetKey()) {
			s.logger.Warnf("selector operator is not matched with key type"+
				", key: %s, type: %s, operator: %s",
				selector.GetKey(), fieldType, selector.GetOperator().String())
			return nilKey, ErrSelectorKeyOperatorTypeNotMatch
		}
	}
	return clickhouse.DbTypeCasting(timeseries.EscapeEventlogFieldName(selector.GetKey()), fieldType), nil
}

func (s *Expression) buildSelector(selector *protos.Selector) string {
	key, err := s.verifySelector(selector)
	if err != nil {
		s.logger.Warnf("verify selector failed, err: %s", err.Error())
		s.err = err
		return nilSelector
	}
	if key == ignoreSelector {
		return nilSelector
	}
	tpl, ok := operatorMap[selector.GetOperator()]
	if !ok {
		log.Warnf("operator doesn't support, operator: %s", selector.GetOperator().String())
		s.err = errors.Errorf("operator doesn't support, operator: %s", selector.GetOperator().String())
		return nilSelector
	}
	var args []any
	args = append(args, key)
	for _, value := range selector.GetValue() {
		args = append(args, utils.ProtoToClickhouseValue(value))
	}
	return fmt.Sprintf(tpl, args...)
}

func (s *Expression) String() string {
	if s.selectorExpr == nil {
		return nilSelector
	}

	var expr string
	switch s.selectorExpr.GetExpr().(type) {
	case *protos.SelectorExpr_Selector:
		expr = s.buildSelector(s.selectorExpr.GetSelector())
	case *protos.SelectorExpr_LogicExpr_:
		logicExpr := s.selectorExpr.GetLogicExpr()
		if logicExpr == nil || len(logicExpr.GetExpressions()) == 0 {
			return nilSelector
		}
		var exprs []string
		for _, expr := range logicExpr.GetExpressions() {
			exprs = append(exprs, NewSelectorExpression2(s.ctx, expr, s.meta).String())
		}
		switch logicExpr.GetOperator() {
		case protos.JoinOperator_AND:
			expr = fmt.Sprintf("(%s)", strings.Join(exprs, " AND "))
		case protos.JoinOperator_OR:
			expr = fmt.Sprintf("(%s)", strings.Join(exprs, " OR "))
		}
	default:
		return nilSelector
	}
	s.logger.Debugf("selector expr: %s", expr)
	return expr
}

func (s *Expression) newCond(selector *protos.Selector) *condition.Cond {
	key, err := s.verifySelector(selector)
	if err != nil {
		log.Warnf("verify selector failed, err: %s", err.Error())
		s.err = err
		return nil
	}
	if key == ignoreSelector {
		return nil
	}

	var args []any
	for _, value := range selector.GetValue() {
		args = append(args, utils.Proto2Any(value))
	}

	switch selector.GetOperator() {
	case protos.Selector_EQ:
		return condition.Equal(key, args[0])
	case protos.Selector_NEQ:
		return condition.NotEqual(key, args[0])
	case protos.Selector_EXISTS:
		return condition.IsNotNull(key)
	case protos.Selector_NOT_EXISTS:
		return condition.IsNull(key)
	case protos.Selector_GT:
		return condition.GreaterThan(key, args[0])
	case protos.Selector_GTE:
		return condition.GreaterEqualThan(key, args[0])
	case protos.Selector_LT:
		return condition.LessThan(key, args[0])
	case protos.Selector_LTE:
		return condition.LessEqualThan(key, args[0])
	case protos.Selector_BETWEEN:
		return condition.Between(key, args[0], args[1])
	case protos.Selector_NOT_BETWEEN:
		return condition.NotBetween(key, args[0], args[1])
	case protos.Selector_CONTAINS:
		return condition.Like(key, args[0])
	case protos.Selector_NOT_CONTAINS:
		return condition.NotLike(key, args[0])
	case protos.Selector_IN:
		return condition.In(key, args...)
	case protos.Selector_NOT_IN:
		return condition.NotIn(key, args...)
	default:
		s.logger.Warnf("operator doesn't support, operator: %s", selector.GetOperator().String())
		s.err = errors.Errorf("operator doesn't support, operator: %s", selector.GetOperator().String())
	}
	return nil
}

func (s *Expression) Cond() *condition.Cond {
	if s.selectorExpr == nil {
		return nil
	}

	var cond *condition.Cond
	switch s.selectorExpr.GetExpr().(type) {
	case *protos.SelectorExpr_Selector:
		cond = s.newCond(s.selectorExpr.GetSelector())
	case *protos.SelectorExpr_LogicExpr_:
		logicExpr := s.selectorExpr.GetLogicExpr()
		if logicExpr == nil || len(logicExpr.GetExpressions()) == 0 {
			return nil
		}
		var conds []*condition.Cond
		for _, expr := range logicExpr.GetExpressions() {
			if c := NewSelectorExpression2(s.ctx, expr, s.meta).Cond(); c != nil {
				conds = append(conds, c)
			}
		}
		switch logicExpr.GetOperator() {
		case protos.JoinOperator_AND:
			cond = condition.And(conds...)
		case protos.JoinOperator_OR:
			cond = condition.Or(conds...)
		}
	}
	return cond
}

func (s *Expression) Error() error {
	return s.err
}
