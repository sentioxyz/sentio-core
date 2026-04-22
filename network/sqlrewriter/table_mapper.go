package sqlrewriter

import (
	"context"
	"sentioxyz/sentio-core/service/processor/models"
	"sync"

	"sentioxyz/sentio-core/common/chx"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	entitychs "sentioxyz/sentio-core/driver/entity/clickhouse"
	entitypersistent "sentioxyz/sentio-core/driver/entity/persistent"
	entityschema "sentioxyz/sentio-core/driver/entity/schema"
	"sentioxyz/sentio-core/driver/timeseries"
	timeserieschs "sentioxyz/sentio-core/driver/timeseries/clickhouse"
	"sentioxyz/sentio-core/network/state"
)

// TableMapper handles logical-to-physical table name mapping for a database.
// Example: "coinbase.Transfer" → "coinbase_id123.Transfer"
type TableMapper interface {
	// Database returns the database identifier (e.g., "coinbase_id123")
	Database() string

	// RawTable resolves a logical table to its physical name.
	// Returns (physicalTable, exists, error).
	RawTable(table string) (string, bool, error)

	// RawTables batch-resolves multiple tables.
	// Returns map[logicalTable]physicalTable, skipping unmapped entries.
	RawTables(tables ...string) (map[string]string, error)

	// All returns the complete mapping snapshot.
	All() map[string]string

	// Reverse looks up the logical table from a physical name.
	Reverse(rawTable string) (string, bool, error)
}

type sentioNetworkTableMapper struct {
	once sync.Once

	privateKeyHex string
	processorId   string
	processorInfo state.ProcessorInfo
	indexerInfo   state.IndexerInfo
	conn          ckhmanager.Conn
	timeseries    timeseries.Store
	entity        entitypersistent.Store
	entitySchema  *entityschema.Schema

	table        map[string]string
	reverseTable map[string]string
}

func NewTableMapper(
	privateKeyHex string,
	processorId string,
	processorReplica int,
	processorTablePattern models.TablePattern,
	ckhManager ckhmanager.Manager,
	indexerInfo state.IndexerInfo,
	processorInfo state.ProcessorInfo,
) (TableMapper, error) {

	var (
		sharding = ckhManager.NewShardByStateIndexer(indexerInfo)
		options  = []func(*ckhmanager.ShardingParameter){
			ckhmanager.WithUnderlyingProxy(true),
			ckhmanager.WithRole(ckhmanager.AdminRole),
		}
		timeseriesConn ckhmanager.Conn
		entity         entitypersistent.Store
		entitySchema   *entityschema.Schema
	)
	if privateKeyHex != "" {
		options = append(options, ckhmanager.WithPrivateKeyHex(privateKeyHex))
	}
	timeseriesConn = sharding.MustGetConn(options...)
	if processorInfo.EntitySchema != "" {
		var err error
		entitySchema, err = entityschema.ParseAndVerifySchema(processorInfo.EntitySchema)
		if err != nil {
			return nil, err
		}
		// table_mapper runs inside the query path (sqlrewriter). It only
		// creates VIEWs via entity.CreateViews; it never creates physical
		// tables and must not hit the on-chain registrar.
		entity = entitychs.NewStore(
			chx.NewController(timeseriesConn),
			processorId,
			processorReplica,
			processorTablePattern,
			entitychs.BuildFeatures(processorInfo.EntitySchemaVersion),
			entitySchema,
			entitychs.DefaultCreateTableOption,
			nil,
		)
	}
	return &sentioNetworkTableMapper{
		privateKeyHex: privateKeyHex,
		processorId:   processorId,
		indexerInfo:   indexerInfo,
		processorInfo: processorInfo,
		conn:          timeseriesConn,
		// Query-only path: no table creation, so pass nil registrar.
		timeseries: timeserieschs.NewStore(
			timeseriesConn,
			timeseriesConn.GetCluster(),
			timeseriesConn.GetDatabase(),
			processorId,
			processorReplica,
			processorTablePattern,
			timeserieschs.Option{},
			nil,
		),
		entity:       entity,
		entitySchema: entitySchema,
		table:        make(map[string]string),
		reverseTable: make(map[string]string),
	}, nil
}

func (r *sentioNetworkTableMapper) retrieve() {
	r.once.Do(func() {
		timeseriesStore, ok := r.timeseries.(*timeserieschs.Store)
		if ok {
			_ = timeseriesStore.ReloadMeta(context.Background(), false)
			for _, metaByType := range r.timeseries.Meta().GetAllMeta() {
				for _, meta := range metaByType {
					rawTable := timeseriesStore.BuildTableNameWithoutDatabase(meta)
					r.table[meta.Name] = rawTable
					r.reverseTable[rawTable] = meta.Name
				}
			}
		}

		entityStore, ok := r.entity.(*entitychs.Store)
		if ok {
			if err := entityStore.CreateViews(context.Background()); err == nil {
				for _, i := range r.entitySchema.ListEntitiesAndInterfacesAndAggregations(false) {
					r.table[i.GetName()] = entityStore.LatestViewName(i)
					r.reverseTable[entityStore.LatestViewName(i)] = i.GetName()
					r.table[i.GetName()+"_raw"] = entityStore.ViewName(i)
					r.reverseTable[entityStore.ViewName(i)] = i.GetName() + "_raw"
				}
			}
		}

	})
}

func (r *sentioNetworkTableMapper) Database() string {
	return r.conn.GetDatabase()
}

func (r *sentioNetworkTableMapper) RawTable(table string) (string, bool, error) {
	r.retrieve()
	rawTable, ok := r.table[table]
	return rawTable, ok, nil
}

func (r *sentioNetworkTableMapper) RawTables(tables ...string) (map[string]string, error) {
	r.retrieve()
	ret := make(map[string]string, len(tables))
	for _, table := range tables {
		rawTable, ok := r.table[table]
		if ok {
			ret[table] = rawTable
		}
	}
	return ret, nil
}

func (r *sentioNetworkTableMapper) All() map[string]string {
	r.retrieve()
	return r.table
}

func (r *sentioNetworkTableMapper) Reverse(rawTable string) (string, bool, error) {
	r.retrieve()
	table, ok := r.reverseTable[rawTable]
	return table, ok, nil
}
