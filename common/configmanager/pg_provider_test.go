package configmanager

import (
	"math/rand"
	"strconv"
	"testing"

	"sentioxyz/sentio-core/common/db"

	"github.com/fergusstrange/embedded-postgres"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

var (
	pgInitializedConfig = []SentioConfig{
		{
			Key:  "test_1",
			Data: []byte(`{"num_1": 1, "num_2": 2}`),
		},
		{
			Key:  "test_2",
			Data: []byte(`{"num_1": 3, "num_2": 4}`),
		},
		{
			Key:  "test_3",
			Data: []byte(`{"str_1": "a"}`),
		},
		{
			Key:  "test_4",
			Data: []byte(`{"str_1": "b"}`),
		},
	}
)

type PgProviderSuite struct {
	suite.Suite
	pg   *embeddedpostgres.EmbeddedPostgres
	db   *gorm.DB
	port int
}

func (s *PgProviderSuite) SetupSuite() {
	s.port = rand.Intn(1000) + 5000
	s.pg = embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Username("default").
		Password("password").
		Database("default").
		Port(uint32(s.port)))
	err := s.pg.Start()
	if err != nil {
		panic(err)
	}
	s.db = db.ConnectDB("postgres://default:password@localhost:" + strconv.FormatInt(int64(s.port), 10) + "/default?sslmode=disable")
	if err := s.db.AutoMigrate(&SentioConfig{}); err != nil {
		panic(err)
	}
	for _, c := range pgInitializedConfig {
		_ = s.db.Create(&c)
	}
}

func (s *PgProviderSuite) TearDownSuite() {
	_ = s.pg.Stop()
}

func Test_PgProviderSuite(t *testing.T) {
	suite.Run(t, new(PgProviderSuite))
}

func (s *PgProviderSuite) Test_ReadInitialConfig() {
	provider1 := NewPgProvider(s.db, WithPgKey("test_1"))
	data, err := provider1.ReadBytes()
	s.Nil(err)
	s.EqualValues(pgInitializedConfig[0].Data, data)

	provider2 := NewPgProvider(s.db, WithPgKey("test_2"))
	data, err = provider2.ReadBytes()
	s.Nil(err)
	s.EqualValues(pgInitializedConfig[1].Data, data)

	provider3 := NewPgProvider(s.db, WithPgKey("test_3"))
	data, err = provider3.ReadBytes()
	s.Nil(err)
	s.EqualValues(pgInitializedConfig[2].Data, data)

	provider4 := NewPgProvider(s.db, WithPgKey("test_4"))
	data, err = provider4.ReadBytes()
	s.Nil(err)
	s.EqualValues(pgInitializedConfig[3].Data, data)
}
