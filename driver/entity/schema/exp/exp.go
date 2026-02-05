package exp

import (
	"bytes"
	"fmt"
	"strings"
)

type Exp struct {
	// <var> or <const>
	Value *Word

	// + - * / and or not <func>
	Operator  *Word
	Arguments []*Exp
}

type Position struct {
	S   int
	E   int
	Lvl int
}

type Word struct {
	Cnt string
	Position
}

func (s Word) BuildUnexpectedError(suffix ...string) error {
	return s.BuildError("unexpected", suffix...)
}

func (s Word) BuildError(prefix string, suffix ...string) error {
	var suf string
	if len(suffix) > 0 {
		suf = suffix[0]
	}
	return fmt.Errorf("%s '%s' at expression[%s]%s", prefix, s.Cnt, s.P(), suf)
}

func (s Position) P() string {
	if s.S == s.E {
		return fmt.Sprintf("%d", s.S)
	} else {
		return fmt.Sprintf("%d..%d", s.S, s.E)
	}
}

func _inWord(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.'
}

func _splitExp(exp string) (words []Word, err error) {
	// split words
	s := 0
	for s < len(exp) {
		c := exp[s]
		switch c {
		case ' ', '\t', '\n', '\r':
			s++
		case '+', '-', '*', '/', '(', ')', ',':
			words = append(words, Word{
				Cnt: string(c),
				Position: Position{
					S: s,
					E: s,
				},
			})
			s++
		default:
			if _inWord(c) {
				// var or function or const
				e := s + 1
				for e < len(exp) && _inWord(exp[e]) {
					e++
				}
				words = append(words, Word{
					Cnt: exp[s:e],
					Position: Position{
						S: s,
						E: e - 1,
					},
				})
				s = e
			} else {
				return nil, fmt.Errorf("invalid character '%s' (0x%x) in expression[%d]", exp[s:s+1], c, s)
			}
		}
	}
	// fill level for words
	if len(words) == 0 {
		return nil, nil
	}
	if words[0].Cnt == ")" {
		return nil, fmt.Errorf("miss '(' for expression[%d]", words[0].S)
	}
	for i := 1; i < len(words); i++ {
		if words[i].Cnt == ")" {
			if words[i-1].Cnt == "(" {
				words[i].Lvl = words[i-1].Lvl
			} else {
				words[i].Lvl = words[i-1].Lvl - 1
				if words[i].Lvl < 0 {
					return nil, fmt.Errorf("miss '(' for expression[%s]", words[i].P())
				}
			}
		} else if words[i-1].Cnt == "(" {
			words[i].Lvl = words[i-1].Lvl + 1
		} else {
			words[i].Lvl = words[i-1].Lvl
		}
	}
	if words[len(words)-1].Lvl != 0 || (len(words) == 1 && words[0].Cnt == "(") {
		return nil, fmt.Errorf("miss ')' at the end of the exp")
	}
	return
}

func _binOpPriority(op string) int {
	switch strings.ToLower(op) {
	case "and":
		return 1
	case "or":
		return 2
	case "*", "/":
		return 3
	case "+", "-":
		return 4
	default:
		return 5
	}
}

