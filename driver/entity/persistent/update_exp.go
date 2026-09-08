package persistent

import (
	"fmt"
	"math/big"
	"reflect"
	"slices"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/graph-gophers/graphql-go/types"
	"github.com/shopspring/decimal"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	"sentioxyz/sentio-core/driver/entity/schema/exp"
)

// expKind is the static type of an update expression or of one of its sub expressions.
type expKind int

const (
	// kindNull is the type of the null literal, it is compatible with every other kind.
	kindNull expKind = iota
	kindBool
	kindNumber
	kindString
)

func (k expKind) String() string {
	switch k {
	case kindNull:
		return "null"
	case kindBool:
		return "boolean"
	case kindNumber:
		return "number"
	case kindString:
		return "string"
	default:
		return fmt.Sprintf("kind(%d)", int(k))
	}
}

// expValue is a runtime value produced while evaluating an update expression.
// Every numeric field type (Int, Int8, BigInt, Float, BigDecimal, Timestamp) is a number and is
// computed with decimal arithmetic; the result is converted back to the field type at the end.
type expValue struct {
	kind expKind
	b    bool
	n    decimal.Decimal
	s    string
}

var expNull = expValue{kind: kindNull}

func expBool(b bool) expValue              { return expValue{kind: kindBool, b: b} }
func expNumber(n decimal.Decimal) expValue { return expValue{kind: kindNumber, n: n} }
func expString(s string) expValue          { return expValue{kind: kindString, s: s} }

func (v expValue) isNull() bool {
	return v.kind == kindNull
}

func (v expValue) String() string {
	switch v.kind {
	case kindNull:
		return "null"
	case kindBool:
		return fmt.Sprintf("%t", v.b)
	case kindNumber:
		return v.n.String()
	case kindString:
		return fmt.Sprintf("%q", v.s)
	default:
		return fmt.Sprintf("<%s>", v.kind)
	}
}

// expRow is the version of the entity an update expression reads its field references from.
// It is the state right before the update that carries the expression, so every expression in
// one update request sees the same values regardless of the order the fields are computed in.
type expRow struct {
	// exists is false when the entity does not exist yet (or has been deleted), in which case
	// every field reference evaluates to null and exist() returns false.
	exists bool
	data   map[string]any
}

// compiledExp is a parsed and type-checked update expression for one field of one entity type.
type compiledExp struct {
	text string
	root *exp.Exp
	// kind is the static result kind, kindNull when the expression is null in every case
	kind expKind
	// fields are the names of the entity fields the expression references
	fields []string
}

func (c *compiledExp) String() string {
	if c == nil {
		return ""
	}
	return c.text
}

// fieldExpKind maps a non-list field type to the expression kind of its values.
func fieldExpKind(typ types.Type) (expKind, error) {
	typeChain := schema.BreakType(typ)
	if typeChain.CountListLayer() > 0 {
		return kindNull, fmt.Errorf("list type %s is not supported in expression", typ.String())
	}
	switch innerType := typeChain.InnerType().(type) {
	case *types.ScalarTypeDefinition:
		switch innerType.Name {
		case "String", "ID", "Bytes":
			return kindString, nil
		case "Boolean":
			return kindBool, nil
		case "Int", "Int8", "BigInt", "Float", "BigDecimal", "Timestamp":
			return kindNumber, nil
		}
	case *types.EnumTypeDefinition, *types.ObjectTypeDefinition, *types.InterfaceTypeDefinition:
		// enum values and foreign keys are both stored as strings
		return kindString, nil
	}
	return kindNull, fmt.Errorf("type %s is not supported in expression", typ.String())
}

// compileUpdateExp parses text and checks it against the schema of entityType, so that a bad
// expression is rejected when the update request arrives instead of when it is resolved later.
func compileUpdateExp(entityType *schema.Entity, field *types.FieldDefinition, text string) (*compiledExp, error) {
	targetKind, err := fieldExpKind(field.Type)
	if err != nil {
		return nil, fmt.Errorf("field %s.%s does not support expression: %w", entityType.Name, field.Name, err)
	}
	root, err := exp.NewExp(text)
	if err != nil {
		return nil, err
	}
	c := &compiledExp{text: text, root: root}
	c.kind, err = c.check(entityType, root)
	if err != nil {
		return nil, err
	}
	if !expKindCompatible(c.kind, targetKind) {
		return nil, fmt.Errorf("expression result is %s but field %s.%s is %s",
			c.kind, entityType.Name, field.Name, targetKind)
	}
	return c, nil
}

