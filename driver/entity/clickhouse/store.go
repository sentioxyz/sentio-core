package clickhouse

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"reflect"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"

	"github.com/ClickHouse/clickhouse-go/v2"
	clickhouselib "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const (
	nameQuoteMarks = '`'
)

type TableOption struct {
	// Determines the batch size of the batch insert
	BatchInsertSizeLimit int
	// for the IN and NOT_IN filter conditions of the primary key,
	// if the id collection size exceeds this value, a temporary table will be used
	HugeIDSetSize uint

	TableSettings map[string]string
}

var (
	defaultBatchInsertSize = envconf.LoadUInt64("SENTIO_DEFAULT_ENTITY_BATCH_INSERT_SIZE",
		1000, envconf.WithMin(10), envconf.WithMax(2000))
	defaultHugeIDSetSize = envconf.LoadUInt64("SENTIO_DEFAULT_ENTITY_HUGE_ID_SET_SIZE",
		1000, envconf.WithMin(10), envconf.WithMax(2000))
	enableClickhouseLightDelete = envconf.LoadBool("SENTIO_ENABLE_CLICKHOUSE_LIGHT_DELETE", true)

	// TODO should depend on the actually number of replicas
	versionedCollapsingInsertQuorum = envconf.LoadUInt64("SENTIO_VERSIONED_COLLAPSING_INSERT_QUORUM", 1, envconf.WithMin(1))
)

var DefaultCreateTableOption = TableOption{
	BatchInsertSizeLimit: int(defaultBatchInsertSize),
	HugeIDSetSize:        uint(defaultHugeIDSetSize),
	TableSettings:        map[string]string{"index_granularity": "8192"},
}

func init() {
	if enableClickhouseLightDelete {
		chx.WithLightDeleteTableSettings(DefaultCreateTableOption.TableSettings)
	}
}

type Features struct {
	// false: BigDecimal use Decimal256(30) in clickhouse
	// true:  BigDecimal use String in clickhouse
	BigDecimalUseString bool

	// true:  BigDecimal columns use native Decimal512(60) in clickhouse.
	// This flag takes precedence over BigDecimalUseString.
	BigDecimalUseDecimal512 bool

	// false: Timestamp use Int64 in clickhouse
	// true:  Timestamp use DateTime64(6) in clickhouse
	TimestampUseDateTime64 bool

	// false: BigInt use Tuple(Bool,Int8,UInt256) in clickhouse
	// true:  BigInt use Int256 in clickhouse
	BigIntUseInt256 bool

	// false: [Int32!]! use (JSON) String in clickhouse
	// true:  [Int32!]! use Array[Int32] in clickhouse and [Int32!] will be rejected
	ArrayUseArray bool

	// true: contains control fields: __version__ and __sign__
	VersionedCollapsing bool
}

func BuildFeatures(schemaVersion int32) Features {
	feaOpt := Features{
		BigDecimalUseString:     (schemaVersion & 1) > 0,
		TimestampUseDateTime64:  (schemaVersion & 2) > 0,
		VersionedCollapsing:     (schemaVersion & 2) > 0,
		BigIntUseInt256:         (schemaVersion & 4) > 0,
		ArrayUseArray:           (schemaVersion & 4) > 0,
		BigDecimalUseDecimal512: (schemaVersion & 8) > 0,
	}
	if schemaVersion > 15 {
		panic(fmt.Errorf("schema version is %d, must be in [0,15]", schemaVersion))
	}
	return feaOpt
}

func (f Features) BuildVerifyOptions() []schema.VerifyOption {
	if f.ArrayUseArray {
		return []schema.VerifyOption{schema.OptionNullableArrayDenied}
	}
	return nil
}

type Store struct {
	ctrl        chx.Controller
	database    string
	processorID string
	feaOpt      Features
	sch         *schema.Schema
	schHash     string
	tableOpt    TableOption
}

func enableVersionedCollapsingInsertSettings() map[string]any {
	return map[string]any{
		"insert_quorum":          int(versionedCollapsingInsertQuorum),
		"insert_quorum_timeout":  60, // unit is second
		"insert_quorum_parallel": 1,
		"async_insert":           0,
	}
}

