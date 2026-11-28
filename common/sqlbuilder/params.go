package builder

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/anyutil"
)

// Values parsed by WithRichStructParameter. They usually come from a request, so they are never substituted
// verbatim: each type knows how to render itself as a literal for the lexical context its placeholder sits in.
type (
	// stringValue is caller-controlled text (RichValue string or bytes).
	stringValue string
	// literalValue is text that is already a safe SQL token (a number, a bool, or empty for null).
	literalValue string
	// timestampValue renders as toDateTime(...) in code and as the bare datetime inside a quoted string.
	timestampValue time.Time
)

const timestampLayout = "2006-01-02 15:04:05"

var (
	bareNumberRegexp = regexp.MustCompile(`^-?(0[xX][0-9a-fA-F]+|[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?)$`)
	bareIdentRegexp  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
	heredocRegexp    = regexp.MustCompile(`^\$[A-Za-z0-9_]*\$`)
)

// renderInCode renders a parameter value placed directly in SQL code, outside any quotes.
func renderInCode(v any) string {
	switch v := v.(type) {
	case stringValue:
		// A template like `where addr = ${addr}` worked only when the caller passed a ready-made token, such as
		// `'0xabc'`, `100`, a column name or a list of those. Keep accepting exactly those shapes, none of which
		// can change the structure of the query, and quote everything else as a string literal.
		if isSafeBareValue(string(v)) {
			return string(v)
		}
		return quoteString(string(v), '\'')
	case literalValue:
		return string(v)
	case timestampValue:
		return fmt.Sprintf("toDateTime('%s', 'UTC')", time.Time(v).Format(timestampLayout))
	default:
		return quoteString(anyutil.ParseString(v), '\'')
	}
}

// renderInQuotes renders a parameter value placed inside a quoted string or identifier, such as `'${addr}'`:
// the value becomes part of that literal, so it is escaped but not quoted again.
func renderInQuotes(v any, quote byte) string {
	switch v := v.(type) {
	case timestampValue:
		return escapeQuoted(time.Time(v).Format(timestampLayout), quote)
	default:
		return escapeQuoted(anyutil.ParseString(v), quote)
	}
}

// quoteString returns s as a ClickHouse quoted literal.
func quoteString(s string, quote byte) string {
	return string(quote) + escapeQuoted(s, quote) + string(quote)
}

