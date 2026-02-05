package utils

import (
	"bytes"
	"fmt"
	"golang.org/x/exp/constraints"
	"regexp"
	"strconv"
	"strings"
)

func ContainsAny(str string, kws []string) bool {
	for _, kw := range kws {
		if strings.Contains(str, kw) {
			return true
		}
	}
	return false
}

func ContainsAnyIgnoreCase(str string, kws []string) bool {
	for _, kw := range kws {
		if strings.Contains(strings.ToLower(str), strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func HasAnyPrefix(str string, prefixes []string) bool {
	for _, kw := range prefixes {
		if strings.HasPrefix(str, kw) {
			return true
		}
	}
	return false
}

func AnyContains(src []string, kw string) bool {
	for _, str := range src {
		if strings.Contains(str, kw) {
			return true
		}
	}
	return false
}

func FindContains(src []string, kw string) (string, int) {
	for i, str := range src {
		if strings.Contains(str, kw) {
			return str, i
		}
	}
	return "", -1
}

func JoinWithQuote(raw []string, delim string, quote string) string {
	var buf bytes.Buffer
	for i, item := range raw {
		if i > 0 {
			buf.WriteString(delim)
		}
		buf.WriteString(quote)
		buf.WriteString(item)
		buf.WriteString(quote)
	}
	return buf.String()
}

func Dup(str, delim string, num int) string {
	if num <= 0 {
		return ""
	}
	s := []byte(str)
	d := []byte(delim)
	bl := len(s) + len(d)
	r := make([]byte, bl*num-len(d))
	for i := 0; i < num; i++ {
		if len(s) > 0 {
			copy(r[i*bl:i*bl+len(s)], s)
		}
		if len(d) > 0 && i+1 < num {
			copy(r[i*bl+len(s):(i+1)*bl], d)
		}
	}
	return string(r)
}

func AllToLower(ss []string) []string {
	r := make([]string, len(ss))
	for i, s := range ss {
		r[i] = strings.ToLower(s)
	}
	return r
}

func SplitAndGet(str, sep string, index int) string {
	s := strings.Split(str, sep)
	if index >= 0 {
		return s[index]
	}
	return s[len(s)+index]
}

func StringSummaryV2(d string) string {
	return StringSummaryV1(d, 100, 100)
}

func StringSummaryV1(d string, pvLen ...int) string {
	var headLen = 100
	var tailLen = 100
	if len(pvLen) > 0 {
		headLen = pvLen[0]
	}
	if len(pvLen) > 1 {
		tailLen = pvLen[1]
	}
	if len(d) <= headLen+tailLen {
		return d
	}
	var b bytes.Buffer
	b.WriteString(d[:headLen])
	b.WriteString(fmt.Sprintf("...(ignored %d)...", len(d)-headLen-tailLen))
	b.WriteString(d[len(d)-tailLen:])
	return b.String()
}

func StringSummary(raw any, maxLen int) string {
	var s string
	if str, is := raw.(fmt.Stringer); is {
		s = str.String()
	} else if sv, is := raw.(string); is {
		s = sv
	} else if bs, is := raw.([]byte); is {
		s = string(bs)
	} else {
		s = MustJSONMarshal(raw)
	}
	if len(s) <= maxLen {
		return s
	}
	return fmt.Sprintf("%s...(total len %d)", s[:maxLen], len(s))
}

// SplitByRunes the result will always be an odd number of strings, and the odd subscripts are delimiters
func SplitByRunes(origin string, delim []rune, escape rune) []string {
	var sectors []string
	raw := []rune(origin)
	cur := make([]rune, 0, len(raw))
	for s, n := 0, len(raw); s < n; s++ {
		if raw[s] == escape && s+1 < n {
			// is escape rune and not the last rune
			s++
			cur = append(cur, raw[s])
			continue
		}
		i := IndexOf(delim, raw[s])
		if i < 0 {
			cur = append(cur, raw[s])
			continue
		}
		sectors = append(sectors, string(cur))
		cur = cur[:0]
		sectors = append(sectors, string(delim[i]))
	}
	return append(sectors, string(cur))
}

func Escape(origin string, ctrl []rune, escape rune) string {
	var result []rune
	raw := []rune(origin)
	for i, n := 0, len(raw); i < n; i++ {
		if IndexOf(ctrl, raw[i]) >= 0 {
			result = append(result, escape, raw[i])
		} else {
			result = append(result, raw[i])
		}
	}
	return string(result)
}

func LikePatternToRegexp(pattern string) *regexp.Regexp {
	const ctrl = ".*+?^$|()[]{}\\"
	sectors := SplitByRunes(pattern, []rune{'_', '%'}, '\\')
	var expr bytes.Buffer
	expr.WriteRune('^')
	for i, n := 0, len(sectors); i < n; i += 2 {
		expr.WriteString(Escape(sectors[i], []rune(ctrl), '\\'))
		if i+1 < n {
			switch sectors[i+1] {
			case "_":
				expr.WriteRune('.')
			case "%":
				expr.WriteString(".*")
			}
		}
	}
	expr.WriteRune('$')
	return regexp.MustCompile(expr.String())
}

func AddPrefix(prefix string, arr []string) []string {
	r := make([]string, len(arr))
	for i, s := range arr {
		r[i] = prefix + s
	}
	return r
}

func NullOrToString[F fmt.Stringer](f *F) *string {
	return NullOrConvert(f, F.String)
}

func NullOrFromString[T any](str *string, loader func(string) T) *T {
	return NullOrConvert(str, loader)
}

func NullOrConvert[F any, T any](src *F, converter func(F) T) *T {
	if src == nil {
		return nil
	}
	dst := converter(*src)
	return &dst
}

func EmptyStringIfNil(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func NilIfEmptyString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func UIntFormatter(base int) func(uint64) string {
	return func(n uint64) string {
		return strconv.FormatUint(n, base)
	}
}

func UIntValueFormatter[V constraints.Unsigned](base int) func(V) string {
	return func(n V) string {
		return strconv.FormatUint(uint64(n), base)
	}
}

func StringsLenSum(arr []string) (sum int) {
	for _, s := range arr {
		sum += len(s)
	}
	return sum
}
