package clickhouse

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
)

const testSchemaCnt = `
type EntityA @entity(immutable: true) {
  id: Bytes!
	propertyA: String
	propertyB: Boolean
	propertyC: Int
	propertyD: [BigInt!]!
	propertyE: [[BigDecimal!]!]
	propertyF: EnumA
	propertyG: [EnumA]
	foreignA: EntityB!                                   # many to one
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
	foreignE: [EntityA]                                  # many to many
	foreignF: [EntityA!]!                                # one  to many
}

type EntityC @entity {
	id: Bytes!
	propertyA: Int!
	propertyB: BigInt!
	propertyC: BigInt
	propertyD: BigDecimal!
	foreignCA: EntityA!
	foreignCB: EntityB!
}

interface EntityD {
	id: ID!
	on: [EntityE!]  # many to many
}

type EntityD1 implements EntityD @entity {
	id: ID!
	propertyA: String!
	on: [EntityE!]  # many to many
}

type EntityD2 implements EntityD @entity {
	id: ID!
	propertyA: Int!
	on: [EntityE!]  # many to many
}

interface EntityE {
	id: ID!
	from: String!
	on: [EntityD!] @derivedFrom(field: "on")  # many to many
}

type EntityE1 implements EntityE @entity {
	id: ID!
	from: String!
	on: [EntityD!] @derivedFrom(field: "on")  # many to many
}

type EntityE2 implements EntityE @entity {
	id: ID!
	from: String! # SimpleField
	by: [Int]     # JSONTextField
	left: BigInt  # TupleField
	on: [EntityD!] @derivedFrom(field: "on")  # many to many
}

type EntityF1 @entity(immutable: true) {
  id: Bytes!
	propertyA: Bytes! @index
	propertyB: String! @index(type: "bloom_filter")
	propertyC: Boolean! @index
	propertyD: Int! @index
	propertyE: BigInt! @index
	propertyF: BigDecimal! @index
	propertyG: EnumA! @index
	propertyH: [Bytes!] @index
	propertyI: [String!] @index
	propertyJ: [Boolean!] @index
	propertyK: [Int!] @index
	propertyL: [BigInt!] @index
	propertyM: [BigDecimal!] @index
	propertyN: [EnumA!] @index
	propertyO: Timestamp! @index
	propertyP: [Timestamp!] @index
	propertyQ: Float! @index
	propertyR: [Float!] @index
	foreignA: EntityA! @index
	foreignB: [EntityA!] @index
}

type EntityF2 @entity(immutable: false) {
  id: Bytes!
	propertyA: Bytes! @index
	propertyB: String! @index
	propertyC: Boolean! @index
	propertyD: Int! @index
	propertyE: BigInt! @index
	propertyF: BigDecimal! @index
	propertyG: EnumA! @index
	propertyH: [Bytes!] @index
	propertyI: [String!] @index
	propertyJ: [Boolean!] @index
	propertyK: [Int!] @index
	propertyL: [BigInt!] @index
	propertyM: [BigDecimal!] @index
	propertyN: [EnumA!] @index
	propertyO: Timestamp! @index
	propertyP: [Timestamp!] @index
	propertyQ: Float! @index
	propertyR: [Float!] @index
	foreignA: EntityA! @index
	foreignB: [EntityA!] @index
}

type EntityG @entity {
	id: ID!
	propA1: String
	propB1: BigInt
	propC1: BigDecimal
	propA2: [String]
	propB2: [BigInt]
	propC2: [BigDecimal]
	forkA1: EntityA
	forkA2: [EntityA]
}

enum EnumA {
  AAA
  BBB
  CCC
}

type EntityH @entity(timeseries: true) {
	id: Int8!
  timestamp: Timestamp!
	dimA: String
  propA: BigDecimal!
  propB: BigDecimal!
}

type AggA @aggregation(intervals: ["hour", "day"], source: "EntityH") {
	id: Int8!
  timestamp: Timestamp!
	dimA: String
  aggA: BigDecimal! @aggregate(fn: "sum", arg: "propA")
  aggB: BigDecimal! @aggregate(fn: "sum", arg: "(propA+propB)/2")
}

type EntityI @entity {
	id: ID!
	propA: String @dbType(type: "json")
}
`

func printSQLMap(sqlMap map[string][]string) {
	find := func(str string, start, end, kw string, off int) (p []int) {
		s := strings.Index(str, start)
		if start == "" {
			s = 0
		}
		e := strings.Index(str, end)
		if end == "" {
			e = len(str)
		}
		for {
			x := strings.Index(str[s:e], kw)
			if x > 0 {
				p = append(p, s+x+off)
			} else {
				break
			}
			s = s + x + 1
		}
		return p
	}
	trim := func(breaks []int) []int {
		set := make(map[int]struct{})
		for _, x := range breaks {
			if x >= 0 {
				set[x] = struct{}{}
			}
		}
		return utils.GetOrderedMapKeys(set)
	}
	cutAndPrint := func(str string, breaks []int) {
		var s int
		for _, b := range breaks {
			fmt.Printf("%q +\n", str[s:b])
			s = b
		}
		fmt.Printf("%q\n", str[s:])
	}
	keep := func(sql string, start, end string) (r [][2]int) {
		var s int
		for s < len(sql) {
			p := strings.Index(sql[s:], start)
			if p < 0 {
				break
			}
			p += s
			q := strings.Index(sql[p:], end)
			if q < 0 {
				q = len(sql)
			} else {
				q += p
			}
			r = append(r, [2]int{p, q})
			s = q
		}
		return r
	}
	remove := func(set []int, keep [][2]int) (r []int) {
		for _, x := range set {
			var need = true
			for _, kp := range keep {
				if kp[0] < x && x < kp[1] {
					need = false
					break
				}
			}
			if need {
				r = append(r, x)
			}
		}
		return r
	}

	for _, entityName := range utils.GetOrderedMapKeys(sqlMap) {
		fmt.Printf("name: %s\n", entityName)
		for _, sql := range sqlMap[entityName] {
			fmt.Printf("------------------------------------------------\n")
			if strings.HasPrefix(sql, "CREATE TABLE") {
				cutAndPrint(sql, trim(utils.MergeArr(
					find(sql, "`id`", ") ENGINE", ", `", 2),
					find(sql, "`id`", ") ENGINE", ", INDEX ", 2),
					find(sql, "", "", "COMMENT 'IMPL", 0),
					[]int{
						strings.Index(sql, "`id`"),
						strings.Index(sql, "ENGINE") - 2,
						strings.Index(sql, "ENGINE"),
						strings.Index(sql, "PARTITION BY"),
						strings.Index(sql, "ORDER BY"),
						strings.Index(sql, "SETTINGS"),
					},
				)))
			} else {
				cutAndPrint(sql, remove(
					trim(utils.MergeArr(
						find(sql, "", "", ", `", 2),
						find(sql, "", "", "(`", 1),
						find(sql, "", "", ", arrayMap(", 2),
						find(sql, "", "", ", any_respect_nulls(", 2),
						find(sql, "", "", ", if(", 2),
						find(sql, "", "", "FROM ", 0),
						find(sql, "", "", "WHERE ", 0),
						find(sql, "", "", "GROUP BY ", 0),
						find(sql, "", "", "HAVING ", 0),
						find(sql, "", "", "COMMENT 'IMPL", 0),
						find(sql, "", "", ", __last__", 2),
						find(sql, "", "", "(SELECT ", 1),
						find(sql, "", "", "SELECT ", 7),
						find(sql, "", "", ") AS SELECT", 0),
						find(sql, "", "", ") AS SELECT", 5),
						find(sql, "", "", ") WHERE", 0),
						find(sql, "", "", "MAX((", 5),
						find(sql, "", "", ")) AS __last__", 0),
					)),
					utils.MergeArr(
						keep(sql, "SELECT * ", "ROM "),
						keep(sql, "GROUP BY ", "HAVING "),
						keep(sql, "HAVING", "> 0"),
						keep(sql, "arrayMap", " AS `"),
						keep(sql, "any_respect_nulls", " AS `"),
						keep(sql, "if(", " AS `"),
					),
				))
			}
		}
		fmt.Printf("==================================================\n")
	}
}

