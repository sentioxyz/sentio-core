package persistent

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// ─── schema ──────────────────────────────────────────────────────────────────

// testSchema is the GraphQL schema shared by all controller tests.
const testSchema = `
type EntityA @entity {
  id: Bytes!
	foreignA: EntityB!                                   # many to one
	foreignB: [EntityB] @derivedFrom(field: "foreignB")  # one  to many
	foreignC: [EntityC] @derivedFrom(field: "foreignCA") # many to many by EntityC
	foreignD: EntityB                                    # one  to one
	foreignE: [EntityB!] @derivedFrom(field: "foreignE") # many to many
	foreignF: EntityB! @derivedFrom(field: "foreignF")   # many to one
}

type EntityB @entity {
	id: String!
	foreignB: EntityA!                                   # many to one
	foreignC: [EntityC] @derivedFrom(field: "foreignCB") # many to many by EntityC
	foreignD: EntityA @derivedFrom(field: "foreignD")    # one  to one
	foreignE: [EntityA]                                  # many to many
	foreignF: [EntityA!]                                 # one  to many
}

type EntityC @entity {
	id: Bytes!
	foreignCA: EntityA!
	foreignCB: EntityB!
}

type EntityD @entity {
	id: ID!

	propA1: String!
	propB1: Bytes!
	propC1: Boolean!
	propD1: Int!
	propE1: BigInt!
	propF1: BigDecimal!
	propG1: EnumA!
  propH1: Timestamp!
	propI1: Float!
	propJ1: Int8!

	propA2: String
	propB2: Bytes
	propC2: Boolean
	propD2: Int
	propE2: BigInt
	propF2: BigDecimal
	propG2: EnumA
  propH2: Timestamp
	propI2: Float
	propJ2: Int8

	propA3: [String!]
	propB3: [Bytes!]
	propC3: [Boolean!]
	propD3: [Int!]
	propE3: [BigInt!]
	propF3: [BigDecimal!]
	propG3: [EnumA!]
  propH3: [Timestamp!]
	propI3: [Float!]
	propJ3: [Int8!]

	propA4: [String]
	propB4: [Bytes]
	propC4: [Boolean]
	propD4: [Int]
	propE4: [BigInt]
	propF4: [BigDecimal]
	propG4: [EnumA]
  propH4: [Timestamp]
	propI4: [Float]
	propJ4: [Int8]

	propA5: [String]!
	propB5: [Bytes]!
	propC5: [Boolean]!
	propD5: [Int]!
	propE5: [BigInt]!
	propF5: [BigDecimal]!
	propG5: [EnumA]!
  propH5: [Timestamp]!
	propI5: [Float]!
	propJ5: [Int8]!

	propA6: [String!]!
	propB6: [Bytes!]!
	propC6: [Boolean!]!
	propD6: [Int!]!
	propE6: [BigInt!]!
	propF6: [BigDecimal!]!
	propG6: [EnumA!]!
  propH6: [Timestamp!]!
	propI6: [Float!]!
	propJ6: [Int8!]!

	propA7: [[String!]!]
	propB7: [[Bytes!]!]
	propC7: [[Boolean!]!]
	propD7: [[Int!]!]
	propE7: [[BigInt!]!]
	propF7: [[BigDecimal!]!]
	propG7: [[EnumA!]!]
  propH7: [[Timestamp!]!]
	propI7: [[Float!]!]
	propJ7: [[Int8!]!]

	propA8: [[String!]]
	propB8: [[Bytes!]]
	propC8: [[Boolean!]]
	propD8: [[Int!]]
	propE8: [[BigInt!]]
	propF8: [[BigDecimal!]]
	propG8: [[EnumA!]]
  propH8: [[Timestamp!]]
	propI8: [[Float!]]
	propJ8: [[Int8!]]

	foreign1: EntityA!
	foreign2: EntityA
	foreign3: [EntityA!]
	foreign4: [EntityA]
	foreign5: [EntityA]!
	foreign6: [EntityA!]!
}

enum EnumA {
  AAA
  BBB
  CCC
}


interface EntityE {
	id: ID!
	propA: String!
}

type EntityE1 implements EntityE @entity {
	id: ID!
	propA: String!
	propB: Int!
}

type EntityE2 implements EntityE @entity {
	id: ID!
	propA: String!
	propB: String!
}
`

// ─── mock store ───────────────────────────────────────────────────────────────

// mockChainStore is a simple in-memory ChainStore used in tests.
// GetEntity/ListEntities return fromCache=false on first access for an entity
// type and fromCache=true on subsequent calls, mimicking the full-cache path.
type mockChainStore struct {
	chain      string
	schema     *schema.Schema
	data       map[string]map[string]*EntityBox
	fullLoaded map[string]bool // tracks which entity types have been "cached"
}