func expKindCompatible(k1, k2 expKind) bool {
	return k1 == kindNull || k2 == kindNull || k1 == k2
}

// expKindJoin returns the common kind of two compatible kinds, null only when both are null.
func expKindJoin(k1, k2 expKind) expKind {
	if k1 == kindNull {
		return k2
	}
	return k1
}

func (c *compiledExp) check(entityType *schema.Entity, e *exp.Exp) (expKind, error) {
	if e.Value != nil {
		return c.checkValue(entityType, e.Value)
	}
	argKinds := make([]expKind, len(e.Arguments))
	for i, arg := range e.Arguments {
		kind, err := c.check(entityType, arg)
		if err != nil {
			return kindNull, err
		}
		argKinds[i] = kind
	}
	op := strings.ToLower(e.Operator.Cnt)
	argc := func(n int) error {
		if len(e.Arguments) != n {
			return e.Operator.BuildError("unsupported operator", fmt.Sprintf(" with %d arguments", len(e.Arguments)))
		}
		return nil
	}
	requireKind := func(i int, want expKind) error {
		if !expKindCompatible(argKinds[i], want) {
			return e.Operator.BuildError("invalid operand",
				fmt.Sprintf(", argument #%d is %s, expect %s", i+1, argKinds[i], want))
		}
		return nil
	}
	requireSame := func(i, j int) (expKind, error) {
		if !expKindCompatible(argKinds[i], argKinds[j]) {
			return kindNull, e.Operator.BuildError("invalid operand",
				fmt.Sprintf(", argument #%d is %s but argument #%d is %s", i+1, argKinds[i], j+1, argKinds[j]))
		}
		return expKindJoin(argKinds[i], argKinds[j]), nil
	}
	switch op {
	case "+", "-", "*", "/":
		if err := argc(2); err != nil {
			return kindNull, err
		}
		for i := range argKinds {
			if err := requireKind(i, kindNumber); err != nil {
				return kindNull, err
			}
		}
		if op == "/" && isZeroLiteral(e.Arguments[1]) {
			return kindNull, e.Operator.BuildError("division by zero")
		}
		return kindNumber, nil
	case "and", "or":
		if err := argc(2); err != nil {
			return kindNull, err
		}
		for i := range argKinds {
			if err := requireKind(i, kindBool); err != nil {
				return kindNull, err
			}
		}
		return kindBool, nil
	case "not":
		if err := argc(1); err != nil {
			return kindNull, err
		}
		if err := requireKind(0, kindBool); err != nil {
			return kindNull, err
		}
		return kindBool, nil
	case "=", "!=":
		if err := argc(2); err != nil {
			return kindNull, err
		}
		if _, err := requireSame(0, 1); err != nil {
			return kindNull, err
		}
		return kindBool, nil
	case ">", ">=", "<", "<=":
		if err := argc(2); err != nil {
			return kindNull, err
		}
		kind, err := requireSame(0, 1)
		if err != nil {
			return kindNull, err
		}
		if kind == kindBool {
			return kindNull, e.Operator.BuildError("invalid operand", ", boolean is not comparable")
		}
		return kindBool, nil
	case "exist", "exists":
		if err := argc(0); err != nil {
			return kindNull, err
		}
		return kindBool, nil
	case "isnull":
		if err := argc(1); err != nil {
			return kindNull, err
		}
		return kindBool, nil
	case "coalesce":
		if len(e.Arguments) == 0 {
			return kindNull, e.Operator.BuildError("unsupported operator", " with 0 arguments")
		}
		kind := argKinds[0]
		for i := 1; i < len(argKinds); i++ {
			if !expKindCompatible(kind, argKinds[i]) {
				return kindNull, e.Operator.BuildError("invalid operand",
					fmt.Sprintf(", argument #%d is %s but argument #1 is %s", i+1, argKinds[i], kind))
			}
			kind = expKindJoin(kind, argKinds[i])
		}
		return kind, nil
	case "if":
		if err := argc(3); err != nil {
			return kindNull, err
		}
		if err := requireKind(0, kindBool); err != nil {
			return kindNull, err
		}
		return requireSame(1, 2)
	default:
		return kindNull, e.Operator.BuildError("unsupported operator",
			fmt.Sprintf(" with %d arguments", len(e.Arguments)))
	}
}

// isZeroLiteral reports whether e is a number literal equal to zero, such as 0, 0.0 or -0.
func isZeroLiteral(e *exp.Exp) bool {
	if e.Value == nil || !exp.IsNumberLiteral(e.Value.Cnt) {
		return false
	}
	n, err := decimal.NewFromString(e.Value.Cnt)
	return err == nil && n.IsZero()
}

