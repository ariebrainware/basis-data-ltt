package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	redisClient *redis.Client
	redisOnce   sync.Once
	redisMutex  sync.Mutex
)

// ConnectRedis initializes a singleton Redis client based on environment variables.
// Returns the client (or nil) and an error if connection/ping failed.
func ConnectRedis() (*redis.Client, error) {
	var err error
	// If Redis is not explicitly enabled via env, don't consume the once
	// so tests can enable/reset the client via helpers.
	if os.Getenv("REDIS_ENABLED") != "true" {
		return nil, nil
	}

	// Use a mutex-protected lazy init so tests can call ConnectRedis multiple
	// times with different env settings without being affected by sync.Once.
	redisMutex.Lock()
	defer redisMutex.Unlock()

	if redisClient != nil {
		return redisClient, nil
	}

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	// Accept either REDIS_PASSWORD (preferred) or legacy REDIS_PASS
	pass := os.Getenv("REDIS_PASSWORD")
	if pass == "" {
		pass = os.Getenv("REDIS_PASS")
	}
	dbNum := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if v, e := strconv.Atoi(dbStr); e == nil {
			dbNum = v
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       dbNum,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err = rdb.Ping(ctx).Err(); err != nil {
		redisClient = nil
		err = fmt.Errorf("redis ping failed: %w", err)
		return redisClient, err
	}

	redisClient = rdb
	log.Printf("Connected to Redis at %s", addr)
	return redisClient, err
}

// GetRedisClient returns the initialized Redis client (may be nil if ConnectRedis failed or not called).
func GetRedisClient() *redis.Client {
	return redisClient
}

// SetRedisClientForTesting allows tests to inject a mock Redis client.
// This should only be used in tests.
func SetRedisClientForTesting(client *redis.Client) {
	redisClient = client
}

// ResetRedisClientForTesting clears and closes the singleton Redis client.
// Intended for use in tests to avoid cross-test interference.
func ResetRedisClientForTesting() {
	redisMutex.Lock()
	defer redisMutex.Unlock()
	if redisClient != nil {
		_ = redisClient.Close()
	}
	redisClient = nil
}
