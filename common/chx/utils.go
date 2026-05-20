package chx

import "unicode"

func cutBySpace(str string) (string, string, bool) {
	var lvl int
	for i, c := range []rune(str) {
		switch {
		case c == '(':
			lvl++
		case c == ')':
			lvl--
		case unicode.IsSpace(c):
			if lvl == 0 {
				return str[:i], str[i+1:], true
			}
		}
	}
	return str, "", false
}

func findNotIn(str string, target, quota rune) int {
	var in bool
	for i, c := range []rune(str) {
		switch c {
		case quota:
			in = !in
		case target:
			if !in {
				return i
			}
		}
	}
	return -1
}

func NewGetter[T any](raw []T, converter func(T) []any) func() ([]any, bool) {
	var cursor int
	return func() ([]any, bool) {
		if cursor >= len(raw) {
			return nil, false
		}
		cursor++
		return converter(raw[cursor-1]), true
	}
}
