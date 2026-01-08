package ckhmanager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/log"

	"github.com/stretchr/testify/suite"
)

type Suite struct {
	suite.Suite
	config Config
}

func (s *Suite) SetupSuite() {
	s.config = loadConfig("testdata/test_config.yaml")
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (s *Suite) LocalDockerClickhouse() {
	m := LoadManager("testdata/test_config.yaml")
	s.NotNil(m)

	shard := m.GetShardByIndex(0)
	s.NotNil(shard)
	shard = m.GetShardByName("shard-1")
	s.NotNil(shard)

	conn, err := shard.GetConn(WithCategory(SentioCategory), WithRole(DefaultRole), WithInternalOnly(true))
	s.NoError(err)
	s.NotNil(conn)

	db := fmt.Sprintf("sentio_test_%d", time.Now().UnixNano())
	table := "t"

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	createDBErr := conn.Exec(ctx, "CREATE DATABASE IF NOT EXISTS "+db)
	s.Nil(createDBErr)

	s.Nil(conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS "+db+"."+table+" (id UInt64, v String) ENGINE = MergeTree ORDER BY id"))
	s.Nil(conn.Exec(ctx, "INSERT INTO "+db+"."+table+" (id, v) VALUES (?, ?)", uint64(1), "hello"))
	var got string
	row := conn.QueryRow(ctx, "SELECT v FROM "+db+"."+table+" WHERE id = ?", uint64(1))
	s.Nil(row.Scan(&got))
	s.EqualValues("hello", got)

	s.Nil(conn.Exec(ctx, "DROP TABLE IF EXISTS "+db+"."+table))
	s.Nil(conn.Exec(ctx, "DROP DATABASE IF EXISTS "+db))
	conn.Close()
}

func (s *Suite) TestManager_All() {
	m := NewManager(s.config)
	shards := m.All()
	s.EqualValues(2, len(shards))
}

func (s *Suite) TestManager_ConfigValueChecker() {
	m := NewManager(s.config)
	shards := m.All()
	s.EqualValues(2, len(shards))

	for _, shard := range shards {
		s.NotNil(shard.GetConn(WithCategory(SentioCategory), WithRole(DefaultRole), WithInternalOnly(true)))
		s.NotNil(shard.GetConn(WithCategory(SentioCategory), WithRole(DefaultRole), WithInternalOnly(false)))
		s.NotNil(shard.GetConn(WithCategory(SubgraphCategory), WithRole(DefaultRole), WithInternalOnly(true)))
		s.NotNil(shard.GetConn(WithCategory(SubgraphCategory), WithRole(DefaultRole), WithInternalOnly(false)))
		s.NotNil(shard.GetConn(WithCategory(SentioCategory), WithRole(SmallEngineRole), WithInternalOnly(true)))
		s.NotNil(shard.GetConn(WithCategory(SentioCategory), WithRole(SmallEngineRole), WithInternalOnly(false)))
		s.NotNil(shard.GetConn(WithCategory(SubgraphCategory), WithRole(SmallEngineRole), WithInternalOnly(true)))
		s.NotNil(shard.GetConn(WithCategory(SubgraphCategory), WithRole(SmallEngineRole), WithInternalOnly(false)))
		s.NotNil(shard.GetConn(WithCategory(SentioCategory), WithRole(MediumEngineRole), WithInternalOnly(true), WithUnderlyingProxy(true)))
		s.NotNil(shard.GetConn(WithCategory(SentioCategory), WithRole(MediumEngineRole), WithInternalOnly(false), WithUnderlyingProxy(true)))
		s.NotNil(shard.GetConn(WithCategory(SubgraphCategory), WithRole(MediumEngineRole), WithInternalOnly(true), WithUnderlyingProxy(true)))
		s.NotNil(shard.GetConn(WithCategory(SubgraphCategory), WithRole(MediumEngineRole), WithInternalOnly(false), WithUnderlyingProxy(true)))
		s.NotNil(shard.GetConn(WithCategory(SentioCategory),
			WithRole(LargeEngineRole),
			WithInternalOnly(true),
			WithUnderlyingProxy(true),
			WithSign("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")))
		s.NotNil(shard.GetConn(WithCategory(SubgraphCategory),
			WithRole(LargeEngineRole),
			WithInternalOnly(true),
			WithUnderlyingProxy(true),
			WithSign("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")))
	}
}

func (s *Suite) TestManager_GetAllConn() {
	m := NewManager(s.config)

	allCategoryConn := m.GetShardByIndex(m.DefaultIndex()).GetAllConn(WithCategory(AllCategory))
	s.EqualValues(14, len(allCategoryConn))
	for name := range allCategoryConn {
		log.Infof("got connection name: %s", name)
	}

	sentioCategoryConn := m.GetShardByIndex(m.DefaultIndex()).GetAllConn(WithCategory(SentioCategory))
	s.EqualValues(7, len(sentioCategoryConn))

	subgraphCategoryConn := m.GetShardByIndex(m.DefaultIndex()).GetAllConn(WithCategory(SubgraphCategory))
	s.EqualValues(7, len(subgraphCategoryConn))
}
