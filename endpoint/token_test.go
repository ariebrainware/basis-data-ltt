package endpoint

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	r.GET("/token/validate", ValidateToken)

	req := httptest.NewRequest(http.MethodGet, "/token/validate", nil)
	req.Header.Set("session-token", "valid-token-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))

	// Verify role is returned in data
	if data, ok := response["data"].(map[string]interface{}); ok {
		assert.Equal(t, "Admin", data["role"])
	}
}

func TestValidateToken_MissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTokenTestDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))

	r.GET("/token/validate", ValidateToken)

	req := httptest.NewRequest(http.MethodGet, "/token/validate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"].(string), "Invalid session token")
}

func TestValidateToken_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTokenTestDB(t)
	r := gin.New()
	r.Use(middleware.DatabaseMiddleware(db))

	r.GET("/token/validate", ValidateToken)

	req := httptest.NewRequest(http.MethodGet, "/token/validate", nil)
	req.Header.Set("session-token", "invalid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
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

	r.GET("/token/validate", ValidateToken)

	req := httptest.NewRequest(http.MethodGet, "/token/validate", nil)
	req.Header.Set("session-token", "expired-token-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

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

	r.GET("/token/validate", ValidateToken)

	req := httptest.NewRequest(http.MethodGet, "/token/validate", nil)
	req.Header.Set("session-token", "deleted-token-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

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

	r.GET("/token/validate", ValidateToken)

	req := httptest.NewRequest(http.MethodGet, "/token/validate", nil)
	req.Header.Set("session-token", "user-deleted-token-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestValidateToken_NoDatabaseConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Don't add database middleware

	r.GET("/token/validate", ValidateToken)

	req := httptest.NewRequest(http.MethodGet, "/token/validate", nil)
	req.Header.Set("session-token", "any-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response["error"].(string), "Database connection not available")
}