var (
	selectCtxSettings = map[string]any{
		"select_sequential_consistency": 1,
		"receive_timeout":               5, // unit is second
	}
)

func SelectCtx(parent context.Context) context.Context {
	return clickhouse.Context(parent, clickhouse.WithSettings(selectCtxSettings))
}

const schemaHashSalt = "2"

func NewStore(
	ctrl chx.Controller,
	processorID string,
	feaOpt Features,
	sch *schema.Schema,
	tableOpt TableOption,
) *Store {
	// build schema hash
	h := sha1.New()
	h.Write([]byte(schemaHashSalt))
	h.Write([]byte{'#'})
	for _, k := range utils.GetOrderedMapKeys(tableOpt.TableSettings) {
		h.Write([]byte(fmt.Sprintf("%s=%s#", k, tableOpt.TableSettings[k])))
	}
	h.Write([]byte(sch.SchemaString))
	return &Store{
		ctrl:        ctrl,
		database:    ctrl.GetDatabase(),
		processorID: processorID,
		feaOpt:      feaOpt,
		sch:         sch,
		schHash:     fmt.Sprintf("%x", h.Sum(nil)),
		tableOpt:    tableOpt,
	}
}

func (s *Store) GetEntityType(entity string) *schema.Entity {
	return s.sch.GetEntity(entity)
}

func (s *Store) GetEntityOrInterfaceType(name string) schema.EntityOrInterface {
	return s.sch.GetEntityOrInterface(name)
}

func (s *Store) buildTableOrViewName(category string, name string) string {
	return fmt.Sprintf("%s_%s_%s", s.processorID, category, name)
}

func (s *Store) fullName(tableName string) string {
	return fmt.Sprintf("`%s`.`%s`", s.database, tableName)
}

func (s *Store) VersionedTableName(item schema.EntityOrInterface) string {
	return s.buildTableOrViewName("versionedEntity", item.GetName())
}

func (s *Store) VersionedLatestTableName(item schema.EntityOrInterface) string {
	return s.buildTableOrViewName("versionedLatestEntity", item.GetName())
}

func (s *Store) VersionedLatestTableMaterializedViewName(item schema.EntityOrInterface) string {
	return s.buildTableOrViewName("versionedLatestEntityMV", item.GetName())
}

func (s *Store) TableName(item schema.EntityOrInterface) string {
	var category string
	switch item.(type) {
	case *schema.Entity:
		category = "entity"
	case *schema.Interface:
		category = "interface"
	case *schema.Aggregation:
		category = "aggregation"
	default:
		panic(fmt.Errorf("unreachable, unexpected item type %T", item))
	}
	return s.buildTableOrViewName(category, item.GetName())
}

func (s *Store) ViewName(item schema.EntityOrInterface) string {
	return s.buildTableOrViewName("view", item.GetName())
}

func (s *Store) LatestViewName(item schema.EntityOrInterface) string {
	return s.buildTableOrViewName("latestView", item.GetName())
}

func quote(cnt string) string {
	buf := bytes.NewBuffer(make([]byte, 0, len(cnt)+2))
	buf.WriteRune(nameQuoteMarks)
	buf.WriteString(cnt)
	buf.WriteRune(nameQuoteMarks)
	return buf.String()
}

func joinWithQuote(arr []string, sep string) string {
	var buf bytes.Buffer
	for i, item := range arr {
		if i > 0 {
			buf.WriteString(sep)
		}
		buf.WriteString(quote(item))
	}
	return buf.String()
}

func buildFieldBufferForScanMap(rows clickhouselib.Rows) []any {
	var buf = make([]any, len(rows.ColumnTypes()))
	for i, col := range rows.ColumnTypes() {
		buf[i] = reflect.New(col.ScanType()).Interface()
	}
	return buf
}

func scanMap(rows clickhouselib.Rows, fieldBuffer []any) (map[string]any, error) {
	err := rows.Scan(fieldBuffer...)
	if err != nil {
		return nil, err
	}
	row := make(map[string]any)
	for i, colName := range rows.Columns() {
		row[colName] = reflect.ValueOf(fieldBuffer[i]).Elem().Interface()
	}
	return row, nil
}
