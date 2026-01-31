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

// newInMemoryDB creates an in-memory sqlite DB and runs required migrations for tests.
func newInMemoryDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Session{}); err != nil {
		t.Fatalf("failed to auto-migrate: %v", err)
	}
	return db
}

type testSessionParams struct {
	roleID    uint32
	token     string
	expiresAt time.Time
}

// createTestUserAndSession creates a user and associated session in the provided DB.
func createTestUserAndSession(t *testing.T, db *gorm.DB, params testSessionParams) (model.User, model.Session) {
	user := model.User{
		Name:     "Test User",
		Email:    "test@example.com",
		Password: "hashedpassword",
		RoleID:   params.roleID,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	if params.expiresAt.IsZero() {
		params.expiresAt = time.Now().Add(time.Hour)
	}
	session := model.Session{
		SessionToken: params.token,
		UserID:       user.ID,
		ExpiresAt:    params.expiresAt,
		ClientIP:     "127.0.0.1",
		Browser:      "test-browser",
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}
	return user, session
}

// newTestDBWithUserSession creates an in-memory DB and seeds a user+session.
func newTestDBWithUserSession(t *testing.T, params testSessionParams) (*gorm.DB, model.User, model.Session) {
	db := newInMemoryDB(t)
	user, session := createTestUserAndSession(t, db, params)
	return db, user, session
}

func runValidateLoginTokenRequest(db *gorm.DB, token string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	if db != nil {
		r.Use(DatabaseMiddleware(db))
	}
	r.GET("/test", ValidateLoginToken(), handler)
	req := httptest.NewRequest("GET", "/test", nil)
	if token != "" {
		req.Header.Set("session-token", token)
	}
	r.ServeHTTP(w, req)
	return w
}

type contextID struct {
	key   string
	label string
}

var (
	userIDContext = contextID{key: UserIDKey, label: "user_id"}
	roleIDContext = contextID{key: RoleIDKey, label: "role_id"}
)

func setGinTestMode() {
	gin.SetMode(gin.TestMode)
}

func setupRedisMock(t *testing.T) redismock.ClientMock {
	rdb, mock := redismock.NewClientMock()
	config.SetRedisClientForTest(rdb)
	t.Cleanup(func() {
		config.ResetRedisClientForTest()
	})
	return mock
}

type contextAssertion struct {
	ctx      contextID
	expected interface{}
	msg      string
}

func assertContextID(t *testing.T, c *gin.Context, assertion contextAssertion) {
	val, exists := c.Get(assertion.ctx.key)
	if !exists {
		t.Errorf("expected %s to be set in context%s", assertion.ctx.label, assertion.msg)
		return
	}
	switch exp := assertion.expected.(type) {
	case uint:
		actual, ok := val.(uint)
		if !ok || actual != exp {
			t.Errorf("expected %s to be %d, got %v%s", assertion.ctx.label, exp, val, assertion.msg)
		}
	case uint32:
		actual, ok := val.(uint32)
		if !ok || actual != exp {
			t.Errorf("expected %s to be %d, got %v%s", assertion.ctx.label, exp, val, assertion.msg)
		}
	default:
		t.Errorf("unsupported expected type for %s%s", assertion.ctx.label, assertion.msg)
	}
}

func assertUserContext(t *testing.T, c *gin.Context, user model.User, msg string) {
	assertContextID(t, c, contextAssertion{ctx: userIDContext, expected: user.ID, msg: msg})
	assertContextID(t, c, contextAssertion{ctx: roleIDContext, expected: user.RoleID, msg: msg})
}

func assertUserIDContext(t *testing.T, c *gin.Context, user model.User, msg string) {
	assertContextID(t, c, contextAssertion{ctx: userIDContext, expected: user.ID, msg: msg})
}

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
	if err := os.Setenv("APITOKEN", "secret-token"); err != nil {
		t.Fatalf("failed to set APITOKEN: %v", err)
	}
	defer func() { _ = os.Unsetenv("APITOKEN") }()

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
	setGinTestMode()

	db := &gorm.DB{}
	w := runValidateLoginTokenRequest(db, "", func(c *gin.Context) {
		c.Status(200)
	})

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when session token missing, got %d", w.Code)
	}
}

func TestValidateLoginToken_MissingDatabase(t *testing.T) {
	// Test that missing database in context returns 500
	setGinTestMode()

	w := runValidateLoginTokenRequest(nil, "test-token", func(c *gin.Context) {
		c.Status(200)
	})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when database missing, got %d", w.Code)
	}
}

