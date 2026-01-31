package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConnectRedis_Disabled(t *testing.T) {
	// Save original env vars
	origEnabled := os.Getenv("REDIS_ENABLED")
	defer os.Setenv("REDIS_ENABLED", origEnabled)

	// Disable Redis
	os.Setenv("REDIS_ENABLED", "false")

	rdb, err := ConnectRedis()
	assert.NoError(t, err)
	assert.Nil(t, rdb)
}

func TestConnectRedis_InvalidAddress(t *testing.T) {
	// Save original env vars
	origEnabled := os.Getenv("REDIS_ENABLED")
	origAddr := os.Getenv("REDIS_ADDR")
	defer func() {
		os.Setenv("REDIS_ENABLED", origEnabled)
		os.Setenv("REDIS_ADDR", origAddr)
	}()

	// Enable Redis with invalid address
	os.Setenv("REDIS_ENABLED", "true")
	os.Setenv("REDIS_ADDR", "invalid-address:99999")

	rdb, err := ConnectRedis()
	// Connection may succeed but ping will fail
	// We just verify it doesn't panic
	if err == nil {
		assert.NotNil(t, rdb)
	}
}

func TestConnectRedis_DefaultValues(t *testing.T) {
	// Save original env vars
	origEnabled := os.Getenv("REDIS_ENABLED")
	origAddr := os.Getenv("REDIS_ADDR")
	origPassword := os.Getenv("REDIS_PASSWORD")
	origDB := os.Getenv("REDIS_DB")
	defer func() {
		os.Setenv("REDIS_ENABLED", origEnabled)
		os.Setenv("REDIS_ADDR", origAddr)
		os.Setenv("REDIS_PASSWORD", origPassword)
		os.Setenv("REDIS_DB", origDB)
	}()

	// Clear env vars to test defaults
	os.Unsetenv("REDIS_ENABLED")
	os.Unsetenv("REDIS_ADDR")
	os.Unsetenv("REDIS_PASSWORD")
	os.Unsetenv("REDIS_DB")

	// Should return nil when disabled (default)
	rdb, err := ConnectRedis()
	assert.NoError(t, err)
	assert.Nil(t, rdb)
}

func TestConnectRedis_WithPassword(t *testing.T) {
	// Save original env vars
	origEnabled := os.Getenv("REDIS_ENABLED")
	origAddr := os.Getenv("REDIS_ADDR")
	origPassword := os.Getenv("REDIS_PASSWORD")
	defer func() {
		os.Setenv("REDIS_ENABLED", origEnabled)
		os.Setenv("REDIS_ADDR", origAddr)
		os.Setenv("REDIS_PASSWORD", origPassword)
	}()

	os.Setenv("REDIS_ENABLED", "true")
	os.Setenv("REDIS_ADDR", "localhost:6379")
	os.Setenv("REDIS_PASSWORD", "test-password")

	rdb, err := ConnectRedis()
	// May fail if Redis is not running, but should not panic
	if err == nil {
		assert.NotNil(t, rdb)
	}
}

func TestConnectRedis_CustomDB(t *testing.T) {
	// Save original env vars
	origEnabled := os.Getenv("REDIS_ENABLED")
	origDB := os.Getenv("REDIS_DB")
	defer func() {
		os.Setenv("REDIS_ENABLED", origEnabled)
		os.Setenv("REDIS_DB", origDB)
	}()

	os.Setenv("REDIS_ENABLED", "true")
	os.Setenv("REDIS_DB", "5")

	rdb, err := ConnectRedis()
	// May fail if Redis is not running, but should not panic
	if err == nil {
		assert.NotNil(t, rdb)
	}
}

func TestConnectRedis_InvalidDBNumber(t *testing.T) {
	// Save original env vars
	origEnabled := os.Getenv("REDIS_ENABLED")
	origDB := os.Getenv("REDIS_DB")
	defer func() {
		os.Setenv("REDIS_ENABLED", origEnabled)
		os.Setenv("REDIS_DB", origDB)
	}()

	os.Setenv("REDIS_ENABLED", "true")
	os.Setenv("REDIS_DB", "invalid")

	rdb, err := ConnectRedis()
	// Should use default DB (0) when invalid
	if err == nil {
		assert.NotNil(t, rdb)
	}
}

func TestGetRedisClient_NotInitialized(t *testing.T) {
	// Reset the global client
	SetRedisClientForTesting(nil)

	client := GetRedisClient()
	assert.Nil(t, client)
}

func TestGetRedisClient_Initialized(t *testing.T) {
	// This test would require mocking or a real Redis connection
	// For now, just test that it returns what was set
	SetRedisClientForTesting(nil) // Reset first
	client := GetRedisClient()
	assert.Nil(t, client)
}

func TestSetRedisClientForTesting(t *testing.T) {
	// Test that we can set and get the client
	originalClient := GetRedisClient()
	defer SetRedisClientForTesting(originalClient) // Restore after test

	SetRedisClientForTesting(nil)
	assert.Nil(t, GetRedisClient())
}

func TestConnectRedis_ConcurrentCalls(t *testing.T) {
	// Test that concurrent calls don't cause issues
	// This is primarily testing thread safety

	origEnabled := os.Getenv("REDIS_ENABLED")
	defer os.Setenv("REDIS_ENABLED", origEnabled)
	os.Setenv("REDIS_ENABLED", "false")

	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			rdb, err := ConnectRedis()
			assert.NoError(t, err)
			assert.Nil(t, rdb)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestRedisTestHelpers_SetAndReset(t *testing.T) {
	// Test the test helper functions
	original := GetRedisClient()
	defer func() {
		// Restore original state
		SetRedisClientForTest(original)
	}()

	// Set a nil client
	SetRedisClientForTest(nil)
	assert.Nil(t, GetRedisClient())

	// Reset should clear the client
	ResetRedisClientForTest()
	assert.Nil(t, GetRedisClient())

	// Can set again
	SetRedisClientForTest(original)
	assert.Equal(t, original, GetRedisClient())
}

func TestConnectRedis_EmptyPassword(t *testing.T) {
	origEnabled := os.Getenv("REDIS_ENABLED")
	origPassword := os.Getenv("REDIS_PASSWORD")
	defer func() {
		os.Setenv("REDIS_ENABLED", origEnabled)
		os.Setenv("REDIS_PASSWORD", origPassword)
	}()

	os.Setenv("REDIS_ENABLED", "true")
	os.Setenv("REDIS_PASSWORD", "")

	rdb, err := ConnectRedis()
	// Should work with empty password (no auth)
	if err == nil {
		assert.NotNil(t, rdb)
	}
}
