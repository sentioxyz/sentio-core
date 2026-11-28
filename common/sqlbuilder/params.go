package builder

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/anyutil"
	"sentioxyz/sentio-core/common/chliteral"
)

// Values parsed by WithRichStructParameter. They usually come from a request, so they are never substituted
// verbatim: each type knows how to render itself as a literal for the lexical context its placeholder sits in.
type (
	// stringValue is caller-controlled text (RichValue string or bytes). A string parameter is always a string
	// value, never SQL: a table name, a column name or a condition cannot be passed through one.
	stringValue string
	// literalValue is text that is already a safe SQL token (a number or a bool).
	literalValue string
	// nullValue renders as NULL in code and as an empty string inside a quoted string.
	nullValue struct{}
	// timestampValue renders as toDateTime(...) in code and as the bare datetime inside a quoted string.
	timestampValue time.Time
)

const timestampLayout = "2006-01-02 15:04:05"

var heredocRegexp = regexp.MustCompile(`^\$[A-Za-z0-9_]*\$`)

// renderInCode renders a parameter value placed directly in SQL code, outside any quotes.
func renderInCode(v any) string {
	switch v := v.(type) {
	case literalValue:
		return string(v)
	case nullValue:
		return "NULL"
	case timestampValue:
		return fmt.Sprintf("toDateTime('%s', 'UTC')", time.Time(v).Format(timestampLayout))
	default:
		return chliteral.String(anyutil.ParseString(v))
	}
}

// renderInQuotes renders a parameter value placed inside a quoted string or identifier, such as `'${addr}'`:
// the value becomes part of that literal, so it is escaped but not quoted again.
func renderInQuotes(v any) string {
	switch v := v.(type) {
	case nullValue:
		return ""
	case timestampValue:
		return chliteral.Escape(time.Time(v).Format(timestampLayout))
	default:
		return chliteral.Escape(anyutil.ParseString(v))
	}
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
		case stringValue, literalValue, nullValue, timestampValue:
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
				rendered = renderInQuotes(p.value)
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
