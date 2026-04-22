package mock

import (
	"context"
	"fmt"
	"time"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/driver/timeseries"
	chstimeseries "sentioxyz/sentio-core/driver/timeseries/clickhouse"
	"sentioxyz/sentio-core/service/common/timerange"
	processormodel "sentioxyz/sentio-core/service/processor/models"

	"github.com/samber/lo"
)

type mockStoreMeta struct {
	meta map[string]timeseries.Meta
}

type MockStore struct {
	*chstimeseries.Store
	storeMeta   timeseries.StoreMeta
	client      ckhmanager.Conn
	processorID string
	database    string
}

func (m *mockStoreMeta) GetHash() string                                                { return "mock" }
func (m *mockStoreMeta) GetAllMeta() map[timeseries.MetaType]map[string]timeseries.Meta { return nil }
func (m *mockStoreMeta) Different(other timeseries.StoreMeta) bool                      { return false }
func (m *mockStoreMeta) MetaTypes() []timeseries.MetaType                               { return nil }
func (m *mockStoreMeta) MetaNames(t timeseries.MetaType) []string                       { return nil }
func (m *mockStoreMeta) MetaByType(t timeseries.MetaType) map[string]timeseries.Meta    { return m.meta }
func (m *mockStoreMeta) Meta(t timeseries.MetaType, name string) (timeseries.Meta, bool) {
	r, ok := m.meta[name]
	return r, ok
}
func (m *mockStoreMeta) MustMeta(t timeseries.MetaType, name string) timeseries.Meta {
	return m.meta[name]
}
func (m *mockStoreMeta) String() string {
	return ""
}

func (m *MockStore) Init(ctx context.Context, _ bool) error {
	for _, meta := range m.storeMeta.MetaByType(timeseries.MetaTypeEvent) {
		if err := m.Store.CreateTable(ctx, meta); err != nil {
			return err
		}
	}
	return nil
}
func (m *MockStore) CleanAll(ctx context.Context) error {
	for _, meta := range m.storeMeta.MetaByType(timeseries.MetaTypeEvent) {
		if err := m.client.Exec(ctx, "DROP TABLE IF EXISTS `"+m.MetaTable(meta)+"`"); err != nil {
			return err
		}
	}
	return nil
}
func (m *MockStore) Meta() timeseries.StoreMeta                 { return m.storeMeta }
func (m *MockStore) ReloadMeta(_ context.Context, _ bool) error { return nil }
func (m *MockStore) MetaTable(meta timeseries.Meta) string {
	return fmt.Sprintf("%s_%s", m.processorID, meta.GetTableSuffix())
}

func (m *MockStore) AppendData(context.Context, []timeseries.Dataset, string, time.Time) error {
	return nil
}
func (m *MockStore) DeleteData(context.Context, string, int64) error {
	return nil
}
func (m *MockStore) Client() timeseries.QueryClient { return m.client }

type fields map[string]timeseries.Field

func newPresetFields() *fields {
	m := fields{
		timeseries.SystemFieldPrefix + "chain": {
			Name:    timeseries.SystemFieldPrefix + "chain",
			Type:    timeseries.FieldTypeString,
			Role:    timeseries.FieldRoleChainID,
			BuiltIn: true,
		},
		timeseries.SystemFieldPrefix + "block_number": {
			Name:    timeseries.SystemFieldPrefix + "block_number",
			Type:    timeseries.FieldTypeInt,
			BuiltIn: true,
		},
		timeseries.SystemFieldPrefix + "block_hash": {
			Name:    timeseries.SystemFieldPrefix + "block_hash",
			Type:    timeseries.FieldTypeString,
			BuiltIn: true,
		},
		timeseries.SystemFieldPrefix + "transaction_hash": {
			Name:    timeseries.SystemFieldPrefix + "transaction_hash",
			Type:    timeseries.FieldTypeString,
			BuiltIn: true,
		},
		timeseries.SystemFieldPrefix + "transaction_index": {
			Name:    timeseries.SystemFieldPrefix + "transaction_index",
			Type:    timeseries.FieldTypeInt,
			BuiltIn: true,
		},
		timeseries.SystemFieldPrefix + "log_index": {
			Name:    timeseries.SystemFieldPrefix + "log_index",
			Type:    timeseries.FieldTypeInt,
			BuiltIn: true,
		},
		timeseries.SystemUserID: {
			Name:    timeseries.SystemUserID,
			Type:    timeseries.FieldTypeString,
			BuiltIn: true,
		},
		timeseries.SystemTimestamp: {
			Name:    timeseries.SystemTimestamp,
			Type:    timeseries.FieldTypeTime,
			Role:    timeseries.FieldRoleTimestamp,
			BuiltIn: true,
		},
	}
	return &m
}

