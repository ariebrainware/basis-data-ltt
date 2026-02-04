package endpoint

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// EndpointTestModels defines the standard set of models migrated for endpoint tests
var EndpointTestModels = []interface{}{
	&model.Patient{},
	&model.Disease{},
	&model.User{},
	&model.Session{},
	&model.Therapist{},
	&model.Role{},
	&model.Treatment{},
	&model.PatientCode{},
}

// setupEndpointTestDB initializes a test database with all standard models migrated.
// It sets the APPENV to "test" and initializes the JWT secret for the test.
// Cleanup is automatically registered via t.Cleanup().
func setupEndpointTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// Set test environment
	t.Setenv("APPENV", "test")
	t.Setenv("JWTSECRET", "test-secret-123")
	util.SetJWTSecret("test-secret-123")

	// Connect to test database
	db, err := config.ConnectMySQL()
	if err != nil {
		t.Fatalf("failed to connect test DB: %v", err)
	}

	// Migrate all standard endpoint test models
	if err := db.AutoMigrate(EndpointTestModels...); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	// Clean up all tables
	for _, m := range EndpointTestModels {
		db.Where("1 = 1").Delete(m)
	}

	// Register cleanup
	t.Cleanup(func() {
		for _, m := range EndpointTestModels {
			_ = db.Migrator().DropTable(m)
		}
	})

	return db
}

// setupEndpointTest returns a Gin engine and database connection configured for endpoint tests.
// It initializes a test database with all standard models migrated.
func setupEndpointTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := setupEndpointTestDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))
	return r, db
}

// newTestRouter returns a new Gin engine configured for tests.
// Use this for tests that don't need a DB injected.
func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// assertStatus asserts that the response HTTP status code matches the expected value
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	assert.Equal(t, expected, w.Code)
}

// assertSuccessResponse asserts that the response indicates success with HTTP 200
func assertSuccessResponse(t *testing.T, w *httptest.ResponseRecorder, response map[string]interface{}) {
	t.Helper()
	assert.Equal(t, http.StatusOK, w.Code)
	if response == nil {
		return
	}
	if success, ok := response["success"].(bool); ok {
		assert.True(t, success)
	}
}