func (s *mockChainStore) InitEntitySchema(_ context.Context) error { return nil }

func (s *mockChainStore) GetChain() string { return s.chain }

func (s *mockChainStore) GetEntityType(entity string) *schema.Entity {
	return s.schema.GetEntity(entity)
}

func (s *mockChainStore) GetEntityOrInterfaceType(name string) schema.EntityOrInterface {
	return s.schema.GetEntityOrInterface(name)
}

func (s *mockChainStore) GetEntity(
	_ context.Context,
	entityType *schema.Entity,
	id string,
) (*EntityBox, bool, error) {
	origin, _ := utils.GetFromK2Map(s.data, entityType.Name, id)
	if origin == nil {
		return nil, false, nil
	}
	return origin.Copy(), false, nil
}

func (s *mockChainStore) ListEntities(
	_ context.Context,
	entityType *schema.Entity,
	filters []EntityFilter,
	limit int,
) ([]*EntityBox, bool, error) {
	log.Debugf("calling mockChainStore.ListEntities(%s, %v, %d)", entityType.Name, filters, limit)
	fromCache := s.fullLoaded[entityType.Name]
	s.fullLoaded[entityType.Name] = true
	var list []*EntityBox
	for _, origin := range s.data[entityType.Name] {
		ok, err := CheckFilters(filters, *origin)
		log.Debugf("calling mockChainStore.ListEntities, %s, %v, %v", origin.ID, ok, err)
		if err != nil {
			return nil, false, err
		} else if !ok {
			continue
		}
		list = append(list, origin.Copy())
	}
	SortEntityBoxes(list)
	if len(list) > limit {
		list = list[:limit]
	}
	return list, fromCache, nil
}

func (s *mockChainStore) GetTimeSeriesEntityMaxID(_ context.Context, _ *schema.Entity) (int64, error) {
	panic("not implemented")
}

func (s *mockChainStore) SetEntities(
	_ context.Context,
	entityType *schema.Entity,
	boxes []EntityBox,
) (int, error) {
	for _, box := range boxes {
		utils.PutIntoK2Map(s.data, entityType.Name, box.ID, utils.WrapPointer(box))
	}
	// Invalidate full-loaded cache so next ListEntities re-fetches from "persistent".
	delete(s.fullLoaded, entityType.Name)
	return 0, nil
}

func (s *mockChainStore) GrowthAggregation(_ context.Context, _ time.Time) error { return nil }

func (s *mockChainStore) CheckValue(_ *schema.Entity, _ map[string]any) error { return nil }

func (s *mockChainStore) Reorg(_ context.Context, _ int64) error { panic("not implemented") }

func (s *mockChainStore) Snapshot() any { return nil }