func (f *fields) Add(name string, fieldType timeseries.FieldType, nestedStruct map[string]timeseries.FieldType) *fields {
	(*f)[name] = timeseries.Field{
		Name:               name,
		Type:               fieldType,
		NestedStructSchema: nestedStruct,
	}
	return f
}

func newMockStoreMeta() timeseries.StoreMeta {
	transferField := newPresetFields().
		Add("from", timeseries.FieldTypeString, nil).
		Add("to", timeseries.FieldTypeString, nil).
		Add("amount", timeseries.FieldTypeJSON, map[string]timeseries.FieldType{
			"data":               timeseries.FieldTypeJSON,
			"data.usd":           timeseries.FieldTypeBigFloat,
			"data.eth":           timeseries.FieldTypeToken,
			"data.number":        timeseries.FieldTypeJSON,
			"data.number.string": timeseries.FieldTypeString,
			"data.number.int":    timeseries.FieldTypeInt,
			"data.timestamp":     timeseries.FieldTypeTime,
			"data.timestamp_utc": timeseries.FieldTypeInt,
			"nc1":                timeseries.FieldTypeToken,
			"c1":                 timeseries.FieldTypeFloat,
		})
	swapField := newPresetFields().
		Add("from", timeseries.FieldTypeString, nil).
		Add("to", timeseries.FieldTypeString, nil).
		Add("amount", timeseries.FieldTypeJSON, map[string]timeseries.FieldType{
			"data":     timeseries.FieldTypeJSON,
			"data.usd": timeseries.FieldTypeBigFloat,
			"data.eth": timeseries.FieldTypeToken,
			"nc1":      timeseries.FieldTypeArray,
			"c1":       timeseries.FieldTypeString,
		})
	depositField := newPresetFields().
		Add("user", timeseries.FieldTypeString, nil).
		Add("amount", timeseries.FieldTypeJSON, map[string]timeseries.FieldType{
			"data":     timeseries.FieldTypeJSON,
			"data.usd": timeseries.FieldTypeBigFloat,
			"c1":       timeseries.FieldTypeBool,
		})
	withdrawField := newPresetFields().
		Add("user", timeseries.FieldTypeString, nil).
		Add("amount", timeseries.FieldTypeBigFloat, nil)
	return &mockStoreMeta{
		meta: map[string]timeseries.Meta{
			"Transfer": {
				Name:   "Transfer",
				Type:   timeseries.MetaTypeEvent,
				Fields: *transferField,
			},
			"Swap": {
				Name:   "Swap",
				Type:   timeseries.MetaTypeEvent,
				Fields: *swapField,
			},
			"Deposit": {
				Name:   "Deposit",
				Type:   timeseries.MetaTypeEvent,
				Fields: *depositField,
			},
			"Withdraw": {
				Name:   "Withdraw",
				Type:   timeseries.MetaTypeEvent,
				Fields: *withdrawField,
			},
		},
	}
}

func NewMockStore(processor *processormodel.Processor, conn ckhmanager.Conn) *MockStore {
	s := chstimeseries.NewStore(conn, "", conn.GetDatabase(), processor.ID, 0, processormodel.TablePatternPlatformV1, chstimeseries.Option{}, nil)
	return &MockStore{
		Store:       s,
		storeMeta:   newMockStoreMeta(),
		processorID: processor.ID,
		client:      conn,
		database:    conn.GetDatabase(),
	}
}

type MockTimeRangeOption struct {
	Step      time.Duration
	Tz        *time.Location
	D         time.Duration
	RangeMode int
}

func NewTimeRange(options ...MockTimeRangeOption) *timerange.TimeRange {
	var option MockTimeRangeOption
	for _, o := range options {
		option = o
	}
	option.Step = lo.If(option.Step == 0, time.Hour).Else(option.Step)
	option.Tz = lo.If(option.Tz == nil, time.UTC).Else(option.Tz)
	option.D = lo.If(option.D == 0, time.Hour*24*30).Else(option.D)
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, option.Tz)
	end := start.Add(option.D)
	return &timerange.TimeRange{
		Start:      start,
		End:        end,
		Step:       option.Step,
		Timezone:   option.Tz,
		RangeMode:  option.RangeMode,
		SampleRate: option.Step,
	}
}
