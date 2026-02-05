package schema

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_parse(t *testing.T) {
	const testSchemaCnt = `
type EntityA @entity(immutable: true, sparse: true) {
  id: Bytes!
	propertyA: String @index
	propertyB: Boolean
	propertyC: Int
	propertyD: [BigInt!]!
	propertyE: [[BigDecimal!]!]
	propertyF: EnumA
	propertyG: [EnumA]
	propertyH: Timestamp
	propertyI: Float
	foreignA: EntityB                                    # many to one
	foreignB: [EntityB] @derivedFrom(field: "foreignB")  # one  to many
	foreignC: [EntityC] @derivedFrom(field: "foreignCA") # many to many by EntityC
	foreignD: EntityB                                    # one  to one
	foreignE: [EntityB!] @derivedFrom(field: "foreignE") # many to many
	foreignF: EntityB! @derivedFrom(field: "foreignF")   # many to one
}

type EntityB @entity {
	id: String!
	propertyA: String!
	foreignB: EntityA!                                   # many to one
	foreignC: [EntityC] @derivedFrom(field: "foreignCB") # many to many by EntityC
	foreignD: EntityA @derivedFrom(field: "foreignD")    # one  to one
	foreignE: [EntityA!]                                 # many to many
	foreignF: [EntityA!]                                 # one  to many
}

type EntityC @entity {
	id: Bytes!
	propertyA: Int!
	foreignCA: EntityA!
	foreignCB: EntityB!
}

type EntityD @entity {
	id: ID!
	foreignA: EntityE!
}

type EntityE1 implements EntityE @entity {
	id: ID!
	propertyA: String!
	propertyB: String!
}

type EntityE2 implements EntityE @entity {
	id: ID!
	propertyA: String!
	propertyB: Int!
}

interface EntityE {
	id: ID!
	propertyA: String!
}

enum EnumA {
  AAA
  BBB
  CCC
}

type EntityF @entity(timeseries: true) {
    id: Int8!
    timestamp: Timestamp!
    dimA: String!
    propA: BigDecimal!
    propB: BigDecimal!
    propC: BigInt!
}

type AggA @aggregation(intervals: ["hour", "day"], source: "EntityF") {
    id: Int8!
    timestamp: Timestamp!
    dimA: String!
    aggA: BigDecimal! @aggregate(fn: "sum", arg: "propA")
    aggB: BigDecimal! @aggregate(fn: "max", arg: "(propA+propB)/2")
    aggC: BigDecimal! @aggregate(fn: "sum", arg: "min(propA,propB)")
    aggD: BigInt! @aggregate(fn: "first", arg: "propC")
    aggE: BigInt! @aggregate(fn: "last", arg: "propC")
    aggF: Int8! @aggregate(fn: "count")
}
`

	sch, err := ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)
	for _, agg := range sch.ListAggregations() {
		for _, f := range agg.AggFields {
			e, _ := f.TryGetAggExp()
			fmt.Printf("%s.%s: %s %s\n", agg.Name, f.Name, f.GetAggFunc(), e.String())
		}
	}
}

func Test_parse2(t *testing.T) {
	_, err := ParseAndVerifySchema(`
type EntityA @entity(immutable: true, sparse: true) {
  id: Bytes!
	propertyA: String @index @dbType(type: "x")
}
`)
	assert.NotNil(t, err)
	assert.Equal(t, `fixed field EntityA.propertyA has db type directive but dbType of field type String cannot be "x"`, err.Error())

	_, err = ParseAndVerifySchema(`
type EntityA @entity(immutable: true, sparse: true) {
  id: Bytes!
	propertyA: String @index @dbType(type: "JSON")
}
`)
	assert.NoError(t, err)
}

func Test_duplicated(t *testing.T) {
	_, err := ParseAndVerifySchema(`
type Account @entity {
  id: ID!
  updatedAt: BigInt!
  balance: BigInt!
}

type Account @aggregation {
  id: ID!
  updatedAt: BigInt!
  balance: BigInt!
}

`)
	assert.NotNil(t, err)
	assert.Equal(t, `"Account" was duplicated`, err.Error())

	_, err = ParseAndVerifySchema(`
type Account @entity {
  id: ID!
  updatedAt: BigInt!
  balance: BigInt!
}

type Account @entity {
  id: ID!
  updatedAt: BigInt!
  balance: BigInt!
}

`)
	assert.NotNil(t, err)
	assert.Equal(t, `"Account" was duplicated`, err.Error())
}
