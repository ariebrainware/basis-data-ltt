package endpoint

import (
	"net/http"
	"testing"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// setupTokenTestDB sets up a database with all necessary migrations for token tests
func setupTokenTestDB(t *testing.T) *gorm.DB {
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

	// Migrate all necessary tables
	testModels := []interface{}{
		&model.Patient{},
		&model.Disease{},
		&model.User{},
		&model.Session{},
		&model.Therapist{},
		&model.Role{},
		&model.Treatment{},
		&model.PatientCode{},
	}

	if err := db.AutoMigrate(testModels...); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	// Clean up all tables
	for _, model := range testModels {
		db.Where("1 = 1").Delete(model)
	}

	// Register cleanup
	t.Cleanup(func() {
		for _, model := range testModels {
			_ = db.Migrator().DropTable(model)
		}
	})

	return db
}

func TestValidateToken_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTokenTestDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))

	// Create test data
	role := model.Role{Name: "Admin"}
	db.Create(&role)

	user := model.User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := model.Session{
		UserID:       user.ID,
		SessionToken: "valid-token-123",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)

	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/token/validate", requestPath: "/token/validate", handler: ValidateToken, headers: map[string]string{"session-token": "valid-token-123"}})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, response["success"].(bool))

	if data, ok := response["data"].(map[string]interface{}); ok {
		assert.Equal(t, "Admin", data["role"])
	}
}

func TestValidateToken_MissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTokenTestDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))

	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/token/validate", requestPath: "/token/validate", handler: ValidateToken})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, response["error"].(string), "Invalid session token")
}

func TestValidateToken_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTokenTestDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))

	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/token/validate", requestPath: "/token/validate", handler: ValidateToken, headers: map[string]string{"session-token": "invalid-token"}})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, response["error"].(string), "Session not found")
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTokenTestDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))

	// Create test data with expired session
	role := model.Role{Name: "Admin"}
	db.Create(&role)

	user := model.User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := model.Session{
		UserID:       user.ID,
		SessionToken: "expired-token-123",
		ExpiresAt:    time.Now().Add(-time.Hour), // Expired
	}
	db.Create(&session)

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/token/validate", requestPath: "/token/validate", handler: ValidateToken, headers: map[string]string{"session-token": "expired-token-123"}})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestValidateToken_SoftDeletedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTokenTestDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))

	// Create test data with soft-deleted session
	role := model.Role{Name: "Admin"}
	db.Create(&role)

	user := model.User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := model.Session{
		UserID:       user.ID,
		SessionToken: "deleted-token-123",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)
	db.Delete(&session) // Soft delete

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/token/validate", requestPath: "/token/validate", handler: ValidateToken, headers: map[string]string{"session-token": "deleted-token-123"}})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestValidateToken_SoftDeletedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTokenTestDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))

	// Create test data with soft-deleted user
	role := model.Role{Name: "Admin"}
	db.Create(&role)

	user := model.User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := model.Session{
		UserID:       user.ID,
		SessionToken: "user-deleted-token-123",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)

	// Soft delete user
	db.Delete(&user)

	w, _, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/token/validate", requestPath: "/token/validate", handler: ValidateToken, headers: map[string]string{"session-token": "user-deleted-token-123"}})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestValidateToken_NoDatabaseConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Don't add database middleware

	w, response, err := doRequestWithHandler(r, requestSpec{method: http.MethodGet, registerPath: "/token/validate", requestPath: "/token/validate", handler: ValidateToken, headers: map[string]string{"session-token": "any-token"}})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, response["error"].(string), "Database connection not available")
}
