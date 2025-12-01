package formula

import (
	"fmt"
	"go/ast"
	"go/parser"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Expression interface {
	ToString() string
}

type Identifier struct {
	Name string
}

func (i *Identifier) ToString() string {
	return i.Name
}

type Constant struct {
	Value float64
}

func (i *Constant) ToString() string {
	return fmt.Sprintf("%f", i.Value)
}

type BinaryOp string

const (
	PLUS  BinaryOp = "+"
	MINUS BinaryOp = "-"
	MUL   BinaryOp = "*"
	DIV   BinaryOp = "/"
	POW   BinaryOp = "^"
)

var BinaryOpSet = map[BinaryOp]struct{}{
	PLUS:  {},
	MINUS: {},
	MUL:   {},
	DIV:   {},
	POW:   {},
}

type BinaryExpression struct {
	Left  Expression
	Right Expression
	Op    BinaryOp
}

func (b *BinaryExpression) ToString() string {
	return b.Left.ToString() + string(b.Op) + b.Right.ToString()
}

type BracketExpression struct {
	Expr Expression
}

func (b *BracketExpression) ToString() string {
	return "(" + b.Expr.ToString() + ")"
}

type AggregateOp string

const (
	SUM AggregateOp = "SUM"
	AVG AggregateOp = "AVG"
	MIN AggregateOp = "MIN"
	MAX AggregateOp = "MAX"
	ABS AggregateOp = "ABS"
)

type AggregateExpression struct {
	Expr Expression
	Op   AggregateOp
	// ignore labels
}

func (b *AggregateExpression) ToString() string {
	return string(b.Op) + "(" + b.Expr.ToString() + ")"
}

func Parse(text string) (Expression, error) {
	root, err := parser.ParseExpr(text)
	if err != nil {
		return nil, err
	}
	return convert(root)
}

func convert(expr ast.Expr) (Expression, error) {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		left, err := convert(e.X)
		if err != nil {
			return nil, err
		}
		right, err := convert(e.Y)
		if err != nil {
			return nil, err
		}
		op := BinaryOp(e.Op.String())
		if _, ok := BinaryOpSet[op]; !ok {
			return nil, errors.Errorf("Unknown binary operator %s", op)
		}
		return &BinaryExpression{Left: left, Right: right, Op: op}, nil
	case *ast.Ident:
		return &Identifier{Name: e.Name}, nil
	case *ast.ParenExpr:
		res, err := convert(e.X)
		if err != nil {
			return nil, err
		}
		return &BracketExpression{Expr: res}, nil
	case *ast.BasicLit:
		value, err := strconv.ParseFloat(e.Value, 64)
		if err != nil {
			return nil, err
		}
		return &Constant{Value: value}, nil
	case *ast.CallExpr:
		if len(e.Args) == 0 {
			return nil, errors.Errorf("aggregate function required at least one argument")
		}
		arg, err := convert(e.Args[0])
		if err != nil {
			return nil, err
		}
		if ident, ok := e.Fun.(*ast.Ident); ok {
			switch strings.ToUpper(ident.Name) {
			case "SUM":
				return &AggregateExpression{Expr: arg, Op: SUM}, nil
			case "AVG":
				return &AggregateExpression{Expr: arg, Op: AVG}, nil
			case "MIN":
				return &AggregateExpression{Expr: arg, Op: MIN}, nil
			case "MAX":
				return &AggregateExpression{Expr: arg, Op: MAX}, nil
			case "ABS":
				return &AggregateExpression{Expr: arg, Op: ABS}, nil
			default:
				return nil, errors.Errorf("Unknown aggregate function %s", ident.Name)
			}
		} else {
			return nil, errors.Errorf("Unknown aggregate function")
		}
	default:
		return nil, errors.Errorf("Unknown expression type %T", expr)
	}
}
