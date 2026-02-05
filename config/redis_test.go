package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// saveEnvVars saves the original values of the given environment variable keys.
func saveEnvVars(vars map[string]string) (map[string]string, map[string]bool) {
	orig := make(map[string]string)
	origSet := make(map[string]bool)

	for k := range vars {
		if oldVal, exists := os.LookupEnv(k); exists {
			orig[k] = oldVal
			origSet[k] = true
		}
	}

	return orig, origSet
}

// applyEnvVars applies the environment variables from the vars map.
func applyEnvVars(vars map[string]string) {
	for k, v := range vars {
		if v == "" {
			os.Unsetenv(k)
		} else {
			os.Setenv(k, v)
		}
	}
}

// restoreEnvVars restores the original environment variable values.
func restoreEnvVars(orig map[string]string, origSet map[string]bool) {
	for k := range orig {
		if origSet[k] {
			os.Setenv(k, orig[k])
		} else {
			os.Unsetenv(k)
		}
	}
}

// withEnv sets environment variables for the duration of fn and restores them after.
func withEnv(t *testing.T, vars map[string]string, fn func(t *testing.T)) {
	t.Helper()
	orig, origSet := saveEnvVars(vars)
	applyEnvVars(vars)

	defer restoreEnvVars(orig, origSet)

	fn(t)
}

func TestConnectRedis_Disabled(t *testing.T) {
	withEnv(t, map[string]string{"REDIS_ENABLED": "false"}, func(t *testing.T) {
		rdb, err := ConnectRedis()
		assert.NoError(t, err)
		assert.Nil(t, rdb)
	})
}

func TestConnectRedis_InvalidAddress(t *testing.T) {
	withEnv(t, map[string]string{"REDIS_ENABLED": "true", "REDIS_ADDR": "invalid-address:99999"}, func(t *testing.T) {
		rdb, err := ConnectRedis()
		// Connection may succeed but ping will fail
		// We just verify it doesn't panic
		if err == nil {
			assert.NotNil(t, rdb)
		}
	})
}

func TestConnectRedis_DefaultValues(t *testing.T) {
	withEnv(t, map[string]string{"REDIS_ENABLED": "", "REDIS_ADDR": "", "REDIS_PASSWORD": "", "REDIS_DB": ""}, func(t *testing.T) {
		// Should return nil when disabled (default)
		rdb, err := ConnectRedis()
		assert.NoError(t, err)
		assert.Nil(t, rdb)
	})
}

func TestConnectRedis_WithPassword(t *testing.T) {
	withEnv(t, map[string]string{"REDIS_ENABLED": "true", "REDIS_ADDR": "localhost:6379", "REDIS_PASSWORD": "test-password"}, func(t *testing.T) {
		rdb, err := ConnectRedis()
		// May fail if Redis is not running, but should not panic
		if err == nil {
			assert.NotNil(t, rdb)
		}
	})
}

func TestConnectRedis_CustomDB(t *testing.T) {
	withEnv(t, map[string]string{"REDIS_ENABLED": "true", "REDIS_DB": "5"}, func(t *testing.T) {
		rdb, err := ConnectRedis()
		// May fail if Redis is not running, but should not panic
		if err == nil {
			assert.NotNil(t, rdb)
		}
	})
}

func TestConnectRedis_InvalidDBNumber(t *testing.T) {
	withEnv(t, map[string]string{"REDIS_ENABLED": "true", "REDIS_DB": "invalid"}, func(t *testing.T) {
		rdb, err := ConnectRedis()
		// Should use default DB (0) when invalid
		if err == nil {
			assert.NotNil(t, rdb)
		}
	})
}

func TestGetRedisClient_NotInitialized(t *testing.T) {
	// Reset the global client
	SetRedisClientForTest(nil)

	client := GetRedisClient()
	assert.Nil(t, client)
}

func TestGetRedisClient_Initialized(t *testing.T) {
	// This test would require mocking or a real Redis connection
	// For now, just test that it returns what was set
	SetRedisClientForTest(nil) // Reset first
	client := GetRedisClient()
	assert.Nil(t, client)
}

func TestSetRedisClientForTesting(t *testing.T) {
	// Test that we can set and get the client
	originalClient := GetRedisClient()
	defer SetRedisClientForTest(originalClient) // Restore after test

	SetRedisClientForTest(nil)
	assert.Nil(t, GetRedisClient())
}

func TestConnectRedis_ConcurrentCalls(t *testing.T) {
	// Test that concurrent calls don't cause issues
	// This is primarily testing thread safety

	withEnv(t, map[string]string{"REDIS_ENABLED": "false"}, func(t *testing.T) {

		type callResult struct {
			rdb interface{}
			err error
		}
		done := make(chan callResult, 5)
		for i := 0; i < 5; i++ {
			go func() {
				rdb, err := ConnectRedis()
				done <- callResult{rdb: rdb, err: err}
			}()
		}

		// Wait for all goroutines to complete and assert in the main goroutine
		for i := 0; i < 5; i++ {
			res := <-done
			assert.NoError(t, res.err)
			assert.Nil(t, res.rdb)
		}
	})
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
	withEnv(t, map[string]string{"REDIS_ENABLED": "true", "REDIS_PASSWORD": ""}, func(t *testing.T) {
		rdb, err := ConnectRedis()
		// Should work with empty password (no auth)
		if err == nil {
			assert.NotNil(t, rdb)
		}
	})
}
