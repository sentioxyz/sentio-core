package persistent

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/log"
	rsh "sentioxyz/sentio-core/common/richstructhelper"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
)

const testSchemaCnt = `
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

type mockPersistentStore struct {
	data   map[string]map[string]*EntityBox
	schema *schema.Schema
}

func (s *mockPersistentStore) InitEntitySchema(ctx context.Context) error {
	panic("not implemented")
}

func (s *mockPersistentStore) EraseEntitySchema(ctx context.Context) error {
	panic("not implemented")
}

func (s *mockPersistentStore) GetEntityType(entity string) *schema.Entity {
	return s.schema.GetEntity(entity)
}

func (s *mockPersistentStore) GetEntityOrInterfaceType(name string) schema.EntityOrInterface {
	return s.schema.GetEntityOrInterface(name)
}

func (s *mockPersistentStore) GetEntity(
	ctx context.Context,
	entityType *schema.Entity,
	chain string,
	id string,
) (*EntityBox, error) {
	//log.Debugf("calling mockPersistentStore.GetEntity(%s, %s)", entityType.Name, id)
	origin, _ := utils.GetFromK2Map(s.data, entityType.Name, id)
	if origin == nil {
		return nil, nil
	}
	return origin.Copy(), nil
}

func (s *mockPersistentStore) ListEntities(
	ctx context.Context,
	entityType *schema.Entity,
	chain string,
	filters []EntityFilter,
	limit int,
) ([]*EntityBox, error) {
	log.Debugf("calling mockPersistentStore.ListEntities(%s, %v, %d)", entityType.Name, filters, limit)
	var list []*EntityBox
	for _, origin := range s.data[entityType.Name] {
		ok, err := checkFilters(filters, *origin)
		log.Debugf("calling mockPersistentStore.ListEntities, %s, %v, %v", origin.ID, ok, err)
		if err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		list = append(list, origin.Copy())
	}
	SortEntityBoxes(list)
	if len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

func (s *mockPersistentStore) GetAllID(ctx context.Context, entityType *schema.Entity, chain string) ([]string, error) {
	var ids []string
	for _, box := range s.data[entityType.Name] {
		if box.Data != nil {
			ids = append(ids, box.ID)
		}
	}
	return ids, nil
}

func (s *mockPersistentStore) GetMaxID(ctx context.Context, entityType *schema.Entity, chain string) (int64, error) {
	panic("not implemented")
}

func (s *mockPersistentStore) CountEntity(ctx context.Context, entityType *schema.Entity, chain string) (uint64, error) {
	var count uint64
	for _, box := range s.data[entityType.Name] {
		if box.Data != nil {
			count++
		}
	}
	return count, nil
}

func (s *mockPersistentStore) SetEntities(
	ctx context.Context,
	entityType *schema.Entity,
	boxes []EntityBox,
) (int, error) {
	for _, box := range boxes {
		//log.Debugf("!!! mockPersistentStore.SetEntities %s/%s", entityType.Name, box.String())
		utils.PutIntoK2Map(s.data, entityType.Name, box.ID, utils.WrapPointer(box))
	}
	return 0, nil
}

func (s *mockPersistentStore) GrowthAggregation(ctx context.Context, chain string, curBlockTime time.Time) error {
	return nil
}

func (s *mockPersistentStore) Reorg(ctx context.Context, blockNumber int64, chain string) error {
	panic("not implemented")
}

func prepareTestStore(sch *schema.Schema, cacheSize int, chain string) (*mockPersistentStore, *CachedStore) {
	ps := &mockPersistentStore{
		schema: sch,
		data: map[string]map[string]*EntityBox{
			"EntityA": {
				"0x0a00": &EntityBox{
					ID: "0x0a00",
					Data: map[string]any{
						"id":       "0x0a00",
						"foreignA": "0x0b00",
						"foreignD": utils.WrapPointer("0x0b00"),
					},
					Operator:       make(map[string]Operator),
					Entity:         "EntityA",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
					GenBlockChain:  chain,
				},
				"0x0a01": &EntityBox{
					ID: "0x0a01",
					Data: map[string]any{
						"id":       "0x0a01",
						"foreignA": "0x0b01",
						"foreignD": utils.WrapPointer("0x0b01"),
					},
					Operator:       make(map[string]Operator),
					Entity:         "EntityA",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
					GenBlockChain:  chain,
				},
			},
			"EntityB": {
				"0x0b00": &EntityBox{
					ID: "0x0b00",
					Data: map[string]any{
						"id":       "0x0b00",
						"foreignB": "0x0a00",
						"foreignE": []*string{utils.WrapPointer("0x0a00")},
						"foreignF": []string{}, // empty
					},
					Operator:       make(map[string]Operator),
					Entity:         "EntityB",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
					GenBlockChain:  chain,
				},
				"0x0b01": &EntityBox{
					ID: "0x0b01",
					Data: map[string]any{
						"id":       "0x0b01",
						"foreignB": "0x0a00",
						"foreignE": []*string{utils.WrapPointer("0x0a00"), utils.WrapPointer("0x0a01")},
						"foreignF": []string{"0x0a00", "0x0a01"},
					},
					Operator:       make(map[string]Operator),
					Entity:         "EntityB",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
					GenBlockChain:  chain,
				},
			},
			"EntityC": {
				"0x0c0000": &EntityBox{
					ID: "0x0c0000",
					Data: map[string]any{
						"id":        "0x0c0000",
						"foreignCA": "0x0a00",
						"foreignCB": "0x0b00",
					},
					Operator:       make(map[string]Operator),
					Entity:         "EntityC",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
					GenBlockChain:  chain,
				},
				"0x0c0001": &EntityBox{
					ID: "0x0c0001",
					Data: map[string]any{
						"id":        "0x0c0001",
						"foreignCA": "0x0a00",
						"foreignCB": "0x0b01",
					},
					Operator:       make(map[string]Operator),
					Entity:         "EntityC",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
					GenBlockChain:  chain,
				},
				"0x0c0101": &EntityBox{
					ID: "0x0c0101",
					Data: map[string]any{
						"id":        "0x0c0101",
						"foreignCA": "0x0a01",
						"foreignCB": "0x0b01",
					},
					Operator:       make(map[string]Operator),
					Entity:         "EntityC",
					GenBlockNumber: 10,
					GenBlockHash:   "0x1234",
					GenBlockChain:  chain,
				},
			},
		},
	}
	return ps, NewCachedStore(ps, chain, cacheSize, 1000000, nil)
}

func Test_cache(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	eaType := sch.GetEntity("EntityA")
	ebType := sch.GetEntity("EntityB")

	const chain = "mainnet"
	var box *EntityBox

	ps, s := prepareTestStore(sch, 2, chain)
	a0, _ := utils.GetFromK2Map(ps.data, eaType.Name, "0x0a00")
	a1, _ := utils.GetFromK2Map(ps.data, eaType.Name, "0x0a01")
	b0, _ := utils.GetFromK2Map(ps.data, ebType.Name, "0x0b00")
	//b1, _ := utils.GetFromK2Map(ps.data, ebType.Name, "0x0b01")

	ctx := context.Background()
	s.fullCacheRefused[eaType.Name] = true
	s.fullCacheRefused[ebType.Name] = true
	txn := s.NewTxn()

	box, err = txn.GetEntity(ctx, eaType, "0x0a00", 11)
	assert.NoError(t, err)
	assert.Equal(t, a0, box)
	assert.Equal(t,
		map[string]map[string]int{
			"persistent": {eaType.GetName(): 1},
		},
		txn.report.TotalGetFrom)
	// now lru cache: 0x0a00

	box, err = txn.GetEntity(ctx, eaType, "0x0a00", 11)
	assert.NoError(t, err)
	assert.Equal(t, a0, box)
	box, err = txn.GetEntity(ctx, eaType, "0x0a00", 11)
	assert.NoError(t, err)
	assert.Equal(t, a0, box)
	assert.Equal(t,
		map[string]map[string]int{
			"persistent": {eaType.GetName(): 1},
			"cache":      {eaType.GetName(): 2},
		},
		txn.report.TotalGetFrom)
	// now lru cache: 0x0a00

	box, err = txn.GetEntity(ctx, eaType, "0x0a01", 11)
	assert.NoError(t, err)
	assert.Equal(t, a1, box)
	box, err = txn.GetEntity(ctx, eaType, "0x0a01", 11)
	assert.NoError(t, err)
	assert.Equal(t, a1, box)
	box, err = txn.GetEntity(ctx, eaType, "0x0a01", 11)
	assert.NoError(t, err)
	assert.Equal(t, a1, box)
	assert.Equal(t,
		map[string]map[string]int{
			"persistent": {eaType.GetName(): 2},
			"cache":      {eaType.GetName(): 4},
		},
		txn.report.TotalGetFrom)
	// now lru cache: 0x0a00, 0x0a01

	box, err = txn.GetEntity(ctx, ebType, "0x0b00", 11)
	assert.NoError(t, err)
	assert.Equal(t, b0, box)
	box, err = txn.GetEntity(ctx, ebType, "0x0b00", 11)
	assert.NoError(t, err)
	assert.Equal(t, b0, box)
	box, err = txn.GetEntity(ctx, ebType, "0x0b00", 11)
	assert.NoError(t, err)
	assert.Equal(t, b0, box)
	assert.Equal(t,
		map[string]map[string]int{
			"persistent": {eaType.GetName(): 2, ebType.GetName(): 1},
			"cache":      {eaType.GetName(): 4, ebType.GetName(): 2},
		},
		txn.report.TotalGetFrom)
	// now lru cache: 0x0a01, 0x0b00

	box, err = txn.GetEntity(ctx, eaType, "0x0a00", 11)
	assert.NoError(t, err)
	assert.Equal(t, a0, box)
	box, err = txn.GetEntity(ctx, eaType, "0x0a00", 11)
	assert.NoError(t, err)
	assert.Equal(t, a0, box)
	box, err = txn.GetEntity(ctx, eaType, "0x0a00", 11)
	assert.NoError(t, err)
	assert.Equal(t, a0, box)
	assert.Equal(t,
		map[string]map[string]int{
			"persistent": {eaType.GetName(): 3, ebType.GetName(): 1},
			"cache":      {eaType.GetName(): 6, ebType.GetName(): 2},
		},
		txn.report.TotalGetFrom)
	// now lru cache: 0x0b00, 0x0a00

	box, err = txn.GetEntity(ctx, eaType, "0x0a02", 11) // not exists, will only use full id cache
	assert.NoError(t, err)
	assert.Nil(t, box)
	box, err = txn.GetEntity(ctx, eaType, "0x0a02", 11) // not exists, will only use full id cache
	assert.NoError(t, err)
	assert.Nil(t, box)
	assert.Equal(t,
		map[string]map[string]int{
			"persistent": {eaType.GetName(): 3, ebType.GetName(): 1},
			"cache":      {eaType.GetName(): 8, ebType.GetName(): 2},
		},
		txn.report.TotalGetFrom)
	// now lru cache: 0x0b00, 0x0a00

	_, _, err = txn.Commit(ctx, math.MaxUint64, time.Time{})
	assert.NoError(t, err)
}

func Test_loadRelated(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	eaType := sch.GetEntity("EntityA")
	ebType := sch.GetEntity("EntityB")

	const chain = "mainnet"
	var boxies []*EntityBox

	ps, s := prepareTestStore(sch, 2, chain)
	ra0, _ := utils.GetFromK2Map(ps.data, eaType.Name, "0x0a00")
	ra1, _ := utils.GetFromK2Map(ps.data, eaType.Name, "0x0a01")
	rb0, _ := utils.GetFromK2Map(ps.data, ebType.Name, "0x0b00")
	rb1, _ := utils.GetFromK2Map(ps.data, ebType.Name, "0x0b01")

	update := func(origin *EntityBox, gbn uint64, kvs ...any) *EntityBox {
		dest := *origin
		dest.Data = utils.CopyMap(origin.Data)
		for i := 0; i+1 < len(kvs); i += 2 {
			propName := kvs[i].(string)
			propValue := kvs[i+1]
			dest.Data[propName] = propValue
		}
		dest.GenBlockNumber = gbn
		return &dest
	}

	ctx := context.Background()
	txn := s.NewTxn()
	a0 := update(ra0, ra0.GenBlockNumber)
	a1 := update(ra1, ra1.GenBlockNumber)
	b0 := update(rb0, rb0.GenBlockNumber)
	b1 := update(rb1, rb1.GenBlockNumber)

	boxies, _, err = txn.ListEntity(ctx, eaType, nil, "", 100, 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{a0, a1}, boxies)

	// update a0.foreignA : 0x0b00 => 0x0b01
	a0 = update(a0, 11, "foreignA", "0x0b01")
	assert.NoError(t, txn.SetEntity(ctx, eaType, *a0))
	// now EntityA loaded full cache
	assert.Equal(t, 1, txn.report.TotalSet)
	assert.Equal(t, 0, txn.report.TotalSetNil)
	assert.Equal(t, 0, txn.report.TotalSetPartly)

	// change one-to-one relation
	// update:
	//   a0.foreignD : 0x0b00 => 0x0b01
	//   a1.foreignD : 0x0b01 => 0x0b00
	// effect:
	//   b0.foreignD : 0x0a00 => 0x0a01
	//   b1.foreignD : 0x0a01 => 0x0a00
	a0_ := update(a0, 12, "foreignD", utils.WrapPointer("0x0b01"))
	a1_ := update(a1, 12, "foreignD", utils.WrapPointer("0x0b00"))
	assert.NoError(t, txn.SetEntity(ctx, eaType, *a0_))
	assert.NoError(t, txn.SetEntity(ctx, eaType, *a1_))
	assert.Equal(t, 3, txn.report.TotalSet)
	assert.Equal(t, 0, txn.report.TotalSetNil)
	assert.Equal(t, 0, txn.report.TotalSetPartly)
	boxies, _, err = txn.ListRelated(ctx, ebType, b0.ID, "foreignD", 11) // ignore the changes in block 12
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{a0}, boxies)
	boxies, _, err = txn.ListRelated(ctx, ebType, b1.ID, "foreignD", 11) // ignore the changes in block 12
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{a1}, boxies)
	a0, a1 = a0_, a1_
	boxies, _, err = txn.ListRelated(ctx, ebType, b0.ID, "foreignD", 12)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{a1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, ebType, b1.ID, "foreignD", 12)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{a0}, boxies)
	assert.Equal(t, 5, txn.report.TotalList)
	assert.Equal(t, 4, txn.report.TotalListForLoadRelated)
	assert.Equal(t,
		map[string]map[string]int{
			"persistent": {eaType.GetName(): 1},
			"cache":      {eaType.GetName(): 4},
		},
		txn.report.TotalListFrom)

	// ================================================================================
	// reset data
	txn = s.NewTxn()

	// change many-to-one relation
	// update:
	//   b0.foreignB : 0x0a00 => 0x0a01
	//   b1.foreignB : 0x0a00 => 0x0a01
	// effect:
	//   a0.foreignB : [0x0b00, 0x0b01] => []
	//   a1.foreignB :               [] => [0x0b00, 0x0b01]
	b0 = update(b0, 11, "foreignB", "0x0a01")
	b1 = update(b1, 11, "foreignB", "0x0a01")
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b0))
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b1))
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignB", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox(nil), boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignB", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0, b1}, boxies)

	// ================================================================================
	// reset data
	txn = s.NewTxn()
	a0 = update(ra0, ra0.GenBlockNumber)
	a1 = update(ra1, ra1.GenBlockNumber)
	b0 = update(rb0, rb0.GenBlockNumber)
	b1 = update(rb1, rb1.GenBlockNumber)

	// change many-to-many relation
	// update:
	//  *b0.foreignE : [0x0a00        ] => [              ]
	//   b1.foreignE : [0x0a00, 0x0a01] => [0x0a00, 0x0a01]
	// effect:
	//   a0.foreignE : [0x0b00, 0x0b01] => [        0x0b01]
	//   a1.foreignE : [        0x0b01] => [        0x0b01]
	b0 = update(b0, 11, "foreignE", []string(nil))
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b0))
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)

	// change many-to-many relation
	// update:
	//   b0.foreignE : [              ] => [              ]
	//  *b1.foreignE : [0x0a00, 0x0a01] => [0x0a00        ]
	// effect:
	//   a0.foreignE : [        0x0b01] => [        0x0b01]
	//   a1.foreignE : [        0x0b01] => [              ]
	b1 = update(b1, 11, "foreignE", []string{"0x0a00"})
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b1))
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox(nil), boxies)

	// change many-to-many relation
	// update:
	//  *b0.foreignE : [              ] => [0x0a00, 0x0a01]
	//   b1.foreignE : [0x0a00        ] => [0x0a00        ]
	// effect:
	//   a0.foreignE : [        0x0b01] => [0x0b00, 0x0b01]
	//   a1.foreignE : [        0x0b01] => [0x0b00        ]
	b0 = update(b0, 11, "foreignE", []string{"0x0a00", "0x0a01"})
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b0))
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0, b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0}, boxies)

	// change many-to-many relation
	// update:
	//   b0.foreignE : [0x0a00, 0x0a01] => [0x0a00, 0x0a01]
	//  *b1.foreignE : [0x0a00        ] => [        0x0a01]
	// effect:
	//   a0.foreignE : [0x0b00, 0x0b01] => [0x0b00        ]
	//   a1.foreignE : [0x0b00        ] => [0x0b00, 0x0b01]
	b1 = update(b1, 11, "foreignE", []string{"0x0a01"})
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b1))
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0, b1}, boxies)

	// ================================================================================
	// reset data
	txn = s.NewTxn()
	a0 = update(ra0, ra0.GenBlockNumber)
	a1 = update(ra1, ra1.GenBlockNumber)
	b0 = update(rb0, rb0.GenBlockNumber)
	b1 = update(rb1, rb1.GenBlockNumber)

	// change one-to-many relation
	// update:
	//   b0.foreignF : [              ] => [              ]
	//  *b1.foreignF : [0x0a00, 0x0a01] => [        0x0a01]
	// effect:
	//   a0.foreignF : 0x0b01           =>
	//   a1.foreignF : 0x0b01           => 0x0b01
	b1 = update(b1, 11, "foreignF", []string{"0x0a01"})
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b1))
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox(nil), boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)

	// change one-to-many relation
	// update:
	//  *b0.foreignF : [              ] => [0x0a00        ]
	//   b1.foreignF : [        0x0a01] => [        0x0a01]
	// effect:
	//   a0.foreignF :                  => 0x0b00
	//   a1.foreignF : 0x0b01           => 0x0b01
	b0 = update(b0, 11, "foreignF", []string{"0x0a00"})
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b0))
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)

	// change one-to-many relation
	// update:
	//   b0.foreignF : [0x0a00        ] => [0x0a00        ]
	//  *b1.foreignF : [        0x0a01] => [              ]
	// effect:
	//   a0.foreignF : 0x0b00           => 0x0b00
	//   a1.foreignF : 0x0b01           =>
	b1 = update(b1, 11, "foreignF", []string(nil))
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b1))
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox(nil), boxies)

	// change one-to-many relation
	// update:
	//  *b0.foreignF : [0x0a00        ] => [0x0a00, 0x0a01]
	//   b1.foreignF : [              ] => [              ]
	// effect:
	//   a0.foreignF : 0x0b00           => 0x0b00
	//   a1.foreignF :                  => 0x0b00
	b0 = update(b0, 11, "foreignF", []string{"0x0a00", "0x0a01"})
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b0))
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0}, boxies)

	log.Debug("a0:%s", a0.String())
	log.Debug("a1:%s", a1.String())
	log.Debug("b0:%s", b0.String())
	log.Debug("b1:%s", b1.String())
}

func Test_loadRelated2(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	eaType := sch.GetEntity("EntityA")
	ebType := sch.GetEntity("EntityB")

	const chain = "mainnet"
	var boxies []*EntityBox

	ps, s := prepareTestStore(sch, 3, chain)
	ra0, _ := utils.GetFromK2Map(ps.data, eaType.Name, "0x0a00")
	ra1, _ := utils.GetFromK2Map(ps.data, eaType.Name, "0x0a01")
	rb0, _ := utils.GetFromK2Map(ps.data, ebType.Name, "0x0b00")
	rb1, _ := utils.GetFromK2Map(ps.data, ebType.Name, "0x0b01")

	update := func(origin *EntityBox, gbn uint64, kvs ...any) *EntityBox {
		dest := *origin
		dest.Data = utils.CopyMap(origin.Data)
		for i := 0; i+1 < len(kvs); i += 2 {
			propName := kvs[i].(string)
			propValue := kvs[i+1]
			dest.Data[propName] = propValue
		}
		dest.GenBlockNumber = gbn
		return &dest
	}

	ctx := context.Background()
	txn := s.NewTxn()
	a0 := update(ra0, ra0.GenBlockNumber)
	a1 := update(ra1, ra1.GenBlockNumber)
	b0 := update(rb0, rb0.GenBlockNumber)
	b1 := update(rb1, rb1.GenBlockNumber)

	// =init=
	// foreignB
	//   b0->a0
	//   b1->a0
	// foreignE
	//   b0->a0
	//   b1->a0,a1
	// foreignF
	//   b0->
	//   b1->a0,a1
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignB", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0, b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignB", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox(nil), boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0, b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)

	// change reserve relation
	// =init=            =new=
	// foreignB
	//   b0->a0         *b0->a1
	//   b1->a0          b1->a0
	// foreignE
	//   b0->a0          b0->a0
	//   b1->a0,a1       b1->a0,a1
	// foreignF
	//   b0->            b0->
	//   b1->a0,a1       b1->a0,a1
	b0 = update(b0, 11, "foreignB", "0x0a01")
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b0))
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignB", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignB", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0, b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignE", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignF", 11)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)

	_, _, err = txn.Commit(ctx, math.MaxUint64, time.Time{})
	assert.NoError(t, err)
	txn = s.NewTxn()

	// change reserve relation
	// =init=            =new=            =new=
	// foreignB
	//   b0->a0          b0->a1          b0->a1
	//   b1->a0          b1->a0          b1->a0
	// foreignE
	//   b0->a0          b0->a0         *b0->a1
	//   b1->a0,a1       b1->a0,a1       b1->a0,a1
	// foreignF
	//   b0->            b0->           *b0->a0
	//   b1->a0,a1       b1->a0,a1       b1->a0,a1
	b0 = update(b0, 12,
		"foreignE", []*string{utils.WrapPointer("0x0a01")},
		"foreignF", []string{"0x0a00"})
	assert.NoError(t, txn.SetEntity(ctx, ebType, *b0))
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignB", 12)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignB", 12)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignE", 12)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignE", 12)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0, b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a0.ID, "foreignF", 12)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b0, b1}, boxies)
	boxies, _, err = txn.ListRelated(ctx, eaType, a1.ID, "foreignF", 12)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{b1}, boxies)
}

func Test_list1(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	eaType := sch.GetEntity("EntityA")

	const chain = "mainnet"
	var boxies []*EntityBox

	ps, s := prepareTestStore(sch, 2, chain)
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
		GenBlockChain:  chain,
	}

	ctx := context.Background()
	txn := s.NewTxn()

	// init
	boxies, _, err = txn.ListEntity(ctx, eaType, nil, "", 100, 12)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{a0, a1}, boxies)
	// insert a2
	assert.NoError(t, txn.SetEntity(ctx, eaType, *a2))
	boxies, _, err = txn.ListEntity(ctx, eaType, nil, "", 100, 12)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{a2, a0, a1}, boxies)
	// delete a1
	assert.NoError(t, txn.SetEntity(ctx, eaType, EntityBox{
		ID:             "0x0a01",
		GenBlockNumber: 12,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}))
	boxies, _, err = txn.ListEntity(ctx, eaType, nil, "", 100, 12)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{a2, a0}, boxies)
	// delete a0
	assert.NoError(t, txn.SetEntity(ctx, eaType, EntityBox{
		ID:             "0x0a00",
		GenBlockNumber: 13,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}))
	boxies, _, err = txn.ListEntity(ctx, eaType, nil, "", 100, 13)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{a2}, boxies)
	// delete a2
	assert.NoError(t, txn.SetEntity(ctx, eaType, EntityBox{
		ID:             "0x0a02",
		GenBlockNumber: 14,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}))
	boxies, _, err = txn.ListEntity(ctx, eaType, nil, "", 100, 14)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox(nil), boxies)
}

func Test_list2(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	ecType := sch.GetEntity("EntityC")

	const chain = "mainnet"
	var boxies []*EntityBox
	var cursor *string

	ps, s := prepareTestStore(sch, 2, chain)
	c00, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0000")
	c01, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0001")
	c11, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0101")
	c10 := &EntityBox{
		ID: "0x0c0100",
		Data: map[string]any{
			"id":        "0x0c0100",
			"foreignCA": "0x0a01",
			"foreignCB": "0x0b00",
		},
		Entity:         "EntityC",
		GenBlockNumber: 11,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}
	c01_ := &EntityBox{
		ID: "0x0c0001",
		Data: map[string]any{
			"id":        "0x0c0001",
			"foreignCA": "0x0a0099",
			"foreignCB": "0x0b0199",
		},
		Entity:         "EntityC",
		GenBlockNumber: 12,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}

	ctx := context.Background()
	txn := s.NewTxn()

	// init
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 100, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c00, c01, c11}, boxies)
	assert.Nil(t, cursor)

	// insert c10, uncommitted: c10, persistent: c00, c01, c11
	assert.NoError(t, txn.SetEntity(ctx, ecType, *c10))
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 2, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c10, c00}, boxies)
	assert.NotNil(t, cursor)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, *cursor, 3, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c01, c11}, boxies)
	assert.Nil(t, cursor)

	// update c01, uncommitted: c01, c10, persistent: c00, c11
	assert.NoError(t, txn.SetEntity(ctx, ecType, *c01_))
	// list: c01 | c10, c00 | c11
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c01_}, boxies)
	assert.NotNil(t, cursor)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, *cursor, 2, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c10, c00}, boxies)
	assert.NotNil(t, cursor)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, *cursor, 3, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c11}, boxies)
	assert.Nil(t, cursor)
	// list: c01, c10 | c00, c11 |
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 2, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c01_, c10}, boxies)
	assert.NotNil(t, cursor)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, *cursor, 2, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c00, c11}, boxies)
	assert.NotNil(t, cursor)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, *cursor, 2, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox(nil), boxies)
	assert.Nil(t, cursor)
	// list: c01, c10, c00 | c11
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 3, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c01_, c10, c00}, boxies)
	assert.NotNil(t, cursor)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, *cursor, 3, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c11}, boxies)
	assert.Nil(t, cursor)
	// list: c01, c10, c00, c11 |
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 4, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c01_, c10, c00, c11}, boxies)
	assert.NotNil(t, cursor)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, *cursor, 4, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox(nil), boxies)
	assert.Nil(t, cursor)
	// list: c01, c10, c00, c11
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 5, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c01_, c10, c00, c11}, boxies)
	assert.Nil(t, cursor)
	// list with filter: c10, c11
	filters := []EntityFilter{{
		Field: ecType.GetFieldByName("foreignCA"),
		Op:    EntityFilterOpGe,
		Value: []any{"0x0a01"},
	}}
	boxies, cursor, err = txn.ListEntity(ctx, ecType, filters, "", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c10}, boxies)
	assert.NotNil(t, cursor)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, filters, *cursor, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c11}, boxies)
	assert.NotNil(t, cursor)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, filters, *cursor, 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox(nil), boxies)
	assert.Nil(t, cursor)

	// delete c01, uncommitted: c10, persistent: c00, c11
	assert.NoError(t, txn.SetEntity(ctx, ecType, EntityBox{
		ID:             "0x0c0001",
		GenBlockNumber: 13,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}))
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 5, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c10, c00, c11}, boxies)
	assert.Nil(t, cursor)

	// delete c10, uncommitted: <empty>, persistent: c00, c11
	assert.NoError(t, txn.SetEntity(ctx, ecType, EntityBox{
		ID:             "0x0c0100",
		GenBlockNumber: 14,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}))
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 5, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c00, c11}, boxies)
	assert.Nil(t, cursor)
}

func Test_listCache(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	ecType := sch.GetEntity("EntityC")

	const chain = "mainnet"
	var boxies []*EntityBox
	var cursor *string

	ps, s := prepareTestStore(sch, 2, chain)
	c00, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0000")
	c01, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0001")
	c11, _ := utils.GetFromK2Map(ps.data, ecType.Name, "0x0c0101")
	c10 := &EntityBox{
		ID: "0x0c0100",
		Data: map[string]any{
			"id":        "0x0c0100",
			"foreignCA": "0x0a01",
			"foreignCB": "0x0b00",
		},
		Entity:         "EntityC",
		GenBlockNumber: 11,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}
	c01_ := &EntityBox{
		ID: "0x0c0001",
		Data: map[string]any{
			"id":        "0x0c0001",
			"foreignCA": "0x0a0099",
			"foreignCB": "0x0b0199",
		},
		Entity:         "EntityC",
		GenBlockNumber: 12,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}

	ctx := context.Background()
	txn := s.NewTxn()

	// init, will load all entity to list cache
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 100, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c00, c01, c11}, boxies)
	assert.Nil(t, cursor)
	assert.Equal(t,
		map[string]map[string]int{
			"persistent": {ecType.GetName(): 1},
		},
		txn.report.TotalListFrom)

	// list use list cache
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 100, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c00, c01, c11}, boxies)
	assert.Nil(t, cursor)
	assert.Equal(t,
		map[string]map[string]int{
			"persistent": {ecType.GetName(): 1},
			"cache":      {ecType.GetName(): 1},
		},
		txn.report.TotalListFrom)

	// insert c10, uncommitted: c10, persistent: c00, c01, c11
	assert.NoError(t, txn.SetEntity(ctx, ecType, *c10))
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 2, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c10, c00}, boxies)
	assert.Equal(t,
		map[string]map[string]int{
			"persistent": {ecType.GetName(): 1},
			"cache":      {ecType.GetName(): 2},
		},
		txn.report.TotalListFrom)
	assert.NotNil(t, cursor)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, *cursor, 3, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c01, c11}, boxies)
	assert.Nil(t, cursor)
	assert.Equal(t,
		map[string]map[string]int{
			"persistent": {ecType.GetName(): 1},
			"cache":      {ecType.GetName(): 3},
		},
		txn.report.TotalListFrom)

	// update c01, uncommitted: c01, c10, persistent: c00, c11
	assert.NoError(t, txn.SetEntity(ctx, ecType, *c01_))
	// list: c01 | c10, c00 | c11
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 1, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c01_}, boxies)
	assert.NotNil(t, cursor)
	assert.Equal(t,
		map[string]map[string]int{
			"uncommitted": {ecType.GetName(): 1},
			"persistent":  {ecType.GetName(): 1},
			"cache":       {ecType.GetName(): 3},
		},
		txn.report.TotalListFrom)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, *cursor, 2, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c10, c00}, boxies)
	assert.NotNil(t, cursor)
	assert.Equal(t,
		map[string]map[string]int{
			"uncommitted": {ecType.GetName(): 1},
			"persistent":  {ecType.GetName(): 1},
			"cache":       {ecType.GetName(): 4},
		},
		txn.report.TotalListFrom)
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, *cursor, 3, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c11}, boxies)
	assert.Nil(t, cursor)
	assert.Equal(t,
		map[string]map[string]int{
			"uncommitted": {ecType.GetName(): 1},
			"persistent":  {ecType.GetName(): 1},
			"cache":       {ecType.GetName(): 5},
		},
		txn.report.TotalListFrom)

	// commit, list cache will be reset
	_, _, err = txn.Commit(ctx, math.MaxUint64, time.Time{})
	assert.NoError(t, err)
	txn = s.NewTxn()

	// will load entities from persistent
	boxies, cursor, err = txn.ListEntity(ctx, ecType, nil, "", 100, 20)
	assert.NoError(t, err)
	assert.Equal(t, []*EntityBox{c00, c01_, c10, c11}, boxies)
	assert.Nil(t, cursor)
	assert.Equal(t,
		map[string]map[string]int{
			"cache": {ecType.GetName(): 1},
		},
		txn.report.TotalListFrom)
}

func Test_getInterface(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	eType := sch.GetEntityOrInterface("EntityE")
	e1Type := sch.GetEntity("EntityE1")
	e2Type := sch.GetEntity("EntityE2")

	const chain = "mainnet"
	var box *EntityBox

	_, s := prepareTestStore(sch, 2, chain)

	ctx := context.Background()
	txn := s.NewTxn()

	// no EntityE1 and EntityE2 object
	box, err = txn.GetEntity(ctx, eType, "0x0e00", 20)
	assert.NoError(t, err)
	assert.Nil(t, box)

	// insert new EntityE1 object, and then we can get it by EntityE
	assert.NoError(t, txn.SetEntity(ctx, e1Type, EntityBox{
		ID: "0x0e00",
		Data: map[string]any{
			"id":    "0x0e00",
			"propA": "aaa",
			"propB": int32(123),
		},
		GenBlockNumber: 11,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}))
	box, err = txn.GetEntity(ctx, eType, "0x0e00", 20)
	assert.NoError(t, err)
	assert.Equal(t, &EntityBox{
		ID: "0x0e00",
		Data: map[string]any{
			"id":    "0x0e00",
			"propA": "aaa",
			"propB": int32(123),
		},
		Entity:         e1Type.Name,
		GenBlockNumber: 11,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}, box)

	// insert new EntityE2 object, get by EntityE will get the EntityE1 object
	assert.NoError(t, txn.SetEntity(ctx, e2Type, EntityBox{
		ID: "0x0e00",
		Data: map[string]any{
			"id":    "0x0e00",
			"propA": "aaa",
			"propB": "456",
		},
		GenBlockNumber: 12,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}))
	box, err = txn.GetEntity(ctx, eType, "0x0e00", 20)
	assert.NoError(t, err)
	assert.Equal(t, &EntityBox{
		ID: "0x0e00",
		Data: map[string]any{
			"id":    "0x0e00",
			"propA": "aaa",
			"propB": int32(123),
		},
		Entity:         e1Type.Name,
		GenBlockNumber: 11,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}, box)

	// delete EntityE1 object, get by EntityE will get the EntityE2 object
	assert.NoError(t, txn.SetEntity(ctx, e1Type, EntityBox{
		ID:             "0x0e00",
		GenBlockNumber: 13,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}))
	box, err = txn.GetEntity(ctx, eType, "0x0e00", 20)
	assert.NoError(t, err)
	assert.Equal(t, &EntityBox{
		ID: "0x0e00",
		Data: map[string]any{
			"id":    "0x0e00",
			"propA": "aaa",
			"propB": "456",
		},
		Entity:         e2Type.Name,
		GenBlockNumber: 12,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}, box)

	// delete EntityE2 object, get by EntityE will get the EntityE2 delete record
	assert.NoError(t, txn.SetEntity(ctx, e2Type, EntityBox{
		ID:             "0x0e00",
		GenBlockNumber: 14,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}))
	box, err = txn.GetEntity(ctx, eType, "0x0e00", 20)
	assert.NoError(t, err)
	assert.Equal(t, &EntityBox{
		ID:             "0x0e00",
		Entity:         e2Type.Name,
		GenBlockNumber: 14,
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}, box)
}

func Test_changeHistoryPush(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	eType := sch.GetEntity("EntityE1")

	var his changeHistory
	his.Push(eType, &EntityBox{GenBlockNumber: 3, GenBlockHash: "3-1", Data: map[string]any{"propB": int32(1)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 3, GenBlockHash: "3-2", Data: map[string]any{},
		Operator: map[string]Operator{
			"propB": {
				NumCalc: &OperatorNumCalc{
					Multi: rsh.NewIntValue(1),
					Add:   rsh.NewIntValue(1234),
				},
			},
		},
	})
	his.Push(eType, &EntityBox{GenBlockNumber: 5, GenBlockHash: "5-1", Data: map[string]any{"propB": int32(3)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 5, GenBlockHash: "5-2", Data: map[string]any{"propB": int32(4)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 1, GenBlockHash: "1-1", Data: map[string]any{"propB": int32(5)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 1, GenBlockHash: "1-2", Data: map[string]any{"propB": int32(6)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 4, GenBlockHash: "4-1", Data: map[string]any{"propB": int32(7)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 4, GenBlockHash: "4-2", Data: map[string]any{"propB": int32(8)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 2, GenBlockHash: "2-1", Data: map[string]any{"propB": int32(9)}})
	his.Push(eType, &EntityBox{GenBlockNumber: 2, GenBlockHash: "2-2", Data: map[string]any{"propB": int32(10)}})

	assert.Equal(t,
		[]string{"1-2", "2-2", "3-2", "4-2", "5-2"},
		utils.MapSliceNoError(his, func(b *EntityBox) string {
			return b.GenBlockHash
		}))
	assert.Equal(t,
		[]map[string]any{
			{"propB": int32(6)},
			{"propB": int32(10)},
			{"propB": int32(1235)},
			{"propB": int32(8)},
			{"propB": int32(4)},
		},
		utils.MapSliceNoError(his, func(b *EntityBox) map[string]any {
			return b.Data
		}))

}

func Test_changeHistorySplit(t *testing.T) {
	his := changeHistory{
		&EntityBox{GenBlockNumber: 1},
		&EntityBox{GenBlockNumber: 2},
		&EntityBox{GenBlockNumber: 3},
		&EntityBox{GenBlockNumber: 4},
		&EntityBox{GenBlockNumber: 5},
	}
	assert.Equal(t, changeHistory{
		&EntityBox{GenBlockNumber: 1},
		&EntityBox{GenBlockNumber: 2},
		&EntityBox{GenBlockNumber: 3},
		&EntityBox{GenBlockNumber: 4},
		&EntityBox{GenBlockNumber: 5},
	}, his.Split(0))
	assert.Equal(t, changeHistory{}, his)

	his = changeHistory{
		&EntityBox{GenBlockNumber: 1},
		&EntityBox{GenBlockNumber: 2},
		&EntityBox{GenBlockNumber: 3},
		&EntityBox{GenBlockNumber: 4},
		&EntityBox{GenBlockNumber: 5},
	}
	assert.Equal(t, changeHistory{
		&EntityBox{GenBlockNumber: 2},
		&EntityBox{GenBlockNumber: 3},
		&EntityBox{GenBlockNumber: 4},
		&EntityBox{GenBlockNumber: 5},
	}, his.Split(1))
	assert.Equal(t, changeHistory{
		&EntityBox{GenBlockNumber: 1},
	}, his)

	his = changeHistory{
		&EntityBox{GenBlockNumber: 1},
		&EntityBox{GenBlockNumber: 2},
		&EntityBox{GenBlockNumber: 3},
		&EntityBox{GenBlockNumber: 4},
		&EntityBox{GenBlockNumber: 5},
	}
	assert.Equal(t, changeHistory{
		&EntityBox{GenBlockNumber: 5},
	}, his.Split(4))
	assert.Equal(t, changeHistory{
		&EntityBox{GenBlockNumber: 1},
		&EntityBox{GenBlockNumber: 2},
		&EntityBox{GenBlockNumber: 3},
		&EntityBox{GenBlockNumber: 4},
	}, his)

	his = changeHistory{
		&EntityBox{GenBlockNumber: 1},
		&EntityBox{GenBlockNumber: 2},
		&EntityBox{GenBlockNumber: 3},
		&EntityBox{GenBlockNumber: 4},
		&EntityBox{GenBlockNumber: 5},
	}
	assert.Nil(t, his.Split(5))
	assert.Equal(t, changeHistory{
		&EntityBox{GenBlockNumber: 1},
		&EntityBox{GenBlockNumber: 2},
		&EntityBox{GenBlockNumber: 3},
		&EntityBox{GenBlockNumber: 4},
		&EntityBox{GenBlockNumber: 5},
	}, his)
}

func Test_changeSetSplit(t *testing.T) {
	cs := changeSet{
		"entityA": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 1},
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
			},
			"2": {
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
				&EntityBox{GenBlockNumber: 4},
			},
			"3": {
				&EntityBox{GenBlockNumber: 3},
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
			},
			"4": {
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
				&EntityBox{GenBlockNumber: 6},
			},
		},
		"entityB": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 1},
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
			},
		},
		"entityC": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
				&EntityBox{GenBlockNumber: 6},
			},
		},
	}
	assert.Equal(t, changeSet{
		"entityA": map[string]changeHistory{
			"2": {
				&EntityBox{GenBlockNumber: 4},
			},
			"3": {
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
			},
			"4": {
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
				&EntityBox{GenBlockNumber: 6},
			},
		},
		"entityC": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 4},
				&EntityBox{GenBlockNumber: 5},
				&EntityBox{GenBlockNumber: 6},
			},
		},
	}, cs.Split(3))
	assert.Equal(t, changeSet{
		"entityA": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 1},
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
			},
			"2": {
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
			},
			"3": {
				&EntityBox{GenBlockNumber: 3},
			},
		},
		"entityB": map[string]changeHistory{
			"1": {
				&EntityBox{GenBlockNumber: 1},
				&EntityBox{GenBlockNumber: 2},
				&EntityBox{GenBlockNumber: 3},
			},
		},
	}, cs)
}
