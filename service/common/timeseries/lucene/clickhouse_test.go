package lucene

import (
	"strings"
	"testing"

	"sentioxyz/sentio-core/driver/timeseries"

	"github.com/blevesearch/bleve/search/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDriver() Driver {
	meta := timeseries.Metaset{
		"Transfer": timeseries.Meta{
			Name: "Transfer",
			Type: timeseries.MetaTypeEvent,
			Fields: map[string]timeseries.Field{
				"symbol": {Name: "symbol", Type: timeseries.FieldTypeString},
				"amount": {Name: "amount", Type: timeseries.FieldTypeBigFloat},
				"count":  {Name: "count", Type: timeseries.FieldTypeInt},
				"active": {Name: "active", Type: timeseries.FieldTypeBool},
				"seen":   {Name: "seen", Type: timeseries.FieldTypeTime},
			},
		},
	}
	return NewClickhouse("attributes", map[string]struct{}{"symbol": {}}, meta)
}

func render(t *testing.T, q string) (string, error) {
	t.Helper()
	ast, err := Parse(q)
	require.NoError(t, err, q)
	return testDriver().Render(ast)
}

// unquoted returns sql with every single-quoted literal removed, honouring backslash escapes.
// Whatever is left is the part of the statement that is parsed as SQL rather than as data.
func unquoted(sql string) string {
	var out strings.Builder
	inLiteral := false
	for i := 0; i < len(sql); i++ {
		switch c := sql[i]; {
		case inLiteral && c == '\\':
			i++
		case c == '\'':
			inLiteral = !inLiteral
		case !inLiteral:
			out.WriteByte(c)
		}
	}
	if inLiteral {
		return "<unterminated literal>"
	}
	return out.String()
}

func TestRender_OrdinaryQueries(t *testing.T) {
	for q, want := range map[string]string{
		`symbol:usdc`:         ", 'usdc')",
		`symbol:"Trader Joe"`: ", 'trader joe')",
		`symbol:"it's"`:       `, 'it\'s')`,
		`amount:"1.5"`:        "equals(",
		`count:"42"`:          ", 42)",
		`active:"true"`:       ", true)",
	} {
		sql, err := render(t, q)
		require.NoError(t, err, q)
		assert.Contains(t, sql, want, q)
	}
}

func TestRender_StringValuesStayInsideTheLiteral(t *testing.T) {
	for _, q := range []string{
		`symbol:"x') OR (SELECT count() FROM system.tables) > 0 OR equals('a', 'a"`,
		`symbol:"x\\"`,
		`symbol:"it's"`,
		`"free text') OR 1=1 --"`,
		`symbol:x'*`,
		`symbol:/a'|b/`,
	} {
		sql, err := render(t, q)
		if err != nil {
			continue // rejecting the query is fine too
		}
		rest := unquoted(sql)
		assert.NotContains(t, rest, "unterminated", "query %q rendered %s", q, sql)
		assert.NotContains(t, strings.ToLower(rest), "select", "query %q rendered %s", q, sql)
		assert.NotContains(t, rest, "1=1", "query %q rendered %s", q, sql)
	}
}

func TestRender_TermRangeBoundsStayInsideTheLiteral(t *testing.T) {
	q := query.NewTermRangeQuery("a') OR (SELECT 1) = ('1", `z\`)
	q.SetField("symbol")
	sql, err := testDriver().Render(q)
	require.NoError(t, err)
	rest := unquoted(sql)
	assert.NotContains(t, rest, "unterminated", sql)
	assert.NotContains(t, strings.ToLower(rest), "select", sql)
}

func TestRender_NonStringExactMatchMustBeALiteral(t *testing.T) {
	for _, q := range []string{
		`amount:"1) OR (SELECT count() FROM system.tables) > (0"`,
		`amount:"1 OR 1"`,
		`count:"(SELECT 1)"`,
		`active:"true) OR (1"`,
		`seen:"now()) OR (1"`,
	} {
		sql, err := render(t, q)
		assert.Error(t, err, "query %q rendered %s", q, sql)
	}
}

func TestRender_ExactTimeMatchKeepsFractionalSeconds(t *testing.T) {
	whole, err := render(t, `seen:"2026-11-26T08:00:00Z"`)
	require.NoError(t, err)
	micro, err := render(t, `seen:"2026-11-26T08:00:00.123456Z"`)
	require.NoError(t, err)
	other, err := render(t, `seen:"2026-11-26T08:00:00.123457Z"`)
	require.NoError(t, err)

	assert.Contains(t, whole, "toDateTime64('2026-11-26 08:00:00.000000', 6, 'UTC')")
	assert.Contains(t, micro, "toDateTime64('2026-11-26 08:00:00.123456', 6, 'UTC')")
	assert.NotEqual(t, whole, micro)
	assert.NotEqual(t, micro, other)
}
