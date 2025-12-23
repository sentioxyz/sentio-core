package condition

import (
	"fmt"
	"strings"

	"sentioxyz/sentio-core/common/anyutil"
)

const (
	equal = iota
	notEqual
	greaterThan
	greaterEqualThan
	lessThan
	lessEqualThan
	in
	notIn
	like
	notLike
	isNull
	isNotNull
	between
	notBetween
	or
	and
)

type builder interface {
	Equal(field string, value interface{}) string
	NotEqual(field string, value interface{}) string
	GreaterThan(field string, value interface{}) string
	GreaterEqualThan(field string, value interface{}) string
	LessThan(field string, value interface{}) string
	LessEqualThan(field string, value interface{}) string
	In(field string, values ...interface{}) string
	NotIn(field string, values ...interface{}) string
	Like(field string, value interface{}) string
	NotLike(field string, value interface{}) string
	IsNull(field string) string
	IsNotNull(field string) string
	Between(field string, from, to interface{}) string
	NotBetween(field string, from, to interface{}) string
	Or(conds ...string) string
	And(conds ...string) string
}

type Cond struct {
	field string
	op    int
	args  []any
}

func (c *Cond) Do(b builder) string {
	if c == nil {
		return "1"
	}
	switch c.op {
	case equal:
		return b.Equal(c.field, c.args[0])
	case notEqual:
		return b.NotEqual(c.field, c.args[0])
	case greaterThan:
		return b.GreaterThan(c.field, c.args[0])
	case greaterEqualThan:
		return b.GreaterEqualThan(c.field, c.args[0])
	case lessThan:
		return b.LessThan(c.field, c.args[0])
	case lessEqualThan:
		return b.LessEqualThan(c.field, c.args[0])
	case in:
		return b.In(c.field, c.args...)
	case notIn:
		return b.NotIn(c.field, c.args...)
	case like:
		s, ok := c.args[0].(string)
		if ok {
			return b.Like(c.field, "%"+s+"%")
		}
		return b.Like(c.field, c.args[0])
	case notLike:
		s, ok := c.args[0].(string)
		if ok {
			return b.NotLike(c.field, "%"+s+"%")
		}
		return b.NotLike(c.field, c.args[0])
	case isNull:
		return b.IsNull(c.field)
	case isNotNull:
		return b.IsNotNull(c.field)
	case between:
		return b.Between(c.field, c.args[0], c.args[1])
	case notBetween:
		return b.NotBetween(c.field, c.args[0], c.args[1])
	case or:
		var conds []string
		for _, arg := range c.args {
			innerCond := arg.(*Cond)
			conds = append(conds, innerCond.Do(b))
		}
		return b.Or(conds...)
	case and:
		var conds []string
		for _, arg := range c.args {
			innerCond := arg.(*Cond)
			conds = append(conds, innerCond.Do(b))
		}
		return b.And(conds...)
	}
	return "1"
}

func (c *Cond) String() string {
	if c == nil {
		return "1"
	}
	switch c.op {
	case equal:
		return fmt.Sprintf("(equals(%s, %s))", c.field, anyutil.ToString(c.args[0]))
	case notEqual:
		return fmt.Sprintf("(notEquals(%s, %s))", c.field, anyutil.ToString(c.args[0]))
	case isNotNull:
		return fmt.Sprintf("(%s is not null)", c.field)
	case isNull:
		return fmt.Sprintf("(%s is null)", c.field)
	case greaterThan:
		return fmt.Sprintf("(greater(%s, %s))", c.field, anyutil.ToString(c.args[0]))
	case greaterEqualThan:
		return fmt.Sprintf("(greaterOrEquals(%s, %s))", c.field, anyutil.ToString(c.args[0]))
	case lessThan:
		return fmt.Sprintf("(less(%s, %s))", c.field, anyutil.ToString(c.args[0]))
	case lessEqualThan:
		return fmt.Sprintf("(lessOrEquals(%s, %s))", c.field, anyutil.ToString(c.args[0]))
	case between:
		return fmt.Sprintf("(%s between %s and %s)", c.field, anyutil.ToString(c.args[0]), anyutil.ToString(c.args[1]))
	case notBetween:
		return fmt.Sprintf(
			"(%s not between %s and %s)",
			c.field,
			anyutil.ToString(c.args[0]),
			anyutil.ToString(c.args[1]),
		)
	case like:
		return fmt.Sprintf("(like(%s, %%%s%%)", c.field, anyutil.ToString(c.args[0]))
	case notLike:
		return fmt.Sprintf("(notLike(%s, %%%s%%)", c.field, anyutil.ToString(c.args[0]))
	case in:
		return fmt.Sprintf("(%s in (%s))", c.field, anyutil.ToString(c.args))
	case notIn:
		return fmt.Sprintf("(%s not in (%s))", c.field, anyutil.ToString(c.args))
	case or:
		var conds []string
		for _, arg := range c.args {
			innerCond := arg.(*Cond)
			conds = append(conds, innerCond.String())
		}
		return fmt.Sprintf("(%s)", strings.Join(conds, " or "))
	case and:
		var conds []string
		for _, arg := range c.args {
			innerCond := arg.(*Cond)
			conds = append(conds, innerCond.String())
		}
		return fmt.Sprintf("(%s)", strings.Join(conds, " and "))
	}
	return "1"
}

func Equal(field string, value any) *Cond {
	return &Cond{
		field: field,
		op:    equal,
		args:  []any{value},
	}
}

func NotEqual(field string, value any) *Cond {
	return &Cond{
		field: field,
		op:    notEqual,
		args:  []any{value},
	}
}

func GreaterThan(field string, value any) *Cond {
	return &Cond{
		field: field,
		op:    greaterThan,
		args:  []any{value},
	}
}

func GreaterEqualThan(field string, value any) *Cond {
	return &Cond{
		field: field,
		op:    greaterEqualThan,
		args:  []any{value},
	}
}

func LessThan(field string, value any) *Cond {
	return &Cond{
		field: field,
		op:    lessThan,
		args:  []any{value},
	}
}

func LessEqualThan(field string, value any) *Cond {
	return &Cond{
		field: field,
		op:    lessEqualThan,
		args:  []any{value},
	}
}

func In(field string, values ...any) *Cond {
	return &Cond{
		field: field,
		op:    in,
		args:  values,
	}
}

func NotIn(field string, values ...any) *Cond {
	return &Cond{
		field: field,
		op:    notIn,
		args:  values,
	}
}

func Like(field string, value any) *Cond {
	return &Cond{
		field: field,
		op:    like,
		args:  []any{value},
	}
}

func NotLike(field string, value any) *Cond {
	return &Cond{
		field: field,
		op:    notLike,
		args:  []any{value},
	}
}

func IsNull(field string) *Cond {
	return &Cond{
		field: field,
		op:    isNull,
	}
}

func IsNotNull(field string) *Cond {
	return &Cond{
		field: field,
		op:    isNotNull,
	}
}

func Between(field string, from, to any) *Cond {
	return &Cond{
		field: field,
		op:    between,
		args:  []any{from, to},
	}
}

func NotBetween(field string, from, to any) *Cond {
	return &Cond{
		field: field,
		op:    notBetween,
		args:  []any{from, to},
	}
}

func And(cond ...*Cond) *Cond {
	var conds []any
	for _, c := range cond {
		conds = append(conds, c)
	}
	return &Cond{
		op:   and,
		args: conds,
	}
}

func Or(cond ...*Cond) *Cond {
	var conds []any
	for _, c := range cond {
		conds = append(conds, c)
	}
	return &Cond{
		op:   or,
		args: conds,
	}
}
