package chv4

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// clickhouse-go treats a query matching this pattern as using server-side query parameters,
// which fails a query binding positional args ("unsupported query parameter type").
var clickhouseQueryParamsRe = regexp.MustCompile("{.+:.+}")

func Test_prunedEffectsColumnHasNoBraces(t *testing.T) {
	assert.NotContains(t, prunedEffectsColumn, "{")
	assert.NotContains(t, prunedEffectsColumn, "}")
	assert.False(t, clickhouseQueryParamsRe.MatchString(prunedEffectsColumn+" FROM t WHERE checkpoint >= ?"))
}