func _buildExp(words []Word) (*Exp, error) {
	if len(words) == 0 {
		return nil, fmt.Errorf("empty expression")
	}
	lvl := words[0].Lvl
	// find lowest priority binary operator
	binOp, binOpPriority := 0, 0
	for i := 1; i < len(words); i++ {
		if words[i].Lvl != lvl {
			continue
		}
		switch strings.ToLower(words[i].Cnt) {
		case ",":
			return nil, words[i].BuildUnexpectedError()
		case "and", "or", "+", "-", "*", "/":
			cp := _binOpPriority(words[i].Cnt)
			if binOpPriority <= cp {
				binOp, binOpPriority = i, cp
			}
		}
	}
	if binOp > 0 {
		// found binary operator
		left, err := _buildExp(words[:binOp])
		if err != nil {
			return nil, err
		}
		right, err := _buildExp(words[binOp+1:])
		if err != nil {
			return nil, err
		}
		return &Exp{Operator: &words[binOp], Arguments: []*Exp{left, right}}, nil
	}
	// no binary operator, must be one part, possible formats:
	// - ( <exp> )
	// - not <exp>
	// - <func> ( <exp> , <exp> , <exp> )
	// - <var>
	// - <const>
	lp, rp := -1, -1
	for i := 0; i < len(words); i++ {
		if words[i].Lvl == lvl {
			switch words[i].Cnt {
			case "(":
				if lp == -1 {
					lp = i
				}
			case ")":
				if rp == -1 {
					rp = i
				}
			}
		}
	}
	if lp == 0 {
		// ( <exp> )
		if rp != len(words)-1 {
			return nil, words[rp+1].BuildUnexpectedError()
		}
		if lp+1 == rp {
			return nil, words[rp].BuildError("missing content before")
		}
		return _buildExp(words[1 : len(words)-1])
	}
	if strings.ToLower(words[0].Cnt) == "not" {
		// not <exp>
		right, err := _buildExp(words[1:])
		if err != nil {
			return nil, err
		}
		return &Exp{Operator: &words[0], Arguments: []*Exp{right}}, nil
	}
	if lp > 0 {
		// <func> ( <exp> , <exp> , <exp> )
		if lp != 1 {
			return nil, words[1].BuildUnexpectedError(", the operator may be missing")
		}
		if rp != len(words)-1 {
			return nil, words[rp+1].BuildUnexpectedError(", the operator may be missing")
		}
		var args []*Exp
		if lp+1 < rp {
			// has arguments for the function
			s := lp + 1
			for i := lp + 1; i <= rp; i++ {
				if words[i].Lvl == lvl+1 && words[i].Cnt == "," || i == rp {
					if s == i {
						return nil, words[i].BuildError("missing content before")
					}
					arg, err := _buildExp(words[s:i])
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					s = i + 1
				}
			}
		}
		return &Exp{Operator: &words[0], Arguments: args}, nil
	}
	// <const> or <var>
	if len(words) > 1 {
		return nil, words[1].BuildUnexpectedError(", the operator may be missing")
	}
	return &Exp{Value: &words[0]}, nil
}

type OperatorProvider interface {
	Check(fnName string, argNum int) bool
}

type VarProvider interface {
	Check(cnt string) error
}

type AliasController interface {
	GetVarName(org string) string
	GetOpName(org string) string
}

type EmptyAliasController struct{}

func (e EmptyAliasController) GetVarName(org string) string {
	return org
}

func (e EmptyAliasController) GetOpName(org string) string {
	return org
}

func NewExp(exp string) (*Exp, error) {
	words, err := _splitExp(exp)
	if err != nil {
		return nil, err
	}
	return _buildExp(words)
}

func (e *Exp) Text(aliasCtl AliasController) string {
	if e == nil {
		return ""
	}
	if e.Value != nil {
		return aliasCtl.GetVarName(e.Value.Cnt)
	}
	switch op := strings.ToLower(e.Operator.Cnt); op {
	case "+", "-", "*", "/", "and", "or":
		left := e.Arguments[0].Text(aliasCtl)
		right := e.Arguments[1].Text(aliasCtl)
		if e.Arguments[0].Operator != nil && _binOpPriority(e.Arguments[0].Operator.Cnt) <= 4 {
			left = "(" + left + ")"
		}
		if e.Arguments[1].Operator != nil && _binOpPriority(e.Arguments[1].Operator.Cnt) <= 4 {
			right = "(" + right + ")"
		}
		return fmt.Sprintf("%s %s %s", left, op, right)
	case "not":
		return "not " + e.Arguments[0].Text(aliasCtl)
	default:
		var buf bytes.Buffer
		buf.WriteString(aliasCtl.GetOpName(e.Operator.Cnt))
		buf.WriteString("(")
		for i, arg := range e.Arguments {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(arg.Text(aliasCtl))
		}
		buf.WriteString(")")
		return buf.String()
	}
}

func (e *Exp) String() string {
	return e.Text(EmptyAliasController{})
}

func (e *Exp) Verify(opProvider OperatorProvider, varProvider VarProvider) error {
	if e.Value != nil {
		if e.Value != nil {
			if err := varProvider.Check(e.Value.Cnt); err != nil {
				return e.Value.BuildError("invalid variable", ", "+err.Error())
			}
		}
	} else {
		if !opProvider.Check(e.Operator.Cnt, len(e.Arguments)) {
			return e.Operator.BuildError("unsupported operator", fmt.Sprintf(" with %d arguments", len(e.Arguments)))
		}
		for _, arg := range e.Arguments {
			if err := arg.Verify(opProvider, varProvider); err != nil {
				return err
			}
		}
	}
	return nil
}
