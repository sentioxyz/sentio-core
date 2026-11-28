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

// Escape returns s with the characters that are special inside a ClickHouse quoted literal or
// quoted identifier escaped, whichever of ', " or ` encloses it. The result carries no surrounding
// quotes, so it can be placed inside a larger literal such as a LIKE pattern.
//
// Control characters are escaped too, so the result never contains a line break: even text that a
// reader takes for a line comment rather than a literal cannot be ended early by the value. ClickHouse
// reads every escape used here (\\, \', \", \`, \n, \r, \t, \0, \xHH) back as the original byte.
func Escape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' || c == '\'' || c == '"' || c == '`':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c == 0:
			b.WriteString(`\0`)
		case c < 0x20 || c == 0x7f:
			b.WriteString(`\x`)
			b.WriteByte(hexDigits[c>>4])
			b.WriteByte(hexDigits[c&0xf])
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

const hexDigits = "0123456789ABCDEF"

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