func Test_createTableSQL(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	ctrl := chx.NewController(nil, "")
	s := Store{
		ctrl:        ctrl,
		database:    "db",
		processorID: "processor0",
		sch:         sch,
		schHash:     "xxx",
		tableOpt:    DefaultCreateTableOption,
	}

	sqlMap := make(map[string][]string)
	for entityName, tvs := range s.buildTablesAndViews(false) {
		for _, tv := range tvs {
			sqlMap[entityName] = append(sqlMap[entityName], ctrl.BuildCreateSQL(tv))
		}
	}
	//printSQLMap(sqlMap)

	expected := map[string][]string{
		"EntityA": {
			"CREATE TABLE `db`.`processor0_entity_EntityA` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propertyB` Nullable(Bool) COMMENT 'SCALAR(Boolean) SCHEMA(Boolean)', " +
				"`propertyC` Nullable(Int32) COMMENT 'SCALAR(Int) SCHEMA(Int)', " +
				"`propertyD` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!]!)', " +
				"`propertyE` String COMMENT 'SCALAR(BigDecimal) SCHEMA([[BigDecimal!]!])', " +
				"`propertyF` Nullable(Enum('AAA', 'BBB', 'CCC')) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA)', " +
				"`propertyG` String COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`foreignB` Array(Nullable(String)) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignB) SCHEMA([EntityB])', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCA) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityB) SCHEMA(EntityB)', " +
				"`foreignE` Array(String) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignE) SCHEMA([EntityB!])', " +
				"`foreignF` String COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignF) SCHEMA(EntityB!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityA` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propertyB` Nullable(Bool) COMMENT 'SCALAR(Boolean) SCHEMA(Boolean)', " +
				"`propertyC` Nullable(Int32) COMMENT 'SCALAR(Int) SCHEMA(Int)', " +
				"`propertyD` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!]!)', " +
				"`propertyE` Array(Array(String)) COMMENT 'SCALAR(BigDecimal) SCHEMA([[BigDecimal!]!])', " +
				"`propertyF` Nullable(Enum('AAA', 'BBB', 'CCC')) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA)', " +
				"`propertyG` Array(Nullable(String)) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`foreignB` Array(Nullable(String)) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignB) SCHEMA([EntityB])', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCA) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityB) SCHEMA(EntityB)', " +
				"`foreignE` Array(String) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignE) SCHEMA([EntityB!])', " +
				"`foreignF` String COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignF) SCHEMA(EntityB!)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyD`)) AS `propertyD`, " +
				"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propertyE`)) AS `propertyE`, " +
				"`propertyF`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propertyG`)) AS `propertyG`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`foreignC`, " +
				"`foreignD`, " +
				"`foreignE`, " +
				"`foreignF`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityA`",
			"CREATE VIEW `db`.`processor0_latestView_EntityA` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propertyB` Nullable(Bool) COMMENT 'SCALAR(Boolean) SCHEMA(Boolean)', " +
				"`propertyC` Nullable(Int32) COMMENT 'SCALAR(Int) SCHEMA(Int)', " +
				"`propertyD` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!]!)', " +
				"`propertyE` Array(Array(String)) COMMENT 'SCALAR(BigDecimal) SCHEMA([[BigDecimal!]!])', " +
				"`propertyF` Nullable(Enum('AAA', 'BBB', 'CCC')) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA)', " +
				"`propertyG` Array(Nullable(String)) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`foreignB` Array(Nullable(String)) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignB) SCHEMA([EntityB])', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCA) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityB) SCHEMA(EntityB)', " +
				"`foreignE` Array(String) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignE) SCHEMA([EntityB!])', " +
				"`foreignF` String COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignF) SCHEMA(EntityB!)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"`propertyE`, " +
				"`propertyF`, " +
				"`propertyG`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`foreignC`, " +
				"`foreignD`, " +
				"`foreignE`, " +
				"`foreignF`, " +
				"`meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM `db`.`processor0_view_EntityA`",
		},
		"EntityB": {
			"CREATE TABLE `db`.`processor0_entity_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`foreignB` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCB) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityA) DERIVED_FROM(foreignD) SCHEMA(EntityA)', " +
				"`foreignE` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__foreignE__isnull__` Bool, " +
				"`foreignF` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!]!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`foreignB` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCB) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityA) DERIVED_FROM(foreignD) SCHEMA(EntityA)', " +
				"`foreignE` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__foreignE__isnull__` Bool, `foreignF` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!]!)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`foreignB`, " +
				"`foreignC`, " +
				"`foreignD`, " +
				"`foreignE`, " +
				"`__foreignE__isnull__`, " +
				"`foreignF`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityB`",
			"CREATE VIEW `db`.`processor0_latestView_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`foreignB` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCB) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityA) DERIVED_FROM(foreignD) SCHEMA(EntityA)', " +
				"`foreignE` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__foreignE__isnull__` Bool, `foreignF` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!]!)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`meta.chain`, " +
				"`meta.chain` AS `__genBlockChain__`, " +
				"__last__.3 AS `propertyA`, " +
				"__last__.4 AS `foreignB`, " +
				"__last__.5 AS `foreignC`, " +
				"__last__.6 AS `foreignD`, " +
				"__last__.7 AS `foreignE`, " +
				"__last__.8 AS `__foreignE__isnull__`, " +
				"__last__.9 AS `foreignF` " +
				"FROM (" +
				"SELECT `id`, `meta.chain`, MAX((" +
				"`meta.block_number`, " +
				"`meta.deleted`, " +
				"`propertyA`, " +
				"`foreignB`, " +
				"`foreignC`, " +
				"`foreignD`, " +
				"`foreignE`, " +
				"`__foreignE__isnull__`, " +
				"`foreignF`" +
				")) AS __last__ " +
				"FROM `db`.`processor0_view_EntityB` " +
				"GROUP BY `id`, `meta.chain`" +
				") " +
				"WHERE NOT __last__.2",
		},
		"EntityC": {
			"CREATE TABLE `db`.`processor0_entity_EntityC` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyB` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyC` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propertyD` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`foreignCA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignCB` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityC` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyB` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyC` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propertyD` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`foreignCA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignCB` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"if(`propertyB`.1,if(`propertyB`.2>=0,toFloat64(`propertyB`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propertyB`.3+1,'UInt256'))),NULL) AS `propertyB`, " +
				"if(`propertyC`.1,if(`propertyC`.2>=0,toFloat64(`propertyC`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propertyC`.3+1,'UInt256'))),NULL) AS `propertyC`, " +
				"`propertyD`, " +
				"`foreignCA`, " +
				"`foreignCB`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityC`",
			"CREATE VIEW `db`.`processor0_latestView_EntityC` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyB` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyC` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propertyD` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`foreignCA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignCB` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`meta.chain`, " +
				"`meta.chain` AS `__genBlockChain__`, " +
				"__last__.3 AS `propertyA`, " +
				"__last__.4 AS `propertyB`, " +
				"__last__.5 AS `propertyC`, " +
				"__last__.6 AS `propertyD`, " +
				"__last__.7 AS `foreignCA`, " +
				"__last__.8 AS `foreignCB` " +
				"FROM (" +
				"SELECT `id`, `meta.chain`, MAX((" +
				"`meta.block_number`, " +
				"`meta.deleted`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"`foreignCA`, " +
				"`foreignCB`" +
				")) AS __last__ " +
				"FROM `db`.`processor0_view_EntityC` " +
				"GROUP BY `id`, `meta.chain`" +
				") " +
				"WHERE NOT __last__.2",
		},
		"EntityD": {
			"CREATE VIEW `db`.`processor0_interface_EntityD` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__`, " +
				"'EntityD1' AS `__implEntity__` " +
				"FROM `db`.`processor0_entity_EntityD1` UNION ALL SELECT " +
				"`id`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__`, " +
				"'EntityD2' AS `__implEntity__` " +
				"FROM `db`.`processor0_entity_EntityD2`",
			"CREATE VIEW `db`.`processor0_view_EntityD` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`meta.impl_entity` String, " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__`, " +
				"`__implEntity__` AS `meta.impl_entity`, " +
				"`__implEntity__` " +
				"FROM `db`.`processor0_interface_EntityD`",
			"CREATE VIEW `db`.`processor0_latestView_EntityD` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.impl_entity` String, " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`meta.chain`, " +
				"`__genBlockChain__`, " +
				"'EntityD1' AS `meta.impl_entity`, " +
				"'EntityD1' AS `__implEntity__` " +
				"FROM `db`.`processor0_latestView_EntityD1` " +
				"UNION ALL " +
				"SELECT " +
				"`id`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`meta.chain`, " +
				"`__genBlockChain__`, " +
				"'EntityD2' AS `meta.impl_entity`, " +
				"'EntityD2' AS `__implEntity__` " +
				"FROM `db`.`processor0_latestView_EntityD2`",
		},
		"EntityD1": {
			"CREATE TABLE `db`.`processor0_entity_EntityD1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityD) SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityD1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityD1`",
			"CREATE VIEW `db`.`processor0_latestView_EntityD1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`meta.chain`, " +
				"`meta.chain` AS `__genBlockChain__`, " +
				"__last__.3 AS `propertyA`, " +
				"__last__.4 AS `on__`, " +
				"__last__.5 AS `__on__isnull__` " +
				"FROM (" +
				"SELECT `id`, `meta.chain`, MAX((" +
				"`meta.block_number`, " +
				"`meta.deleted`, " +
				"`propertyA`, " +
				"`on__`, " +
				"`__on__isnull__`" +
				")) AS __last__ " +
				"FROM `db`.`processor0_view_EntityD1` " +
				"GROUP BY `id`, `meta.chain`" +
				") " +
				"WHERE NOT __last__.2",
		},
		"EntityD2": {
			"CREATE TABLE `db`.`processor0_entity_EntityD2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityD) SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityD2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityD2`",
			"CREATE VIEW `db`.`processor0_latestView_EntityD2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`meta.chain`, " +
				"`meta.chain` AS `__genBlockChain__`, " +
				"__last__.3 AS `propertyA`, " +
				"__last__.4 AS `on__`, " +
				"__last__.5 AS `__on__isnull__` " +
				"FROM (" +
				"SELECT `id`, `meta.chain`, MAX((" +
				"`meta.block_number`, " +
				"`meta.deleted`, " +
				"`propertyA`, " +
				"`on__`, " +
				"`__on__isnull__`" +
				")) AS __last__ " +
				"FROM `db`.`processor0_view_EntityD2` " +
				"GROUP BY `id`, `meta.chain`" +
				") " +
				"WHERE NOT __last__.2",
		},
		"EntityE": {
			"CREATE VIEW `db`.`processor0_interface_EntityE` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__`, " +
				"'EntityE1' AS `__implEntity__` " +
				"FROM `db`.`processor0_entity_EntityE1` " +
				"UNION ALL " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__`, " +
				"'EntityE2' AS `__implEntity__` " +
				"FROM `db`.`processor0_entity_EntityE2`",
			"CREATE VIEW `db`.`processor0_view_EntityE` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`meta.impl_entity` String, " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__`, " +
				"`__implEntity__` AS `meta.impl_entity`, " +
				"`__implEntity__` " +
				"FROM `db`.`processor0_interface_EntityE`",
			"CREATE VIEW `db`.`processor0_latestView_EntityE` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.impl_entity` String, " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`meta.chain`, " +
				"`__genBlockChain__`, " +
				"'EntityE1' AS `meta.impl_entity`, " +
				"'EntityE1' AS `__implEntity__` " +
				"FROM `db`.`processor0_latestView_EntityE1` " +
				"UNION ALL " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`meta.chain`, " +
				"`__genBlockChain__`, " +
				"'EntityE2' AS `meta.impl_entity`, " +
				"'EntityE2' AS `__implEntity__` " +
				"FROM `db`.`processor0_latestView_EntityE2`",
		},
		"EntityE1": {
			"CREATE TABLE `db`.`processor0_entity_EntityE1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityE) SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityE1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityE1`",
			"CREATE VIEW `db`.`processor0_latestView_EntityE1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`meta.chain`, " +
				"`meta.chain` AS `__genBlockChain__`, " +
				"__last__.3 AS `from__`, " +
				"__last__.4 AS `on__` " +
				"FROM (" +
				"SELECT `id`, `meta.chain`, MAX((" +
				"`meta.block_number`, " +
				"`meta.deleted`, " +
				"`from__`, " +
				"`on__`" +
				")) AS __last__ " +
				"FROM `db`.`processor0_view_EntityE1` " +
				"GROUP BY `id`, `meta.chain`" +
				") " +
				"WHERE NOT __last__.2",
		},
		"EntityE2": {
			"CREATE TABLE `db`.`processor0_entity_EntityE2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`by__` String COMMENT 'SCALAR(Int) SCHEMA([Int])', " +
				"`left__` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityE) SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityE2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`by__` Array(Nullable(Int32)) COMMENT 'SCALAR(Int) SCHEMA([Int])', " +
				"`left__` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, toInt32(x0)), JSONExtractArrayRaw(`by__`)) AS `by__`, " +
				"if(`left__`.1,if(`left__`.2>=0,toFloat64(`left__`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`left__`.3+1,'UInt256'))),NULL) AS `left__`, " +
				"`on__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityE2`",
			"CREATE VIEW `db`.`processor0_latestView_EntityE2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`by__` Array(Nullable(Int32)) COMMENT 'SCALAR(Int) SCHEMA([Int])', " +
				"`left__` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`meta.chain`, " +
				"`meta.chain` AS `__genBlockChain__`, " +
				"__last__.3 AS `from__`, " +
				"__last__.4 AS `by__`, " +
				"__last__.5 AS `left__`, " +
				"__last__.6 AS `on__` " +
				"FROM (" +
				"SELECT `id`, `meta.chain`, MAX((" +
				"`meta.block_number`, " +
				"`meta.deleted`, " +
				"`from__`, " +
				"`by__`, " +
				"`left__`, " +
				"`on__`" +
				")) AS __last__ " +
				"FROM `db`.`processor0_view_EntityE2` " +
				"GROUP BY `id`, `meta.chain`" +
				") " +
				"WHERE NOT __last__.2",
		},
		"EntityF1": {
			"CREATE TABLE `db`.`processor0_entity_EntityF1` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` String COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` String COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` String COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` String COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` String COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` String COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` String COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"INDEX `idx_propertyA` `propertyA` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_propertyB` `propertyB` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_propertyC` `propertyC` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyD` `propertyD` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyE` `propertyE` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyF` `propertyF` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyG` `propertyG` TYPE set(0) GRANULARITY 1, " +
				"INDEX `idx_propertyO` `propertyO` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyQ` `propertyQ` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_foreignA` `foreignA` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_foreignB` `foreignB` TYPE bloom_filter GRANULARITY 1" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityF1` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` Array(String) COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` Array(String) COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` Array(Bool) COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` Array(Int32) COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` Array(String) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` Array(String) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` Array(Int64) COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` Array(Float64) COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"if(`propertyE`.1,if(`propertyE`.2>=0,toFloat64(`propertyE`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propertyE`.3+1,'UInt256'))),NULL) AS `propertyE`, " +
				"`propertyF`, " +
				"`propertyG`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyH`)) AS `propertyH`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyI`)) AS `propertyI`, " +
				"arrayMap(x0 -> JSONExtractBool(x0), JSONExtractArrayRaw(`propertyJ`)) AS `propertyJ`, " +
				"arrayMap(x0 -> toInt32(x0), JSONExtractArrayRaw(`propertyK`)) AS `propertyK`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyL`)) AS `propertyL`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyM`)) AS `propertyM`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyN`)) AS `propertyN`, " +
				"`propertyO`, " +
				"arrayMap(x0 -> JSONExtractInt(x0), JSONExtractArrayRaw(`propertyP`)) AS `propertyP`, " +
				"`propertyQ`, " +
				"arrayMap(x0 -> JSONExtractFloat(x0), JSONExtractArrayRaw(`propertyR`)) AS `propertyR`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`__foreignB__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityF1`",
			"CREATE VIEW `db`.`processor0_latestView_EntityF1` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` Array(String) COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` Array(String) COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` Array(Bool) COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` Array(Int32) COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` Array(String) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` Array(String) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` Array(Int64) COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` Array(Float64) COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"`propertyE`, " +
				"`propertyF`, " +
				"`propertyG`, " +
				"`propertyH`, " +
				"`propertyI`, " +
				"`propertyJ`, " +
				"`propertyK`, " +
				"`propertyL`, " +
				"`propertyM`, " +
				"`propertyN`, " +
				"`propertyO`, " +
				"`propertyP`, " +
				"`propertyQ`, " +
				"`propertyR`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`__foreignB__isnull__`, " +
				"`meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM `db`.`processor0_view_EntityF1`",
		},
		"EntityF2": {
			"CREATE TABLE `db`.`processor0_entity_EntityF2` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` String COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` String COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` String COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` String COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` String COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` String COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` String COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"INDEX `idx_propertyA` `propertyA` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_propertyB` `propertyB` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_propertyC` `propertyC` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyD` `propertyD` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyE` `propertyE` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyF` `propertyF` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyG` `propertyG` TYPE set(0) GRANULARITY 1, " +
				"INDEX `idx_propertyO` `propertyO` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyQ` `propertyQ` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_foreignA` `foreignA` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_foreignB` `foreignB` TYPE bloom_filter GRANULARITY 1" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityF2` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` Array(String) COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` Array(String) COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` Array(Bool) COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` Array(Int32) COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` Array(String) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` Array(String) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` Array(Int64) COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` Array(Float64) COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"if(`propertyE`.1,if(`propertyE`.2>=0,toFloat64(`propertyE`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propertyE`.3+1,'UInt256'))),NULL) AS `propertyE`, " +
				"`propertyF`, " +
				"`propertyG`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyH`)) AS `propertyH`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyI`)) AS `propertyI`, " +
				"arrayMap(x0 -> JSONExtractBool(x0), JSONExtractArrayRaw(`propertyJ`)) AS `propertyJ`, " +
				"arrayMap(x0 -> toInt32(x0), JSONExtractArrayRaw(`propertyK`)) AS `propertyK`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyL`)) AS `propertyL`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyM`)) AS `propertyM`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyN`)) AS `propertyN`, " +
				"`propertyO`, " +
				"arrayMap(x0 -> JSONExtractInt(x0), JSONExtractArrayRaw(`propertyP`)) AS `propertyP`, " +
				"`propertyQ`, " +
				"arrayMap(x0 -> JSONExtractFloat(x0), JSONExtractArrayRaw(`propertyR`)) AS `propertyR`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`__foreignB__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityF2`",
			"CREATE VIEW `db`.`processor0_latestView_EntityF2` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` Array(String) COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` Array(String) COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` Array(Bool) COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` Array(Int32) COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` Array(String) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` Array(String) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` Array(Int64) COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` Array(Float64) COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`meta.chain`, " +
				"`meta.chain` AS `__genBlockChain__`, " +
				"__last__.3 AS `propertyA`, " +
				"__last__.4 AS `propertyB`, " +
				"__last__.5 AS `propertyC`, " +
				"__last__.6 AS `propertyD`, " +
				"__last__.7 AS `propertyE`, " +
				"__last__.8 AS `propertyF`, " +
				"__last__.9 AS `propertyG`, " +
				"__last__.10 AS `propertyH`, " +
				"__last__.11 AS `propertyI`, " +
				"__last__.12 AS `propertyJ`, " +
				"__last__.13 AS `propertyK`, " +
				"__last__.14 AS `propertyL`, " +
				"__last__.15 AS `propertyM`, " +
				"__last__.16 AS `propertyN`, " +
				"__last__.17 AS `propertyO`, " +
				"__last__.18 AS `propertyP`, " +
				"__last__.19 AS `propertyQ`, " +
				"__last__.20 AS `propertyR`, " +
				"__last__.21 AS `foreignA`, " +
				"__last__.22 AS `foreignB`, " +
				"__last__.23 AS `__foreignB__isnull__` " +
				"FROM (" +
				"SELECT `id`, `meta.chain`, MAX((" +
				"`meta.block_number`, " +
				"`meta.deleted`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"`propertyE`, " +
				"`propertyF`, " +
				"`propertyG`, " +
				"`propertyH`, " +
				"`propertyI`, " +
				"`propertyJ`, " +
				"`propertyK`, " +
				"`propertyL`, " +
				"`propertyM`, " +
				"`propertyN`, " +
				"`propertyO`, " +
				"`propertyP`, " +
				"`propertyQ`, " +
				"`propertyR`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`__foreignB__isnull__`" +
				")) AS __last__ " +
				"FROM `db`.`processor0_view_EntityF2` " +
				"GROUP BY `id`, `meta.chain`" +
				") " +
				"WHERE NOT __last__.2",
		},
		"EntityG": {
			"CREATE TABLE `db`.`processor0_entity_EntityG` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA1` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propB1` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propC1` Nullable(Decimal256(30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)', " +
				"`propA2` String COMMENT 'SCALAR(String) SCHEMA([String])', " +
				"`propB2` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt])', " +
				"`propC2` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal])', " +
				"`forkA1` Nullable(String) COMMENT 'OBJECT(EntityA) SCHEMA(EntityA)', " +
				"`forkA2` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__forkA2__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityG` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA1` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propB1` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propC1` Nullable(Decimal256(30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)', " +
				"`propA2` Array(Nullable(String)) COMMENT 'SCALAR(String) SCHEMA([String])', " +
				"`propB2` Array(Nullable(String)) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt])', " +
				"`propC2` Array(Nullable(String)) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal])', " +
				"`forkA1` Nullable(String) COMMENT 'OBJECT(EntityA) SCHEMA(EntityA)', " +
				"`forkA2` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__forkA2__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propA1`, " +
				"if(`propB1`.1,if(`propB1`.2>=0,toFloat64(`propB1`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propB1`.3+1,'UInt256'))),NULL) AS `propB1`, " +
				"`propC1`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propA2`)) AS `propA2`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propB2`)) AS `propB2`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propC2`)) AS `propC2`, " +
				"`forkA1`, " +
				"`forkA2`, " +
				"`__forkA2__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityG`",
			"CREATE VIEW `db`.`processor0_latestView_EntityG` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA1` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propB1` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propC1` Nullable(Decimal256(30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)', " +
				"`propA2` Array(Nullable(String)) COMMENT 'SCALAR(String) SCHEMA([String])', " +
				"`propB2` Array(Nullable(String)) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt])', " +
				"`propC2` Array(Nullable(String)) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal])', " +
				"`forkA1` Nullable(String) COMMENT 'OBJECT(EntityA) SCHEMA(EntityA)', " +
				"`forkA2` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__forkA2__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`meta.chain`, " +
				"`meta.chain` AS `__genBlockChain__`, " +
				"__last__.3 AS `propA1`, " +
				"__last__.4 AS `propB1`, " +
				"__last__.5 AS `propC1`, " +
				"__last__.6 AS `propA2`, " +
				"__last__.7 AS `propB2`, " +
				"__last__.8 AS `propC2`, " +
				"__last__.9 AS `forkA1`, " +
				"__last__.10 AS `forkA2`, " +
				"__last__.11 AS `__forkA2__isnull__` " +
				"FROM (" +
				"SELECT `id`, `meta.chain`, MAX((" +
				"`meta.block_number`, " +
				"`meta.deleted`, " +
				"`propA1`, " +
				"`propB1`, " +
				"`propC1`, " +
				"`propA2`, " +
				"`propB2`, " +
				"`propC2`, " +
				"`forkA1`, " +
				"`forkA2`, " +
				"`__forkA2__isnull__`" +
				")) AS __last__ " +
				"FROM `db`.`processor0_view_EntityG` " +
				"GROUP BY `id`, `meta.chain`" +
				") " +
				"WHERE NOT __last__.2",
		},
		"EntityH": {
			"CREATE TABLE `db`.`processor0_entity_EntityH` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityH` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`timestamp`, " +
				"`dimA`, " +
				"`propA`, " +
				"`propB`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityH`",
			"CREATE VIEW `db`.`processor0_latestView_EntityH` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`timestamp`, " +
				"`dimA`, " +
				"`propA`, " +
				"`propB`, " +
				"`meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM `db`.`processor0_view_EntityH`",
		},
		"AggA": {
			"CREATE TABLE `db`.`processor0_aggregation_AggA` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`aggA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG(propA)', " +
				"`aggB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG((propA + propB) / 2)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__interval__` Enum('hour', 'day') COMMENT 'ENUM(AggAInterval(hour,day)) SCHEMA(AggAInterval!)'" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`__interval__`,`timestamp`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SRC(EntityH) SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_AggA` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`aggA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG(propA)', " +
				"`aggB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG((propA + propB) / 2)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`meta.aggregation_interval` Enum('hour', 'day') COMMENT 'ENUM(AggAInterval(hour,day)) SCHEMA(AggAInterval!)', " +
				"`__interval__` Enum('hour', 'day') COMMENT 'ENUM(AggAInterval(hour,day)) SCHEMA(AggAInterval!)'" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`timestamp`, " +
				"`dimA`, " +
				"`aggA`, " +
				"`aggB`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__`, " +
				"`__interval__` AS `meta.aggregation_interval`, " +
				"`__interval__` " +
				"FROM `db`.`processor0_aggregation_AggA`",
			"CREATE VIEW `db`.`processor0_latestView_AggA` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` Int64 COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`aggA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG(propA)', " +
				"`aggB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG((propA + propB) / 2)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.aggregation_interval` Enum('hour', 'day') COMMENT 'ENUM(AggAInterval(hour,day)) SCHEMA(AggAInterval!)', " +
				"`__interval__` Enum('hour', 'day') COMMENT 'ENUM(AggAInterval(hour,day)) SCHEMA(AggAInterval!)'" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`timestamp`, " +
				"`dimA`, " +
				"`aggA`, " +
				"`aggB`, " +
				"`meta.chain`, " +
				"`__genBlockChain__`, " +
				"`meta.aggregation_interval`, " +
				"`__interval__` " +
				"FROM `db`.`processor0_view_AggA`",
		},
		"EntityI": {
			"CREATE TABLE `db`.`processor0_entity_EntityI` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA` json COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityI` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA` json COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propA`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityI`",
			"CREATE VIEW `db`.`processor0_latestView_EntityI` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA` json COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`meta.chain`, " +
				"`meta.chain` AS `__genBlockChain__`, " +
				"__last__.3 AS `propA` " +
				"FROM (" +
				"SELECT `id`, `meta.chain`, MAX((" +
				"`meta.block_number`, " +
				"`meta.deleted`, " +
				"`propA`" +
				")) AS __last__ " +
				"FROM `db`.`processor0_view_EntityI` " +
				"GROUP BY `id`, `meta.chain`" +
				") WHERE NOT __last__.2",
		},
	}
	assert.Equal(t, expected, sqlMap)
}