func TestValidateLoginToken_RedisSuccessfulParse(t *testing.T) {
	// Test successful Redis parse with valid uint values
	setGinTestMode()

	// Create mock Redis client
	mock := setupRedisMock(t)

	// Set up mock expectations
	mock.ExpectGet("session:valid-token").SetVal("123:1")

	db := &gorm.DB{}
	w := runValidateLoginTokenRequest(db, "valid-token", func(c *gin.Context) {
		assertContextID(t, c, contextAssertion{ctx: userIDContext, expected: uint(123), msg: ""})
		assertContextID(t, c, contextAssertion{ctx: roleIDContext, expected: uint32(1), msg: ""})
		c.Status(200)
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when Redis parse succeeds, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}

func TestValidateLoginToken_RedisMalformedValue_NonNumeric(t *testing.T) {
	// Test Redis parse error with non-numeric string - should fallback to DB
	setGinTestMode()

	// Create mock Redis client
	mock := setupRedisMock(t)

	// Set up mock expectations - Redis returns malformed data
	mock.ExpectGet("session:malformed-token").SetVal("abc:1")

	// Set up in-memory database and test data for fallback
	db, user, _ := newTestDBWithUserSession(t, testSessionParams{roleID: 1, token: "malformed-token"})

	w := runValidateLoginTokenRequest(db, "malformed-token", func(c *gin.Context) {
		assertUserContext(t, c, user, " from DB fallback")
		c.Status(200)
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB fallback succeeds after Redis parse error, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}

func TestValidateLoginToken_RedisInvalidFormat_MissingColon(t *testing.T) {
	// Test Redis value with invalid format (missing colon separator) - should fallback to DB
	setGinTestMode()

	// Create mock Redis client
	mock := setupRedisMock(t)

	// Set up mock expectations - Redis returns invalid format
	mock.ExpectGet("session:invalid-format-token").SetVal("123")

	// Set up in-memory database and test data for fallback
	db, user, _ := newTestDBWithUserSession(t, testSessionParams{roleID: 2, token: "invalid-format-token"})

	w := runValidateLoginTokenRequest(db, "invalid-format-token", func(c *gin.Context) {
		assertUserIDContext(t, c, user, " from DB fallback")
		c.Status(200)
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB fallback succeeds after invalid format, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}

func TestValidateLoginToken_RedisZeroUserID(t *testing.T) {
	// Test Redis parse with zero user ID - should fallback to DB
	setGinTestMode()

	// Create mock Redis client
	mock := setupRedisMock(t)

	// Set up mock expectations - Redis returns zero user ID
	mock.ExpectGet("session:zero-uid-token").SetVal("0:1")

	// Set up in-memory database and test data for fallback
	db, user, _ := newTestDBWithUserSession(t, testSessionParams{roleID: 1, token: "zero-uid-token"})

	w := runValidateLoginTokenRequest(db, "zero-uid-token", func(c *gin.Context) {
		assertUserIDContext(t, c, user, " from DB fallback")
		c.Status(200)
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB fallback succeeds after zero UID, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}

func TestValidateLoginToken_RedisRoleIDParseError(t *testing.T) {
	// Test Redis parse error on role ID - should fallback to DB
	setGinTestMode()

	// Create mock Redis client
	mock := setupRedisMock(t)

	// Set up mock expectations - Redis returns non-numeric role ID
	mock.ExpectGet("session:bad-rid-token").SetVal("456:xyz")

	// Set up in-memory database for fallback
	db, user, _ := newTestDBWithUserSession(t, testSessionParams{roleID: 3, token: "bad-rid-token"})

	w := runValidateLoginTokenRequest(db, "bad-rid-token", func(c *gin.Context) {
		assertUserContext(t, c, user, " from DB fallback")
		c.Status(200)
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB fallback succeeds after role ID parse error, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}

func TestValidateLoginToken_RedisNotAvailable_DBFallback(t *testing.T) {
	// Test fallback to DB when Redis is not available
	setGinTestMode()

	// Ensure Redis client is nil
	config.ResetRedisClientForTest()
	defer config.ResetRedisClientForTest()

	// Set up in-memory database and test data
	db, user, _ := newTestDBWithUserSession(t, testSessionParams{roleID: 1, token: "db-only-token"})

	w := runValidateLoginTokenRequest(db, "db-only-token", func(c *gin.Context) {
		assertUserContext(t, c, user, " from DB")
		c.Status(200)
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB lookup succeeds, got %d", w.Code)
	}
}

func TestValidateLoginToken_DBFallback_ExpiredSession(t *testing.T) {
	// Test DB fallback returns 401 for expired session
	setGinTestMode()

	// Ensure Redis client is nil
	config.ResetRedisClientForTest()
	defer config.ResetRedisClientForTest()

	// Set up in-memory database and test data with expired session
	db, _, _ := newTestDBWithUserSession(t, testSessionParams{roleID: 1, token: "expired-token", expiresAt: time.Now().Add(-time.Hour)})

	w := runValidateLoginTokenRequest(db, "expired-token", func(c *gin.Context) {
		c.Status(200)
	})

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when session is expired, got %d", w.Code)
	}
}

func TestValidateLoginToken_RedisKeyNotFound_DBFallback(t *testing.T) {
	// Test fallback to DB when Redis key is not found
	setGinTestMode()

	// Create mock Redis client
	mock := setupRedisMock(t)

	// Set up mock expectations - Redis returns key not found error
	mock.ExpectGet("session:notfound-token").RedisNil()

	// Set up in-memory database and test data for fallback
	db, user, _ := newTestDBWithUserSession(t, testSessionParams{roleID: 1, token: "notfound-token"})

	w := runValidateLoginTokenRequest(db, "notfound-token", func(c *gin.Context) {
		assertUserIDContext(t, c, user, " from DB fallback")
		c.Status(200)
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 when DB fallback succeeds after Redis key not found, got %d", w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}