// newTestStore returns a mockChainStore pre-loaded with EntityA, EntityB and
// EntityC fixtures, and the same value as a ChainStore interface.
func newTestStore(sch *schema.Schema, chain string) (*mockChainStore, ChainStore) {
	s := &mockChainStore{
		chain:      chain,
		schema:     sch,
		fullLoaded: make(map[string]bool),
		data: map[string]map[string]*EntityBox{
			"EntityA": {
				"0x0a00": {
					ID: "0x0a00",
					Data: map[string]any{
						"id":       "0x0a00",
						"foreignA": "0x0b00",
						"foreignD": utils.WrapPointer("0x0b00"),
					},
					Entity:         "EntityA",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
				},
				"0x0a01": {
					ID: "0x0a01",
					Data: map[string]any{
						"id":       "0x0a01",
						"foreignA": "0x0b01",
						"foreignD": utils.WrapPointer("0x0b01"),
					},
					Entity:         "EntityA",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
				},
			},
			"EntityB": {
				"0x0b00": {
					ID: "0x0b00",
					Data: map[string]any{
						"id":       "0x0b00",
						"foreignB": "0x0a00",
						"foreignE": []*string{utils.WrapPointer("0x0a00")},
						"foreignF": []string{},
					},
					Entity:         "EntityB",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
				},
				"0x0b01": {
					ID: "0x0b01",
					Data: map[string]any{
						"id":       "0x0b01",
						"foreignB": "0x0a00",
						"foreignE": []*string{utils.WrapPointer("0x0a00"), utils.WrapPointer("0x0a01")},
						"foreignF": []string{"0x0a00", "0x0a01"},
					},
					Entity:         "EntityB",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
				},
			},
			"EntityC": {
				"0x0c0000": {
					ID: "0x0c0000",
					Data: map[string]any{
						"id":        "0x0c0000",
						"foreignCA": "0x0a00",
						"foreignCB": "0x0b00",
					},
					Entity:         "EntityC",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
				},
				"0x0c0001": {
					ID: "0x0c0001",
					Data: map[string]any{
						"id":        "0x0c0001",
						"foreignCA": "0x0a00",
						"foreignCB": "0x0b01",
					},
					Entity:         "EntityC",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
				},
				"0x0c0101": {
					ID: "0x0c0101",
					Data: map[string]any{
						"id":        "0x0c0101",
						"foreignCA": "0x0a01",
						"foreignCB": "0x0b01",
					},
					Entity:         "EntityC",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
				},
			},
		},
	}
	return s, s
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// copyWith returns a shallow copy of box with GenBlockNumber set to gbn and the
// given key-value pairs merged into Data.
func copyWith(origin *EntityBox, gbn uint64, kvs ...any) *EntityBox {
	dest := *origin
	dest.Data = utils.CopyMap(origin.Data)
	for i := 0; i+1 < len(kvs); i += 2 {
		dest.Data[kvs[i].(string)] = kvs[i+1]
	}
	dest.GenBlockNumber = gbn
	return &dest
}

// newCtrl creates a Controller and a reset ReportMonitor bound to s.
func newCtrl(s ChainStore) (*Controller, *ReportMonitor) {
	m := NewReportMonitor(nil)
	m.Reset()
	return NewController(s, m), m
}

// ─── TestController_ListEntity ────────────────────────────────────────────────

// TestController_ListEntity covers insertion, deletion, cursor pagination and
// persistent-cache statistics for the list path.
func TestController_ListEntity(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchema)
	assert.NoError(t, err)

	const chain = "mainnet"

	t.Run("insert and delete", func(t *testing.T) {
		ps, s := newTestStore(sch, chain)
		ctrl, _ := newCtrl(s)
		ctx := context.Background()

		eaType := sch.GetEntity("EntityA")
		a0, _ := utils.GetFromK2Map(ps.data, eaType.GetName(), "0x0a00")
		a1, _ := utils.GetFromK2Map(ps.data, eaType.GetName(), "0x0a01")
		a2 := &EntityBox{
			ID: "0x0a02",
			Data: map[string]any{
				"id":       "0x0a02",
				"foreignA": "0x0b00",
				"foreignD": utils.WrapPointer("0x0b00"),
			},
			Entity:         "EntityA",
			GenBlockNumber: 11,
			GenBlockHash:   "0x1234",
		}

		// init: [a0, a1]
		boxes, _, err := ctrl.ListEntity(ctx, eaType, nil, "", 100, 12)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{a0, a1}, boxes)

		// insert a2 → [a2, a0, a1]
		assert.NoError(t, ctrl.SetEntity(ctx, eaType, UncommittedEntityBox{EntityBox: *a2}))
		boxes, _, err = ctrl.ListEntity(ctx, eaType, nil, "", 100, 12)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{a2, a0, a1}, boxes)

		// delete a1 → [a2, a0]
		assert.NoError(t, ctrl.SetEntity(ctx, eaType, UncommittedEntityBox{EntityBox: EntityBox{
			ID: "0x0a01", GenBlockNumber: 12, GenBlockHash: "0x1234",
		}}))
		boxes, _, err = ctrl.ListEntity(ctx, eaType, nil, "", 100, 12)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{a2, a0}, boxes)

		// delete a0 → [a2]
		assert.NoError(t, ctrl.SetEntity(ctx, eaType, UncommittedEntityBox{EntityBox: EntityBox{
			ID: "0x0a00", GenBlockNumber: 13, GenBlockHash: "0x1234",
		}}))
		boxes, _, err = ctrl.ListEntity(ctx, eaType, nil, "", 100, 13)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{a2}, boxes)

		// delete a2 → []
		assert.NoError(t, ctrl.SetEntity(ctx, eaType, UncommittedEntityBox{EntityBox: EntityBox{
			ID: "0x0a02", GenBlockNumber: 14, GenBlockHash: "0x1234",
		}}))
		boxes, _, err = ctrl.ListEntity(ctx, eaType, nil, "", 100, 14)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox(nil), boxes)
	})

	t.Run("cursor pagination", func(t *testing.T) {
		ps, s := newTestStore(sch, chain)
		ctrl, _ := newCtrl(s)
		ctx := context.Background()

		ecType := sch.GetEntity("EntityC")
		c00, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0000")
		c01, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0001")
		c11, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0101")
		c10 := &EntityBox{
			ID: "0x0c0100",
			Data: map[string]any{
				"id": "0x0c0100", "foreignCA": "0x0a01", "foreignCB": "0x0b00",
			},
			Entity:         "EntityC",
			GenBlockNumber: 11,
			GenBlockHash:   "0x1234",
		}
		c01_ := &EntityBox{
			ID: "0x0c0001",
			Data: map[string]any{
				"id": "0x0c0001", "foreignCA": "0x0a0099", "foreignCB": "0x0b0199",
			},
			Entity:         "EntityC",
			GenBlockNumber: 12,
			GenBlockHash:   "0x1234",
		}

		// init: [c00, c01, c11]
		boxes, cursor, err := ctrl.ListEntity(ctx, ecType, nil, "", 100, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c00, c01, c11}, boxes)
		assert.Nil(t, cursor)

		// insert c10: uncommitted=[c10], persistent=[c00, c01, c11]
		assert.NoError(t, ctrl.SetEntity(ctx, ecType, UncommittedEntityBox{EntityBox: *c10}))
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 2, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c10, c00}, boxes)
		assert.NotNil(t, cursor)
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, *cursor, 3, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c01, c11}, boxes)
		assert.Nil(t, cursor)

		// update c01: uncommitted=[c01_, c10], persistent=[c00, c11]
		assert.NoError(t, ctrl.SetEntity(ctx, ecType, UncommittedEntityBox{EntityBox: *c01_}))
		// page size 1 → [c01_]
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 1, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c01_}, boxes)
		assert.NotNil(t, cursor)
		// page size 2 → [c10, c00]
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, *cursor, 2, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c10, c00}, boxes)
		assert.NotNil(t, cursor)
		// page size 3 → [c11]
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, *cursor, 3, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c11}, boxes)
		assert.Nil(t, cursor)

		// page size 2 spans both parts: [c01_, c10] | [c00, c11]
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 2, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c01_, c10}, boxes)
		assert.NotNil(t, cursor)
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, *cursor, 2, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c00, c11}, boxes)
		assert.NotNil(t, cursor)
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, *cursor, 2, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox(nil), boxes)
		assert.Nil(t, cursor)

		// page size 3: [c01_, c10, c00] | [c11]
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 3, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c01_, c10, c00}, boxes)
		assert.NotNil(t, cursor)
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, *cursor, 3, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c11}, boxes)
		assert.Nil(t, cursor)

		// page size 4 fits all: [c01_, c10, c00, c11] | []
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 4, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c01_, c10, c00, c11}, boxes)
		assert.NotNil(t, cursor)
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, *cursor, 4, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox(nil), boxes)
		assert.Nil(t, cursor)

		// page size 5 returns all at once
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 5, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c01_, c10, c00, c11}, boxes)
		assert.Nil(t, cursor)

		// filter: foreignCA >= "0x0a01" → [c10, c11]
		filters := []EntityFilter{{
			Field: ecType.GetFieldByName("foreignCA"),
			Op:    EntityFilterOpGe,
			Value: []any{"0x0a01"},
		}}
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, filters, "", 1, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c10}, boxes)
		assert.NotNil(t, cursor)
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, filters, *cursor, 1, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c11}, boxes)
		assert.NotNil(t, cursor)
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, filters, *cursor, 1, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox(nil), boxes)
		assert.Nil(t, cursor)

		// delete c01: uncommitted=[c10], persistent=[c00, c11]
		assert.NoError(t, ctrl.SetEntity(ctx, ecType, UncommittedEntityBox{EntityBox: EntityBox{
			ID: "0x0c0001", GenBlockNumber: 13, GenBlockHash: "0x1234",
		}}))
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 5, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c10, c00, c11}, boxes)
		assert.Nil(t, cursor)

		// delete c10: uncommitted=[], persistent=[c00, c11]
		assert.NoError(t, ctrl.SetEntity(ctx, ecType, UncommittedEntityBox{EntityBox: EntityBox{
			ID: "0x0c0100", GenBlockNumber: 14, GenBlockHash: "0x1234",
		}}))
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 5, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c00, c11}, boxes)
		assert.Nil(t, cursor)
	})

	t.Run("persistent cache stats", func(t *testing.T) {
		ps, s := newTestStore(sch, chain)
		ctrl, monitor := newCtrl(s)
		ctx := context.Background()

		ecType := sch.GetEntity("EntityC")
		c00, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0000")
		c01, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0001")
		c11, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0101")
		c10 := &EntityBox{
			ID: "0x0c0100",
			Data: map[string]any{
				"id": "0x0c0100", "foreignCA": "0x0a01", "foreignCB": "0x0b00",
			},
			Entity:         "EntityC",
			GenBlockNumber: 11,
			GenBlockHash:   "0x1234",
		}
		c01_ := &EntityBox{
			ID: "0x0c0001",
			Data: map[string]any{
				"id": "0x0c0001", "foreignCA": "0x0a0099", "foreignCB": "0x0b0199",
			},
			Entity:         "EntityC",
			GenBlockNumber: 12,
			GenBlockHash:   "0x1234",
		}

		// first list: hits persistent store
		boxes, cursor, err := ctrl.ListEntity(ctx, ecType, nil, "", 100, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c00, c01, c11}, boxes)
		assert.Nil(t, cursor)
		assert.Equal(t,
			map[string]map[string]int{"persistent": {ecType.GetName(): 1}},
			monitor.report.TotalListFrom)

		// second list: served from cache
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 100, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c00, c01, c11}, boxes)
		assert.Nil(t, cursor)
		assert.Equal(t,
			map[string]map[string]int{
				"persistent": {ecType.GetName(): 1},
				"cache":      {ecType.GetName(): 1},
			},
			monitor.report.TotalListFrom)

		// insert c10, list partial (2): uncommitted=[c10], cache=[c00, c01, c11]
		assert.NoError(t, ctrl.SetEntity(ctx, ecType, UncommittedEntityBox{EntityBox: *c10}))
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 2, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c10, c00}, boxes)
		assert.Equal(t,
			map[string]map[string]int{
				"persistent": {ecType.GetName(): 1},
				"cache":      {ecType.GetName(): 2},
			},
			monitor.report.TotalListFrom)
		assert.NotNil(t, cursor)
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, *cursor, 3, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c01, c11}, boxes)
		assert.Nil(t, cursor)
		assert.Equal(t,
			map[string]map[string]int{
				"persistent": {ecType.GetName(): 1},
				"cache":      {ecType.GetName(): 3},
			},
			monitor.report.TotalListFrom)

		// update c01: uncommitted=[c01_, c10], cache=[c00, c11]
		assert.NoError(t, ctrl.SetEntity(ctx, ecType, UncommittedEntityBox{EntityBox: *c01_}))
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 1, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c01_}, boxes)
		assert.NotNil(t, cursor)
		assert.Equal(t,
			map[string]map[string]int{
				"uncommitted": {ecType.GetName(): 1},
				"persistent":  {ecType.GetName(): 1},
				"cache":       {ecType.GetName(): 3},
			},
			monitor.report.TotalListFrom)
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, *cursor, 2, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c10, c00}, boxes)
		assert.NotNil(t, cursor)
		assert.Equal(t,
			map[string]map[string]int{
				"uncommitted": {ecType.GetName(): 1},
				"persistent":  {ecType.GetName(): 1},
				"cache":       {ecType.GetName(): 4},
			},
			monitor.report.TotalListFrom)
		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, *cursor, 3, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c11}, boxes)
		assert.Nil(t, cursor)
		assert.Equal(t,
			map[string]map[string]int{
				"uncommitted": {ecType.GetName(): 1},
				"persistent":  {ecType.GetName(): 1},
				"cache":       {ecType.GetName(): 5},
			},
			monitor.report.TotalListFrom)

		// after commit, cache is invalidated → next list hits persistent again
		_, _, err = ctrl.Commit(ctx, math.MaxUint64, time.Time{})
		assert.NoError(t, err)
		ctrl, monitor = newCtrl(s)

		boxes, cursor, err = ctrl.ListEntity(ctx, ecType, nil, "", 100, 20)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{c00, c01_, c10, c11}, boxes)
		assert.Nil(t, cursor)
		assert.Equal(t,
			map[string]map[string]int{"persistent": {ecType.GetName(): 1}},
			monitor.report.TotalListFrom)
	})
}