func Test_createTableSQLEnableVersionedCollapsing(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	ctrl := chx.NewController(nil, "")
	s := Store{
		ctrl:        ctrl,
		database:    "db",
		processorID: "processor0",
		sch:         sch,
		schHash:     "xxx",
		feaOpt: Features{
			VersionedCollapsing:    true,
			TimestampUseDateTime64: true,
		},
		tableOpt: DefaultCreateTableOption,
	}

	sqlMap := make(map[string][]string)
	for entityName, tvs := range s.buildTablesAndViews(false) {
		for _, tv := range tvs {
			sqlMap[entityName] = append(sqlMap[entityName], ctrl.BuildCreateSQL(tv))
		}
	}
	//printSQLMap(sqlMap)

	expected := map[string][]string{
		"EntityA": {
			"CREATE TABLE `db`.`processor0_entity_EntityA` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propertyB` Nullable(Bool) COMMENT 'SCALAR(Boolean) SCHEMA(Boolean)', " +
				"`propertyC` Nullable(Int32) COMMENT 'SCALAR(Int) SCHEMA(Int)', " +
				"`propertyD` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!]!)', " +
				"`propertyE` String COMMENT 'SCALAR(BigDecimal) SCHEMA([[BigDecimal!]!])', " +
				"`propertyF` Nullable(Enum('AAA', 'BBB', 'CCC')) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA)', " +
				"`propertyG` String COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`foreignB` Array(Nullable(String)) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignB) SCHEMA([EntityB])', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCA) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityB) SCHEMA(EntityB)', " +
				"`foreignE` Array(String) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignE) SCHEMA([EntityB!])', " +
				"`foreignF` String COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignF) SCHEMA(EntityB!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityA` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propertyB` Nullable(Bool) COMMENT 'SCALAR(Boolean) SCHEMA(Boolean)', " +
				"`propertyC` Nullable(Int32) COMMENT 'SCALAR(Int) SCHEMA(Int)', " +
				"`propertyD` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!]!)', " +
				"`propertyE` Array(Array(String)) COMMENT 'SCALAR(BigDecimal) SCHEMA([[BigDecimal!]!])', " +
				"`propertyF` Nullable(Enum('AAA', 'BBB', 'CCC')) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA)', " +
				"`propertyG` Array(Nullable(String)) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`foreignB` Array(Nullable(String)) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignB) SCHEMA([EntityB])', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCA) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityB) SCHEMA(EntityB)', " +
				"`foreignE` Array(String) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignE) SCHEMA([EntityB!])', " +
				"`foreignF` String COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignF) SCHEMA(EntityB!)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyD`)) AS `propertyD`, " +
				"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propertyE`)) AS `propertyE`, " +
				"`propertyF`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propertyG`)) AS `propertyG`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`foreignC`, " +
				"`foreignD`, " +
				"`foreignE`, " +
				"`foreignF`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityA`",
			"CREATE VIEW `db`.`processor0_latestView_EntityA` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propertyB` Nullable(Bool) COMMENT 'SCALAR(Boolean) SCHEMA(Boolean)', " +
				"`propertyC` Nullable(Int32) COMMENT 'SCALAR(Int) SCHEMA(Int)', " +
				"`propertyD` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!]!)', " +
				"`propertyE` Array(Array(String)) COMMENT 'SCALAR(BigDecimal) SCHEMA([[BigDecimal!]!])', " +
				"`propertyF` Nullable(Enum('AAA', 'BBB', 'CCC')) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA)', " +
				"`propertyG` Array(Nullable(String)) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`foreignB` Array(Nullable(String)) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignB) SCHEMA([EntityB])', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCA) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityB) SCHEMA(EntityB)', " +
				"`foreignE` Array(String) COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignE) SCHEMA([EntityB!])', " +
				"`foreignF` String COMMENT 'OBJECT(EntityB) DERIVED_FROM(foreignF) SCHEMA(EntityB!)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"`propertyE`, " +
				"`propertyF`, " +
				"`propertyG`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`foreignC`, " +
				"`foreignD`, " +
				"`foreignE`, " +
				"`foreignF`, " +
				"`meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM `db`.`processor0_view_EntityA`",
		},
		"EntityB": {
			"CREATE TABLE `db`.`processor0_versionedEntity_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`foreignB` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCB) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityA) DERIVED_FROM(foreignD) SCHEMA(EntityA)', " +
				"`foreignE` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__foreignE__isnull__` Bool, " +
				"`foreignF` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!]!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__version__`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE TABLE `db`.`processor0_versionedLatestEntity_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`foreignB` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCB) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityA) DERIVED_FROM(foreignD) SCHEMA(EntityA)', " +
				"`foreignE` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__foreignE__isnull__` Bool, " +
				"`foreignF` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!]!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = VersionedCollapsingMergeTree(`__sign__`,`__version__`) " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE MATERIALIZED VIEW `db`.`processor0_versionedLatestEntityMV_EntityB` TO `db`.`processor0_versionedLatestEntity_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`foreignB` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCB) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityA) DERIVED_FROM(foreignD) SCHEMA(EntityA)', " +
				"`foreignE` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__foreignE__isnull__` Bool, " +
				"`foreignF` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!]!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") AS SELECT * FROM `db`.`processor0_versionedEntity_EntityB`",
			"CREATE VIEW `db`.`processor0_entity_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`foreignB` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCB) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityA) DERIVED_FROM(foreignD) SCHEMA(EntityA)', " +
				"`foreignE` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__foreignE__isnull__` Bool, " +
				"`foreignF` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!]!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`foreignB`, " +
				"`foreignC`, " +
				"`foreignD`, " +
				"`foreignE`, " +
				"`__foreignE__isnull__`, " +
				"`foreignF`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_versionedEntity_EntityB` " +
				"WHERE __sign__ > 0",
			"CREATE VIEW `db`.`processor0_view_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`foreignB` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCB) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityA) DERIVED_FROM(foreignD) SCHEMA(EntityA)', " +
				"`foreignE` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__foreignE__isnull__` Bool, " +
				"`foreignF` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!]!)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`foreignB`, " +
				"`foreignC`, " +
				"`foreignD`, " +
				"`foreignE`, " +
				"`__foreignE__isnull__`, " +
				"`foreignF`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityB`",
			"CREATE VIEW `db`.`processor0_latestView_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`foreignB` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignC` Array(Nullable(String)) COMMENT 'OBJECT(EntityC) DERIVED_FROM(foreignCB) SCHEMA([EntityC])', " +
				"`foreignD` Nullable(String) COMMENT 'OBJECT(EntityA) DERIVED_FROM(foreignD) SCHEMA(EntityA)', " +
				"`foreignE` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__foreignE__isnull__` Bool, " +
				"`foreignF` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!]!)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`foreignB`, " +
				"`foreignC`, " +
				"`foreignD`, " +
				"`foreignE`, " +
				"`__foreignE__isnull__`, " +
				"`foreignF`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM (" +
				"SELECT " +
				"`id`, " +
				"`__genBlockChain__`, " +
				"any_respect_nulls(`propertyA`) AS `propertyA`, " +
				"any_respect_nulls(`foreignB`) AS `foreignB`, " +
				"any_respect_nulls(`foreignC`) AS `foreignC`, " +
				"any_respect_nulls(`foreignD`) AS `foreignD`, " +
				"any_respect_nulls(`foreignE`) AS `foreignE`, " +
				"any_respect_nulls(`__foreignE__isnull__`) AS `__foreignE__isnull__`, " +
				"any_respect_nulls(`foreignF`) AS `foreignF` " +
				"FROM `db`.`processor0_versionedLatestEntity_EntityB` " +
				"WHERE NOT `__deleted__` " +
				"GROUP BY `id`, `__genBlockChain__`, `__version__` " +
				"HAVING SUM(`__sign__`) > 0" +
				")",
		},
		"EntityC": {
			"CREATE TABLE `db`.`processor0_versionedEntity_EntityC` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyB` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyC` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propertyD` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`foreignCA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignCB` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__version__`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE TABLE `db`.`processor0_versionedLatestEntity_EntityC` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyB` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyC` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propertyD` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`foreignCA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignCB` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = VersionedCollapsingMergeTree(`__sign__`,`__version__`) " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE MATERIALIZED VIEW `db`.`processor0_versionedLatestEntityMV_EntityC` TO `db`.`processor0_versionedLatestEntity_EntityC` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyB` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyC` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propertyD` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`foreignCA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignCB` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") AS SELECT * FROM `db`.`processor0_versionedEntity_EntityC`",
			"CREATE VIEW `db`.`processor0_entity_EntityC` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyB` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyC` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propertyD` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`foreignCA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignCB` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"`foreignCA`, " +
				"`foreignCB`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_versionedEntity_EntityC` " +
				"WHERE __sign__ > 0",
			"CREATE VIEW `db`.`processor0_view_EntityC` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyB` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyC` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propertyD` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`foreignCA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignCB` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"if(`propertyB`.1,if(`propertyB`.2>=0,toFloat64(`propertyB`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propertyB`.3+1,'UInt256'))),NULL) AS `propertyB`, " +
				"if(`propertyC`.1,if(`propertyC`.2>=0,toFloat64(`propertyC`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propertyC`.3+1,'UInt256'))),NULL) AS `propertyC`, " +
				"`propertyD`, " +
				"`foreignCA`, " +
				"`foreignCB`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityC`",
			"CREATE VIEW `db`.`processor0_latestView_EntityC` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyB` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyC` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propertyD` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`foreignCA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignCB` String COMMENT 'OBJECT(EntityB) SCHEMA(EntityB!)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"if(`propertyB`.1,if(`propertyB`.2>=0,toFloat64(`propertyB`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propertyB`.3+1,'UInt256'))),NULL) AS `propertyB`, " +
				"if(`propertyC`.1,if(`propertyC`.2>=0,toFloat64(`propertyC`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propertyC`.3+1,'UInt256'))),NULL) AS `propertyC`, " +
				"`propertyD`, " +
				"`foreignCA`, " +
				"`foreignCB`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM (" +
				"SELECT " +
				"`id`, " +
				"`__genBlockChain__`, " +
				"any_respect_nulls(`propertyA`) AS `propertyA`, " +
				"any_respect_nulls(`propertyB`) AS `propertyB`, " +
				"any_respect_nulls(`propertyC`) AS `propertyC`, " +
				"any_respect_nulls(`propertyD`) AS `propertyD`, " +
				"any_respect_nulls(`foreignCA`) AS `foreignCA`, " +
				"any_respect_nulls(`foreignCB`) AS `foreignCB` " +
				"FROM `db`.`processor0_versionedLatestEntity_EntityC` " +
				"WHERE NOT `__deleted__` " +
				"GROUP BY `id`, `__genBlockChain__`, `__version__` " +
				"HAVING SUM(`__sign__`) > 0" +
				")",
		},
		"EntityD": {
			"CREATE VIEW `db`.`processor0_interface_EntityD` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__`, " +
				"'EntityD1' AS `__implEntity__` " +
				"FROM `db`.`processor0_entity_EntityD1` UNION ALL SELECT " +
				"`id`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__`, " +
				"'EntityD2' AS `__implEntity__` " +
				"FROM `db`.`processor0_entity_EntityD2`",
			"CREATE VIEW `db`.`processor0_view_EntityD` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`meta.impl_entity` String, " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__`, " +
				"`__implEntity__` AS `meta.impl_entity`, " +
				"`__implEntity__` " +
				"FROM `db`.`processor0_interface_EntityD`",
			"CREATE VIEW `db`.`processor0_latestView_EntityD` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.impl_entity` String, " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`meta.chain`, " +
				"`__genBlockChain__`, " +
				"'EntityD1' AS `meta.impl_entity`, " +
				"'EntityD1' AS `__implEntity__` " +
				"FROM `db`.`processor0_latestView_EntityD1` " +
				"UNION ALL " +
				"SELECT " +
				"`id`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`meta.chain`, " +
				"`__genBlockChain__`, " +
				"'EntityD2' AS `meta.impl_entity`, " +
				"'EntityD2' AS `__implEntity__` " +
				"FROM `db`.`processor0_latestView_EntityD2`",
		},
		"EntityD1": {
			"CREATE TABLE `db`.`processor0_versionedEntity_EntityD1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__version__`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityD) SCHEMA_HASH(xxx)'",
			"CREATE TABLE `db`.`processor0_versionedLatestEntity_EntityD1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = VersionedCollapsingMergeTree(`__sign__`,`__version__`) " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityD) SCHEMA_HASH(xxx)'",
			"CREATE MATERIALIZED VIEW `db`.`processor0_versionedLatestEntityMV_EntityD1` TO `db`.`processor0_versionedLatestEntity_EntityD1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") AS SELECT * FROM `db`.`processor0_versionedEntity_EntityD1`",
			"CREATE VIEW `db`.`processor0_entity_EntityD1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_versionedEntity_EntityD1` " +
				"WHERE __sign__ > 0",
			"CREATE VIEW `db`.`processor0_view_EntityD1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityD1`",
			"CREATE VIEW `db`.`processor0_latestView_EntityD1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM (" +
				"SELECT " +
				"`id`, " +
				"`__genBlockChain__`, " +
				"any_respect_nulls(`propertyA`) AS `propertyA`, " +
				"any_respect_nulls(`on__`) AS `on__`, " +
				"any_respect_nulls(`__on__isnull__`) AS `__on__isnull__` " +
				"FROM `db`.`processor0_versionedLatestEntity_EntityD1` " +
				"WHERE NOT `__deleted__` " +
				"GROUP BY `id`, `__genBlockChain__`, `__version__` " +
				"HAVING SUM(`__sign__`) > 0" +
				")",
		},
		"EntityD2": {
			"CREATE TABLE `db`.`processor0_versionedEntity_EntityD2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__version__`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityD) SCHEMA_HASH(xxx)'",
			"CREATE TABLE `db`.`processor0_versionedLatestEntity_EntityD2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = VersionedCollapsingMergeTree(`__sign__`,`__version__`) " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityD) SCHEMA_HASH(xxx)'",
			"CREATE MATERIALIZED VIEW `db`.`processor0_versionedLatestEntityMV_EntityD2` TO `db`.`processor0_versionedLatestEntity_EntityD2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") AS SELECT * FROM `db`.`processor0_versionedEntity_EntityD2`",
			"CREATE VIEW `db`.`processor0_entity_EntityD2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_versionedEntity_EntityD2` " +
				"WHERE __sign__ > 0",
			"CREATE VIEW `db`.`processor0_view_EntityD2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityD2`",
			"CREATE VIEW `db`.`processor0_latestView_EntityD2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propertyA` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityE) SCHEMA([EntityE!])', " +
				"`__on__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`on__`, " +
				"`__on__isnull__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM (" +
				"SELECT " +
				"`id`, " +
				"`__genBlockChain__`, " +
				"any_respect_nulls(`propertyA`) AS `propertyA`, " +
				"any_respect_nulls(`on__`) AS `on__`, " +
				"any_respect_nulls(`__on__isnull__`) AS `__on__isnull__` " +
				"FROM `db`.`processor0_versionedLatestEntity_EntityD2` " +
				"WHERE NOT `__deleted__` " +
				"GROUP BY `id`, `__genBlockChain__`, `__version__` " +
				"HAVING SUM(`__sign__`) > 0" +
				")",
		},
		"EntityE": {
			"CREATE VIEW `db`.`processor0_interface_EntityE` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__`, " +
				"'EntityE1' AS `__implEntity__` " +
				"FROM `db`.`processor0_entity_EntityE1` " +
				"UNION ALL " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__`, " +
				"'EntityE2' AS `__implEntity__` " +
				"FROM `db`.`processor0_entity_EntityE2`",
			"CREATE VIEW `db`.`processor0_view_EntityE` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`meta.impl_entity` String, " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__`, " +
				"`__implEntity__` AS `meta.impl_entity`, " +
				"`__implEntity__` " +
				"FROM `db`.`processor0_interface_EntityE`",
			"CREATE VIEW `db`.`processor0_latestView_EntityE` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.impl_entity` String, " +
				"`__implEntity__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`meta.chain`, " +
				"`__genBlockChain__`, " +
				"'EntityE1' AS `meta.impl_entity`, " +
				"'EntityE1' AS `__implEntity__` " +
				"FROM `db`.`processor0_latestView_EntityE1` " +
				"UNION ALL " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`meta.chain`, " +
				"`__genBlockChain__`, " +
				"'EntityE2' AS `meta.impl_entity`, " +
				"'EntityE2' AS `__implEntity__` " +
				"FROM `db`.`processor0_latestView_EntityE2`",
		},
		"EntityE1": {
			"CREATE TABLE `db`.`processor0_versionedEntity_EntityE1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__version__`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityE) SCHEMA_HASH(xxx)'",
			"CREATE TABLE `db`.`processor0_versionedLatestEntity_EntityE1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = VersionedCollapsingMergeTree(`__sign__`,`__version__`) " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityE) SCHEMA_HASH(xxx)'",
			"CREATE MATERIALIZED VIEW `db`.`processor0_versionedLatestEntityMV_EntityE1` TO `db`.`processor0_versionedLatestEntity_EntityE1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") AS SELECT * FROM `db`.`processor0_versionedEntity_EntityE1`",
			"CREATE VIEW `db`.`processor0_entity_EntityE1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_versionedEntity_EntityE1` " +
				"WHERE __sign__ > 0",
			"CREATE VIEW `db`.`processor0_view_EntityE1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityE1`",
			"CREATE VIEW `db`.`processor0_latestView_EntityE1` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`on__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM (" +
				"SELECT " +
				"`id`, " +
				"`__genBlockChain__`, " +
				"any_respect_nulls(`from__`) AS `from__`, " +
				"any_respect_nulls(`on__`) AS `on__` " +
				"FROM `db`.`processor0_versionedLatestEntity_EntityE1` " +
				"WHERE NOT `__deleted__` " +
				"GROUP BY `id`, `__genBlockChain__`, `__version__` " +
				"HAVING SUM(`__sign__`) > 0" +
				")",
		},
		"EntityE2": {
			"CREATE TABLE `db`.`processor0_versionedEntity_EntityE2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`by__` String COMMENT 'SCALAR(Int) SCHEMA([Int])', " +
				"`left__` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__version__`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityE) SCHEMA_HASH(xxx)'",
			"CREATE TABLE `db`.`processor0_versionedLatestEntity_EntityE2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`by__` String COMMENT 'SCALAR(Int) SCHEMA([Int])', " +
				"`left__` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = VersionedCollapsingMergeTree(`__sign__`,`__version__`) " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'IMPL(EntityE) SCHEMA_HASH(xxx)'",
			"CREATE MATERIALIZED VIEW `db`.`processor0_versionedLatestEntityMV_EntityE2` TO `db`.`processor0_versionedLatestEntity_EntityE2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`by__` String COMMENT 'SCALAR(Int) SCHEMA([Int])', " +
				"`left__` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") AS SELECT * FROM `db`.`processor0_versionedEntity_EntityE2`",
			"CREATE VIEW `db`.`processor0_entity_EntityE2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`by__` String COMMENT 'SCALAR(Int) SCHEMA([Int])', " +
				"`left__` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"`by__`, " +
				"`left__`, " +
				"`on__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_versionedEntity_EntityE2` " +
				"WHERE __sign__ > 0",
			"CREATE VIEW `db`.`processor0_view_EntityE2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`by__` Array(Nullable(Int32)) COMMENT 'SCALAR(Int) SCHEMA([Int])', " +
				"`left__` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, toInt32(x0)), JSONExtractArrayRaw(`by__`)) AS `by__`, " +
				"if(`left__`.1,if(`left__`.2>=0,toFloat64(`left__`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`left__`.3+1,'UInt256'))),NULL) AS `left__`, " +
				"`on__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityE2`",
			"CREATE VIEW `db`.`processor0_latestView_EntityE2` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`from__` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`by__` Array(Nullable(Int32)) COMMENT 'SCALAR(Int) SCHEMA([Int])', " +
				"`left__` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`on__` Array(String) COMMENT 'INTERFACE(EntityD) DERIVED_FROM(on) SCHEMA([EntityD!])', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`from__`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, toInt32(x0)), JSONExtractArrayRaw(`by__`)) AS `by__`, " +
				"if(`left__`.1,if(`left__`.2>=0,toFloat64(`left__`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`left__`.3+1,'UInt256'))),NULL) AS `left__`, " +
				"`on__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM (" +
				"SELECT " +
				"`id`, " +
				"`__genBlockChain__`, " +
				"any_respect_nulls(`from__`) AS `from__`, " +
				"any_respect_nulls(`by__`) AS `by__`, " +
				"any_respect_nulls(`left__`) AS `left__`, " +
				"any_respect_nulls(`on__`) AS `on__` " +
				"FROM `db`.`processor0_versionedLatestEntity_EntityE2` " +
				"WHERE NOT `__deleted__` " +
				"GROUP BY `id`, `__genBlockChain__`, `__version__` " +
				"HAVING SUM(`__sign__`) > 0" +
				")",
		},
		"EntityF1": {
			"CREATE TABLE `db`.`processor0_entity_EntityF1` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` String COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` String COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` String COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` String COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` String COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` String COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` String COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"INDEX `idx_propertyA` `propertyA` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_propertyB` `propertyB` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_propertyC` `propertyC` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyD` `propertyD` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyE` `propertyE` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyF` `propertyF` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyG` `propertyG` TYPE set(0) GRANULARITY 1, " +
				"INDEX `idx_propertyO` `propertyO` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyQ` `propertyQ` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_foreignA` `foreignA` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_foreignB` `foreignB` TYPE bloom_filter GRANULARITY 1" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityF1` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` Array(String) COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` Array(String) COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` Array(Bool) COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` Array(Int32) COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` Array(String) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` Array(String) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` Array(DateTime64(6)) COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` Array(Float64) COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"if(`propertyE`.1,if(`propertyE`.2>=0,toFloat64(`propertyE`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propertyE`.3+1,'UInt256'))),NULL) AS `propertyE`, " +
				"`propertyF`, " +
				"`propertyG`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyH`)) AS `propertyH`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyI`)) AS `propertyI`, " +
				"arrayMap(x0 -> JSONExtractBool(x0), JSONExtractArrayRaw(`propertyJ`)) AS `propertyJ`, " +
				"arrayMap(x0 -> toInt32(x0), JSONExtractArrayRaw(`propertyK`)) AS `propertyK`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyL`)) AS `propertyL`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyM`)) AS `propertyM`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyN`)) AS `propertyN`, " +
				"`propertyO`, " +
				"arrayMap(x0 -> toDateTime64(JSONExtractInt(x0)/1000000,6), JSONExtractArrayRaw(`propertyP`)) AS `propertyP`, " +
				"`propertyQ`, " +
				"arrayMap(x0 -> JSONExtractFloat(x0), JSONExtractArrayRaw(`propertyR`)) AS `propertyR`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`__foreignB__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityF1`",
			"CREATE VIEW `db`.`processor0_latestView_EntityF1` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` Array(String) COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` Array(String) COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` Array(Bool) COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` Array(Int32) COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` Array(String) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` Array(String) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` Array(DateTime64(6)) COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` Array(Float64) COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"`propertyE`, " +
				"`propertyF`, " +
				"`propertyG`, " +
				"`propertyH`, " +
				"`propertyI`, " +
				"`propertyJ`, " +
				"`propertyK`, " +
				"`propertyL`, " +
				"`propertyM`, " +
				"`propertyN`, " +
				"`propertyO`, " +
				"`propertyP`, " +
				"`propertyQ`, " +
				"`propertyR`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`__foreignB__isnull__`, " +
				"`meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM `db`.`processor0_view_EntityF1`",
		},
		"EntityF2": {
			"CREATE TABLE `db`.`processor0_versionedEntity_EntityF2` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` String COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` String COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` String COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` String COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` String COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` String COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` String COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64, " +
				"INDEX `idx_propertyA` `propertyA` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_propertyB` `propertyB` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_propertyC` `propertyC` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyD` `propertyD` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyE` `propertyE` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyF` `propertyF` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyG` `propertyG` TYPE set(0) GRANULARITY 1, " +
				"INDEX `idx_propertyO` `propertyO` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyQ` `propertyQ` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_foreignA` `foreignA` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_foreignB` `foreignB` TYPE bloom_filter GRANULARITY 1" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__version__`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE TABLE `db`.`processor0_versionedLatestEntity_EntityF2` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` String COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` String COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` String COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` String COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` String COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` String COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` String COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64, " +
				"INDEX `idx_propertyA` `propertyA` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_propertyB` `propertyB` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_propertyC` `propertyC` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyD` `propertyD` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyE` `propertyE` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyF` `propertyF` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyG` `propertyG` TYPE set(0) GRANULARITY 1, " +
				"INDEX `idx_propertyO` `propertyO` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_propertyQ` `propertyQ` TYPE minmax GRANULARITY 1, " +
				"INDEX `idx_foreignA` `foreignA` TYPE bloom_filter GRANULARITY 1, " +
				"INDEX `idx_foreignB` `foreignB` TYPE bloom_filter GRANULARITY 1" +
				") " +
				"ENGINE = VersionedCollapsingMergeTree(`__sign__`,`__version__`) " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE MATERIALIZED VIEW `db`.`processor0_versionedLatestEntityMV_EntityF2` TO `db`.`processor0_versionedLatestEntity_EntityF2` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` String COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` String COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` String COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` String COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` String COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` String COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` String COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") AS SELECT * FROM `db`.`processor0_versionedEntity_EntityF2`",
			"CREATE VIEW `db`.`processor0_entity_EntityF2` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` String COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` String COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` String COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` String COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` String COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` String COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` String COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"`propertyE`, " +
				"`propertyF`, " +
				"`propertyG`, " +
				"`propertyH`, " +
				"`propertyI`, " +
				"`propertyJ`, " +
				"`propertyK`, " +
				"`propertyL`, " +
				"`propertyM`, " +
				"`propertyN`, " +
				"`propertyO`, " +
				"`propertyP`, " +
				"`propertyQ`, " +
				"`propertyR`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`__foreignB__isnull__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_versionedEntity_EntityF2` " +
				"WHERE __sign__ > 0",
			"CREATE VIEW `db`.`processor0_view_EntityF2` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` Array(String) COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` Array(String) COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` Array(Bool) COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` Array(Int32) COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` Array(String) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` Array(String) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` Array(DateTime64(6)) COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` Array(Float64) COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"if(`propertyE`.1,if(`propertyE`.2>=0,toFloat64(`propertyE`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propertyE`.3+1,'UInt256'))),NULL) AS `propertyE`, " +
				"`propertyF`, " +
				"`propertyG`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyH`)) AS `propertyH`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyI`)) AS `propertyI`, " +
				"arrayMap(x0 -> JSONExtractBool(x0), JSONExtractArrayRaw(`propertyJ`)) AS `propertyJ`, " +
				"arrayMap(x0 -> toInt32(x0), JSONExtractArrayRaw(`propertyK`)) AS `propertyK`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyL`)) AS `propertyL`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyM`)) AS `propertyM`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyN`)) AS `propertyN`, " +
				"`propertyO`, " +
				"arrayMap(x0 -> toDateTime64(JSONExtractInt(x0)/1000000,6), JSONExtractArrayRaw(`propertyP`)) AS `propertyP`, " +
				"`propertyQ`, " +
				"arrayMap(x0 -> JSONExtractFloat(x0), JSONExtractArrayRaw(`propertyR`)) AS `propertyR`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`__foreignB__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityF2`",
			"CREATE VIEW `db`.`processor0_latestView_EntityF2` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyA` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`propertyB` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`propertyC` Bool COMMENT 'SCALAR(Boolean) SCHEMA(Boolean!)', " +
				"`propertyD` Int32 COMMENT 'SCALAR(Int) SCHEMA(Int!)', " +
				"`propertyE` Float64 COMMENT 'SCALAR(BigInt) SCHEMA(BigInt!)', " +
				"`propertyF` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propertyG` Enum('AAA', 'BBB', 'CCC') COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA(EnumA!)', " +
				"`propertyH` Array(String) COMMENT 'SCALAR(Bytes) SCHEMA([Bytes!])', " +
				"`propertyI` Array(String) COMMENT 'SCALAR(String) SCHEMA([String!])', " +
				"`propertyJ` Array(Bool) COMMENT 'SCALAR(Boolean) SCHEMA([Boolean!])', " +
				"`propertyK` Array(Int32) COMMENT 'SCALAR(Int) SCHEMA([Int!])', " +
				"`propertyL` Array(String) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt!])', " +
				"`propertyM` Array(String) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal!])', " +
				"`propertyN` Array(String) COMMENT 'ENUM(EnumA(AAA,BBB,CCC)) SCHEMA([EnumA!])', " +
				"`propertyO` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`propertyP` Array(DateTime64(6)) COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!])', " +
				"`propertyQ` Float64 COMMENT 'SCALAR(Float) SCHEMA(Float!)', " +
				"`propertyR` Array(Float64) COMMENT 'SCALAR(Float) SCHEMA([Float!])', " +
				"`foreignA` String COMMENT 'OBJECT(EntityA) SCHEMA(EntityA!)', " +
				"`foreignB` Array(String) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA!])', " +
				"`__foreignB__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propertyA`, " +
				"`propertyB`, " +
				"`propertyC`, " +
				"`propertyD`, " +
				"if(`propertyE`.1,if(`propertyE`.2>=0,toFloat64(`propertyE`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propertyE`.3+1,'UInt256'))),NULL) AS `propertyE`, " +
				"`propertyF`, " +
				"`propertyG`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyH`)) AS `propertyH`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyI`)) AS `propertyI`, " +
				"arrayMap(x0 -> JSONExtractBool(x0), JSONExtractArrayRaw(`propertyJ`)) AS `propertyJ`, " +
				"arrayMap(x0 -> toInt32(x0), JSONExtractArrayRaw(`propertyK`)) AS `propertyK`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyL`)) AS `propertyL`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyM`)) AS `propertyM`, " +
				"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propertyN`)) AS `propertyN`, " +
				"`propertyO`, " +
				"arrayMap(x0 -> toDateTime64(JSONExtractInt(x0)/1000000,6), JSONExtractArrayRaw(`propertyP`)) AS `propertyP`, " +
				"`propertyQ`, " +
				"arrayMap(x0 -> JSONExtractFloat(x0), JSONExtractArrayRaw(`propertyR`)) AS `propertyR`, " +
				"`foreignA`, " +
				"`foreignB`, " +
				"`__foreignB__isnull__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM (" +
				"SELECT " +
				"`id`, " +
				"`__genBlockChain__`, " +
				"any_respect_nulls(`propertyA`) AS `propertyA`, " +
				"any_respect_nulls(`propertyB`) AS `propertyB`, " +
				"any_respect_nulls(`propertyC`) AS `propertyC`, " +
				"any_respect_nulls(`propertyD`) AS `propertyD`, " +
				"any_respect_nulls(`propertyE`) AS `propertyE`, " +
				"any_respect_nulls(`propertyF`) AS `propertyF`, " +
				"any_respect_nulls(`propertyG`) AS `propertyG`, " +
				"any_respect_nulls(`propertyH`) AS `propertyH`, " +
				"any_respect_nulls(`propertyI`) AS `propertyI`, " +
				"any_respect_nulls(`propertyJ`) AS `propertyJ`, " +
				"any_respect_nulls(`propertyK`) AS `propertyK`, " +
				"any_respect_nulls(`propertyL`) AS `propertyL`, " +
				"any_respect_nulls(`propertyM`) AS `propertyM`, " +
				"any_respect_nulls(`propertyN`) AS `propertyN`, " +
				"any_respect_nulls(`propertyO`) AS `propertyO`, " +
				"any_respect_nulls(`propertyP`) AS `propertyP`, " +
				"any_respect_nulls(`propertyQ`) AS `propertyQ`, " +
				"any_respect_nulls(`propertyR`) AS `propertyR`, " +
				"any_respect_nulls(`foreignA`) AS `foreignA`, " +
				"any_respect_nulls(`foreignB`) AS `foreignB`, " +
				"any_respect_nulls(`__foreignB__isnull__`) AS `__foreignB__isnull__` " +
				"FROM `db`.`processor0_versionedLatestEntity_EntityF2` " +
				"WHERE NOT `__deleted__` " +
				"GROUP BY `id`, `__genBlockChain__`, `__version__` " +
				"HAVING SUM(`__sign__`) > 0" +
				")",
		},
		"EntityG": {
			"CREATE TABLE `db`.`processor0_versionedEntity_EntityG` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA1` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propB1` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propC1` Nullable(Decimal256(30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)', " +
				"`propA2` String COMMENT 'SCALAR(String) SCHEMA([String])', " +
				"`propB2` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt])', " +
				"`propC2` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal])', " +
				"`forkA1` Nullable(String) COMMENT 'OBJECT(EntityA) SCHEMA(EntityA)', " +
				"`forkA2` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__forkA2__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__version__`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE TABLE `db`.`processor0_versionedLatestEntity_EntityG` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA1` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propB1` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propC1` Nullable(Decimal256(30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)', " +
				"`propA2` String COMMENT 'SCALAR(String) SCHEMA([String])', " +
				"`propB2` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt])', " +
				"`propC2` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal])', " +
				"`forkA1` Nullable(String) COMMENT 'OBJECT(EntityA) SCHEMA(EntityA)', " +
				"`forkA2` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__forkA2__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = VersionedCollapsingMergeTree(`__sign__`,`__version__`) " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE MATERIALIZED VIEW `db`.`processor0_versionedLatestEntityMV_EntityG` TO `db`.`processor0_versionedLatestEntity_EntityG` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA1` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propB1` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propC1` Nullable(Decimal256(30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)', " +
				"`propA2` String COMMENT 'SCALAR(String) SCHEMA([String])', " +
				"`propB2` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt])', " +
				"`propC2` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal])', " +
				"`forkA1` Nullable(String) COMMENT 'OBJECT(EntityA) SCHEMA(EntityA)', " +
				"`forkA2` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__forkA2__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") AS SELECT * FROM `db`.`processor0_versionedEntity_EntityG`",
			"CREATE VIEW `db`.`processor0_entity_EntityG` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA1` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propB1` Tuple(has Bool,sign Int8,val UInt256) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propC1` Nullable(Decimal256(30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)', " +
				"`propA2` String COMMENT 'SCALAR(String) SCHEMA([String])', " +
				"`propB2` String COMMENT 'SCALAR(BigInt) SCHEMA([BigInt])', " +
				"`propC2` String COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal])', " +
				"`forkA1` Nullable(String) COMMENT 'OBJECT(EntityA) SCHEMA(EntityA)', " +
				"`forkA2` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__forkA2__isnull__` Bool, " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propA1`, " +
				"`propB1`, " +
				"`propC1`, " +
				"`propA2`, " +
				"`propB2`, " +
				"`propC2`, " +
				"`forkA1`, " +
				"`forkA2`, " +
				"`__forkA2__isnull__`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_versionedEntity_EntityG` " +
				"WHERE __sign__ > 0",
			"CREATE VIEW `db`.`processor0_view_EntityG` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA1` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propB1` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propC1` Nullable(Decimal256(30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)', " +
				"`propA2` Array(Nullable(String)) COMMENT 'SCALAR(String) SCHEMA([String])', " +
				"`propB2` Array(Nullable(String)) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt])', " +
				"`propC2` Array(Nullable(String)) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal])', " +
				"`forkA1` Nullable(String) COMMENT 'OBJECT(EntityA) SCHEMA(EntityA)', " +
				"`forkA2` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__forkA2__isnull__` Bool, " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propA1`, " +
				"if(`propB1`.1,if(`propB1`.2>=0,toFloat64(`propB1`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propB1`.3+1,'UInt256'))),NULL) AS `propB1`, " +
				"`propC1`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propA2`)) AS `propA2`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propB2`)) AS `propB2`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propC2`)) AS `propC2`, " +
				"`forkA1`, " +
				"`forkA2`, " +
				"`__forkA2__isnull__`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityG`",
			"CREATE VIEW `db`.`processor0_latestView_EntityG` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA1` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propB1` Nullable(Float64) COMMENT 'SCALAR(BigInt) SCHEMA(BigInt)', " +
				"`propC1` Nullable(Decimal256(30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)', " +
				"`propA2` Array(Nullable(String)) COMMENT 'SCALAR(String) SCHEMA([String])', " +
				"`propB2` Array(Nullable(String)) COMMENT 'SCALAR(BigInt) SCHEMA([BigInt])', " +
				"`propC2` Array(Nullable(String)) COMMENT 'SCALAR(BigDecimal) SCHEMA([BigDecimal])', " +
				"`forkA1` Nullable(String) COMMENT 'OBJECT(EntityA) SCHEMA(EntityA)', " +
				"`forkA2` Array(Nullable(String)) COMMENT 'OBJECT(EntityA) SCHEMA([EntityA])', " +
				"`__forkA2__isnull__` Bool, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propA1`, " +
				"if(`propB1`.1,if(`propB1`.2>=0,toFloat64(`propB1`.3),-toFloat64(CAST(CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')-`propB1`.3+1,'UInt256'))),NULL) AS `propB1`, " +
				"`propC1`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propA2`)) AS `propA2`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propB2`)) AS `propB2`, " +
				"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propC2`)) AS `propC2`, " +
				"`forkA1`, " +
				"`forkA2`, " +
				"`__forkA2__isnull__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM (" +
				"SELECT " +
				"`id`, " +
				"`__genBlockChain__`, " +
				"any_respect_nulls(`propA1`) AS `propA1`, " +
				"any_respect_nulls(`propB1`) AS `propB1`, " +
				"any_respect_nulls(`propC1`) AS `propC1`, " +
				"any_respect_nulls(`propA2`) AS `propA2`, " +
				"any_respect_nulls(`propB2`) AS `propB2`, " +
				"any_respect_nulls(`propC2`) AS `propC2`, " +
				"any_respect_nulls(`forkA1`) AS `forkA1`, " +
				"any_respect_nulls(`forkA2`) AS `forkA2`, " +
				"any_respect_nulls(`__forkA2__isnull__`) AS `__forkA2__isnull__` " +
				"FROM `db`.`processor0_versionedLatestEntity_EntityG` " +
				"WHERE NOT `__deleted__` " +
				"GROUP BY `id`, `__genBlockChain__`, `__version__` " +
				"HAVING SUM(`__sign__`) > 0" +
				")",
		},
		"EntityH": {
			"CREATE TABLE `db`.`processor0_entity_EntityH` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityH` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`timestamp`, " +
				"`dimA`, " +
				"`propA`, " +
				"`propB`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityH`",
			"CREATE VIEW `db`.`processor0_latestView_EntityH` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`propA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`propB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`timestamp`, " +
				"`dimA`, " +
				"`propA`, " +
				"`propB`, " +
				"`meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM `db`.`processor0_view_EntityH`",
		},
		"AggA": {
			"CREATE TABLE `db`.`processor0_aggregation_AggA` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`aggA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG(propA)', " +
				"`aggB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG((propA + propB) / 2)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__interval__` Enum('hour', 'day') COMMENT 'ENUM(AggAInterval(hour,day)) SCHEMA(AggAInterval!)'" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`__interval__`,`timestamp`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SRC(EntityH) SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_AggA` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`aggA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG(propA)', " +
				"`aggB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG((propA + propB) / 2)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`meta.aggregation_interval` Enum('hour', 'day') COMMENT 'ENUM(AggAInterval(hour,day)) SCHEMA(AggAInterval!)', " +
				"`__interval__` Enum('hour', 'day') COMMENT 'ENUM(AggAInterval(hour,day)) SCHEMA(AggAInterval!)'" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`timestamp`, " +
				"`dimA`, " +
				"`aggA`, " +
				"`aggB`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__`, " +
				"`__interval__` AS `meta.aggregation_interval`, " +
				"`__interval__` " +
				"FROM `db`.`processor0_aggregation_AggA`",
			"CREATE VIEW `db`.`processor0_latestView_AggA` (" +
				"`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)', " +
				"`timestamp` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)', " +
				"`dimA` Nullable(String) COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`aggA` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG(propA)', " +
				"`aggB` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!) AGG_FN(sum) AGG_ARG((propA + propB) / 2)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.aggregation_interval` Enum('hour', 'day') COMMENT 'ENUM(AggAInterval(hour,day)) SCHEMA(AggAInterval!)', " +
				"`__interval__` Enum('hour', 'day') COMMENT 'ENUM(AggAInterval(hour,day)) SCHEMA(AggAInterval!)'" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`timestamp`, " +
				"`dimA`, " +
				"`aggA`, " +
				"`aggB`, " +
				"`meta.chain`, " +
				"`__genBlockChain__`, " +
				"`meta.aggregation_interval`, " +
				"`__interval__` " +
				"FROM `db`.`processor0_view_AggA`",
		},
		"EntityI": {
			"CREATE TABLE `db`.`processor0_versionedEntity_EntityI` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA` json COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__version__`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE TABLE `db`.`processor0_versionedLatestEntity_EntityI` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA` json COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now(), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") " +
				"ENGINE = VersionedCollapsingMergeTree(`__sign__`,`__version__`) " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE MATERIALIZED VIEW `db`.`processor0_versionedLatestEntityMV_EntityI` TO `db`.`processor0_versionedLatestEntity_EntityI` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA` json COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC'), " +
				"`__sign__` Int8, " +
				"`__version__` UInt64" +
				") AS SELECT * FROM `db`.`processor0_versionedEntity_EntityI`",
			"CREATE VIEW `db`.`processor0_entity_EntityI` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA` json COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propA`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__`, " +
				"`__deleted__`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_versionedEntity_EntityI` " +
				"WHERE __sign__ > 0",
			"CREATE VIEW `db`.`processor0_view_EntityI` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA` json COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propA`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityI`",
			"CREATE VIEW `db`.`processor0_latestView_EntityI` (" +
				"`id` String COMMENT 'SCALAR(ID) SCHEMA(ID!)', " +
				"`propA` json COMMENT 'SCALAR(String) SCHEMA(String)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`propA`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM (" +
				"SELECT " +
				"`id`, " +
				"`__genBlockChain__`, " +
				"any_respect_nulls(`propA`) AS `propA` " +
				"FROM `db`.`processor0_versionedLatestEntity_EntityI` " +
				"WHERE NOT `__deleted__` " +
				"GROUP BY `id`, `__genBlockChain__`, `__version__` " +
				"HAVING SUM(`__sign__`) > 0" +
				")",
		},
	}
	assert.Equal(t, expected, sqlMap)
}

