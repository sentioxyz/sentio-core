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
