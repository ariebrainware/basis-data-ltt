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
	os.Exit(m.Run())
}