func (c *compiledExp) checkValue(entityType *schema.Entity, word *exp.Word) (expKind, error) {
	cnt := word.Cnt
	if exp.IsStringLiteral(cnt) {
		if _, err := exp.UnquoteStringLiteral(cnt); err != nil {
			return kindNull, word.BuildError("invalid string literal", ", "+err.Error())
		}
		return kindString, nil
	}
	if exp.IsNumberLiteral(cnt) {
		if _, err := decimal.NewFromString(cnt); err != nil {
			return kindNull, word.BuildError("invalid number literal")
		}
		return kindNumber, nil
	}
	switch strings.ToLower(cnt) {
	case "true", "false":
		return kindBool, nil
	case "null":
		return kindNull, nil
	}
	field := entityType.GetFieldByName(cnt)
	if field == nil {
		return kindNull, word.BuildError("unknown field", fmt.Sprintf(", %s.%s does not exist", entityType.Name, cnt))
	}
	if field.Directives.Get(schema.DerivedFromDirectiveName) != nil {
		return kindNull, word.BuildError("invalid field",
			fmt.Sprintf(", %s.%s is derived and cannot be referenced", entityType.Name, cnt))
	}
	kind, err := fieldExpKind(field.Type)
	if err != nil {
		return kindNull, word.BuildError("invalid field", ", "+err.Error())
	}
	if !slices.Contains(c.fields, cnt) {
		c.fields = append(c.fields, cnt)
	}
	return kind, nil
}

// eval computes the expression against row.
func (c *compiledExp) eval(row expRow) (expValue, error) {
	v, err := c.evalNode(row, c.root)
	if err != nil {
		return expNull, fmt.Errorf("evaluate expression %q failed: %w", c.text, err)
	}
	return v, nil
}

func (c *compiledExp) evalNode(row expRow, e *exp.Exp) (expValue, error) {
	if e.Value != nil {
		return c.evalValue(row, e.Value)
	}
	op := strings.ToLower(e.Operator.Cnt)
	// if, coalesce, and, or evaluate lazily so an error in an unused branch is not reported
	switch op {
	case "exist", "exists":
		return expBool(row.exists), nil
	case "if":
		cond, err := c.evalNode(row, e.Arguments[0])
		if err != nil {
			return expNull, err
		}
		if cond.kind == kindBool && cond.b {
			return c.evalNode(row, e.Arguments[1])
		}
		return c.evalNode(row, e.Arguments[2])
	case "coalesce":
		for _, arg := range e.Arguments {
			v, err := c.evalNode(row, arg)
			if err != nil {
				return expNull, err
			}
			if !v.isNull() {
				return v, nil
			}
		}
		return expNull, nil
	}
	args := make([]expValue, len(e.Arguments))
	for i, arg := range e.Arguments {
		v, err := c.evalNode(row, arg)
		if err != nil {
			return expNull, err
		}
		args[i] = v
	}
	switch op {
	case "isnull":
		return expBool(args[0].isNull()), nil
	case "not":
		if args[0].isNull() {
			return expNull, nil
		}
		return expBool(!args[0].b), nil
	case "and":
		// three-valued logic: false wins over null, null wins over true
		if args[0].kind == kindBool && !args[0].b || args[1].kind == kindBool && !args[1].b {
			return expBool(false), nil
		}
		if args[0].isNull() || args[1].isNull() {
			return expNull, nil
		}
		return expBool(true), nil
	case "or":
		if args[0].kind == kindBool && args[0].b || args[1].kind == kindBool && args[1].b {
			return expBool(true), nil
		}
		if args[0].isNull() || args[1].isNull() {
			return expNull, nil
		}
		return expBool(false), nil
	}
	// the rest are strict: any null operand makes the result null
	for _, arg := range args {
		if arg.isNull() {
			return expNull, nil
		}
	}
	switch op {
	case "+":
		return expNumber(args[0].n.Add(args[1].n)), nil
	case "-":
		return expNumber(args[0].n.Sub(args[1].n)), nil
	case "*":
		return expNumber(args[0].n.Mul(args[1].n)), nil
	case "/":
		if args[1].n.IsZero() {
			return expNull, fmt.Errorf("division by zero at expression[%s]", e.Operator.P())
		}
		return expNumber(args[0].n.Div(args[1].n)), nil
	case "=":
		return expBool(expCompare(args[0], args[1]) == 0), nil
	case "!=":
		return expBool(expCompare(args[0], args[1]) != 0), nil
	case ">":
		return expBool(expCompare(args[0], args[1]) > 0), nil
	case ">=":
		return expBool(expCompare(args[0], args[1]) >= 0), nil
	case "<":
		return expBool(expCompare(args[0], args[1]) < 0), nil
	case "<=":
		return expBool(expCompare(args[0], args[1]) <= 0), nil
	default:
		// unreachable, check rejects unknown operators
		return expNull, e.Operator.BuildError("unsupported operator")
	}
}