// ─── TestController_ListRelated ───────────────────────────────────────────────

// TestController_ListRelated covers all reverse foreign-key relation types
// (one-to-one, many-to-one, many-to-many, one-to-many) within a single
// uncommitted cycle.
func TestController_ListRelated(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchema)
	assert.NoError(t, err)

	eaType := sch.GetEntity("EntityA")
	ebType := sch.GetEntity("EntityB")

	const chain = "mainnet"
	ctx := context.Background()

	ps, s := newTestStore(sch, chain)
	ctrl, monitor := newCtrl(s)

	ra0, _ := utils.GetFromK2Map(ps.data, eaType.Name, "0x0a00")
	ra1, _ := utils.GetFromK2Map(ps.data, eaType.Name, "0x0a01")
	rb0, _ := utils.GetFromK2Map(ps.data, ebType.Name, "0x0b00")
	rb1, _ := utils.GetFromK2Map(ps.data, ebType.Name, "0x0b01")

	a0 := copyWith(ra0, ra0.GenBlockNumber)
	a1 := copyWith(ra1, ra1.GenBlockNumber)
	b0 := copyWith(rb0, rb0.GenBlockNumber)
	b1 := copyWith(rb1, rb1.GenBlockNumber)

	// Verify initial state via ListEntity (populates list cache).
	boxes, _, err := ctrl.ListEntity(ctx, eaType, nil, "", 100, 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{a0, a1}, boxes)

	t.Run("many-to-one (foreignA)", func(t *testing.T) {
		// update a0.foreignA: 0x0b00 → 0x0b01
		a0 = copyWith(a0, 11, "foreignA", "0x0b01")
		assert.NoError(t, ctrl.SetEntity(ctx, eaType, UncommittedEntityBox{EntityBox: *a0}))
		assert.Equal(t, 1, monitor.report.TotalSet)
		assert.Equal(t, 0, monitor.report.TotalSetNil)
		assert.Equal(t, 0, monitor.report.TotalSetPartly)
	})

	t.Run("one-to-one (foreignD)", func(t *testing.T) {
		// update a0.foreignD: 0x0b00 → 0x0b01
		// update a1.foreignD: 0x0b01 → 0x0b00
		// effect: b0.foreignD → a1, b1.foreignD → a0
		a0_ := copyWith(a0, 12, "foreignD", utils.WrapPointer("0x0b01"))
		a1_ := copyWith(a1, 12, "foreignD", utils.WrapPointer("0x0b00"))
		assert.NoError(t, ctrl.SetEntity(ctx, eaType, UncommittedEntityBox{EntityBox: *a0_}))
		assert.NoError(t, ctrl.SetEntity(ctx, eaType, UncommittedEntityBox{EntityBox: *a1_}))
		assert.Equal(t, 3, monitor.report.TotalSet)

		// at block 11: a0 still has foreignD→b0 and a1 still has foreignD→b1
		boxesB, _, err := ctrl.ListRelated(ctx, ebType, b0.ID, "foreignD", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{a0}, boxesB)
		boxesB, _, err = ctrl.ListRelated(ctx, ebType, b1.ID, "foreignD", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{a1}, boxesB)

		// at block 12: relations are swapped
		boxesB, _, err = ctrl.ListRelated(ctx, ebType, b0.ID, "foreignD", 12)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{a1_}, boxesB)
		boxesB, _, err = ctrl.ListRelated(ctx, ebType, b1.ID, "foreignD", 12)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{a0_}, boxesB)

		assert.Equal(t, 5, monitor.report.TotalList)
		assert.Equal(t, 4, monitor.report.TotalListForLoadRelated)
		assert.Equal(t,
			map[string]map[string]int{
				"persistent": {eaType.GetName(): 1},
				"cache":      {eaType.GetName(): 4},
			},
			monitor.report.TotalListFrom)

		a0, a1 = a0_, a1_
	})

	t.Run("many-to-one (foreignB)", func(t *testing.T) {
		ctrl, monitor = newCtrl(s)

		// update b0.foreignB and b1.foreignB: 0x0a00 → 0x0a01
		// effect: a0.foreignB → [], a1.foreignB → [b0, b1]
		b0 = copyWith(b0, 11, "foreignB", "0x0a01")
		b1 = copyWith(b1, 11, "foreignB", "0x0a01")
		assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b0}))
		assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b1}))

		boxesA, _, err := ctrl.ListRelated(ctx, eaType, a0.ID, "foreignB", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox(nil), boxesA)
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a1.ID, "foreignB", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b0, b1}, boxesA)
	})

	t.Run("many-to-many (foreignE)", func(t *testing.T) {
		ctrl, monitor = newCtrl(s)
		a0 = copyWith(ra0, ra0.GenBlockNumber)
		a1 = copyWith(ra1, ra1.GenBlockNumber)
		b0 = copyWith(rb0, rb0.GenBlockNumber)
		b1 = copyWith(rb1, rb1.GenBlockNumber)
		_ = monitor

		// b0.foreignE: [a0] → []
		b0 = copyWith(b0, 11, "foreignE", []string(nil))
		assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b0}))
		boxesA, _, err := ctrl.ListRelated(ctx, eaType, a0.ID, "foreignE", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b1}, boxesA)
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a1.ID, "foreignE", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b1}, boxesA)

		// b1.foreignE: [a0, a1] → [a0]
		b1 = copyWith(b1, 11, "foreignE", []string{"0x0a00"})
		assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b1}))
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a0.ID, "foreignE", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b1}, boxesA)
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a1.ID, "foreignE", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox(nil), boxesA)

		// b0.foreignE: [] → [a0, a1]
		b0 = copyWith(b0, 11, "foreignE", []string{"0x0a00", "0x0a01"})
		assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b0}))
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a0.ID, "foreignE", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b0, b1}, boxesA)
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a1.ID, "foreignE", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b0}, boxesA)

		// b1.foreignE: [a0] → [a1]
		b1 = copyWith(b1, 11, "foreignE", []string{"0x0a01"})
		assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b1}))
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a0.ID, "foreignE", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b0}, boxesA)
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a1.ID, "foreignE", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b0, b1}, boxesA)
	})

	t.Run("one-to-many (foreignF)", func(t *testing.T) {
		ctrl, monitor = newCtrl(s)
		a0 = copyWith(ra0, ra0.GenBlockNumber)
		a1 = copyWith(ra1, ra1.GenBlockNumber)
		b0 = copyWith(rb0, rb0.GenBlockNumber)
		b1 = copyWith(rb1, rb1.GenBlockNumber)
		_ = monitor

		// b1.foreignF: [a0, a1] → [a1]
		b1 = copyWith(b1, 11, "foreignF", []string{"0x0a01"})
		assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b1}))
		boxesA, _, err := ctrl.ListRelated(ctx, eaType, a0.ID, "foreignF", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox(nil), boxesA)
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a1.ID, "foreignF", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b1}, boxesA)

		// b0.foreignF: [] → [a0]
		b0 = copyWith(b0, 11, "foreignF", []string{"0x0a00"})
		assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b0}))
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a0.ID, "foreignF", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b0}, boxesA)
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a1.ID, "foreignF", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b1}, boxesA)

		// b1.foreignF: [a1] → []
		b1 = copyWith(b1, 11, "foreignF", []string(nil))
		assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b1}))
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a0.ID, "foreignF", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b0}, boxesA)
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a1.ID, "foreignF", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox(nil), boxesA)

		// b0.foreignF: [a0] → [a0, a1]
		b0 = copyWith(b0, 11, "foreignF", []string{"0x0a00", "0x0a01"})
		assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b0}))
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a0.ID, "foreignF", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b0}, boxesA)
		boxesA, _, err = ctrl.ListRelated(ctx, eaType, a1.ID, "foreignF", 11)
		assert.NoError(t, err)
		assert.Equal(t, []*EntityBox{b0}, boxesA)
	})
}

