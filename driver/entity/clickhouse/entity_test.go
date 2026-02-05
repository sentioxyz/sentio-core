package clickhouse

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/chx"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
)

const (
	localClickhouseDSN = "clickhouse://default:password@127.0.0.1:9000/lzxtestdb"
	testClickhouseDB   = "lzxtestdb"
	skip               = true
	//skip = false
)

func Test_usingTempTable(t *testing.T) {
	if skip {
		t.Skip("test filter condition with too many elements")
	}

	ctx := context.Background()
	options, err := clickhouse.ParseDSN(localClickhouseDSN)
	assert.NoError(t, err)
	db, err := clickhouse.Open(options)
	assert.NoError(t, err)

	assert.NoError(t, db.Exec(ctx, "DROP TABLE IF EXISTS book"))
	assert.NoError(t, db.Exec(ctx, "CREATE TABLE `book` (id String, name String) ENGINE = MergeTree() ORDER BY id"))
	assert.NoError(t, db.Exec(ctx, "INSERT INTO `book` (id, name) VALUES ('b0', 'book0'), ('b1', 'book1')"))

	query := func(ctx context.Context, ids []string) []string {
		err = db.Exec(ctx, "CREATE TEMPORARY TABLE ids (id String) ENGINE = Memory")
		assert.NoError(t, err)
		for s := 0; s < len(ids); s += 1000 {
			n := min(1000, len(ids)-s)
			b := make([]any, n)
			for k := 0; k < n; k++ {
				b[k] = ids[s+k]
			}
			assert.NoError(t, db.Exec(ctx, fmt.Sprintf("INSERT INTO ids (id) VALUES %s", utils.Dup("(?)", ",", n)), b...))
		}
		sql := "select id from book where id not in (select id from ids) order by id"
		args := make([]any, 0)

		//sql := fmt.Sprintf("select id from book where id not in (%s)", utils.Dup("?", ",", num))
		//args := utils.ToAnyArray(ids)

		//sql := "select id from book where id not in ?"
		//args := []any{ids}

		rows, err := db.Query(ctx, sql, args...)
		assert.NoError(t, err)
		var result []string
		for rows.Next() {
			var id string
			assert.NoError(t, rows.Scan(&id))
			result = append(result, id)
		}
		assert.NoError(t, db.Exec(ctx, "DROP TEMPORARY TABLE ids"))
		return result
	}

	const num = 9000
	ids := make([]string, num)
	for i := 1; i < num; i++ {
		ids[i] = fmt.Sprintf("0x04D1ce989ed91cA507Ac5b71A484303cEb80%04d", i)
	}

	ids[0] = "b0"
	assert.Equal(t, []string{"b1"}, query(context.Background(), ids))
	ids[0] = "b1"
	assert.Equal(t, []string{"b0"}, query(context.Background(), ids))
	ids[0] = "b2"
	assert.Equal(t, []string{"b0", "b1"}, query(context.Background(), ids))

	assert.NoError(t, db.Exec(ctx, "DROP TABLE IF EXISTS book"))
}

type EntitySuite struct {
	conn chx.Conn
	ctrl chx.Controller
	s    *Store
	t    *testing.T
}

func (es *EntitySuite) init(ctx context.Context) {
	es.conn = ckhmanager.NewConn(localClickhouseDSN)
	es.ctrl = chx.NewController(es.conn)

	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(es.t, err)

	processorID := "processor0"
	es.s = NewStore(
		es.ctrl,
		processorID,
		Features{
			VersionedCollapsing:    true,
			TimestampUseDateTime64: true,
			BigDecimalUseString:    true,
		},
		sch,
		TableOption{
			BatchInsertSizeLimit: 2,
			HugeIDSetSize:        3,
			TableSettings:        DefaultCreateTableOption.TableSettings,
		},
	)

	assert.NoError(es.t, es.ctrl.DropAll(context.Background(), testClickhouseDB, processorID+"\\_%"))
	assert.NoError(es.t, es.s.InitEntitySchema(ctx))
}

func genBlockTime(bn uint64) time.Time {
	return time.UnixMicro(1000000000123456 + int64(bn)*1000000).UTC()
}

func (es *EntitySuite) getEntityTypeFromID(id string) *schema.Entity {
	sch := es.s.sch
	switch {
	case strings.HasPrefix(id, "0x0a"):
		return sch.GetEntity("EntityA")
	case strings.HasPrefix(id, "0x0b"):
		return sch.GetEntity("EntityB")
	case strings.HasPrefix(id, "0x0c"):
		return sch.GetEntity("EntityC")
	case strings.HasPrefix(id, "0x0d01"):
		return sch.GetEntity("EntityD1")
	case strings.HasPrefix(id, "0x0d02"):
		return sch.GetEntity("EntityD2")
	case strings.HasPrefix(id, "0x0e01"):
		return sch.GetEntity("EntityE1")
	case strings.HasPrefix(id, "0x0e02"):
		return sch.GetEntity("EntityE2")
	}
	return nil
}

func (es *EntitySuite) pushData(ctx context.Context, entities map[string]persistent.EntityBox) {
	var err error
	var data *persistent.EntityBox

	// set
	for id, entity := range entities {
		_, err = es.s.SetEntities(ctx, es.getEntityTypeFromID(id), []persistent.EntityBox{entity})
		assert.NoError(es.t, err)
	}

	// get
	for id, entity := range entities {
		data, err = es.s.GetEntity(ctx, es.getEntityTypeFromID(id), entity.GenBlockChain, id)
		assert.NoError(es.t, err)
		assert.Equal(es.t, entity, *data)
	}

	data, err = es.s.GetEntity(ctx, es.s.sch.GetEntity("EntityA"), "", "a-id-2")
	assert.NoError(es.t, err)
	assert.Nil(es.t, data)
	data, err = es.s.GetEntity(ctx, es.s.sch.GetEntity("EntityB"), "", "b-id-2")
	assert.NoError(es.t, err)
	assert.Nil(es.t, data)
	data, err = es.s.GetEntity(ctx, es.s.sch.GetEntity("EntityC"), "", "c-id-10")
	assert.NoError(es.t, err)
	assert.Nil(es.t, data)
}

func Test_syncTables(t *testing.T) {
	if skip {
		t.Skip("use local db, will only be executed manually locally")
	}

	ctx := context.Background()
	es := EntitySuite{t: t}
	es.init(ctx)

	es.s.schHash = "xxx1"

	assert.NoError(t, es.s.InitEntitySchema(ctx))
}

