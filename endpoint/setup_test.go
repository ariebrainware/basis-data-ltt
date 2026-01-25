package endpoint_test

import (
	"os"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

const testJWTSecret = "test-secret-123"

// TestMain sets up consistent test configuration for all tests in the endpoint_test package.
// This prevents test order dependency issues caused by the singleton config pattern.
//
// IMPORTANT: This function calls os.Exit(), which bypasses deferred cleanup functions and
// prevents test coverage reports from being written. This is standard Go testing practice.
// Individual tests MUST handle their own cleanup using t.Cleanup() to ensure proper resource
// management. This is especially critical since tests share an in-memory SQLite database
// (via cache=shared DSN) - failing to clean up tables can cause test contamination.
func TestMain(m *testing.M) {
	// Set consistent environment variables for all tests
	os.Setenv("APPENV", "test")
	os.Setenv("JWTSECRET", testJWTSecret)
	os.Setenv("APITOKEN", "test-api-token")
	os.Setenv("GINMODE", "release")

	// Initialize util's JWT secret
	util.SetJWTSecret(testJWTSecret)

	// Initialize the singleton config once before any tests run
	// This ensures ConnectMySQL() uses consistent config values
	config.LoadConfig()

	// Set Gin mode to match the environment variable
	gin.SetMode("release")

	// Run all tests and exit with the result code
	// Note: os.Exit bypasses deferred functions - tests must use t.Cleanup()
	os.Exit(m.Run())
}