// expCompare compares two non-null values of the same kind.
func expCompare(a, b expValue) int {
	switch a.kind {
	case kindNumber:
		return a.n.Cmp(b.n)
	case kindString:
		return strings.Compare(a.s, b.s)
	case kindBool:
		switch {
		case a.b == b.b:
			return 0
		case a.b:
			return 1
		default:
			return -1
		}
	default:
		return 0
	}
}

func (c *compiledExp) evalValue(row expRow, word *exp.Word) (expValue, error) {
	cnt := word.Cnt
	if exp.IsStringLiteral(cnt) {
		s, err := exp.UnquoteStringLiteral(cnt)
		if err != nil {
			return expNull, err
		}
		return expString(s), nil
	}
	if exp.IsNumberLiteral(cnt) {
		n, err := decimal.NewFromString(cnt)
		if err != nil {
			return expNull, word.BuildError("invalid number literal")
		}
		return expNumber(n), nil
	}
	switch strings.ToLower(cnt) {
	case "true":
		return expBool(true), nil
	case "false":
		return expBool(false), nil
	case "null":
		return expNull, nil
	}
	if !row.exists {
		return expNull, nil
	}
	v, has := row.data[cnt]
	if !has {
		return expNull, nil
	}
	return expValueFromField(v)
}

// expValueFromField converts a value stored in EntityBox.Data to an expression value.
func expValueFromField(v any) (expValue, error) {
	if utils.IsNil(v) {
		return expNull, nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if bi, is := v.(*big.Int); is {
			return expNumber(decimal.NewFromBigInt(bi, 0)), nil
		}
		v = rv.Elem().Interface()
	}
	switch x := v.(type) {
	case string:
		return expString(x), nil
	case []byte:
		return expString(hexutil.Encode(x)), nil
	case bool:
		return expBool(x), nil
	case int32:
		return expNumber(decimal.NewFromInt32(x)), nil
	case int64:
		return expNumber(decimal.NewFromInt(x)), nil
	case int:
		return expNumber(decimal.NewFromInt(int64(x))), nil
	case float64:
		return expNumber(decimal.NewFromFloat(x)), nil
	case big.Int:
		return expNumber(decimal.NewFromBigInt(&x, 0)), nil
	case decimal.Decimal:
		return expNumber(x), nil
	default:
		return expNull, fmt.Errorf("unsupported field value %T %v in expression", v, v)
	}
}

// expValueToField converts an expression result to the representation EntityBox.Data uses for
// a field of type typ. A null result becomes a typed nil for a nullable field and an untyped nil
// for a non-null field, which CheckValue then rejects with "cannot be null".
func expValueToField(typ types.Type, v expValue) (any, error) {
	typeChain := schema.BreakType(typ)
	nullable := typeChain.InnerTypeNullable()
	if v.isNull() {
		if !nullable {
			return nil, nil
		}
		_, zero := buildType(typ)
		return zero, nil
	}
	kind, err := fieldExpKind(typ)
	if err != nil {
		return nil, err
	}
	if kind != v.kind {
		return nil, fmt.Errorf("expression result %s is %s, expect %s for type %s", v, v.kind, kind, typ.String())
	}
	innerType := typeChain.InnerType()
	if scalarType, is := innerType.(*types.ScalarTypeDefinition); is {
		switch scalarType.Name {
		case "Int":
			result := int32(v.n.Round(0).IntPart())
			return utils.Select[any](nullable, &result, result), nil
		case "Int8", "Timestamp":
			result := v.n.Round(0).IntPart()
			return utils.Select[any](nullable, &result, result), nil
		case "BigInt":
			// BigInt is special, always use *big.Int regardless of nonNull declaration
			return v.n.Round(0).BigInt(), nil
		case "Float":
			result, _ := v.n.Float64()
			return utils.Select[any](nullable, &result, result), nil
		case "BigDecimal":
			result := v.n
			return utils.Select[any](nullable, &result, result), nil
		case "Boolean":
			result := v.b
			return utils.Select[any](nullable, &result, result), nil
		}
	}
	// String, ID, Bytes, enum and foreign key are all stored as string
	result := v.s
	return utils.Select[any](nullable, &result, result), nil
}
