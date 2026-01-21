package config

import (
	"sync"

	"github.com/redis/go-redis/v9"
)

// SetRedisClientForTest sets the Redis client for testing purposes.
// This function is only available for testing and should not be used in production code.
func SetRedisClientForTest(client *redis.Client) {
	redisClient = client
}

// ResetRedisClientForTest resets the Redis client singleton for testing purposes.
// This function is only available for testing and should not be used in production code.
func ResetRedisClientForTest() {
	redisClient = nil
	redisOnce = sync.Once{}
}