// TestController_ListRelatedAfterCommit tests that relation lookups work
// correctly after changes have been committed to the persistent store.
func TestController_ListRelatedAfterCommit(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchema)
	assert.NoError(t, err)

	eaType := sch.GetEntity("EntityA")
	ebType := sch.GetEntity("EntityB")

	const chain = "mainnet"
	ctx := context.Background()

	ps, s := newTestStore(sch, chain)
	ctrl, _ := newCtrl(s)

	ra0, _ := utils.GetFromK2Map(ps.data, eaType.Name, "0x0a00")
	ra1, _ := utils.GetFromK2Map(ps.data, eaType.Name, "0x0a01")
	rb0, _ := utils.GetFromK2Map(ps.data, ebType.Name, "0x0b00")
	rb1, _ := utils.GetFromK2Map(ps.data, ebType.Name, "0x0b01")

	a0 := copyWith(ra0, ra0.GenBlockNumber)
	a1 := copyWith(ra1, ra1.GenBlockNumber)
	b0 := copyWith(rb0, rb0.GenBlockNumber)
	b1 := copyWith(rb1, rb1.GenBlockNumber)

	// Verify initial relations.
	//   foreignB: b0→a0, b1→a0
	//   foreignE: b0→[a0], b1→[a0,a1]
	//   foreignF: b0→[], b1→[a0,a1]
	assertRelated := func(entityType *schema.Entity, id, field string, blockN uint64, want []*EntityBox) {
		t.Helper()
		got, _, err := ctrl.ListRelated(ctx, entityType, id, field, blockN)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	}
	assertRelated(eaType, a0.ID, "foreignB", 11, []*EntityBox{b0, b1})
	assertRelated(eaType, a1.ID, "foreignB", 11, []*EntityBox(nil))
	assertRelated(eaType, a0.ID, "foreignE", 11, []*EntityBox{b0, b1})
	assertRelated(eaType, a1.ID, "foreignE", 11, []*EntityBox{b1})
	assertRelated(eaType, a0.ID, "foreignF", 11, []*EntityBox{b1})
	assertRelated(eaType, a1.ID, "foreignF", 11, []*EntityBox{b1})

	// Commit: b0.foreignB → a1 (all other fields unchanged).
	b0 = copyWith(b0, 11, "foreignB", "0x0a01")
	assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b0}))
	assertRelated(eaType, a0.ID, "foreignB", 11, []*EntityBox{b1})
	assertRelated(eaType, a1.ID, "foreignB", 11, []*EntityBox{b0})
	assertRelated(eaType, a0.ID, "foreignE", 11, []*EntityBox{b0, b1})
	assertRelated(eaType, a1.ID, "foreignE", 11, []*EntityBox{b1})
	assertRelated(eaType, a0.ID, "foreignF", 11, []*EntityBox{b1})
	assertRelated(eaType, a1.ID, "foreignF", 11, []*EntityBox{b1})

	_, _, err = ctrl.Commit(ctx, math.MaxUint64, time.Time{})
	assert.NoError(t, err)
	ctrl, _ = newCtrl(s)

	// After commit: additionally change b0.foreignE and b0.foreignF.
	b0 = copyWith(b0, 12,
		"foreignE", []*string{utils.WrapPointer("0x0a01")},
		"foreignF", []string{"0x0a00"})
	assert.NoError(t, ctrl.SetEntity(ctx, ebType, UncommittedEntityBox{EntityBox: *b0}))

	assertRelated(eaType, a0.ID, "foreignB", 12, []*EntityBox{b1})
	assertRelated(eaType, a1.ID, "foreignB", 12, []*EntityBox{b0})
	assertRelated(eaType, a0.ID, "foreignE", 12, []*EntityBox{b1})
	assertRelated(eaType, a1.ID, "foreignE", 12, []*EntityBox{b0, b1})
	assertRelated(eaType, a0.ID, "foreignF", 12, []*EntityBox{b0, b1})
	assertRelated(eaType, a1.ID, "foreignF", 12, []*EntityBox{b1})
}