func Test_createTableSQL2(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityA @entity(immutable: true) {
  id: Bytes!
}
type EntityB @entity(immutable: false) {
  id: String!
}
`)
	assert.NoError(t, err)

	ctrl := chx.NewController(nil, "")
	s := Store{
		ctrl:        ctrl,
		database:    "db",
		processorID: "processor0",
		sch:         sch,
		schHash:     "xxx",
		tableOpt:    DefaultCreateTableOption,
	}

	sqlMap := make(map[string][]string)
	for entityName, tvs := range s.buildTablesAndViews(false) {
		for _, tv := range tvs {
			sqlMap[entityName] = append(sqlMap[entityName], ctrl.BuildCreateSQL(tv))
		}
	}
	//printSQLMap(sqlMap)
	expected := map[string][]string{
		"EntityA": {
			"CREATE TABLE `db`.`processor0_entity_EntityA` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityA` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityA`",
			"CREATE VIEW `db`.`processor0_latestView_EntityA` (" +
				"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`meta.chain`, " +
				"`__genBlockChain__` " +
				"FROM `db`.`processor0_view_EntityA`",
		},
		"EntityB": {
			"CREATE TABLE `db`.`processor0_entity_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`__genBlockNumber__` UInt64, " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`__genBlockHash__` String, " +
				"`__genBlockChain__` String, " +
				"`__deleted__` Bool, " +
				"`__timestamp__` DateTime64(3, 'UTC') DEFAULT now()" +
				") " +
				"ENGINE = MergeTree() " +
				"PARTITION BY __genBlockChain__ " +
				"ORDER BY (`__genBlockChain__`,`id`,`__genBlockNumber__`) " +
				"SETTINGS enable_block_number_column=1,enable_block_offset_column=1,index_granularity=8192 " +
				"COMMENT 'SCHEMA_HASH(xxx)'",
			"CREATE VIEW `db`.`processor0_view_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`meta.block_number` UInt64, " +
				"`__genBlockNumber__` UInt64, " +
				"`meta.block_time` DateTime64(6, 'UTC'), " +
				"`__genBlockTime__` DateTime64(6, 'UTC'), " +
				"`meta.block_hash` String, " +
				"`__genBlockHash__` String, " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String, " +
				"`meta.deleted` Bool, " +
				"`__deleted__` Bool, " +
				"`meta.timestamp` DateTime64(3, 'UTC'), " +
				"`__timestamp__` DateTime64(3, 'UTC')" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`__genBlockNumber__` AS `meta.block_number`, " +
				"`__genBlockNumber__`, " +
				"`__genBlockTime__` AS `meta.block_time`, " +
				"`__genBlockTime__`, " +
				"`__genBlockHash__` AS `meta.block_hash`, " +
				"`__genBlockHash__`, " +
				"`__genBlockChain__` AS `meta.chain`, " +
				"`__genBlockChain__`, " +
				"`__deleted__` AS `meta.deleted`, " +
				"`__deleted__`, " +
				"`__timestamp__` AS `meta.timestamp`, " +
				"`__timestamp__` " +
				"FROM `db`.`processor0_entity_EntityB`",
			"CREATE VIEW `db`.`processor0_latestView_EntityB` (" +
				"`id` String COMMENT 'SCALAR(String) SCHEMA(String!)', " +
				"`meta.chain` String, " +
				"`__genBlockChain__` String" +
				") AS " +
				"SELECT " +
				"`id`, " +
				"`meta.chain`, " +
				"`meta.chain` AS `__genBlockChain__` " +
				"FROM (" +
				"SELECT `id`, `meta.chain`, MAX((" +
				"`meta.block_number`, " +
				"`meta.deleted`" +
				")) AS __last__ " +
				"FROM `db`.`processor0_view_EntityB` " +
				"GROUP BY `id`, `meta.chain`" +
				") " +
				"WHERE NOT __last__.2",
		},
	}
	assert.Equal(t, expected, sqlMap)
}
