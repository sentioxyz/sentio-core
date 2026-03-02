package lucene

import (
	"unicode"
)

const (
	threshold = 0.000001
)

func isAlphaASCII(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func isNumericASCII(r rune) bool {
	return r >= '0' && r <= '9'
}

func isAlphaNumericASCII(r rune) bool {
	return isAlphaASCII(r) || isNumericASCII(r)
}

func isASCII(r rune) bool {
	return r <= unicode.MaxASCII
}

func isTokenSeparator(r rune) bool {
	return !isAlphaNumericASCII(r) && isASCII(r)
}

func hasTokenSeparator(s string) bool {
	// refer to https://clickhouse.com/codebrowser/ClickHouse/src/Common/StringSearcher.h.html#871
	for _, r := range s {
		if isTokenSeparator(r) {
			return true
		}
	}
	return false
}