// ─── TestController_GetEntityByInterface ──────────────────────────────────────

// TestController_GetEntityByInterface verifies that entities can be fetched
// through an interface type and that the most recently set concrete type wins.
func TestController_GetEntityByInterface(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchema)
	assert.NoError(t, err)

	eType := sch.GetEntityOrInterface("EntityE")
	e1Type := sch.GetEntity("EntityE1")
	e2Type := sch.GetEntity("EntityE2")

	const chain = "mainnet"
	ctx := context.Background()

	_, s := newTestStore(sch, chain)
	ctrl, _ := newCtrl(s)

	// no entities yet
	box, err := ctrl.GetEntity(ctx, eType, "0x0e00", 20)
	assert.NoError(t, err)
	assert.Nil(t, box)

	// set EntityE1 → GetEntity via interface returns E1
	assert.NoError(t, ctrl.SetEntity(ctx, e1Type, UncommittedEntityBox{EntityBox: EntityBox{
		ID:             "0x0e00",
		Data:           map[string]any{"id": "0x0e00", "propA": "aaa", "propB": int32(123)},
		GenBlockNumber: 11,
		GenBlockHash:   "0x1234",
	}}))
	box, err = ctrl.GetEntity(ctx, eType, "0x0e00", 20)
	assert.NoError(t, err)
	assert.Equal(t, &EntityBox{
		ID:             "0x0e00",
		Data:           map[string]any{"id": "0x0e00", "propA": "aaa", "propB": int32(123)},
		Entity:         e1Type.Name,
		GenBlockNumber: 11,
		GenBlockHash:   "0x1234",
	}, box)

	// set EntityE2 → still returns E1 (earlier block number wins in this block range)
	assert.NoError(t, ctrl.SetEntity(ctx, e2Type, UncommittedEntityBox{EntityBox: EntityBox{
		ID:             "0x0e00",
		Data:           map[string]any{"id": "0x0e00", "propA": "aaa", "propB": "456"},
		GenBlockNumber: 12,
		GenBlockHash:   "0x1234",
	}}))
	box, err = ctrl.GetEntity(ctx, eType, "0x0e00", 20)
	assert.NoError(t, err)
	assert.Equal(t, &EntityBox{
		ID:             "0x0e00",
		Data:           map[string]any{"id": "0x0e00", "propA": "aaa", "propB": int32(123)},
		Entity:         e1Type.Name,
		GenBlockNumber: 11,
		GenBlockHash:   "0x1234",
	}, box)

	// delete EntityE1 → returns E2
	assert.NoError(t, ctrl.SetEntity(ctx, e1Type, UncommittedEntityBox{EntityBox: EntityBox{
		ID:             "0x0e00",
		GenBlockNumber: 13,
		GenBlockHash:   "0x1234",
	}}))
	box, err = ctrl.GetEntity(ctx, eType, "0x0e00", 20)
	assert.NoError(t, err)
	assert.Equal(t, &EntityBox{
		ID:             "0x0e00",
		Data:           map[string]any{"id": "0x0e00", "propA": "aaa", "propB": "456"},
		Entity:         e2Type.Name,
		GenBlockNumber: 12,
		GenBlockHash:   "0x1234",
	}, box)

	// delete EntityE2 → returns the E2 delete record (nil Data)
	assert.NoError(t, ctrl.SetEntity(ctx, e2Type, UncommittedEntityBox{EntityBox: EntityBox{
		ID:             "0x0e00",
		GenBlockNumber: 14,
		GenBlockHash:   "0x1234",
	}}))
	box, err = ctrl.GetEntity(ctx, eType, "0x0e00", 20)
	assert.NoError(t, err)
	assert.Equal(t, &EntityBox{
		ID:             "0x0e00",
		Entity:         e2Type.Name,
		GenBlockNumber: 14,
		GenBlockHash:   "0x1234",
	}, box)
}
