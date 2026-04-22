package clickhouse

import (
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"math/big"
	"sentioxyz/sentio-core/common/chx"
	"testing"
	"time"
)

func Test_BuildTable(t *testing.T) {
	type obj struct {
		P1 uint64          `clickhouse:"p1" number_field:"true"`
		P2 string          `clickhouse:"p2" compression:"CODEC(ZSTD(1))"`
		P3 decimal.Decimal `clickhouse:"p3" projection:"pa/2"`
		P4 *big.Int        `clickhouse:"p4" projection:"pa/1"`
		P5 time.Time       `clickhouse:"p5" index:"minmax GRANULARITY 3"`
		P6 *big.Int        `clickhouse:"p6" index:"minmax"`
		P7 string          `clickhouse:"p7" type:"FixedString(66)"`
		P8 *string         `clickhouse:"p8" type:"Nullable(FixedString(66))" sub_number_field:"true"`
	}
	assert.Equal(t, TableSchema{
		Table: chx.Table{
			FullName: chx.FullName{Database: "db", Name: "object"},
			Config: chx.TableConfig{
				Engine:      chx.NewDefaultMergeTreeEngine(true),
				PartitionBy: "p1",
				OrderBy:     []string{"p2"},
			},
			Comment: "",
			Fields: chx.Fields{
				{Name: "p1", Type: chx.FieldTypeUInt64},
				{Name: "p2", Type: chx.FieldTypeString, CompressionCodec: "CODEC(ZSTD(1))"},
				{Name: "p3", Type: chx.FieldTypeDecimal{Precision: 76, Scale: 30}},
				{Name: "p4", Type: chx.FieldTypeNullable{Inner: chx.FieldTypeInt256}},
				{Name: "p5", Type: chx.FieldTypeDateTime64{Precision: 3, Timezone: "UTC"}},
				{Name: "p6", Type: chx.FieldTypeNullable{Inner: chx.FieldTypeInt256}},
				{Name: "p7", Type: chx.FieldTypeNormal("FixedString(66)")},
				{Name: "p8", Type: chx.FieldTypeNullable{Inner: chx.FieldTypeNormal("FixedString(66)")}},
			},
			Indexes: []chx.Index{
				{Name: "idx_p5", Type: "minmax", Expr: "`p5`", Granularity: 3},
				{Name: "idx_p6", Type: "minmax", Expr: "`p6`", Granularity: 1},
			},
			Projections: []chx.Projection{{
				Name:  "pa",
				Query: "SELECT p4, p3 ORDER BY p4, p3",
			}},
		},
		NumberField:    "p1",
		SubNumberField: "p8",
	}, BuildTable(
		chx.FullName{Database: "db", Name: "object"},
		obj{},
		chx.TableConfig{
			Engine:      chx.NewDefaultMergeTreeEngine(true),
			PartitionBy: "p1",
			OrderBy:     []string{"p2"},
		},
		"",
	))
}
