package clickhouse

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"reflect"
	"sentioxyz/sentio-core/common/chx"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	"strings"

	clickhouselib "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const (
	nameQuoteMarks = '`'
)

type TableOption struct {
	// Determines the batch size of the batch insert
	BatchInsertSizeLimit int
	// for the IN and NOT_IN filter conditions of the primary key,
	// if the id collection size exceeds this value, the ids are sent as an external table instead of SQL args
	HugeIDSetSize uint

	TableSettings map[string]string
}

var (
	defaultBatchInsertSize = envconf.LoadUInt64("SENTIO_DEFAULT_ENTITY_BATCH_INSERT_SIZE",
		1000, envconf.WithMin(10), envconf.WithMax(2000))
	defaultHugeIDSetSize = envconf.LoadUInt64("SENTIO_DEFAULT_ENTITY_HUGE_ID_SET_SIZE",
		1000, envconf.WithMin(10), envconf.WithMax(2000))

	// TODO should depend on the actually number of replicas
	versionedCollapsingInsertQuorum = envconf.LoadUInt64("SENTIO_VERSIONED_COLLAPSING_INSERT_QUORUM", 1, envconf.WithMin(1))
)

var DefaultCreateTableOption = TableOption{
	BatchInsertSizeLimit: int(defaultBatchInsertSize),
	HugeIDSetSize:        uint(defaultHugeIDSetSize),
	TableSettings:        map[string]string{"index_granularity": "8192"},
}

func init() {
	chx.WithLightDeleteTableSettings(DefaultCreateTableOption.TableSettings)
}

type Features struct {
	// false: BigDecimal use Decimal256(30) in clickhouse
	// true:  BigDecimal use String in clickhouse
	BigDecimalUseString bool

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
		BigDecimalUseString:    (schemaVersion & 1) > 0,
		TimestampUseDateTime64: (schemaVersion & 2) > 0,
		VersionedCollapsing:    (schemaVersion & 2) > 0,
		BigIntUseInt256:        (schemaVersion & 4) > 0,
		ArrayUseArray:          (schemaVersion & 4) > 0,
	}
	if schemaVersion > 7 {
		panic(fmt.Errorf("schema version is %d, must be in [0,7]", schemaVersion))
	}
	return feaOpt
}

func (f Features) BuildVerifyOptions() []schema.VerifyOption {
	if f.ArrayUseArray {
		return []schema.VerifyOption{schema.OptionNullableArrayDenied}
	}
	return nil
}

type Probe interface {
	PreCreateTable(ctx context.Context, tv chx.TableOrView) error
}

type emptyProbe struct{}

func (e emptyProbe) PreCreateTable(_ context.Context, _ chx.TableOrView) error { return nil }

type Store struct {
	ctrl     chx.Controller
	feaOpt   Features
	sch      *schema.Schema
	schHash  string
	tableOpt TableOption
	probe    Probe

	// gate holds the reads back until the replicas have what this store wrote, see waitSynced
	gate *chx.ReplicaGate
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
	return ckhmanager.ContextMergeSettings(parent, selectCtxSettings)
}

const schemaHashSalt = "2"

func NewStore(
	ctrl chx.Controller,
	feaOpt Features,
	sch *schema.Schema,
	tableOpt TableOption,
	probe Probe,
) *Store {
	// build schema hash
	h := sha1.New()
	h.Write([]byte(schemaHashSalt))
	h.Write([]byte{'#'})
	for _, k := range utils.GetOrderedMapKeys(tableOpt.TableSettings) {
		h.Write([]byte(fmt.Sprintf("%s=%s#", k, tableOpt.TableSettings[k])))
	}
	h.Write([]byte(sch.SchemaString))
	if probe == nil {
		probe = emptyProbe{}
	}
	return &Store{
		ctrl:     ctrl,
		feaOpt:   feaOpt,
		sch:      sch,
		schHash:  fmt.Sprintf("%x", h.Sum(nil)),
		tableOpt: tableOpt,
		probe:    probe,
		gate:     chx.NewReplicaGate(ctrl),
	}
}

