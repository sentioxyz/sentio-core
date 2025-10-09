package redis

import (
	"flag"
	"sentioxyz/sentio-core/common/log"
	"strings"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

var Address = flag.String("redis", "localhost:6379", "The redis address, or sentinel addresses separated by commas")
var PoolSize = flag.Int("redis-pool", 100, "The redis connection pool size")
var SentinelName = flag.String("redis-sentinel-master", "mymaster", "The sentinel master name")

type Options struct {
	PoolSize int
	DB       int
}

func NewClientWithDefaultOptions() *redis.Client {
	return NewClient(*Address, Options{})
}

func NewClient(address string, options Options) *redis.Client {
	var client *redis.Client
	addresses := strings.Split(address, ",")

	if options.PoolSize == 0 {
		options.PoolSize = *PoolSize
	}

	if len(addresses) == 1 {
		client = redis.NewClient(&redis.Options{
			Addr:     address,
			PoolSize: options.PoolSize,
			DB:       options.DB,
		})
	} else {
		client = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    *SentinelName,
			SentinelAddrs: addresses,
			PoolSize:      options.PoolSize,
			DB:            options.DB,
		})
	}
	err := redisotel.InstrumentTracing(client)
	if err != nil {
		log.Errore(err, "Error setting up redis tracing")
	}
	return client
}
