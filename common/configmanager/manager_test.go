package configmanager

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/db"

	"github.com/alicebob/miniredis/v2"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/knadh/koanf/parsers/json"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type ManagerSuite struct {
	suite.Suite
	pgPort    int
	pg        *embeddedpostgres.EmbeddedPostgres
	redisPort string
	db        *gorm.DB
	redis     *redis.Client
}

func (s *ManagerSuite) SetupSuite() {
	s.pgPort = rand.Intn(1000) + 5000
	s.pg = embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Username("default").
		Password("password").
		Database("default").
		Port(uint32(s.pgPort)))
	err := s.pg.Start()
	if err != nil {
		panic(err)
	}
	r, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	s.redisPort = r.Port()

	s.db = db.ConnectDB("postgres://default:password@localhost:" + strconv.FormatInt(int64(s.pgPort), 10) + "/default?sslmode=disable")
	if err := s.db.AutoMigrate(&SentioConfig{}); err != nil {
		panic(err)
	}
	for _, c := range pgInitializedConfig {
		_ = s.db.Create(&c)
	}
	s.redis = redis.NewClient(&redis.Options{
		Addr:     "localhost:" + s.redisPort,
		Password: "",
		DB:       0,
	})
	if err := s.redis.HSet(context.Background(), RedisDefaultCategory, "redis_test", `{"key": "value"}`).Err(); err != nil {
		panic(err)
	}
}

func (s *ManagerSuite) TearDownSuite() {
	_ = s.pg.Stop()
	_ = s.redis.Close()
}

func (s *ManagerSuite) SetupTest() {
	_ = Shutdown()
}

func Test_ManagerSuite(t *testing.T) {
	suite.Run(t, new(ManagerSuite))
}

func (s *ManagerSuite) Test_Init() {
	c := Get("config1")
	s.Nil(c)
}

func (s *ManagerSuite) Test_Set() {
	s.Nil(Set("config1", NewPgProvider(s.db, WithPgKey("test_1")), json.Parser(), &LoadParams{}))
	s.Nil(Set("config2", NewPgProvider(s.db, WithPgKey("test_3")), json.Parser(), &LoadParams{}))
	s.NotNil(Set("config2", NewPgProvider(s.db, WithPgKey("redis_test")), json.Parser(), &LoadParams{}))
}

func (s *ManagerSuite) Test_Get() {
	s.Nil(Set("config1", NewPgProvider(s.db, WithPgKey("test_1")), json.Parser(), &LoadParams{}))
	c := Get("config1")
	s.NotNil(c)
	s.EqualValues(1, c.Int64("num_1"))
	s.EqualValues(2, c.MustInt64("num_2"))

	s.Nil(Set("config2", NewPgProvider(s.db, WithPgKey("test_3")), json.Parser(), &LoadParams{}))
	c = Get("config2")
	s.NotNil(c)
	s.EqualValues("a", c.String("str_1"))

	s.Nil(Set("r_config", NewRedisProvider(s.redis, WithRedisKey("redis_test")), json.Parser(), &LoadParams{}))
	c = Get("r_config")
	s.NotNil(c)
	s.EqualValues("value", c.String("key"))
}

func (s *ManagerSuite) Test_Refresh() {
	s.Nil(Set("config1", NewPgProvider(s.db, WithPgKey("test_1")), json.Parser(), &LoadParams{
		EnableReload: true,
		ReloadPeriod: time.Second,
	}))

	s.db.Model(&SentioConfig{}).Where("key = ?", "test_1").Update("data", `{"num_1": 3}`)
	time.Sleep(time.Second * 3)
	c := Get("config1")
	s.NotNil(c)
	s.EqualValues(3, c.Int64("num_1"))

	s.db.Model(&SentioConfig{}).Where("key = ?", "test_1").Update("data", `{"num_1": 4}`)
	time.Sleep(time.Second * 3)
	c = Get("config1")
	s.NotNil(c)
	s.EqualValues(4, c.Int64("num_1"))

	s.Nil(Set("r_config", NewRedisProvider(s.redis, WithRedisKey("redis_test")), json.Parser(), &LoadParams{
		EnableReload: true,
		ReloadPeriod: time.Second,
	}))
	c = Get("r_config")
	s.NotNil(c)
	s.EqualValues("value", c.String("key"))

	s.redis.HSet(context.Background(), RedisDefaultCategory, "redis_test", `{"key": "value2"}`)
	time.Sleep(time.Second * 3)
	c = Get("r_config")
	s.NotNil(c)
	s.EqualValues("value2", c.String("key"))
}
