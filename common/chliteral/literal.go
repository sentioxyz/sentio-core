// Package chliteral renders Go values as ClickHouse SQL literals and quoted identifiers.
//
// Use it wherever a value has to be written into the SQL text itself. Prefer query parameters
// when the statement is built in one place; these helpers are for the builders that assemble a
// statement from independently rendered fragments.
package chliteral

import (
	"regexp"
	"strings"
)

var escaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`, "`", "\\`")

// Escape returns s with the characters that are special inside a ClickHouse quoted literal or
// back-quoted identifier escaped. The result carries no surrounding quotes, so it can be placed
// inside a larger literal such as a LIKE pattern.
func Escape(s string) string {
	return escaper.Replace(s)
}

// String returns s as a single-quoted ClickHouse string literal.
func String(s string) string {
	return "'" + Escape(s) + "'"
}

// Identifier returns name as a back-quoted ClickHouse identifier.
func Identifier(name string) string {
	return "`" + Escape(name) + "`"
}

var numberExpr = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)

// IsNumber reports whether s is a plain decimal number that can be written into SQL unquoted.
// It accepts integers and decimals of any size, with an optional sign and exponent.
func IsNumber(s string) bool {
	return numberExpr.MatchString(s)
}