func Test_getSetDel(t *testing.T) {
	if skip {
		t.Skip("use local db, will only be executed manually locally")
	}

	const chain = "1"
	const skipDelete = false
	//const skipDelete = true

	ctx := context.Background()
	es := EntitySuite{t: t}
	es.init(ctx)
	sch, s := es.s.sch, es.s

	maxBigInt := new(big.Int).Sub(num2e256, big.NewInt(1))

	entities := map[string]persistent.EntityBox{
		"0x0a00": {
			ID: "0x0a00",
			Data: map[string]any{
				"id":        "0x0a00",
				"propertyA": utils.WrapPointer("pa"),
				"propertyB": utils.WrapPointer(true),
				"propertyC": utils.WrapPointer(int32(123)),
				"propertyD": []any{big.NewInt(234), big.NewInt(345)},
				"propertyE": []any{
					[]any{decimal.New(111111, -30), decimal.New(222222, -10)},
					[]any{decimal.New(3, -30), decimal.New(400000004, -30)},
				},
				"propertyF": utils.WrapPointer("AAA"),
				"propertyG": []any{"AAA", "AAA", "CCC"},
				"foreignA":  "0x0b00",
				"foreignD":  utils.WrapPointer("0x0b00"),
			},
			Entity:         "EntityA",
			GenBlockNumber: 100,
			GenBlockTime:   genBlockTime(100),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0a01": {
			ID: "0x0a01",
			Data: map[string]any{
				"id":        "0x0a01",
				"propertyA": (*string)(nil),
				"propertyB": (*bool)(nil),
				"propertyC": (*int32)(nil),
				"propertyD": []any{big.NewInt(234222), big.NewInt(3453333)},
				"propertyE": nil,
				"propertyF": (*string)(nil),
				"propertyG": []any{"BBB", "CCC", nil},
				"foreignA":  "0x0b01",
				"foreignD":  utils.WrapPointer("0x0b01"),
			},
			Entity:         "EntityA",
			GenBlockNumber: 110,
			GenBlockTime:   genBlockTime(110),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0b00": {
			ID: "0x0b00",
			Data: map[string]any{
				"id":        "0x0b00",
				"propertyA": "pb",
				"foreignB":  "0x0a00",
				"foreignE":  []*string{utils.WrapPointer("0x0a00")},
				"foreignF":  []string{}, // empty
			},
			Entity:         "EntityB",
			GenBlockNumber: 120,
			GenBlockTime:   genBlockTime(120),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0b01": {
			ID: "0x0b01",
			Data: map[string]any{
				"id":        "0x0b01",
				"propertyA": "pbbbbb",
				"foreignB":  "0x0a00",
				"foreignE":  []*string{utils.WrapPointer("0x0a01"), utils.WrapPointer("0x0a00")},
				"foreignF":  []string{"0x0a01", "0x0a00"},
			},
			Entity:         "EntityB",
			GenBlockNumber: 130,
			GenBlockTime:   genBlockTime(130),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0c0000": {
			ID: "0x0c0000",
			Data: map[string]any{
				"id":        "0x0c0000",
				"propertyA": int32(100),
				"propertyB": maxBigInt,
				"propertyC": maxBigInt,
				"propertyD": decimal.New(1, -30),
				"foreignCA": "0x0a00",
				"foreignCB": "0x0b00",
			},
			Entity:         "EntityC",
			GenBlockNumber: 140,
			GenBlockTime:   genBlockTime(140),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0c0001": {
			ID: "0x0c0001",
			Data: map[string]any{
				"id":        "0x0c0001",
				"propertyA": int32(101),
				"propertyB": big.NewInt(1),
				"propertyC": nil,
				"propertyD": decimal.New(123456789, -30),
				"foreignCA": "0x0a00",
				"foreignCB": "0x0b01",
			},
			Entity:         "EntityC",
			GenBlockNumber: 150,
			GenBlockTime:   genBlockTime(150),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0c0101": {
			ID: "0x0c0101",
			Data: map[string]any{
				"id":        "0x0c0101",
				"propertyA": int32(111),
				"propertyB": big.NewInt(-100),
				"propertyC": new(big.Int).Neg(maxBigInt),
				"propertyD": decimal.New(123456789, -30),
				"foreignCA": "0x0a01",
				"foreignCB": "0x0b01",
			},
			Entity:         "EntityC",
			GenBlockNumber: 160,
			GenBlockTime:   genBlockTime(160),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
	}

	entityAType := sch.GetEntity("EntityA")
	entityBType := sch.GetEntity("EntityB")

	es.pushData(ctx, entities)

	// === remove entity
	if skipDelete {
		return
	}

	var data *persistent.EntityBox
	var err error

	// set 0x0b01 foreignE to nil
	_, err = s.SetEntities(ctx, entityBType, []persistent.EntityBox{{
		ID: "0x0b01",
		Data: map[string]any{
			"id":        "0x0b01",
			"propertyA": "pbbbbbx",
			"foreignB":  "0x0a00",
			"foreignE":  nil,
			"foreignF":  []string{"0x0a01", "0x0a00"},
		},
		Entity:         "EntityB",
		GenBlockNumber: 170,
		GenBlockTime:   genBlockTime(170),
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}})
	assert.NoError(t, err)

	data, err = s.GetEntity(ctx, entityAType, chain, "0x0a00")
	assert.NoError(t, err)
	assert.Equal(t, &persistent.EntityBox{
		ID: "0x0a00",
		Data: map[string]any{
			"id":        "0x0a00",
			"propertyA": utils.WrapPointer("pa"),
			"propertyB": utils.WrapPointer(true),
			"propertyC": utils.WrapPointer(int32(123)),
			"propertyD": []any{big.NewInt(234), big.NewInt(345)},
			"propertyE": []any{
				[]any{decimal.New(111111, -30), decimal.New(222222, -10)},
				[]any{decimal.New(3, -30), decimal.New(400000004, -30)},
			},
			"propertyF": utils.WrapPointer("AAA"),
			"propertyG": []any{"AAA", "AAA", "CCC"},
			"foreignA":  "0x0b00",
			"foreignD":  utils.WrapPointer("0x0b00"),
		},
		Entity:         "EntityA",
		GenBlockNumber: 100,
		GenBlockTime:   genBlockTime(100),
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}, data)

	data, err = s.GetEntity(ctx, entityAType, chain, "0x0a01")
	assert.NoError(t, err)
	assert.Equal(t, &persistent.EntityBox{
		ID: "0x0a01",
		Data: map[string]any{
			"id":        "0x0a01",
			"propertyA": (*string)(nil),
			"propertyB": (*bool)(nil),
			"propertyC": (*int32)(nil),
			"propertyD": []any{big.NewInt(234222), big.NewInt(3453333)},
			"propertyE": nil,
			"propertyF": (*string)(nil),
			"propertyG": []any{"BBB", "CCC", nil},
			"foreignA":  "0x0b01",
			"foreignD":  utils.WrapPointer("0x0b01"),
		},
		Entity:         "EntityA",
		GenBlockNumber: 110,
		GenBlockTime:   genBlockTime(110),
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}, data)

	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b01")
	assert.NoError(t, err)
	assert.Equal(t, &persistent.EntityBox{
		ID: "0x0b01",
		Data: map[string]any{
			"id":        "0x0b01",
			"propertyA": "pbbbbbx",
			"foreignB":  "0x0a00",
			"foreignE":  nil,
			"foreignF":  []string{"0x0a01", "0x0a00"},
		},
		Entity:         "EntityB",
		GenBlockNumber: 170,
		GenBlockTime:   genBlockTime(170),
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}, data)

	// del 0x0b01
	_, err = s.SetEntities(ctx, entityBType, []persistent.EntityBox{{
		ID:             "0x0b01",
		Data:           nil,
		Entity:         "EntityB",
		GenBlockNumber: 180,
		GenBlockTime:   genBlockTime(180),
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}})
	assert.NoError(t, err)

	data, err = s.GetEntity(ctx, entityAType, chain, "0x0a00")
	assert.NoError(t, err)
	assert.Equal(t, &persistent.EntityBox{
		ID: "0x0a00",
		Data: map[string]any{
			"id":        "0x0a00",
			"propertyA": utils.WrapPointer("pa"),
			"propertyB": utils.WrapPointer(true),
			"propertyC": utils.WrapPointer(int32(123)),
			"propertyD": []any{big.NewInt(234), big.NewInt(345)},
			"propertyE": []any{
				[]any{decimal.New(111111, -30), decimal.New(222222, -10)},
				[]any{decimal.New(3, -30), decimal.New(400000004, -30)},
			},
			"propertyF": utils.WrapPointer("AAA"),
			"propertyG": []any{"AAA", "AAA", "CCC"},
			"foreignA":  "0x0b00",
			"foreignD":  utils.WrapPointer("0x0b00"),
		},
		Entity:         "EntityA",
		GenBlockNumber: 100,
		GenBlockTime:   genBlockTime(100),
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}, data)

	data, err = s.GetEntity(ctx, entityAType, chain, "0x0a01")
	assert.NoError(t, err)
	assert.Equal(t, &persistent.EntityBox{
		ID: "0x0a01",
		Data: map[string]any{
			"id":        "0x0a01",
			"propertyA": (*string)(nil),
			"propertyB": (*bool)(nil),
			"propertyC": (*int32)(nil),
			"propertyD": []any{big.NewInt(234222), big.NewInt(3453333)},
			"propertyE": nil,
			"propertyF": (*string)(nil),
			"propertyG": []any{"BBB", "CCC", nil},
			"foreignA":  "0x0b01",
			"foreignD":  utils.WrapPointer("0x0b01"),
		},
		Entity:         "EntityA",
		GenBlockNumber: 110,
		GenBlockTime:   genBlockTime(110),
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}, data)

	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b00")
	assert.NoError(t, err)
	assert.Equal(t, entities["0x0b00"], *data)

	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b01")
	assert.NoError(t, err)
	assert.Equal(t, &persistent.EntityBox{
		ID:             "0x0b01",
		Data:           nil,
		Entity:         "EntityB",
		GenBlockNumber: 180,
		GenBlockTime:   genBlockTime(180),
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}, data)

	// === clean genBlockNumber > 100

	err = s.Reorg(ctx, 100, chain)
	assert.NoError(t, err)
	// only 0x0a00 is remained

	data, err = s.GetEntity(ctx, entityAType, chain, "0x0a00")
	assert.NoError(t, err)
	assert.Equal(t, &persistent.EntityBox{
		ID: "0x0a00",
		Data: map[string]any{
			"id":        "0x0a00",
			"propertyA": utils.WrapPointer("pa"),
			"propertyB": utils.WrapPointer(true),
			"propertyC": utils.WrapPointer(int32(123)),
			"propertyD": []any{big.NewInt(234), big.NewInt(345)},
			"propertyE": []any{
				[]any{decimal.New(111111, -30), decimal.New(222222, -10)},
				[]any{decimal.New(3, -30), decimal.New(400000004, -30)},
			},
			"propertyF": utils.WrapPointer("AAA"),
			"propertyG": []any{"AAA", "AAA", "CCC"},
			"foreignA":  "0x0b00",
			"foreignD":  utils.WrapPointer("0x0b00"),
		},
		Entity:         "EntityA",
		GenBlockNumber: 100,
		GenBlockTime:   genBlockTime(100),
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain,
	}, data)

	data, err = s.GetEntity(ctx, entityAType, chain, "0x0a01")
	assert.NoError(t, err)
	assert.Nil(t, data)

}

func Test_getSetDel2(t *testing.T) {
	if skip {
		t.Skip("use local db, will only be executed manually locally")
	}

	const chain1 = "1"
	const chain2 = "2"

	ctx := context.Background()
	es := EntitySuite{t: t}
	es.init(ctx)
	sch, s := es.s.sch, es.s

	es.pushData(ctx, map[string]persistent.EntityBox{
		"0x0d0100": {
			ID: "0x0d0100",
			Data: map[string]any{
				"id":        "0x0d0100",
				"propertyA": "d1pa0c1",
				"on":        []string{"0x0e0100"},
			},
			Entity:         "EntityD1",
			GenBlockNumber: 101,
			GenBlockTime:   genBlockTime(101),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain1,
		},
		"0x0d0101": {
			ID: "0x0d0101",
			Data: map[string]any{
				"id":        "0x0d0101",
				"propertyA": "d1pa1c1",
				"on":        []string{"0x0e0101"},
			},
			Entity:         "EntityD1",
			GenBlockNumber: 102,
			GenBlockTime:   genBlockTime(102),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain1,
		},
		"0x0e0100": {
			ID: "0x0e0100",
			Data: map[string]any{
				"id":   "0x0e0100",
				"from": "e1from0c1",
			},
			Entity:         "EntityE1",
			GenBlockNumber: 105,
			GenBlockTime:   genBlockTime(105),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain1,
		},
		"0x0e0101": {
			ID: "0x0e0101",
			Data: map[string]any{
				"id":   "0x0e0101",
				"from": "e1from1c1",
			},
			Entity:         "EntityE1",
			GenBlockNumber: 106,
			GenBlockTime:   genBlockTime(106),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain1,
		},
	})

	es.pushData(ctx, map[string]persistent.EntityBox{
		"0x0d0100": {
			ID: "0x0d0100",
			Data: map[string]any{
				"id":        "0x0d0100",
				"propertyA": "d1pa0c2",
				"on":        []string{"0x0e0101"},
			},
			Entity:         "EntityD1",
			GenBlockNumber: 1010,
			GenBlockTime:   genBlockTime(1010),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain2,
		},
		"0x0d0101": {
			ID: "0x0d0101",
			Data: map[string]any{
				"id":        "0x0d0101",
				"propertyA": "d1pa1c2",
				"on":        []string{"0x0e0100"},
			},
			Entity:         "EntityD1",
			GenBlockNumber: 1020,
			GenBlockTime:   genBlockTime(1020),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain2,
		},
		"0x0e0100": {
			ID: "0x0e0100",
			Data: map[string]any{
				"id":   "0x0e0100",
				"from": "e1from0c2",
			},
			Entity:         "EntityE1",
			GenBlockNumber: 1050,
			GenBlockTime:   genBlockTime(1050),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain2,
		},
		"0x0e0101": {
			ID: "0x0e0101",
			Data: map[string]any{
				"id":   "0x0e0101",
				"from": "e1from1c2",
			},
			Entity:         "EntityE1",
			GenBlockNumber: 1060,
			GenBlockTime:   genBlockTime(1060),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain2,
		},
	})

	entityD1Type := sch.GetEntity("EntityD1")
	entityE1Type := sch.GetEntity("EntityE1")

	data, err := s.GetEntity(ctx, entityD1Type, chain1, "0x0d0100")
	assert.NoError(t, err)
	assert.Equal(t, "d1pa0c1", data.Data["propertyA"])
	data, err = s.GetEntity(ctx, entityD1Type, chain1, "0x0d0101")
	assert.NoError(t, err)
	assert.Equal(t, "d1pa1c1", data.Data["propertyA"])

	data, err = s.GetEntity(ctx, entityD1Type, chain2, "0x0d0100")
	assert.NoError(t, err)
	assert.Equal(t, "d1pa0c2", data.Data["propertyA"])
	data, err = s.GetEntity(ctx, entityD1Type, chain2, "0x0d0101")
	assert.NoError(t, err)
	assert.Equal(t, "d1pa1c2", data.Data["propertyA"])

	data, err = s.GetEntity(ctx, entityE1Type, chain1, "0x0e0100")
	assert.NoError(t, err)
	assert.Equal(t, "e1from0c1", data.Data["from"])
	data, err = s.GetEntity(ctx, entityE1Type, chain1, "0x0e0101")
	assert.NoError(t, err)
	assert.Equal(t, "e1from1c1", data.Data["from"])

	data, err = s.GetEntity(ctx, entityE1Type, chain2, "0x0e0100")
	assert.NoError(t, err)
	assert.Equal(t, "e1from0c2", data.Data["from"])
	data, err = s.GetEntity(ctx, entityE1Type, chain2, "0x0e0101")
	assert.NoError(t, err)
	assert.Equal(t, "e1from1c2", data.Data["from"])

}

func Test_getSetDel3(t *testing.T) {
	if skip {
		t.Skip("use local db, will only be executed manually locally")
	}

	const chain1 = "1"

	ctx := context.Background()
	es := EntitySuite{t: t}
	es.init(ctx)
	sch, s := es.s.sch, es.s

	entityIType := sch.GetEntity("EntityI")

	box, err := s.GetEntity(ctx, entityIType, chain1, "I01")
	assert.NoError(t, err)
	assert.Nil(t, box)

	i01 := persistent.EntityBox{
		ID: "I01",
		Data: map[string]any{
			"id":    "I01",
			"propA": `{"a":123}`,
		},
		Entity:         "EntityI",
		GenBlockNumber: 101,
		GenBlockTime:   genBlockTime(101),
		GenBlockHash:   "0x1234",
		GenBlockChain:  chain1,
	}

	created, err := s.SetEntities(ctx, entityIType, []persistent.EntityBox{i01})
	assert.NoError(t, err)
	assert.Equal(t, 1, created)

	box, err = s.GetEntity(ctx, entityIType, chain1, "I01")
	assert.NoError(t, err)
	assert.Equal(t, &i01, box)

}

func Test_batchSet1(t *testing.T) {
	if skip {
		t.Skip("use local db, will only be executed manually locally")
	}

	const chain = "1"
	ctx := context.Background()
	es := EntitySuite{t: t}
	es.init(ctx)
	sch, s := es.s.sch, es.s

	entities := []persistent.EntityBox{
		{
			ID: "0x0b00",
			Data: map[string]any{
				"id":        "0x0b00",
				"propertyA": "pa0",
				"foreignB":  "0x0a00",
			},
			Entity:         "EntityB",
			GenBlockNumber: 100,
			GenBlockTime:   genBlockTime(100),
			GenBlockHash:   "0x000000",
			GenBlockChain:  chain,
		},
		{
			ID: "0x0b01",
			Data: map[string]any{
				"id":        "0x0b01",
				"propertyA": "pa1",
				"foreignB":  "0x0a01",
			},
			Entity:         "EntityB",
			GenBlockNumber: 101,
			GenBlockTime:   genBlockTime(101),
			GenBlockHash:   "0x000001",
			GenBlockChain:  chain,
		},
		{
			ID: "0x0b02",
			Data: map[string]any{
				"id":        "0x0b02",
				"propertyA": "pa2",
				"foreignB":  "0x0a02",
			},
			Entity:         "EntityB",
			GenBlockNumber: 102,
			GenBlockTime:   genBlockTime(102),
			GenBlockHash:   "0x000002",
			GenBlockChain:  chain,
		},
		{
			ID: "0x0b03",
			Data: map[string]any{
				"id":        "0x0b03",
				"propertyA": "pa3",
				"foreignB":  "0x0a03",
			},
			Entity:         "EntityB",
			GenBlockNumber: 103,
			GenBlockTime:   genBlockTime(103),
			GenBlockHash:   "0x000003",
			GenBlockChain:  chain,
		},
		{
			ID: "0x0b04",
			Data: map[string]any{
				"id":        "0x0b04",
				"propertyA": "pa4",
				"foreignB":  "0x0a04",
			},
			Entity:         "EntityB",
			GenBlockNumber: 104,
			GenBlockTime:   genBlockTime(104),
			GenBlockHash:   "0x000004",
			GenBlockChain:  chain,
		},
	}

	entityBType := sch.GetEntity("EntityB")
	assert.NotNil(t, entityBType)

	var err error

	_, err = s.SetEntities(ctx, entityBType, entities)
	assert.NoError(t, err)

	data, err := s.GetEntity(ctx, entityBType, chain, "0x0b00")
	assert.NoError(t, err)
	assert.Equal(t, "0x0a00", data.Data["foreignB"])
	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b01")
	assert.NoError(t, err)
	assert.Equal(t, "0x0a01", data.Data["foreignB"])
	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b02")
	assert.NoError(t, err)
	assert.Equal(t, "0x0a02", data.Data["foreignB"])
	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b03")
	assert.NoError(t, err)
	assert.Equal(t, "0x0a03", data.Data["foreignB"])
	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b04")
	assert.NoError(t, err)
	assert.Equal(t, "0x0a04", data.Data["foreignB"])

	entities = []persistent.EntityBox{
		{
			ID:             "0x0b00",
			Data:           nil,
			Entity:         "EntityB",
			GenBlockNumber: 200,
			GenBlockTime:   genBlockTime(200),
			GenBlockHash:   "0x000000",
			GenBlockChain:  chain,
		},
		{
			ID: "0x0b01",
			Data: map[string]any{
				"id":        "0x0b01",
				"propertyA": "paaaa1",
				"foreignB":  "0x0a01",
			},
			Entity:         "EntityB",
			GenBlockNumber: 201,
			GenBlockTime:   genBlockTime(201),
			GenBlockHash:   "0x000001",
			GenBlockChain:  chain,
		},
		{
			ID: "0x0b02",
			Data: map[string]any{
				"id":        "0x0b02",
				"propertyA": "paaaa2",
				"foreignB":  "0x0a0202",
			},
			Entity:         "EntityB",
			GenBlockNumber: 202,
			GenBlockTime:   genBlockTime(202),
			GenBlockHash:   "0x000002",
			GenBlockChain:  chain,
		},
		{
			ID:             "0x0b03",
			Data:           nil,
			Entity:         "EntityB",
			GenBlockNumber: 203,
			GenBlockTime:   genBlockTime(203),
			GenBlockHash:   "0x000003",
			GenBlockChain:  chain,
		},
		{
			ID:             "0x0b04",
			Data:           nil,
			Entity:         "EntityB",
			GenBlockNumber: 204,
			GenBlockTime:   genBlockTime(204),
			GenBlockHash:   "0x000004",
			GenBlockChain:  chain,
		},
	}

	_, err = s.SetEntities(ctx, entityBType, entities)
	assert.NoError(t, err)

	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b00")
	assert.NoError(t, err)
	assert.Equal(t, entities[0], *data)
	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b01")
	assert.NoError(t, err)
	assert.Equal(t, "paaaa1", data.Data["propertyA"])
	assert.Equal(t, "0x0a01", data.Data["foreignB"])
	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b02")
	assert.NoError(t, err)
	assert.Equal(t, "paaaa2", data.Data["propertyA"])
	assert.Equal(t, "0x0a0202", data.Data["foreignB"])
	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b03")
	assert.NoError(t, err)
	assert.Equal(t, entities[3], *data)
	data, err = s.GetEntity(ctx, entityBType, chain, "0x0b04")
	assert.NoError(t, err)
	assert.Equal(t, entities[4], *data)

}

func Test_batchSet2(t *testing.T) {
	if skip {
		t.Skip("use local db, will only be executed manually locally")
	}

	const chain = "1"
	ctx := context.Background()
	es := EntitySuite{t: t}
	es.init(ctx)
	sch, s := es.s.sch, es.s

	build := func(start, end, stage int, delete bool) []persistent.EntityBox {
		entities := make([]persistent.EntityBox, 0, end-start+1)
		for i := start; i <= end; i++ {
			id := fmt.Sprintf("0x0d%02d", i)
			var data map[string]any
			if !delete {
				data = map[string]any{
					"id":        id,
					"propertyA": fmt.Sprintf("pa-%d-%d", i, stage),
					"on":        nil,
				}
			}
			entities = append(entities, persistent.EntityBox{
				ID:             id,
				Data:           data,
				Entity:         "EntityD1",
				GenBlockNumber: uint64(stage),
				GenBlockTime:   genBlockTime(uint64(stage)),
				GenBlockHash:   "0x000000",
				GenBlockChain:  chain,
			})
		}
		return entities
	}

	entities1 := build(0, 9, 1, false)
	entities2 := build(0, 9, 2, false)
	entities3 := build(5, 7, 3, true)

	entityD1Type := sch.GetEntity("EntityD1")
	assert.NotNil(t, entityD1Type)

	var err error
	var boxes []*persistent.EntityBox

	// init is empty
	boxes, err = s.ListEntities(ctx, entityD1Type, chain, nil, 10)
	assert.NoError(t, err)
	assert.Equal(t, []*persistent.EntityBox(nil), boxes)
	// insert 0-9 entities
	_, err = s.setEntities(ctx, entityD1Type, chain, entities1)
	assert.NoError(t, err)
	// all 10 entities exists
	boxes, err = s.ListEntities(ctx, entityD1Type, chain, nil, 10)
	assert.NoError(t, err)
	assert.Equal(t, utils.WrapPointerForArray(entities1), boxes)
	// update 0-9 entities
	_, err = s.setEntities(ctx, entityD1Type, chain, entities2)
	assert.NoError(t, err)
	// all 10 entities updated to stage 2
	boxes, err = s.ListEntities(ctx, entityD1Type, chain, nil, 10)
	assert.NoError(t, err)
	assert.Equal(t, utils.WrapPointerForArray(entities2), boxes)
	// delete 5-7
	_, err = s.setEntities(ctx, entityD1Type, chain, entities3)
	assert.NoError(t, err)
	// only have 0-4,8-9
	boxes, err = s.ListEntities(ctx, entityD1Type, chain, nil, 10)
	assert.NoError(t, err)
	assert.Equal(t, utils.WrapPointerForArray(utils.MergeArr(entities2[:5], entities2[8:])), boxes)

}

func Test_list(t *testing.T) {
	if skip {
		t.Skip("use local db, will only be executed manually locally")
	}

	const chain = "1"
	ctx := context.Background()
	es := EntitySuite{t: t}
	es.init(ctx)
	sch, s := es.s.sch, es.s

	decimal0 := decimal.New(1, -30)
	decimal1 := decimal.New(2, -30)

	build := func(start, end, stage, impl int, delete bool) []persistent.EntityBox {
		entities := make([]persistent.EntityBox, 0, end-start+1)
		for i := start; i <= end; i++ {
			id := fmt.Sprintf("0x0f%02d", i)
			var data map[string]any
			if !delete {
				data = map[string]any{
					"id":        id,
					"propertyA": fmt.Sprintf("%d", i%2),
					"propertyB": fmt.Sprintf("%d", i%2),
					"propertyC": i%2 != 0,
					"propertyD": int32(i % 2),
					"propertyE": big.NewInt(int64(i) % 2),
					"propertyF": utils.Select(i%2 == 0, decimal0, decimal1),
					"propertyG": utils.Select(i%2 == 0, "AAA", "BBB"),
					"propertyH": []any{fmt.Sprintf("%d", i%2)},
					"propertyI": []any{fmt.Sprintf("%d", i%2)},
					"propertyJ": []any{i%2 != 0},
					"propertyK": []any{int32(i % 2)},
					"propertyL": []any{big.NewInt(int64(i) % 2)},
					"propertyM": []any{utils.Select(i%2 == 0, decimal0, decimal1)},
					"propertyN": []any{utils.Select(i%2 == 0, "AAA", "BBB")},
					"propertyO": int64(i % 2),
					"propertyP": []any{int64(i % 2)},
					"propertyQ": float64(i % 2),
					"propertyR": []any{float64(i % 2)},
					"foreignA":  fmt.Sprintf("0x0a0%d", i%2),
					"foreignB":  []string{fmt.Sprintf("0x0a0%d", i%2)},
				}
			}
			entities = append(entities, persistent.EntityBox{
				ID:             id,
				Data:           data,
				Entity:         fmt.Sprintf("EntityF%d", impl),
				GenBlockNumber: uint64(stage),
				GenBlockTime:   genBlockTime(uint64(stage)),
				GenBlockHash:   "0x000000",
				GenBlockChain:  chain,
			})
		}
		return entities
	}

	for fx := 1; fx <= 2; fx++ {
		entities := build(0, 9, 1, fx, false)

		group0 := []*persistent.EntityBox{&entities[0], &entities[2], &entities[4], &entities[6], &entities[8]}
		group1 := []*persistent.EntityBox{&entities[1], &entities[3], &entities[5], &entities[7], &entities[9]}
		empty := []*persistent.EntityBox(nil)
		full := utils.WrapPointerForArray(entities)

		entityF := sch.GetEntity(fmt.Sprintf("EntityF%d", fx))
		assert.NotNil(t, entityF)

		var err error
		var boxes []*persistent.EntityBox

		// init is empty
		boxes, err = s.ListEntities(ctx, entityF, chain, nil, 3)
		assert.NoError(t, err)
		assert.Equal(t, []*persistent.EntityBox(nil), boxes)

		// insert 0-9 entities
		_, err = s.setEntities(ctx, entityF, chain, entities)
		assert.NoError(t, err)

		// list all
		boxes, err = s.ListEntities(ctx, entityF, chain, nil, 10)
		assert.NoError(t, err)
		assert.Equal(t, full, boxes)

		// list all
		boxes, err = s.ListEntities(ctx, entityF, chain, nil, 11)
		assert.NoError(t, err)
		assert.Equal(t, full, boxes)

		// all 10 entities exists and list by two page
		boxes, err = s.ListEntities(ctx, entityF, chain, nil, 8)
		assert.NoError(t, err)
		assert.Equal(t, full[:8], boxes)

		// === with filter condition

		testcases := []struct {
			F string
			O persistent.EntityFilterOp
			V []any
			R []*persistent.EntityBox
		}{
			// id
			{F: "id", O: persistent.EntityFilterOpEq, V: []any{"0x0f00"}, R: []*persistent.EntityBox{&entities[0]}},
			{F: "id", O: persistent.EntityFilterOpNe, V: []any{"0x0f00"}, R: utils.WrapPointerForArray(entities[1:])},
			{F: "id", O: persistent.EntityFilterOpIn, V: []any{"0x0f00", "0x0f02", "0x0f04"}, R: group0[:3]},
			{F: "id", O: persistent.EntityFilterOpNotIn, V: []any{"0x0f01", "0x0f03", "0x0f05", "0x0f07", "0x0f09"}, R: group0},
			// propertyA: Bytes!
			{F: "propertyA", O: persistent.EntityFilterOpEq, V: []any{"0"}, R: group0},
			{F: "propertyA", O: persistent.EntityFilterOpNe, V: []any{"1"}, R: group0},
			{F: "propertyA", O: persistent.EntityFilterOpGt, V: []any{"0"}, R: group1},
			{F: "propertyA", O: persistent.EntityFilterOpGe, V: []any{"1"}, R: group1},
			{F: "propertyA", O: persistent.EntityFilterOpLt, V: []any{"1"}, R: group0},
			{F: "propertyA", O: persistent.EntityFilterOpLe, V: []any{"0"}, R: group0},
			{F: "propertyA", O: persistent.EntityFilterOpIn, V: []any{"0"}, R: group0},
			{F: "propertyA", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
			{F: "propertyA", O: persistent.EntityFilterOpNotIn, V: []any{"1"}, R: group0},
			{F: "propertyA", O: persistent.EntityFilterOpNotIn, V: []any{"0", "1"}, R: empty},
			{F: "propertyA", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
			// propertyB: String!
			{F: "propertyB", O: persistent.EntityFilterOpEq, V: []any{"0"}, R: group0},
			{F: "propertyB", O: persistent.EntityFilterOpNe, V: []any{"1"}, R: group0},
			{F: "propertyB", O: persistent.EntityFilterOpGt, V: []any{"0"}, R: group1},
			{F: "propertyB", O: persistent.EntityFilterOpGe, V: []any{"1"}, R: group1},
			{F: "propertyB", O: persistent.EntityFilterOpLt, V: []any{"1"}, R: group0},
			{F: "propertyB", O: persistent.EntityFilterOpLe, V: []any{"0"}, R: group0},
			{F: "propertyB", O: persistent.EntityFilterOpIn, V: []any{"0"}, R: group0},
			{F: "propertyB", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
			{F: "propertyB", O: persistent.EntityFilterOpNotIn, V: []any{"1"}, R: group0},
			{F: "propertyB", O: persistent.EntityFilterOpNotIn, V: []any{"0", "1"}, R: empty},
			{F: "propertyB", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
			{F: "propertyB", O: persistent.EntityFilterOpLike, V: []any{"0%"}, R: group0},
			{F: "propertyB", O: persistent.EntityFilterOpLike, V: []any{"%1"}, R: group1},
			{F: "propertyB", O: persistent.EntityFilterOpNotLike, V: []any{"0%"}, R: group1},
			{F: "propertyB", O: persistent.EntityFilterOpNotLike, V: []any{"%1"}, R: group0},
			// propertyC: Boolean!
			{F: "propertyC", O: persistent.EntityFilterOpEq, V: []any{false}, R: group0},
			{F: "propertyC", O: persistent.EntityFilterOpNe, V: []any{true}, R: group0},
			{F: "propertyC", O: persistent.EntityFilterOpGt, V: []any{false}, R: group1},
			{F: "propertyC", O: persistent.EntityFilterOpGe, V: []any{true}, R: group1},
			{F: "propertyC", O: persistent.EntityFilterOpLt, V: []any{true}, R: group0},
			{F: "propertyC", O: persistent.EntityFilterOpLe, V: []any{false}, R: group0},
			{F: "propertyC", O: persistent.EntityFilterOpIn, V: []any{false}, R: group0},
			{F: "propertyC", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
			{F: "propertyC", O: persistent.EntityFilterOpNotIn, V: []any{true}, R: group0},
			{F: "propertyC", O: persistent.EntityFilterOpNotIn, V: []any{false, true}, R: empty},
			{F: "propertyC", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
			// propertyD: Int!
			{F: "propertyD", O: persistent.EntityFilterOpEq, V: []any{int32(0)}, R: group0},
			{F: "propertyD", O: persistent.EntityFilterOpNe, V: []any{int32(1)}, R: group0},
			{F: "propertyD", O: persistent.EntityFilterOpGt, V: []any{int32(0)}, R: group1},
			{F: "propertyD", O: persistent.EntityFilterOpGe, V: []any{int32(1)}, R: group1},
			{F: "propertyD", O: persistent.EntityFilterOpLt, V: []any{int32(1)}, R: group0},
			{F: "propertyD", O: persistent.EntityFilterOpLe, V: []any{int32(0)}, R: group0},
			{F: "propertyD", O: persistent.EntityFilterOpIn, V: []any{int32(0)}, R: group0},
			{F: "propertyD", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
			{F: "propertyD", O: persistent.EntityFilterOpNotIn, V: []any{int32(1)}, R: group0},
			{F: "propertyD", O: persistent.EntityFilterOpNotIn, V: []any{int32(0), int32(1)}, R: empty},
			{F: "propertyD", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
			// propertyE: BigInt!
			{F: "propertyE", O: persistent.EntityFilterOpEq, V: []any{big.NewInt(0)}, R: group0},
			{F: "propertyE", O: persistent.EntityFilterOpNe, V: []any{big.NewInt(1)}, R: group0},
			{F: "propertyE", O: persistent.EntityFilterOpGt, V: []any{big.NewInt(0)}, R: group1},
			{F: "propertyE", O: persistent.EntityFilterOpGe, V: []any{big.NewInt(1)}, R: group1},
			{F: "propertyE", O: persistent.EntityFilterOpLt, V: []any{big.NewInt(1)}, R: group0},
			{F: "propertyE", O: persistent.EntityFilterOpLe, V: []any{big.NewInt(0)}, R: group0},
			{F: "propertyE", O: persistent.EntityFilterOpIn, V: []any{big.NewInt(0)}, R: group0},
			{F: "propertyE", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
			{F: "propertyE", O: persistent.EntityFilterOpNotIn, V: []any{big.NewInt(1)}, R: group0},
			{F: "propertyE", O: persistent.EntityFilterOpNotIn, V: []any{big.NewInt(0), big.NewInt(1)}, R: empty},
			{F: "propertyE", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
			// propertyF: BigDecimal!
			{F: "propertyF", O: persistent.EntityFilterOpEq, V: []any{decimal0}, R: group0},
			{F: "propertyF", O: persistent.EntityFilterOpNe, V: []any{decimal1}, R: group0},
			{F: "propertyF", O: persistent.EntityFilterOpGt, V: []any{decimal0}, R: group1},
			{F: "propertyF", O: persistent.EntityFilterOpGe, V: []any{decimal1}, R: group1},
			{F: "propertyF", O: persistent.EntityFilterOpLt, V: []any{decimal1}, R: group0},
			{F: "propertyF", O: persistent.EntityFilterOpLe, V: []any{decimal0}, R: group0},
			{F: "propertyF", O: persistent.EntityFilterOpIn, V: []any{decimal0}, R: group0},
			{F: "propertyF", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
			{F: "propertyF", O: persistent.EntityFilterOpNotIn, V: []any{decimal1}, R: group0},
			{F: "propertyF", O: persistent.EntityFilterOpNotIn, V: []any{decimal0, decimal1}, R: empty},
			{F: "propertyF", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
			// propertyG: EnumA!
			{F: "propertyG", O: persistent.EntityFilterOpEq, V: []any{"AAA"}, R: group0},
			{F: "propertyG", O: persistent.EntityFilterOpNe, V: []any{"BBB"}, R: group0},
			{F: "propertyG", O: persistent.EntityFilterOpGt, V: []any{"AAA"}, R: group1},
			{F: "propertyG", O: persistent.EntityFilterOpGe, V: []any{"BBB"}, R: group1},
			{F: "propertyG", O: persistent.EntityFilterOpLt, V: []any{"BBB"}, R: group0},
			{F: "propertyG", O: persistent.EntityFilterOpLe, V: []any{"AAA"}, R: group0},
			{F: "propertyG", O: persistent.EntityFilterOpIn, V: []any{"AAA"}, R: group0},
			{F: "propertyG", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
			{F: "propertyG", O: persistent.EntityFilterOpNotIn, V: []any{"BBB"}, R: group0},
			{F: "propertyG", O: persistent.EntityFilterOpNotIn, V: []any{"AAA", "BBB"}, R: empty},
			{F: "propertyG", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
			// propertyO: Timestamp!
			{F: "propertyO", O: persistent.EntityFilterOpEq, V: []any{int64(0)}, R: group0},
			{F: "propertyO", O: persistent.EntityFilterOpNe, V: []any{int64(1)}, R: group0},
			{F: "propertyO", O: persistent.EntityFilterOpGt, V: []any{int64(0)}, R: group1},
			{F: "propertyO", O: persistent.EntityFilterOpGe, V: []any{int64(1)}, R: group1},
			{F: "propertyO", O: persistent.EntityFilterOpLt, V: []any{int64(1)}, R: group0},
			{F: "propertyO", O: persistent.EntityFilterOpLe, V: []any{int64(0)}, R: group0},
			{F: "propertyO", O: persistent.EntityFilterOpIn, V: []any{int64(0)}, R: group0},
			{F: "propertyO", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
			{F: "propertyO", O: persistent.EntityFilterOpNotIn, V: []any{int64(1)}, R: group0},
			{F: "propertyO", O: persistent.EntityFilterOpNotIn, V: []any{int64(0), int64(1)}, R: empty},
			{F: "propertyO", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
			// propertyQ: Float!
			{F: "propertyQ", O: persistent.EntityFilterOpEq, V: []any{float64(0)}, R: group0},
			{F: "propertyQ", O: persistent.EntityFilterOpNe, V: []any{float64(1)}, R: group0},
			{F: "propertyQ", O: persistent.EntityFilterOpGt, V: []any{float64(0)}, R: group1},
			{F: "propertyQ", O: persistent.EntityFilterOpGe, V: []any{float64(1)}, R: group1},
			{F: "propertyQ", O: persistent.EntityFilterOpLt, V: []any{float64(1)}, R: group0},
			{F: "propertyQ", O: persistent.EntityFilterOpLe, V: []any{float64(0)}, R: group0},
			{F: "propertyQ", O: persistent.EntityFilterOpIn, V: []any{float64(0)}, R: group0},
			{F: "propertyQ", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
			{F: "propertyQ", O: persistent.EntityFilterOpNotIn, V: []any{float64(1)}, R: group0},
			{F: "propertyQ", O: persistent.EntityFilterOpNotIn, V: []any{float64(0), float64(1)}, R: empty},
			{F: "propertyQ", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
			// propertyH: [Bytes!]
			{F: "propertyH", O: persistent.EntityFilterOpEq, V: []any{[]string{"0"}}, R: group0},
			{F: "propertyH", O: persistent.EntityFilterOpEq, V: []any{[]string{}}, R: empty},
			{F: "propertyH", O: persistent.EntityFilterOpNe, V: []any{[]string{"0"}}, R: group1},
			{F: "propertyH", O: persistent.EntityFilterOpNe, V: []any{[]string{}}, R: full},
			// propertyI: [String!]
			{F: "propertyI", O: persistent.EntityFilterOpEq, V: []any{[]string{"0"}}, R: group0},
			{F: "propertyI", O: persistent.EntityFilterOpEq, V: []any{[]string{}}, R: empty},
			{F: "propertyI", O: persistent.EntityFilterOpNe, V: []any{[]string{"0"}}, R: group1},
			{F: "propertyI", O: persistent.EntityFilterOpNe, V: []any{[]string{}}, R: full},
			{F: "propertyI", O: persistent.EntityFilterOpHasAll, V: []any{"0"}, R: group0},
			{F: "propertyI", O: persistent.EntityFilterOpHasAll, V: []any{"1"}, R: group1},
			{F: "propertyI", O: persistent.EntityFilterOpHasAll, V: []any{"0", "1"}, R: empty},
			{F: "propertyI", O: persistent.EntityFilterOpHasAll, V: []any{}, R: full},
			{F: "propertyI", O: persistent.EntityFilterOpHasAny, V: []any{"0"}, R: group0},
			{F: "propertyI", O: persistent.EntityFilterOpHasAny, V: []any{"1"}, R: group1},
			{F: "propertyI", O: persistent.EntityFilterOpHasAny, V: []any{"0", "1"}, R: full},
			{F: "propertyI", O: persistent.EntityFilterOpHasAny, V: []any{}, R: empty},
			// propertyJ: [Boolean!]
			{F: "propertyJ", O: persistent.EntityFilterOpEq, V: []any{[]bool{false}}, R: group0},
			{F: "propertyJ", O: persistent.EntityFilterOpEq, V: []any{[]bool{}}, R: empty},
			{F: "propertyJ", O: persistent.EntityFilterOpNe, V: []any{[]bool{false}}, R: group1},
			{F: "propertyJ", O: persistent.EntityFilterOpNe, V: []any{[]bool{}}, R: full},
			// propertyK: [Int!]
			{F: "propertyK", O: persistent.EntityFilterOpEq, V: []any{[]int32{0}}, R: group0},
			{F: "propertyK", O: persistent.EntityFilterOpEq, V: []any{[]int32{}}, R: empty},
			{F: "propertyK", O: persistent.EntityFilterOpNe, V: []any{[]int32{0}}, R: group1},
			{F: "propertyK", O: persistent.EntityFilterOpNe, V: []any{[]int32{}}, R: full},
			// propertyL: [BigInt!]
			{F: "propertyL", O: persistent.EntityFilterOpEq, V: []any{[]*big.Int{big.NewInt(0)}}, R: group0},
			{F: "propertyL", O: persistent.EntityFilterOpEq, V: []any{[]*big.Int{}}, R: empty},
			{F: "propertyL", O: persistent.EntityFilterOpNe, V: []any{[]*big.Int{big.NewInt(0)}}, R: group1},
			{F: "propertyL", O: persistent.EntityFilterOpNe, V: []any{[]*big.Int{}}, R: full},
			// propertyM: [BigDecimal!]
			{F: "propertyM", O: persistent.EntityFilterOpEq, V: []any{[]decimal.Decimal{decimal0}}, R: group0},
			{F: "propertyM", O: persistent.EntityFilterOpEq, V: []any{[]decimal.Decimal{}}, R: empty},
			{F: "propertyM", O: persistent.EntityFilterOpNe, V: []any{[]decimal.Decimal{decimal0}}, R: group1},
			{F: "propertyM", O: persistent.EntityFilterOpNe, V: []any{[]decimal.Decimal{}}, R: full},
			// propertyN: [EnumA!]
			{F: "propertyN", O: persistent.EntityFilterOpEq, V: []any{[]string{"AAA"}}, R: group0},
			{F: "propertyN", O: persistent.EntityFilterOpEq, V: []any{[]string{}}, R: empty},
			{F: "propertyN", O: persistent.EntityFilterOpNe, V: []any{[]string{"AAA"}}, R: group1},
			{F: "propertyN", O: persistent.EntityFilterOpNe, V: []any{[]string{}}, R: full},
			// propertyP: [Timestamp!]
			{F: "propertyP", O: persistent.EntityFilterOpEq, V: []any{[]int64{0}}, R: group0},
			{F: "propertyP", O: persistent.EntityFilterOpEq, V: []any{[]int64{}}, R: empty},
			{F: "propertyP", O: persistent.EntityFilterOpNe, V: []any{[]int64{0}}, R: group1},
			{F: "propertyP", O: persistent.EntityFilterOpNe, V: []any{[]int64{}}, R: full},
			// propertyR: [Float!]
			{F: "propertyR", O: persistent.EntityFilterOpEq, V: []any{[]float64{0}}, R: group0},
			{F: "propertyR", O: persistent.EntityFilterOpEq, V: []any{[]float64{}}, R: empty},
			{F: "propertyR", O: persistent.EntityFilterOpNe, V: []any{[]float64{0}}, R: group1},
			{F: "propertyR", O: persistent.EntityFilterOpNe, V: []any{[]float64{}}, R: full},
			// foreignA: EntityA!
			{F: "foreignA", O: persistent.EntityFilterOpEq, V: []any{"0x0a00"}, R: group0},
			{F: "foreignA", O: persistent.EntityFilterOpEq, V: []any{"0x0a01"}, R: group1},
			{F: "foreignA", O: persistent.EntityFilterOpEq, V: []any{"0x0a02"}, R: empty},
			{F: "foreignA", O: persistent.EntityFilterOpNe, V: []any{"0x0a00"}, R: group1},
			{F: "foreignA", O: persistent.EntityFilterOpNe, V: []any{"0x0a01"}, R: group0},
			{F: "foreignA", O: persistent.EntityFilterOpNe, V: []any{"0x0a02"}, R: full},
			// foreignB: [EntityA!]
			{F: "foreignB", O: persistent.EntityFilterOpHasAll, V: []any{"0x0a00"}, R: group0},
			{F: "foreignB", O: persistent.EntityFilterOpHasAll, V: []any{"0x0a01"}, R: group1},
			{F: "foreignB", O: persistent.EntityFilterOpHasAll, V: []any{"0x0a00", "0x0a01"}, R: empty},
			{F: "foreignB", O: persistent.EntityFilterOpHasAny, V: []any{"0x0a00"}, R: group0},
			{F: "foreignB", O: persistent.EntityFilterOpHasAny, V: []any{"0x0a01"}, R: group1},
			{F: "foreignB", O: persistent.EntityFilterOpHasAny, V: []any{"0x0a00", "0x0a01"}, R: full},
		}

		for i, testcase := range testcases {
			boxes, err = s.ListEntities(ctx, entityF, chain, []persistent.EntityFilter{
				{Field: entityF.GetFieldByName(testcase.F), Op: testcase.O, Value: testcase.V},
			}, 10)
			msg := fmt.Sprintf("testcase #%d-%d %#v", fx, i, testcase)
			assert.NoError(t, err, msg)
			assert.Equal(t, testcase.R, boxes, msg)
		}

		// === with multi filter conditions

		// propertyA = '0' AND propertyB = '0'
		boxes, err = s.ListEntities(ctx, entityF, chain, []persistent.EntityFilter{
			{Field: entityF.GetFieldByName("propertyA"), Op: persistent.EntityFilterOpEq, Value: []any{"0"}},
			{Field: entityF.GetFieldByName("propertyB"), Op: persistent.EntityFilterOpEq, Value: []any{"0"}},
		}, 10)
		assert.NoError(t, err)
		assert.Equal(t, group0, boxes)
		// propertyA = '0' AND propertyB = '1'
		boxes, err = s.ListEntities(ctx, entityF, chain, []persistent.EntityFilter{
			{Field: entityF.GetFieldByName("propertyA"), Op: persistent.EntityFilterOpEq, Value: []any{"0"}},
			{Field: entityF.GetFieldByName("propertyB"), Op: persistent.EntityFilterOpEq, Value: []any{"1"}},
		}, 10)
		assert.NoError(t, err)
		assert.Equal(t, empty, boxes)
		// propertyA = '0' AND propertyB != '1'
		boxes, err = s.ListEntities(ctx, entityF, chain, []persistent.EntityFilter{
			{Field: entityF.GetFieldByName("propertyA"), Op: persistent.EntityFilterOpEq, Value: []any{"0"}},
			{Field: entityF.GetFieldByName("propertyB"), Op: persistent.EntityFilterOpNe, Value: []any{"1"}},
		}, 10)
		assert.NoError(t, err)
		assert.Equal(t, group0, boxes)
	}

}

func Test_filterWithNullValue(t *testing.T) {
	if skip {
		t.Skip("use local db, will only be executed manually locally")
	}

	const chain = "1"
	ctx := context.Background()
	es := EntitySuite{t: t}
	es.init(ctx)
	sch, s := es.s.sch, es.s

	num := big.NewInt(123)
	dec := decimal.New(1, -30)

	entities := []persistent.EntityBox{
		{
			ID: "0x1000",
			Data: map[string]any{
				"id":     "0x1000",
				"propA1": utils.WrapPointer("pa1"),
				"propB1": num,
				"propC1": &dec,
				"propA2": []any{"pa1", "pa2"},
				"propB2": []any{num},
				"propC2": []any{dec},
				"forkA1": utils.WrapPointer("0x0a00"),
				"forkA2": []*string{utils.WrapPointer("0x0a00")},
			},
			Entity:         "EntityG",
			GenBlockNumber: 10,
			GenBlockTime:   genBlockTime(10),
			GenBlockHash:   "0x000000",
			GenBlockChain:  chain,
		},
		{
			ID: "0x1001",
			Data: map[string]any{
				"id":     "0x1001",
				"propA1": (*string)(nil),
				"propB1": nil,
				"propC1": (*decimal.Decimal)(nil),
				"propA2": nil,
				"propB2": nil,
				"propC2": nil,
				"forkA1": (*string)(nil),
				"forkA2": nil,
			},
			Entity:         "EntityG",
			GenBlockNumber: 10,
			GenBlockTime:   genBlockTime(10),
			GenBlockHash:   "0x000000",
			GenBlockChain:  chain,
		},
	}

	entityG := sch.GetEntity("EntityG")
	var boxes []*persistent.EntityBox
	var err error

	_, err = s.SetEntities(ctx, entityG, entities)
	assert.NoError(t, err)

	full := utils.WrapPointerForArray(entities)
	group0 := full[:1]
	group1 := full[1:]
	var empty []*persistent.EntityBox

	testcases := []struct {
		F string
		O persistent.EntityFilterOp
		V []any
		R []*persistent.EntityBox
	}{
		// eq
		{F: "propA1", O: persistent.EntityFilterOpEq, V: []any{nil}, R: group1},
		{F: "propB1", O: persistent.EntityFilterOpEq, V: []any{nil}, R: group1},
		{F: "propC1", O: persistent.EntityFilterOpEq, V: []any{nil}, R: group1},
		{F: "propA2", O: persistent.EntityFilterOpEq, V: []any{nil}, R: group1},
		{F: "propB2", O: persistent.EntityFilterOpEq, V: []any{nil}, R: group1},
		{F: "propC2", O: persistent.EntityFilterOpEq, V: []any{nil}, R: group1},
		{F: "forkA1", O: persistent.EntityFilterOpEq, V: []any{nil}, R: group1},
		{F: "forkA2", O: persistent.EntityFilterOpEq, V: []any{nil}, R: group1},
		//---
		{F: "propA1", O: persistent.EntityFilterOpEq, V: []any{"pa1"}, R: group0},
		{F: "propB1", O: persistent.EntityFilterOpEq, V: []any{num}, R: group0},
		{F: "propC1", O: persistent.EntityFilterOpEq, V: []any{dec}, R: group0},
		{F: "propA2", O: persistent.EntityFilterOpEq, V: []any{[]string{"pa1", "pa2"}}, R: group0},
		{F: "propB2", O: persistent.EntityFilterOpEq, V: []any{[]*big.Int{num}}, R: group0},
		{F: "propC2", O: persistent.EntityFilterOpEq, V: []any{[]decimal.Decimal{dec}}, R: group0},
		{F: "forkA1", O: persistent.EntityFilterOpEq, V: []any{"0x0a00"}, R: group0},
		{F: "forkA2", O: persistent.EntityFilterOpEq, V: []any{[]string{"0x0a00"}}, R: group0},
		// ne
		{F: "propA1", O: persistent.EntityFilterOpNe, V: []any{nil}, R: group0},
		{F: "propB1", O: persistent.EntityFilterOpNe, V: []any{nil}, R: group0},
		{F: "propC1", O: persistent.EntityFilterOpNe, V: []any{nil}, R: group0},
		{F: "propA2", O: persistent.EntityFilterOpNe, V: []any{nil}, R: group0},
		{F: "propB2", O: persistent.EntityFilterOpNe, V: []any{nil}, R: group0},
		{F: "propC2", O: persistent.EntityFilterOpNe, V: []any{nil}, R: group0},
		{F: "forkA1", O: persistent.EntityFilterOpNe, V: []any{nil}, R: group0},
		{F: "forkA2", O: persistent.EntityFilterOpNe, V: []any{nil}, R: group0},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpNe, V: []any{"pa1"}, R: empty},
		{F: "propB1", O: persistent.EntityFilterOpNe, V: []any{num}, R: empty},
		{F: "propC1", O: persistent.EntityFilterOpNe, V: []any{dec}, R: empty},
		{F: "propA2", O: persistent.EntityFilterOpNe, V: []any{[]string{"pa1", "pa2"}}, R: empty},
		{F: "propB2", O: persistent.EntityFilterOpNe, V: []any{[]*big.Int{num}}, R: empty},
		{F: "propC2", O: persistent.EntityFilterOpNe, V: []any{[]decimal.Decimal{dec}}, R: empty},
		{F: "forkA1", O: persistent.EntityFilterOpNe, V: []any{"0x0a00"}, R: empty},
		{F: "forkA2", O: persistent.EntityFilterOpNe, V: []any{[]string{"0x0a00"}}, R: empty},
		// gt
		{F: "propA1", O: persistent.EntityFilterOpGt, V: []any{nil}, R: empty},
		{F: "propB1", O: persistent.EntityFilterOpGt, V: []any{nil}, R: empty},
		{F: "propC1", O: persistent.EntityFilterOpGt, V: []any{nil}, R: empty},
		{F: "forkA1", O: persistent.EntityFilterOpGt, V: []any{nil}, R: empty},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpGt, V: []any{"pa1"}, R: empty},
		{F: "propB1", O: persistent.EntityFilterOpGt, V: []any{num}, R: empty},
		{F: "propC1", O: persistent.EntityFilterOpGt, V: []any{dec}, R: empty},
		{F: "forkA1", O: persistent.EntityFilterOpGt, V: []any{"0x0a00"}, R: empty},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpGt, V: []any{"pa"}, R: group0},
		{F: "propB1", O: persistent.EntityFilterOpGt, V: []any{big.NewInt(0)}, R: group0},
		{F: "propC1", O: persistent.EntityFilterOpGt, V: []any{decimal.Zero}, R: group0},
		{F: "forkA1", O: persistent.EntityFilterOpGt, V: []any{"0x0a0"}, R: group0},
		// ge
		{F: "propA1", O: persistent.EntityFilterOpGe, V: []any{nil}, R: empty},
		{F: "propB1", O: persistent.EntityFilterOpGe, V: []any{nil}, R: empty},
		{F: "propC1", O: persistent.EntityFilterOpGe, V: []any{nil}, R: empty},
		{F: "forkA1", O: persistent.EntityFilterOpGe, V: []any{nil}, R: empty},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpGe, V: []any{"pa1"}, R: group0},
		{F: "propB1", O: persistent.EntityFilterOpGe, V: []any{num}, R: group0},
		{F: "propC1", O: persistent.EntityFilterOpGe, V: []any{dec}, R: group0},
		{F: "forkA1", O: persistent.EntityFilterOpGe, V: []any{"0x0a00"}, R: group0},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpGe, V: []any{"pa"}, R: group0},
		{F: "propB1", O: persistent.EntityFilterOpGe, V: []any{big.NewInt(0)}, R: group0},
		{F: "propC1", O: persistent.EntityFilterOpGe, V: []any{decimal.Zero}, R: group0},
		{F: "forkA1", O: persistent.EntityFilterOpGe, V: []any{"0x0a0"}, R: group0},
		// lt
		{F: "propA1", O: persistent.EntityFilterOpLt, V: []any{nil}, R: empty},
		{F: "propB1", O: persistent.EntityFilterOpLt, V: []any{nil}, R: empty},
		{F: "propC1", O: persistent.EntityFilterOpLt, V: []any{nil}, R: empty},
		{F: "forkA1", O: persistent.EntityFilterOpLt, V: []any{nil}, R: empty},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpLt, V: []any{"pa1"}, R: empty},
		{F: "propB1", O: persistent.EntityFilterOpLt, V: []any{num}, R: empty},
		{F: "propC1", O: persistent.EntityFilterOpLt, V: []any{dec}, R: empty},
		{F: "forkA1", O: persistent.EntityFilterOpLt, V: []any{"0x0a00"}, R: empty},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpLt, V: []any{"pa11"}, R: group0},
		{F: "propB1", O: persistent.EntityFilterOpLt, V: []any{big.NewInt(999)}, R: group0},
		{F: "propC1", O: persistent.EntityFilterOpLt, V: []any{decimal.NewFromFloat(111111.1111111)}, R: group0},
		{F: "forkA1", O: persistent.EntityFilterOpLt, V: []any{"0x0a000"}, R: group0},
		// le
		{F: "propA1", O: persistent.EntityFilterOpLe, V: []any{nil}, R: empty},
		{F: "propB1", O: persistent.EntityFilterOpLe, V: []any{nil}, R: empty},
		{F: "propC1", O: persistent.EntityFilterOpLe, V: []any{nil}, R: empty},
		{F: "forkA1", O: persistent.EntityFilterOpLe, V: []any{nil}, R: empty},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpLe, V: []any{"pa1"}, R: group0},
		{F: "propB1", O: persistent.EntityFilterOpLe, V: []any{num}, R: group0},
		{F: "propC1", O: persistent.EntityFilterOpLe, V: []any{dec}, R: group0},
		{F: "forkA1", O: persistent.EntityFilterOpLe, V: []any{"0x0a00"}, R: group0},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpLe, V: []any{"pa11"}, R: group0},
		{F: "propB1", O: persistent.EntityFilterOpLe, V: []any{big.NewInt(999)}, R: group0},
		{F: "propC1", O: persistent.EntityFilterOpLe, V: []any{decimal.NewFromFloat(111111.1111111)}, R: group0},
		{F: "forkA1", O: persistent.EntityFilterOpLe, V: []any{"0x0a000"}, R: group0},
		// in
		{F: "propA1", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
		{F: "propB1", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
		{F: "propC1", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
		{F: "forkA1", O: persistent.EntityFilterOpIn, V: []any{}, R: empty},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpIn, V: []any{nil}, R: group1},
		{F: "propB1", O: persistent.EntityFilterOpIn, V: []any{nil}, R: group1},
		{F: "propC1", O: persistent.EntityFilterOpIn, V: []any{nil}, R: group1},
		{F: "forkA1", O: persistent.EntityFilterOpIn, V: []any{nil}, R: group1},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpIn, V: []any{"pa1"}, R: group0},
		{F: "propB1", O: persistent.EntityFilterOpIn, V: []any{num}, R: group0},
		{F: "propC1", O: persistent.EntityFilterOpIn, V: []any{dec}, R: group0},
		{F: "forkA1", O: persistent.EntityFilterOpIn, V: []any{"0x0a00"}, R: group0},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpIn, V: []any{nil, "pa1"}, R: full},
		{F: "propB1", O: persistent.EntityFilterOpIn, V: []any{nil, num}, R: full},
		{F: "propC1", O: persistent.EntityFilterOpIn, V: []any{nil, dec}, R: full},
		{F: "forkA1", O: persistent.EntityFilterOpIn, V: []any{nil, "0x0a00"}, R: full},
		// not in
		{F: "propA1", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
		{F: "propB1", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
		{F: "propC1", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
		{F: "forkA1", O: persistent.EntityFilterOpNotIn, V: []any{}, R: full},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpNotIn, V: []any{nil}, R: group0},
		{F: "propB1", O: persistent.EntityFilterOpNotIn, V: []any{nil}, R: group0},
		{F: "propC1", O: persistent.EntityFilterOpNotIn, V: []any{nil}, R: group0},
		{F: "forkA1", O: persistent.EntityFilterOpNotIn, V: []any{nil}, R: group0},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpNotIn, V: []any{"pa1"}, R: group1},
		{F: "propB1", O: persistent.EntityFilterOpNotIn, V: []any{num}, R: group1},
		{F: "propC1", O: persistent.EntityFilterOpNotIn, V: []any{dec}, R: group1},
		{F: "forkA1", O: persistent.EntityFilterOpNotIn, V: []any{"0x0a00"}, R: group1},
		// ---
		{F: "propA1", O: persistent.EntityFilterOpNotIn, V: []any{nil, "pa1"}, R: empty},
		{F: "propB1", O: persistent.EntityFilterOpNotIn, V: []any{nil, num}, R: empty},
		{F: "propC1", O: persistent.EntityFilterOpNotIn, V: []any{nil, dec}, R: empty},
		{F: "forkA1", O: persistent.EntityFilterOpNotIn, V: []any{nil, "0x0a00"}, R: empty},
		// like
		{F: "propA1", O: persistent.EntityFilterOpLike, V: []any{nil}, R: empty},
		{F: "propA1", O: persistent.EntityFilterOpLike, V: []any{"%"}, R: group0},
		{F: "propA1", O: persistent.EntityFilterOpLike, V: []any{"pp%"}, R: empty},
		// not lke
		{F: "propA1", O: persistent.EntityFilterOpNotLike, V: []any{nil}, R: empty},
		{F: "propA1", O: persistent.EntityFilterOpNotLike, V: []any{"%"}, R: empty},
		{F: "propA1", O: persistent.EntityFilterOpNotLike, V: []any{"pp%"}, R: group0},
	}

	for i, testcase := range testcases {
		boxes, err = s.ListEntities(ctx, entityG, chain, []persistent.EntityFilter{{
			Field: entityG.GetFieldByName(testcase.F), Op: testcase.O, Value: testcase.V,
		}}, 10)
		msg := fmt.Sprintf("testcase #%d %#v", i, testcase)
		assert.NoError(t, err, msg)
		assert.Equal(t, testcase.R, boxes, msg)
		break
	}
}

func Test_interfaceForeignKeyField(t *testing.T) {
	if skip {
		t.Skip("use local db, will only be executed manually locally")
	}

	const chain = "1"
	ctx := context.Background()
	es := EntitySuite{t: t}
	es.init(ctx)
	//sch, s := es.s.sch, es.s

	es.pushData(ctx, map[string]persistent.EntityBox{
		"0x0d0100": {
			ID: "0x0d0100",
			Data: map[string]any{
				"id":        "0x0d0100",
				"propertyA": "d1pa0",
				"on":        []string{"0x0e0100"},
			},
			Entity:         "EntityD1",
			GenBlockNumber: 101,
			GenBlockTime:   genBlockTime(101),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0d0101": {
			ID: "0x0d0101",
			Data: map[string]any{
				"id":        "0x0d0101",
				"propertyA": "d1pa1",
				"on":        []string{"0x0e0101"},
			},
			Entity:         "EntityD1",
			GenBlockNumber: 102,
			GenBlockTime:   genBlockTime(102),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0d0200": {
			ID: "0x0d0200",
			Data: map[string]any{
				"id":        "0x0d0200",
				"propertyA": int32(11),
				"on":        []string{"0x0e0200"},
			},
			Entity:         "EntityD2",
			GenBlockNumber: 103,
			GenBlockTime:   genBlockTime(103),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0d0201": {
			ID: "0x0d0201",
			Data: map[string]any{
				"id":        "0x0d0201",
				"propertyA": int32(22),
				"on":        []string{"0x0e0201"},
			},
			Entity:         "EntityD2",
			GenBlockNumber: 104,
			GenBlockTime:   genBlockTime(104),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0e0100": {
			ID: "0x0e0100",
			Data: map[string]any{
				"id":   "0x0e0100",
				"from": "e1from0",
			},
			Entity:         "EntityE1",
			GenBlockNumber: 105,
			GenBlockTime:   genBlockTime(105),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0e0101": {
			ID: "0x0e0101",
			Data: map[string]any{
				"id":   "0x0e0101",
				"from": "e1from1",
			},
			Entity:         "EntityE1",
			GenBlockNumber: 106,
			GenBlockTime:   genBlockTime(106),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0e0200": {
			ID: "0x0e0200",
			Data: map[string]any{
				"id":   "0x0e0200",
				"from": "e2from0",
				"by":   []any{int32(1), int32(2), int32(3)},
				"left": big.NewInt(1111),
			},
			Entity:         "EntityE2",
			GenBlockNumber: 107,
			GenBlockTime:   genBlockTime(107),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0e0201": {
			ID: "0x0e0201",
			Data: map[string]any{
				"id":   "0x0e0201",
				"from": "e2from1",
				"by":   nil,
				"left": nil,
			},
			Entity:         "EntityE2",
			GenBlockNumber: 108,
			GenBlockTime:   genBlockTime(108),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
	})

	es.pushData(ctx, map[string]persistent.EntityBox{
		"0x0d0100": {
			ID: "0x0d0100",
			Data: map[string]any{
				"id":        "0x0d0100",
				"propertyA": "d1pa0",
				"on":        []string{"0x0e0101", "0x0e0201"},
			},
			Entity:         "EntityD1",
			GenBlockNumber: 201,
			GenBlockTime:   genBlockTime(201),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0e0100": {
			ID: "0x0e0100",
			Data: map[string]any{
				"id":   "0x0e0100",
				"from": "e1from0-2",
			},
			Entity:         "EntityE1",
			GenBlockNumber: 202,
			GenBlockTime:   genBlockTime(202),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0e0101": {
			ID: "0x0e0101",
			Data: map[string]any{
				"id":   "0x0e0101",
				"from": "e1from1-2",
			},
			Entity:         "EntityE1",
			GenBlockNumber: 203,
			GenBlockTime:   genBlockTime(203),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0e0200": {
			ID: "0x0e0200",
			Data: map[string]any{
				"id":   "0x0e0200",
				"from": "e2from0-2",
				"by":   []any{int32(1), int32(2), int32(3)},
				"left": big.NewInt(1111),
			},
			Entity:         "EntityE2",
			GenBlockNumber: 204,
			GenBlockTime:   genBlockTime(204),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0e0201": {
			ID: "0x0e0201",
			Data: map[string]any{
				"id":   "0x0e0201",
				"from": "e2from1-2",
				"by":   nil,
				"left": nil,
			},
			Entity:         "EntityE2",
			GenBlockNumber: 205,
			GenBlockTime:   genBlockTime(205),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
	})

}

func Test_viewTable(t *testing.T) {
	if skip {
		t.Skip("use local db, will only be executed manually locally")
	}

	const chain = "1"
	ctx := context.Background()
	es := EntitySuite{t: t}
	es.init(ctx)
	sch, s := es.s.sch, es.s

	maxBigInt := new(big.Int).Sub(num2e256, big.NewInt(1))
	maxBigIntAsFloat64, _ := new(big.Float).SetInt(maxBigInt).Float64()

	entities := map[string]persistent.EntityBox{
		"0x0c0000": {
			ID: "0x0c0000",
			Data: map[string]any{
				"id":        "0x0c0000",
				"propertyA": int32(100),
				"propertyB": maxBigInt,
				"propertyC": maxBigInt,
				"propertyD": decimal.New(1, -30),
				"foreignCA": "0x0a00",
				"foreignCB": "0x0b00",
			},
			Entity:         "EntityC",
			GenBlockNumber: 140,
			GenBlockTime:   genBlockTime(140),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0c0001": {
			ID: "0x0c0001",
			Data: map[string]any{
				"id":        "0x0c0001",
				"propertyA": int32(101),
				"propertyB": big.NewInt(1),
				"propertyC": nil,
				"propertyD": decimal.New(123456789, -30),
				"foreignCA": "0x0a00",
				"foreignCB": "0x0b01",
			},
			Entity:         "EntityC",
			GenBlockNumber: 150,
			GenBlockTime:   genBlockTime(150),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
		"0x0c0101": {
			ID: "0x0c0101",
			Data: map[string]any{
				"id":        "0x0c0101",
				"propertyA": int32(111),
				"propertyB": big.NewInt(-100),
				"propertyC": new(big.Int).Neg(maxBigInt),
				"propertyD": decimal.New(123456789, -30),
				"foreignCA": "0x0a01",
				"foreignCB": "0x0b01",
			},
			Entity:         "EntityC",
			GenBlockNumber: 160,
			GenBlockTime:   genBlockTime(160),
			GenBlockHash:   "0x1234",
			GenBlockChain:  chain,
		},
	}

	es.pushData(ctx, entities)

	entityCType := sch.GetEntity("EntityC")

	sql := fmt.Sprintf("SELECT propertyB, propertyC, `meta.chain` FROM %s ORDER BY %s",
		s.ViewName(entityCType), quote(schema.EntityPrimaryFieldName))
	rows, err := es.conn.Query(ctx, sql)
	assert.NoError(t, err)
	defer rows.Close()

	fieldBuffer := buildFieldBufferForScanMap(rows)
	var result []map[string]any
	for rows.Next() {
		var data map[string]any
		data, err = scanMap(rows, fieldBuffer)
		assert.NoError(t, err)
		result = append(result, data)
	}

	assert.Equal(t, []map[string]any{{
		"propertyB":  maxBigIntAsFloat64,
		"propertyC":  utils.WrapPointer(maxBigIntAsFloat64),
		"meta.chain": chain,
	}, {
		"propertyB":  float64(1),
		"propertyC":  (*float64)(nil),
		"meta.chain": chain,
	}, {
		"propertyB":  float64(-100),
		"propertyC":  utils.WrapPointer(-maxBigIntAsFloat64),
		"meta.chain": chain,
	}}, result)

}

func Test_timeFormat(t *testing.T) {
	assert.Equal(t, "20260107021606", time.Unix(1767752166, 0).Format(timeLayoutAllDigital))
}
