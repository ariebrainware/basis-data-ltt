package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redismock/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSetCorsHeadersDefaults(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// Build a dummy request to attach headers map
	req := httptest.NewRequest("GET", "/", nil)
	c.Request = req

	setCorsHeaders(c)

	if got := c.Writer.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Origin header to be set")
	}
	if got := c.Writer.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Methods header to be set")
	}
}

func TestTokenValidator(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// OPTIONS should bypass token validation
	c.Request = httptest.NewRequest("OPTIONS", "/", nil)
	if !tokenValidator(c, "anything") {
		t.Fatalf("expected tokenValidator to allow OPTIONS method")
	}

	// Non-OPTIONS must match expected token
	expected := "Bearer secret-token"
	os.Setenv("APITOKEN", "secret-token")
	defer os.Unsetenv("APITOKEN")

	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Authorization", expected)
	ok := tokenValidator(c, expected)
	if !ok {
		t.Fatalf("expected tokenValidator to accept matching token")
	}

	// mismatch should abort and return false
	c2w := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(c2w)
	c2.Request = httptest.NewRequest("GET", "/", nil)
	c2.Request.Header.Set("Authorization", "Bearer bad")
	ok2 := tokenValidator(c2, expected)
	if ok2 {
		t.Fatalf("expected tokenValidator to reject bad token")
	}
	if c2w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on bad token, got %d", c2w.Code)
	}
}

func TestDatabaseMiddlewareAndGetDB(t *testing.T) {
	r := gin.New()
	// Use a zero-value gorm.DB pointer as a placeholder
	db := &gorm.DB{}
	r.Use(DatabaseMiddleware(db))
	r.GET("/testdb", func(c *gin.Context) {
		got := GetDB(c)
		if got == nil {
			c.AbortWithStatus(500)
			return
		}
		if got != db {
			c.AbortWithStatus(500)
			return
		}
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/testdb", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 from handler with DB set, got %d", w.Code)
	}
}

