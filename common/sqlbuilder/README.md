# `sqlbuilder` package

This package provides small helpers for formatting SQL templates by replacing named placeholders with values.

It is primarily used to build ClickHouse and other SQL queries from templates while:

- Allowing **named parameters** instead of positional placeholders.
- Supporting **multiple placeholder syntaxes** (e.g. `{name}`, `$name`, `${name}`) in the same template.
- Accepting values both from a simple Go `map[string]any` and from a protobuf `RichStruct`.
- Correctly handling **overlapping parameter names** (e.g. `a`, `aa`, `aaa`).

## Files

- `builder.go` – implementation of the formatting functions and options.
- `builder_test.go` – unit tests for the different formatting styles and edge cases.
- `BUILD.bazel` – Bazel rules for the library and tests.

## Core API

### `FormatSQLTemplate`

```go
func FormatSQLTemplate(
    sqlTemplate string,
    context map[string]any,
    opts ...FormatOption,
) string
```

This is the flexible entry point. It:

1. Builds a list of **parameter identities** (prefix/suffix pairs).
2. Optionally parses a protobuf `RichStruct` into additional parameters.
3. Merges parameters from the `RichStruct` and the `context` map.
4. Sorts parameter names by **descending length** so that longer names are replaced first, avoiding conflicts like `a`, `aa`, `aaa`.
5. Constructs a `strings.Replacer` for all identities and parameters, then applies it to the template.

#### Parameter identities

Use `WithParameterIdentity` to configure which placeholder syntax is recognized:

```go
func WithParameterIdentity(prefix, suffix string) FormatOption
```

Examples:

- `{name}` (default – used when no identities are provided)
- `$name` with `WithParameterIdentity("$", "")`
- `${name}` with `WithParameterIdentity("${", "}")`

You can provide **multiple identities** in a single call:

```go
sql := FormatSQLTemplateWithOptions(
    "select $column from $table where ${column} = $value",
    map[string]any{
        "column": "id",
        "table":  "users",
        "value":  1,
    },
    WithParameterIdentity("$", ""),
    WithParameterIdentity("${", "}"),
)
// sql == "select id from users where id = 1"
```

If no `WithParameterIdentity` is provided, the default identity `{`/`}` is used, matching the behavior of `FormatSQLTemplate`.

#### RichStruct parameters

Use `WithRichStructParameter` to add parameters from a protobuf `RichStruct`:

```go
func WithRichStructParameter(richStruct *protoscommon.RichStruct) FormatOption
```

The helper converts field values into Go values (string, int, big.Int, decimal, bool, etc.). For timestamp values, it formats them as ClickHouse-compatible expressions:

```text
toDateTime('YYYY-MM-DD HH:MM:SS', 'UTC')
```

Example combining `map` args with `RichStruct` timestamps:

```go
opt := WithRichStructParameter(&protoscommon.RichStruct{
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
})

sql := FormatSQLTemplateWithOptions(
    "select $column from $table where start_time >= $start_time and end_time <= $end_time",
    map[string]any{
        "column": "id",
        "table":  "users",
    },
    WithParameterIdentity("$", ""),
    opt,
)

// sql == "select id from users where start_time >= toDateTime('2020-01-01 00:00:00', 'UTC') " +
//        "and end_time <= toDateTime('2020-01-02 00:00:00', 'UTC')"
```

If the same key exists in both the `RichStruct` and the `context` map, the implementation currently appends both into the replacer argument list; because keys are sorted and then passed to `strings.NewReplacer`, the **later value for a given identity wins**. Prefer to keep keys unique across the two sources.

## Behavior notes and edge cases

Some important properties covered by tests:

- **Overlapping names** – because parameter names are sorted by length, `aaa` is replaced before `aa`, which is replaced before `a`, so templates like `select $a from $aa where $aaa=c` work as expected.
- **Missing arguments** – placeholders with no matching entry in either map or `RichStruct` are left unchanged in the output.
- **Extra arguments** – keys that do not appear in the template are simply ignored.
- **Repeated placeholders** – every occurrence of a placeholder is replaced.
- **Empty/constant templates** – if there are no placeholders, or the template is empty, the original string is returned.

## Building and testing

This package is wired into Bazel via `BUILD.bazel`:

- Library target: `//common/sqlbuilder:sqlbuilder`
- Test target: `//common/sqlbuilder:sqlbuilder_test`

From the repo root you can run tests either via Go or Bazel.

### Using `go test`

```bash
cd common/sqlbuilder
go test ./...
```

### Using Bazel

```bash
bazel test //common/sqlbuilder:sqlbuilder_test
```

## When to use which function

- Use **`FormatSQLTemplate`** when:
  - You only need `{name}` placeholders.
  - You do not need protobuf-based parameters.

- Use **`FormatSQLTemplateWithOptions`** when:
  - You need custom or multiple placeholder syntaxes (`$name`, `${name}`, `{name}`, etc.).
  - You want to pull parameters from a `RichStruct` (e.g., timestamps formatted for ClickHouse).
  - You are dealing with overlapping parameter names and want deterministic replacement order.

This README is intentionally kept focused on the `sqlbuilder` package; see the repository-level `README.md` for broader project information.

