package repository

import (
	"github.com/redis/go-redis/v9"
)

// RedisRepositoryFactory creates Redis-based repository implementations
type RedisRepositoryFactory struct {
	client *redis.Client
}

// NewRedisRepositoryFactory creates a new factory for Redis repositories
func NewRedisRepositoryFactory(redisAddr string, poolSize int, db int) (*RedisRepositoryFactory, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		PoolSize: poolSize,
		DB:       db,
	})

	return &RedisRepositoryFactory{
		client: client,
	}, nil
}

// NewRedisRepositoryFactoryWithClient creates a factory with an existing Redis client
func NewRedisRepositoryFactoryWithClient(client *redis.Client) *RedisRepositoryFactory {
	return &RedisRepositoryFactory{
		client: client,
	}
}

// CreateProcessorRepo creates a new Redis-based ProcessorRepo
func (f *RedisRepositoryFactory) CreateProcessorRepo() RedisProcessorRepoInterface {
	return NewRedisProcessorRepo(f.client)
}

// CreateChainStateRepo creates a new Redis-based ChainStateRepo
func (f *RedisRepositoryFactory) CreateChainStateRepo() RedisChainStateRepoInterface {
	return NewRedisChainStateRepo(f.client)
}

// GetClient returns the underlying Redis client
func (f *RedisRepositoryFactory) GetClient() *redis.Client {
	return f.client
}

// Close closes the Redis client connection
func (f *RedisRepositoryFactory) Close() error {
	return f.client.Close()
}
