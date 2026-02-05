package schema

import (
	"fmt"
	"github.com/graph-gophers/graphql-go/types"
	"github.com/shopspring/decimal"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema/exp"
	"sentioxyz/sentio-core/driver/entity/schema/interval"
	"strconv"
	"strings"
)

type Aggregation struct {
	*types.ObjectTypeDefinition
	fieldSet

	DimFields types.FieldsDefinition
	AggFields []*AggregationAggField
}

func NewAggregation(obj *types.ObjectTypeDefinition) *Aggregation {
	agg := &Aggregation{
		ObjectTypeDefinition: obj,
		fieldSet:             fieldSet{FieldsDefinition: obj.Fields},
	}
	for _, field := range obj.Fields {
		d := field.Directives.Get(AggregateDirectiveName)
		if d == nil {
			agg.DimFields = append(agg.DimFields, field)
		} else {
			agg.AggFields = append(agg.AggFields, &AggregationAggField{FieldDefinition: field})
		}
	}
	return agg
}

func (a *Aggregation) GetIntervals() []interval.Interval {
	intervals, err := a.TryGetIntervals()
	if err != nil {
		panic(err)
	}
	return intervals
}

func (a *Aggregation) TryGetIntervals() ([]interval.Interval, error) {
	d := a.Directives.Get(AggregationDirectiveName)
	if d == nil {
		return nil, fmt.Errorf("aggregation %s do not have @%s directive", a.Name, AggregationDirectiveName)
	}
	var intervals []interval.Interval
	intervalsArgValue, has := d.Arguments.Get("intervals")
	if !has {
		return nil, nil
	}
	value, is := intervalsArgValue.(*types.ListValue)
	if !is {
		return nil, fmt.Errorf("type of intervals in aggregation %s is %T, not ListValue", a.Name, intervalsArgValue)
	}
	for _, v := range value.Values {
		org, err := strconv.Unquote(v.String())
		if err != nil {
			org = v.String()
		}
		itv, err := interval.Parse(org)
		if err != nil {
			return nil, fmt.Errorf("get interval of aggregation %q failed: %w", a.Name, err)
		}
		intervals = append(intervals, itv)
	}
	return intervals, nil
}

func (a *Aggregation) TryGetSource() (string, error) {
	d := a.Directives.Get(AggregationDirectiveName)
	if d == nil {
		return "", fmt.Errorf("aggregation %q do not have @%s directive", a.Name, AggregationDirectiveName)
	}
	val, has := d.Arguments.Get("source")
	if !has {
		return "", fmt.Errorf("aggregation %q do not have source entity", a.Name)
	}
	valStr, err := strconv.Unquote(val.String())
	if err != nil {
		valStr = val.String()
	}
	return valStr, nil
}

func (a *Aggregation) GetSource() string {
	src, err := a.TryGetSource()
	if err != nil {
		panic(err)
	}
	return src
}

func (a *Aggregation) GetName() string {
	return a.Name
}

func (a *Aggregation) GetFullName() string {
	return fmt.Sprintf("aggregation %q", a.GetName())
}

func (a *Aggregation) ListEntities() []*Entity {
	panic("not implement")
}

type AggregationAggField struct {
	*types.FieldDefinition

	aggExp *exp.Exp
}

func (f *AggregationAggField) TryGetAggExp() (*exp.Exp, error) {
	if f.aggExp != nil {
		return f.aggExp, nil
	}
	d := f.Directives.Get(AggregateDirectiveName)
	if d == nil {
		return nil, fmt.Errorf("miss @%s directive", AggregateDirectiveName)
	}
	arg, has := d.Arguments.Get("arg")
	if !has || arg == nil {
		return nil, fmt.Errorf("@%s directive miss arg", AggregateDirectiveName)
	}
	argStr, _ := strconv.Unquote(arg.String())
	aggExp, err := exp.NewExp(argStr)
	if err != nil {
		return nil, fmt.Errorf("invalid arg (%s) in @%s directive: %w", arg, AggregateDirectiveName, err)
	}
	f.aggExp = aggExp
	return aggExp, nil
}

var validAggFunc = []string{"sum", "count", "min", "max", "first", "last"}

func (f *AggregationAggField) TryGetAggFunc() (string, error) {
	d := f.Directives.Get(AggregateDirectiveName)
	if d == nil {
		return "", fmt.Errorf("miss @%s directive", AggregateDirectiveName)
	}
	fn, has := d.Arguments.Get("fn")
	if !has || fn == nil {
		return "", fmt.Errorf("@%s directive miss fn", AggregateDirectiveName)
	}
	fnStr, _ := strconv.Unquote(fn.String())
	if utils.IndexOf(validAggFunc, fnStr) < 0 {
		return "", fmt.Errorf("invalid fn %q, should in %v", fn.String(), validAggFunc)
	}
	return fnStr, nil
}

func (f *AggregationAggField) GetAggExp() *exp.Exp {
	aggExp, err := f.TryGetAggExp()
	if err != nil {
		panic(err)
	}
	return aggExp
}

func (f *AggregationAggField) GetAggFunc() string {
	fn, err := f.TryGetAggFunc()
	if err != nil {
		panic(err)
	}
	return fn
}

type AggregateOperatorProvider struct{}

var aggregateOperatorProvider AggregateOperatorProvider

func (p AggregateOperatorProvider) Check(fnName string, argNum int) bool {
	switch strings.ToLower(fnName) {
	case "+", "-", "*", "/", "and", "or":
		return argNum == 2
	case "not":
		return argNum == 1
	case "greatest", "least", "max", "min":
		return argNum > 0
	default:
		return false
	}
}

type AggregateVarProvider struct {
	*Entity
}

func (p AggregateVarProvider) Check(cnt string) error {
	if p.Entity.Get(cnt) != nil {
		return nil
	}
	_, err := decimal.NewFromString(cnt)
	return err
}