func TestValidateLoginToken_MissingSessionToken(t *testing.T) {
	// Test that missing session token returns 401
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	db := &gorm.DB{}
	r.Use(DatabaseMiddleware(db))
	r.GET("/test", ValidateLoginToken(), func(c *gin.Context) {
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	c.Request = req
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when session token missing, got %d", w.Code)
	}
}

func TestValidateLoginToken_MissingDatabase(t *testing.T) {
	// Test that missing database in context returns 500
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	r.GET("/test", ValidateLoginToken(), func(c *gin.Context) {
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("session-token", "test-token")
	c.Request = req
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when database missing, got %d", w.Code)
	}
}

func TestValidateLoginToken_RedisSuccessfulParse(t *testing.T) {
	// Test successful Redis parse with valid uint values
	gin.SetMode(gin.TestMode)

	// Create mock Redis client
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	// Set up mock expectations
	mock.ExpectGet("session:valid-token").SetVal("123:1")

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	db := &gorm.DB{}
	r.Use(DatabaseMiddleware(db))
	r.GET("/test", ValidateLoginToken(), func(c *gin.Context) {
		userID, exists := c.Get(UserIDKey)
		if !exists {
			t.Errorf("expected user_id to be set in context")
		}
		if userID != uint(123) {
			t.Errorf("expected user_id to be 123, got %v", userID)
		}

		roleID, exists := c.Get(RoleIDKey)
		if !exists {
			t.Errorf("expected role_id to be set in context")
		}
		if roleID != uint32(1) {
			t.Errorf("expected role_id to be 1, got %v", roleID)
		}
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("session-token", "valid-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when Redis parse succeeds, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}

func TestValidateLoginToken_RedisMalformedValue_NonNumeric(t *testing.T) {
	// Test Redis parse error with non-numeric string - should fallback to DB
	gin.SetMode(gin.TestMode)

	// Create mock Redis client
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	// Set up mock expectations - Redis returns malformed data
	mock.ExpectGet("session:malformed-token").SetVal("abc:1")

	// Set up in-memory database for fallback
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Auto-migrate tables
	db.AutoMigrate(&model.User{}, &model.Session{})

	// Create test data
	user := model.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "hashedpassword",
		RoleID:   1,
	}
	db.Create(&user)

	session := model.Session{
		SessionToken: "malformed-token",
		UserID:       user.ID,
		ExpiresAt:    time.Now().Add(time.Hour),
		ClientIP:     "127.0.0.1",
		Browser:      "test-browser",
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(DatabaseMiddleware(db))
	r.GET("/test", ValidateLoginToken(), func(c *gin.Context) {
		userID, exists := c.Get(UserIDKey)
		if !exists {
			t.Errorf("expected user_id to be set in context from DB fallback")
		}
		if userID != user.ID {
			t.Errorf("expected user_id to be %d, got %v", user.ID, userID)
		}

		roleID, exists := c.Get(RoleIDKey)
		if !exists {
			t.Errorf("expected role_id to be set in context from DB fallback")
		}
		if roleID != user.RoleID {
			t.Errorf("expected role_id to be %d, got %v", user.RoleID, roleID)
		}
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("session-token", "malformed-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB fallback succeeds after Redis parse error, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}

func TestValidateLoginToken_RedisInvalidFormat_MissingColon(t *testing.T) {
	// Test Redis value with invalid format (missing colon separator) - should fallback to DB
	gin.SetMode(gin.TestMode)

	// Create mock Redis client
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	// Set up mock expectations - Redis returns invalid format
	mock.ExpectGet("session:invalid-format-token").SetVal("123")

	// Set up in-memory database for fallback
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Auto-migrate tables
	db.AutoMigrate(&model.User{}, &model.Session{})

	// Create test data
	user := model.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "hashedpassword",
		RoleID:   2,
	}
	db.Create(&user)

	session := model.Session{
		SessionToken: "invalid-format-token",
		UserID:       user.ID,
		ExpiresAt:    time.Now().Add(time.Hour),
		ClientIP:     "127.0.0.1",
		Browser:      "test-browser",
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(DatabaseMiddleware(db))
	r.GET("/test", ValidateLoginToken(), func(c *gin.Context) {
		userID, exists := c.Get(UserIDKey)
		if !exists {
			t.Errorf("expected user_id to be set in context from DB fallback")
		}
		if userID != user.ID {
			t.Errorf("expected user_id to be %d, got %v", user.ID, userID)
		}
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("session-token", "invalid-format-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB fallback succeeds after invalid format, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}

func TestValidateLoginToken_RedisZeroUserID(t *testing.T) {
	// Test Redis parse with zero user ID - should fallback to DB
	gin.SetMode(gin.TestMode)

	// Create mock Redis client
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	// Set up mock expectations - Redis returns zero user ID
	mock.ExpectGet("session:zero-uid-token").SetVal("0:1")

	// Set up in-memory database for fallback
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Auto-migrate tables
	db.AutoMigrate(&model.User{}, &model.Session{})

	// Create test data
	user := model.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "hashedpassword",
		RoleID:   1,
	}
	db.Create(&user)

	session := model.Session{
		SessionToken: "zero-uid-token",
		UserID:       user.ID,
		ExpiresAt:    time.Now().Add(time.Hour),
		ClientIP:     "127.0.0.1",
		Browser:      "test-browser",
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(DatabaseMiddleware(db))
	r.GET("/test", ValidateLoginToken(), func(c *gin.Context) {
		userID, exists := c.Get(UserIDKey)
		if !exists {
			t.Errorf("expected user_id to be set in context from DB fallback")
		}
		if userID != user.ID {
			t.Errorf("expected user_id to be %d, got %v", user.ID, userID)
		}
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("session-token", "zero-uid-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB fallback succeeds after zero UID, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}

func TestValidateLoginToken_RedisRoleIDParseError(t *testing.T) {
	// Test Redis parse error on role ID - should fallback to DB
	gin.SetMode(gin.TestMode)

	// Create mock Redis client
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	// Set up mock expectations - Redis returns non-numeric role ID
	mock.ExpectGet("session:bad-rid-token").SetVal("456:xyz")

	// Set up in-memory database for fallback
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Auto-migrate tables
	db.AutoMigrate(&model.User{}, &model.Session{})

	// Create test data
	user := model.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "hashedpassword",
		RoleID:   3,
	}
	db.Create(&user)

	session := model.Session{
		SessionToken: "bad-rid-token",
		UserID:       user.ID,
		ExpiresAt:    time.Now().Add(time.Hour),
		ClientIP:     "127.0.0.1",
		Browser:      "test-browser",
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(DatabaseMiddleware(db))
	r.GET("/test", ValidateLoginToken(), func(c *gin.Context) {
		userID, exists := c.Get(UserIDKey)
		if !exists {
			t.Errorf("expected user_id to be set in context from DB fallback")
		}
		if userID != user.ID {
			t.Errorf("expected user_id to be %d, got %v", user.ID, userID)
		}

		roleID, exists := c.Get(RoleIDKey)
		if !exists {
			t.Errorf("expected role_id to be set in context from DB fallback")
		}
		if roleID != user.RoleID {
			t.Errorf("expected role_id to be %d, got %v", user.RoleID, roleID)
		}
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("session-token", "bad-rid-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB fallback succeeds after role ID parse error, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}

func TestValidateLoginToken_RedisNotAvailable_DBFallback(t *testing.T) {
	// Test fallback to DB when Redis is not available
	gin.SetMode(gin.TestMode)

	// Ensure Redis client is nil
	config.ResetRedisClientForTest()
	defer config.ResetRedisClientForTest()

	// Set up in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Auto-migrate tables
	db.AutoMigrate(&model.User{}, &model.Session{})

	// Create test data
	user := model.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "hashedpassword",
		RoleID:   1,
	}
	db.Create(&user)

	session := model.Session{
		SessionToken: "db-only-token",
		UserID:       user.ID,
		ExpiresAt:    time.Now().Add(time.Hour),
		ClientIP:     "127.0.0.1",
		Browser:      "test-browser",
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(DatabaseMiddleware(db))
	r.GET("/test", ValidateLoginToken(), func(c *gin.Context) {
		userID, exists := c.Get(UserIDKey)
		if !exists {
			t.Errorf("expected user_id to be set in context from DB")
		}
		if userID != user.ID {
			t.Errorf("expected user_id to be %d, got %v", user.ID, userID)
		}

		roleID, exists := c.Get(RoleIDKey)
		if !exists {
			t.Errorf("expected role_id to be set in context from DB")
		}
		if roleID != user.RoleID {
			t.Errorf("expected role_id to be %d, got %v", user.RoleID, roleID)
		}
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("session-token", "db-only-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB lookup succeeds, got %d", w.Code)
	}
}

func TestValidateLoginToken_DBFallback_ExpiredSession(t *testing.T) {
	// Test DB fallback returns 401 for expired session
	gin.SetMode(gin.TestMode)

	// Ensure Redis client is nil
	config.ResetRedisClientForTest()
	defer config.ResetRedisClientForTest()

	// Set up in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Auto-migrate tables
	db.AutoMigrate(&model.User{}, &model.Session{})

	// Create test data with expired session
	user := model.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "hashedpassword",
		RoleID:   1,
	}
	db.Create(&user)

	session := model.Session{
		SessionToken: "expired-token",
		UserID:       user.ID,
		ExpiresAt:    time.Now().Add(-time.Hour), // Expired 1 hour ago
		ClientIP:     "127.0.0.1",
		Browser:      "test-browser",
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(DatabaseMiddleware(db))
	r.GET("/test", ValidateLoginToken(), func(c *gin.Context) {
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("session-token", "expired-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when session is expired, got %d", w.Code)
	}
}

func TestValidateLoginToken_RedisKeyNotFound_DBFallback(t *testing.T) {
	// Test fallback to DB when Redis key is not found
	gin.SetMode(gin.TestMode)

	// Create mock Redis client
	rdb, mock := redismock.NewClientMock()
	defer config.ResetRedisClientForTest()
	config.SetRedisClientForTest(rdb)

	// Set up mock expectations - Redis returns key not found error
	mock.ExpectGet("session:notfound-token").RedisNil()

	// Set up in-memory database for fallback
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Auto-migrate tables
	db.AutoMigrate(&model.User{}, &model.Session{})

	// Create test data
	user := model.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "hashedpassword",
		RoleID:   1,
	}
	db.Create(&user)

	session := model.Session{
		SessionToken: "notfound-token",
		UserID:       user.ID,
		ExpiresAt:    time.Now().Add(time.Hour),
		ClientIP:     "127.0.0.1",
		Browser:      "test-browser",
	}
	db.Create(&session)

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)

	r.Use(DatabaseMiddleware(db))
	r.GET("/test", ValidateLoginToken(), func(c *gin.Context) {
		userID, exists := c.Get(UserIDKey)
		if !exists {
			t.Errorf("expected user_id to be set in context from DB fallback")
		}
		if userID != user.ID {
			t.Errorf("expected user_id to be %d, got %v", user.ID, userID)
		}
		c.Status(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("session-token", "notfound-token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB fallback succeeds after Redis key not found, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}
