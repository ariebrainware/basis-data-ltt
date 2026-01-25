package endpoint_test

import (
	"os"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

// TestMain sets up consistent test configuration for all tests in the endpoint_test package.
// This prevents test order dependency issues caused by the singleton config pattern.
func TestMain(m *testing.M) {
	// Set consistent environment variables for all tests
	os.Setenv("APPENV", "test")
	os.Setenv("JWTSECRET", "test-secret-123")
	os.Setenv("APITOKEN", "test-api-token")
	os.Setenv("GINMODE", "release")

	// Initialize util's JWT secret
	util.SetJWTSecret("test-secret-123")

	// Initialize the singleton config once before any tests run
	cfg := config.LoadConfig()

	// Set Gin mode from initialized config
	gin.SetMode(cfg.GinMode)

	// Run all tests
	exitCode := m.Run()

	// Exit with the test result code
	os.Exit(exitCode)
}