// inserts is the scope every INSERT of the store into the tables of chain is tagged with, so that
// a reorg can end the ones a failed commit left running on the server, see chx.InsertScope. It is
// per chain because the tables are shared by the chains of the processor while each chain commits
// and reorgs on its own.
func (s *Store) inserts(chain string) chx.InsertScope {
	return chx.NewInsertScope(s.ctrl, "entity@"+chain)
}

// rowTable is the table that physically holds the rows of an entity or aggregation, which is where
// a write lands and what a replica has to fetch. The plain entity name of a versioned-collapsing
// entity is a view over it. A cache entity has none.
func (s *Store) rowTable(item schema.EntityOrInterface) (string, bool) {
	if entityType, is := item.(*schema.Entity); is && entityType.IsCache() {
		return "", false
	}
	if s.useVersionedCollapsingTable(item) {
		return s.VersionedTableName(item), true
	}
	return s.TableName(item), true
}

// dataTables adds to rowTable the latest table of a versioned-collapsing entity, which a
// materialized view feeds on every insert into the versioned one and which only the scans over
// every id read (countEntity, getAllID).
func (s *Store) dataTables(item schema.EntityOrInterface) []string {
	table, has := s.rowTable(item)
	if !has {
		return nil
	}
	if s.useVersionedCollapsingTable(item) {
		return []string{table, s.VersionedLatestTableName(item)}
	}
	return []string{table}
}

// waitSynced must precede every read of the rows of item. A commit prunes its changes from memory
// as soon as its inserts return, which is before the other replicas have fetched them, and the
// read may be answered by any replica: without the wait it can miss the rows of the commit that
// just landed, read a pre-value that is one version behind, or load a cache that stays short of
// them. It only costs something for a table written since the last wait.
func (s *Store) waitSynced(ctx context.Context, item schema.EntityOrInterface) error {
	table, has := s.rowTable(item)
	if !has {
		return nil
	}
	return s.gate.WaitSynced(ctx, table)
}

// waitAllSynced is waitSynced for the reads that also go to the latest table, see dataTables.
func (s *Store) waitAllSynced(ctx context.Context, item schema.EntityOrInterface) error {
	return s.gate.WaitSynced(ctx, s.dataTables(item)...)
}

func (s *Store) GetEntityType(entity string) *schema.Entity {
	return s.sch.GetEntity(entity)
}

func (s *Store) GetEntityOrInterfaceType(name string) schema.EntityOrInterface {
	return s.sch.GetEntityOrInterface(name)
}

func (s *Store) buildName(category string, name string) string {
	return fmt.Sprintf("%s_%s", category, name)
}

func (s *Store) cutName(tableName string) (category, name string) {
	category, name, _ = strings.Cut(tableName, "_")
	return
}

func (s *Store) fullName(name string) string {
	return s.ctrl.FullLogicName(name)
}

func (s *Store) VersionedTableName(item schema.EntityOrInterface) string {
	return s.buildName("versionedEntity", item.GetName())
}

func (s *Store) VersionedLatestTableName(item schema.EntityOrInterface) string {
	return s.buildName("versionedLatestEntity", item.GetName())
}

func (s *Store) VersionedLatestTableMaterializedViewName(item schema.EntityOrInterface) string {
	return s.buildName("versionedLatestEntityMV", item.GetName())
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
	return s.buildName(category, item.GetName())
}

func (s *Store) ViewName(item schema.EntityOrInterface) string {
	return s.buildName("view", item.GetName())
}

func (s *Store) LatestViewName(item schema.EntityOrInterface) string {
	return s.buildName("latestView", item.GetName())
}

func (s *Store) ViewFullName(item schema.EntityOrInterface) string {
	return s.fullName(s.ViewName(item))
}

func (s *Store) LatestViewFullName(item schema.EntityOrInterface) string {
	return s.fullName(s.LatestViewName(item))
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
