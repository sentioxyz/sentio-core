package chx

import (
	"bytes"
	"fmt"
	"sentioxyz/sentio-core/common/utils"
	"strings"
)

type TableOrView interface {
	GetKind() string
	GetFullName() FullName
	GetComment() string
}

type FullName struct {
	Database string
	Name     string
}

func (fn FullName) String() string {
	return fn.Database + "." + fn.Name
}

func (fn FullName) InSQL() string {
	return fmt.Sprintf("`%s`.`%s`", fn.Database, fn.Name)
}

type Fields []Field

func (f Fields) FindByName(name string) (Field, int) {
	for i, field := range f {
		if field.Name == name {
			return field, i
		}
	}
	return Field{}, -1
}

func (f Fields) Names() []string {
	return utils.MapSliceNoError(f, func(fd Field) string {
		return fd.Name
	})
}

type Table struct {
	FullName

	Config  TableConfig
	Comment string

	Fields Fields

	Indexes []Index

	Projections []Projection
}

func (t Table) GetKind() string {
	return "table"
}

func (t Table) GetFullName() FullName {
	return t.FullName
}

func (t Table) GetComment() string {
	return t.Comment
}

type TableConfig struct {
	Engine      Engine
	PartitionBy string
	OrderBy     []string
	Settings    map[string]string
}

type Field struct {
	Name             string
	Type             string
	CompressionCodec string // for example 'CODEC(ZSTD(1))'
	DefaultExpr      string
	Comment          string
}

func (f Field) CreateSQL() string {
	var sql bytes.Buffer
	sql.WriteString(fmt.Sprintf("`%s` %s", f.Name, f.Type))
	if f.DefaultExpr != "" {
		sql.WriteString(fmt.Sprintf(" DEFAULT %s", f.DefaultExpr))
	}
	if f.Comment != "" {
		sql.WriteString(fmt.Sprintf(" COMMENT '%s'", f.Comment))
	}
	if f.CompressionCodec != "" {
		sql.WriteString(" ")
		sql.WriteString(f.CompressionCodec)
	}
	return sql.String()
}

func (f Field) HasSameType(a Field) bool {
	// remove all space and ignore case
	return strings.EqualFold(
		strings.ReplaceAll(f.Type, " ", ""),
		strings.ReplaceAll(a.Type, " ", ""))
}

type Index struct {
	Name        string
	Type        string
	Expr        string
	Granularity uint64
}

func (i Index) CreateSQL() string {
	return fmt.Sprintf("INDEX `%s` %s TYPE %s GRANULARITY %d", i.Name, i.Expr, i.Type, i.Granularity)
}

func (i Index) HasSameExpr(a Index) bool {
	e1, e2 := i.Expr, a.Expr
	if e1 == e2 {
		return true
	}
	if len(e1) < len(e2) {
		e1, e2 = e2, e1
	}
	return strings.ReplaceAll(e1, "`", "") == e2
}

func (i Index) Equal(a Index) bool {
	return i.Name == a.Name &&
		strings.EqualFold(i.Type, a.Type) &&
		i.HasSameExpr(a) &&
		i.Granularity == a.Granularity
}

type Projection struct {
	Name  string
	Query string
}

func (p Projection) CreateSQL() string {
	return fmt.Sprintf("PROJECTION `%s` (%s)", p.Name, p.Query)
}

func (p Projection) Equal(a Projection) bool {
	return p.Name == a.Name && p.Query == a.Query
}

type View struct {
	FullName

	Fields Fields

	Select string

	Comment string
}

func (v View) Equal(a View) bool {
	return v.FullName == a.FullName &&
		v.Select == a.Select &&
		v.Comment == a.Comment &&
		utils.ArrEqual(v.Fields, a.Fields)
}

func (v View) GetKind() string {
	return "view"
}

func (v View) GetFullName() FullName {
	return v.FullName
}

func (v View) GetComment() string {
	return v.Comment
}

type MaterializedView struct {
	View

	To FullName
}

func (v MaterializedView) Equal(a MaterializedView) bool {
	return v.To == a.To && v.View.Equal(a.View)
}

func (v MaterializedView) GetKind() string {
	return "materialized view"
}

func (v MaterializedView) GetFullName() FullName {
	return v.FullName
}

func (v MaterializedView) GetComment() string {
	return v.Comment
}