// escapeQuoted escapes s for use between two `quote` characters, following ClickHouse's backslash escapes.
// Control characters are escaped too, so the result never contains a line break: even a context misjudged as
// code rather than a line comment could not be ended early by the value.
func escapeQuoted(s string, quote byte) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' || c == quote:
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
			fmt.Fprintf(&b, `\x%02X`, c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// isSafeBareValue reports whether s is a comma-separated list of items that are each a number, a (dotted)
// identifier or one complete single-quoted string literal. Items cannot contain whitespace, so no sequence
// of keywords can be formed, and a string literal must be closed where the item ends.
func isSafeBareValue(s string) bool {
	i := 0
	for {
		i = skipBlanks(s, i)
		end, ok := scanBareItem(s, i)
		if !ok {
			return false
		}
		i = skipBlanks(s, end)
		if i == len(s) {
			return true
		}
		if s[i] != ',' {
			return false
		}
		i++
	}
}

func skipBlanks(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

func scanBareItem(s string, i int) (int, bool) {
	if i >= len(s) {
		return i, false
	}
	if s[i] == '\'' {
		return scanStringLiteral(s, i)
	}
	j := i
	for j < len(s) && isBareChar(s[j]) {
		j++
	}
	item := s[i:j]
	return j, bareNumberRegexp.MatchString(item) || bareIdentRegexp.MatchString(item)
}

func isBareChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '_' || c == '.' || c == '+' || c == '-'
}

// scanStringLiteral scans the single-quoted literal starting at s[i] and returns the index just after it.
// Literals holding control characters are rejected, so that a bare value never spans lines.
func scanStringLiteral(s string, i int) (int, bool) {
	for j := i + 1; j < len(s); j++ {
		c := s[j]
		switch {
		case c < 0x20 || c == 0x7f:
			return j, false
		case c == '\\':
			if j+1 >= len(s) || s[j+1] < 0x20 || s[j+1] == 0x7f {
				return j, false
			}
			j++
		case c == '\'':
			if j+1 < len(s) && s[j+1] == '\'' {
				j++
				continue
			}
			return j + 1, true
		}
	}
	return len(s), false
}

type lexState int

const (
	stateCode lexState = iota
	stateQuoted
	stateLineComment
	stateBlockComment
	stateHeredoc
)

type placeholder struct {
	text  string
	value any
	rich  bool
}

// formatRichTemplate substitutes the placeholders of sqlTemplate while tracking where it is in the ClickHouse
// lexical structure. Context-map values are trusted SQL fragments and are substituted verbatim everywhere, as
// strings.Replacer did. RichStruct values are rendered by renderInCode in SQL code, by renderInQuotes inside a
// quoted string or identifier, and left untouched in comments and heredoc strings, where no escaping exists.
//
// When the lexer is unsure, it errs towards "comment": a placeholder it leaves unreplaced can only break the
// query, while one it wrongly renders as code could escape a context the server sees differently.
func formatRichTemplate(sqlTemplate string, args args, prefixList, suffixList []string) string {
	// Same candidates in the same order as the strings.Replacer path: longer names first, then identity order.
	// At any position the first candidate that matches wins, as with strings.Replacer.
	var placeholders []placeholder
	for _, a := range args {
		var rich bool
		switch a.v.(type) {
		case stringValue, literalValue, timestampValue:
			rich = true
		}
		for i := range prefixList {
			placeholders = append(placeholders, placeholder{text: prefixList[i] + a.k + suffixList[i], value: a.v, rich: rich})
		}
	}
	match := func(pos int) *placeholder {
		for i := range placeholders {
			if strings.HasPrefix(sqlTemplate[pos:], placeholders[i].text) {
				return &placeholders[i]
			}
		}
		return nil
	}

	var (
		out          strings.Builder
		state        = stateCode
		quote        byte
		commentDepth int
		heredocTag   string
	)
	out.Grow(len(sqlTemplate))
	for pos := 0; pos < len(sqlTemplate); {
		if p := match(pos); p != nil {
			var rendered string
			switch {
			case !p.rich:
				rendered = anyutil.ParseString(p.value)
			case state == stateCode:
				rendered = renderInCode(p.value)
				// `-${x}` with x = -1 would otherwise become `--1`, a comment hiding the rest of the line.
				if strings.HasSuffix(out.String(), "-") && strings.HasPrefix(rendered, "-") {
					out.WriteByte(' ')
				}
			case state == stateQuoted:
				rendered = renderInQuotes(p.value, quote)
			default:
				rendered = p.text
			}
			out.WriteString(rendered)
			pos += len(p.text)
			continue
		}

		rest := sqlTemplate[pos:]
		c := sqlTemplate[pos]
		n := 1
		switch state {
		case stateCode:
			switch {
			case c == '\'' || c == '"' || c == '`':
				state, quote = stateQuoted, c
			case strings.HasPrefix(rest, "--") || c == '#':
				state = stateLineComment
			case strings.HasPrefix(rest, "/*"):
				state, commentDepth, n = stateBlockComment, 1, 2
			case c == '$':
				if tag := heredocRegexp.FindString(rest); tag != "" {
					state, heredocTag, n = stateHeredoc, tag, len(tag)
				}
			}
		case stateQuoted:
			switch {
			case c == '\\' && pos+1 < len(sqlTemplate):
				n = 2
			case c == quote && pos+1 < len(sqlTemplate) && sqlTemplate[pos+1] == quote:
				n = 2
			case c == quote:
				state = stateCode
			}
		case stateLineComment:
			if c == '\n' {
				state = stateCode
			}
		case stateBlockComment:
			// Nested comments are tracked, so `/* /* */ ${x} */` still counts as inside a comment.
			switch {
			case strings.HasPrefix(rest, "/*"):
				commentDepth, n = commentDepth+1, 2
			case strings.HasPrefix(rest, "*/"):
				commentDepth, n = commentDepth-1, 2
				if commentDepth == 0 {
					state = stateCode
				}
			}
		case stateHeredoc:
			if strings.HasPrefix(rest, heredocTag) {
				state, n = stateCode, len(heredocTag)
			}
		}
		out.WriteString(sqlTemplate[pos : pos+n])
		pos += n
	}
	return out.String()
}
