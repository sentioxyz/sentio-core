package builder

import (
	"math/big"
	"testing"
	"time"

	protoscommon "sentioxyz/sentio-core/service/common/protos"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func str(s string) *protoscommon.RichValue {
	return &protoscommon.RichValue{Value: &protoscommon.RichValue_StringValue{StringValue: s}}
}

// formatEndpoint formats tpl the way the SQL endpoint path does.
func formatEndpoint(tpl string, fields map[string]*protoscommon.RichValue) string {
	return FormatSQLTemplate(tpl, map[string]any{},
		WithParameterIdentity("$", ""),
		WithParameterIdentity("${", "}"),
		WithRichStructParameter(&protoscommon.RichStruct{Fields: fields}))
}

func Test_RichParameter_StringInjection(t *testing.T) {
	payloads := map[string]struct {
		value    string
		unquoted string // rendered for `where addr = ${addr}`
		quoted   string // rendered for `where addr = '${addr}'`
	}{
		"single quote": {
			value:    `x' OR 1=1 OR '`,
			unquoted: `'x\' OR 1=1 OR \''`,
			quoted:   `'x\' OR 1=1 OR \''`,
		},
		"backslash before quote": {
			value:    `x\' OR 1=1 --`,
			unquoted: `'x\\\' OR 1=1 --'`,
			quoted:   `'x\\\' OR 1=1 --'`,
		},
		"doubled quote": {
			value:    `x'' OR 1=1 --`,
			unquoted: `'x\'\' OR 1=1 --'`,
			quoted:   `'x\'\' OR 1=1 --'`,
		},
		"line comment": {
			value:    "1 -- ",
			unquoted: `'1 -- '`,
			quoted:   `'1 -- '`,
		},
		"block comment": {
			value:    "1 /* x */",
			unquoted: `'1 /* x */'`,
			quoted:   `'1 /* x */'`,
		},
		"union": {
			value:    "0 UNION ALL SELECT * FROM other",
			unquoted: `'0 UNION ALL SELECT * FROM other'`,
			quoted:   `'0 UNION ALL SELECT * FROM other'`,
		},
		"quoted union": {
			value:    "' UNION ALL SELECT * FROM other --",
			unquoted: `'\' UNION ALL SELECT * FROM other --'`,
			quoted:   `'\' UNION ALL SELECT * FROM other --'`,
		},
		"multiple statements": {
			value:    "1; DROP TABLE t",
			unquoted: `'1; DROP TABLE t'`,
			quoted:   `'1; DROP TABLE t'`,
		},
		"newline": {
			value:    "1\nUNION ALL SELECT 2",
			unquoted: `'1\nUNION ALL SELECT 2'`,
			quoted:   `'1\nUNION ALL SELECT 2'`,
		},
		"unterminated literal": {
			value:    `'abc`,
			unquoted: `'\'abc'`,
			quoted:   `'\'abc'`,
		},
		"literal followed by code": {
			value:    `'a' OR 1=1`,
			unquoted: `'\'a\' OR 1=1'`,
			quoted:   `'\'a\' OR 1=1'`,
		},
		"control characters": {
			value:    "a\r\t\x00\x01",
			unquoted: `'a\r\t\0\x01'`,
			quoted:   `'a\r\t\0\x01'`,
		},
		"settings": {
			value:    "1 SETTINGS readonly=0",
			unquoted: `'1 SETTINGS readonly=0'`,
			quoted:   `'1 SETTINGS readonly=0'`,
		},
		"heredoc": {
			value:    "$$ UNION ALL SELECT 1 $$",
			unquoted: `'$$ UNION ALL SELECT 1 $$'`,
			quoted:   `'$$ UNION ALL SELECT 1 $$'`,
		},
	}
	for name, p := range payloads {
		t.Run(name, func(t *testing.T) {
			fields := map[string]*protoscommon.RichValue{"addr": str(p.value)}
			assert.Equal(t, "select * from t where addr = "+p.unquoted+" limit 1",
				formatEndpoint("select * from t where addr = ${addr} limit 1", fields))
			assert.Equal(t, "select * from t where addr = "+p.unquoted+" limit 1",
				formatEndpoint("select * from t where addr = $addr limit 1", fields))
			assert.Equal(t, "select * from t where addr = "+p.quoted+" limit 1",
				formatEndpoint("select * from t where addr = '${addr}' limit 1", fields))
			assert.Equal(t, "select * from t where addr = "+p.quoted+" limit 1",
				formatEndpoint("select * from t where addr = '$addr' limit 1", fields))
		})
	}
}

func Test_RichParameter_StringIsAlwaysAValue(t *testing.T) {
	// A string parameter is a string value. Numbers, names, pre-quoted literals and SQL fragments passed through
	// one all become a string literal: a template cannot use a string parameter as a table, a column or a filter.
	for v, expected := range map[string]string{
		"100":           `'100'`,
		"-1":            `'-1'`,
		"0xdeadBEEF":    `'0xdeadBEEF'`,
		"amount":        `'amount'`,
		"otherdb.table": `'otherdb.table'`,
		"'0xabc'":       `'\'0xabc\''`,
		"'a','b'":       `'\'a\',\'b\''`,
		"balance > 100": `'balance > 100'`,
		"hello world":   `'hello world'`,
		"":              `''`,
		"f(1)":          `'f(1)'`,
		"a\"b`c":        "'a\\\"b\\`c'",
		"中文":            `'中文'`,
	} {
		t.Run(v, func(t *testing.T) {
			fields := map[string]*protoscommon.RichValue{"x": str(v)}
			assert.Equal(t, "select count(*) from "+expected, formatEndpoint("select count(*) from ${x}", fields))
			assert.Equal(t, "select count(*) from t where "+expected, formatEndpoint("select count(*) from t where ${x}", fields))
			assert.Equal(t, "select * from t where a in ("+expected+")", formatEndpoint("select * from t where a in ($x)", fields))
		})
	}
}

func Test_RichParameter_QuotedContexts(t *testing.T) {
	fields := map[string]*protoscommon.RichValue{"x": str("a'b\"c`d\\e")}
	escaped := "a\\'b\\\"c\\`d\\\\e"
	assert.Equal(t, `select 'pre `+escaped+` post'`, formatEndpoint(`select 'pre ${x} post'`, fields))
	assert.Equal(t, `select "`+escaped+`"`, formatEndpoint(`select "${x}"`, fields))
	assert.Equal(t, "select `"+escaped+"`", formatEndpoint("select `${x}`", fields))
	// Escaped and doubled quotes in the template do not end the literal.
	assert.Equal(t, `select 'it\'s `+escaped+`'`, formatEndpoint(`select 'it\'s ${x}'`, fields))
	assert.Equal(t, `select 'it''s `+escaped+`'`, formatEndpoint(`select 'it''s ${x}'`, fields))
	// After the literal closes, the placeholder is code again.
	assert.Equal(t, `select 'it''s', '`+escaped+`'`, formatEndpoint(`select 'it''s', ${x}`, fields))
}

func Test_RichParameter_CommentsAndHeredocsUntouched(t *testing.T) {
	fields := map[string]*protoscommon.RichValue{"x": str("1\n UNION ALL SELECT 2")}
	for _, tpl := range []string{
		"select 1 -- ${x}",
		"select 1 # ${x}",
		"select 1 /* ${x} */",
		"select 1 /* /* */ ${x} */",
		"select $$ ${x} $$",
		"select $tag$ ${x} $tag$",
	} {
		assert.Equal(t, tpl, formatEndpoint(tpl, fields))
	}
	assert.Equal(t, "select 1 -- ${x}\nwhere a = '1\\n UNION ALL SELECT 2'",
		formatEndpoint("select 1 -- ${x}\nwhere a = ${x}", fields))
	assert.Equal(t, "select /* c */ '1\\n UNION ALL SELECT 2'", formatEndpoint("select /* c */ ${x}", fields))
}

func Test_RichParameter_NoCommentFromMinus(t *testing.T) {
	assert.Equal(t, "select a - -5 from t where b = 1",
		formatEndpoint("select a -${x} from t where b = 1", map[string]*protoscommon.RichValue{
			"x": {Value: &protoscommon.RichValue_IntValue{IntValue: -5}},
		}))
}

func Test_RichParameter_Types(t *testing.T) {
	ts := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	fields := map[string]*protoscommon.RichValue{
		"i":    {Value: &protoscommon.RichValue_IntValue{IntValue: -42}},
		"i64":  {Value: &protoscommon.RichValue_Int64Value{Int64Value: 1 << 40}},
		"f":    {Value: &protoscommon.RichValue_FloatValue{FloatValue: 1.5}},
		"fi":   {Value: &protoscommon.RichValue_FloatValue{FloatValue: 100}},
		"b":    {Value: &protoscommon.RichValue_BoolValue{BoolValue: true}},
		"n":    {Value: &protoscommon.RichValue_NullValue_{}},
		"bi":   {Value: &protoscommon.RichValue_BigintValue{BigintValue: bigInt(-123456789)}},
		"bd":   {Value: &protoscommon.RichValue_BigdecimalValue{BigdecimalValue: bigDecimal(12345, -2)}},
		"ts":   {Value: &protoscommon.RichValue_TimestampValue{TimestampValue: timestamppb.New(ts)}},
		"by":   {Value: &protoscommon.RichValue_BytesValue{BytesValue: []byte("x' OR '1")}},
		"list": {Value: &protoscommon.RichValue_ListValue{ListValue: &protoscommon.RichValueList{}}},
	}
	assert.Equal(t,
		"select -42, 1099511627776, 1.50, 100, true, , -123456789, 123.45, "+
			"toDateTime('2024-03-04 05:06:07', 'UTC'), 'x\\' OR \\'1', ${list}",
		formatEndpoint("select ${i}, ${i64}, ${f}, ${fi}, ${b}, ${n}, ${bi}, ${bd}, ${ts}, ${by}, ${list}", fields))
	assert.Equal(t,
		"select '-42', '1099511627776', '1.50', '100', 'true', '', '-123456789', '123.45', "+
			"'2024-03-04 05:06:07', 'x\\' OR \\'1'",
		formatEndpoint("select '${i}', '${i64}', '${f}', '${fi}', '${b}', '${n}', '${bi}', '${bd}', '${ts}', '${by}'",
			fields))
}

func Test_RichParameter_OverlappingNamesAndContext(t *testing.T) {
	fields := map[string]*protoscommon.RichValue{
		"a":  str("x y"),
		"aa": str("1"),
	}
	assert.Equal(t, "select 'x y', '1', '1'b, $missing",
		formatEndpoint("select ${a}, ${aa}, $aab, $missing", fields))
	// Context-map values stay trusted fragments, substituted verbatim even with rich parameters present.
	assert.Equal(t, "select count() from events where a = 'x y'",
		FormatSQLTemplate("select {fn} from {table} where a = ${a}",
			map[string]any{"fn": "count()", "table": "events"},
			WithParameterIdentity("{", "}"),
			WithParameterIdentity("${", "}"),
			WithRichStructParameter(&protoscommon.RichStruct{Fields: fields})))
}

func bigInt(v int64) *protoscommon.BigInteger {
	b := big.NewInt(v)
	return &protoscommon.BigInteger{Negative: b.Sign() < 0, Data: new(big.Int).Abs(b).Bytes()}
}

func bigDecimal(v int64, exp int32) *protoscommon.BigDecimal {
	return &protoscommon.BigDecimal{Value: bigInt(v), Exp: exp}
}
