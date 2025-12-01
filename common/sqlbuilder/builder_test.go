package builder

import (
	"testing"
	"time"

	protoscommon "sentioxyz/sentio-core/service/common/protos"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type sqlTemplateTest struct {
	sqlTemplate string
	args        map[string]any
	expected    string
	options     []FormatOption
}

var (
	sqlTemplateTests = []sqlTemplateTest{
		{
			"select $column from $table where start_time >= $start_time and end_time <= $end_time",
			map[string]any{
				"column": "id",
				"table":  "users",
			},
			"select id from users where start_time >= toDateTime('2020-01-01 00:00:00', 'UTC') and end_time <= toDateTime('2020-01-02 00:00:00', 'UTC')",
			[]FormatOption{
				WithParameterIdentity("$", ""),
				WithRichStructParameter(&protoscommon.RichStruct{
					Fields: map[string]*protoscommon.RichValue{
						"start_time": {
							Value: &protoscommon.RichValue_TimestampValue{
								TimestampValue: timestamppb.New(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
							},
						},
						"end_time": {
							Value: &protoscommon.RichValue_TimestampValue{
								TimestampValue: timestamppb.New(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)),
							},
						},
					},
				}),
			},
		},
		{
			"select $column from $table where ${column} = $value",
			map[string]any{
				"column": "id",
				"table":  "users",
				"value":  1,
			},
			"select id from users where id = 1",
			[]FormatOption{
				WithParameterIdentity("$", ""),
				WithParameterIdentity("${", "}"),
			},
		},
		{
			"select $a from $aa where $aaa=c",
			map[string]any{
				"aa":  "users",
				"a":   "id",
				"aaa": "type",
			},
			"select id from users where type=c",
			[]FormatOption{
				WithParameterIdentity("$", ""),
				WithParameterIdentity("${", "}"),
			},
		},
		{
			"select {a} from {aa} where {aaaa}={b}",
			map[string]any{
				"a":    "id",
				"aa":   "users",
				"aaaa": "type",
				"b":    "'transfer'",
			},
			"select id from users where type='transfer'",
			[]FormatOption{},
		},
		{
			"select ${a} from ${aa} where ${aaaa}=${b}",
			map[string]any{
				"a":    "id",
				"aa":   "users",
				"aaaa": "type",
				"b":    "'transfer'",
			},
			"select id from users where type='transfer'",
			[]FormatOption{
				WithParameterIdentity("$", ""),
				WithParameterIdentity("${", "}"),
			},
		},
		{
			"select $a from $aa where $aaaa='transfer'",
			map[string]any{
				"a":    "id",
				"aa":   "a",
				"aaaa": "aa",
			},
			"select id from a where aa='transfer'",
			[]FormatOption{
				WithParameterIdentity("$", ""),
				WithParameterIdentity("${", "}"),
			},
		},
	}
)

func Test_FormatSQLTemplate(t *testing.T) {
	for _, test := range sqlTemplateTests {
		actual := FormatSQLTemplate(test.sqlTemplate, test.args, test.options...)
		if actual != test.expected {
			t.Errorf("Expected %s, but got %s", test.expected, actual)
		}
	}
}

// Test additional edge cases for FormatSQLTemplateWithOptions.
func Test_FormatSQLTemplate_EdgeCases(t *testing.T) {
	tests := []sqlTemplateTest{
		// Missing argument: placeholder without value in args or RichStruct should stay unchanged.
		{
			"select $column from $table where id = $missing",
			map[string]any{
				"column": "id",
				"table":  "users",
			},
			"select id from users where id = $missing",
			[]FormatOption{
				WithParameterIdentity("$", ""),
			},
		},
		// Extra args not used in template are ignored.
		{
			"select $column from $table",
			map[string]any{
				"column":  "id",
				"table":   "users",
				"ignored": "something",
			},
			"select id from users",
			[]FormatOption{
				WithParameterIdentity("$", ""),
			},
		},
		// Repeated placeholder occurrences are all replaced.
		{
			"select * from $table where $table.id = 1 or $table.id = 2",
			map[string]any{
				"table": "users",
			},
			"select * from users where users.id = 1 or users.id = 2",
			[]FormatOption{
				WithParameterIdentity("$", ""),
			},
		},
		// Empty template remains empty.
		{
			"",
			map[string]any{
				"anything": "value",
			},
			"",
			[]FormatOption{
				WithParameterIdentity("$", ""),
			},
		},
		// Template with no placeholders is unchanged.
		{
			"select 1 + 1",
			map[string]any{
				"x": 10,
			},
			"select 1 + 1",
			[]FormatOption{
				WithParameterIdentity("$", ""),
			},
		},
		// Overlapping names with default { } identity.
		{
			"select {a} from {aa} where {aaa} = 1",
			map[string]any{
				"a":   "id",
				"aa":  "users",
				"aaa": "type",
			},
			"select id from users where type = 1",
			[]FormatOption{},
		},
		// Mixed args and RichStruct in one template.
		{
			"select $column from $table where ts between $start_time and $end_time",
			map[string]any{
				"column": "id",
				"table":  "events",
			},
			"select id from events where ts between toDateTime('2020-01-01 00:00:00', 'UTC') and toDateTime('2020-01-02 00:00:00', 'UTC')",
			[]FormatOption{
				WithParameterIdentity("$", ""),
				WithRichStructParameter(&protoscommon.RichStruct{
					Fields: map[string]*protoscommon.RichValue{
						"start_time": {
							Value: &protoscommon.RichValue_TimestampValue{
								TimestampValue: timestamppb.New(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
							},
						},
						"end_time": {
							Value: &protoscommon.RichValue_TimestampValue{
								TimestampValue: timestamppb.New(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)),
							},
						},
					},
				}),
			},
		},
		// Default { } identity with RichStruct timestamp.
		{
			"where created_at >= {start_time}",
			map[string]any{},
			"where created_at >= toDateTime('2020-01-01 00:00:00', 'UTC')",
			[]FormatOption{
				WithRichStructParameter(&protoscommon.RichStruct{
					Fields: map[string]*protoscommon.RichValue{
						"start_time": {
							Value: &protoscommon.RichValue_TimestampValue{
								TimestampValue: timestamppb.New(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
							},
						},
					},
				}),
			},
		},
	}

	for _, test := range tests {
		actual := FormatSQLTemplate(test.sqlTemplate, test.args, test.options...)
		if actual != test.expected {
			t.Errorf("sqlTemplate: %q, expected %q, got %q", test.sqlTemplate, test.expected, actual)
		}
	}
}

// Basic tests for FormatSQLTemplate (without options, using default { } identity).
func Test_FormatSQLTemplate_Default(t *testing.T) {
	tests := []struct {
		name        string
		sqlTemplate string
		args        map[string]any
		expected    string
	}{
		{
			name:        "simple replacement",
			sqlTemplate: "select {column} from {table}",
			args: map[string]any{
				"column": "id",
				"table":  "users",
			},
			expected: "select id from users",
		},
		{
			name:        "overlapping names",
			sqlTemplate: "select {a} from {aa}",
			args: map[string]any{
				"a":  "id",
				"aa": "users",
			},
			expected: "select id from users",
		},
	}

	for _, tt := range tests {
		actual := FormatSQLTemplate(tt.sqlTemplate, tt.args)
		if actual != tt.expected {
			t.Errorf("%s: expected %q, got %q", tt.name, tt.expected, actual)
		}
	}
}
