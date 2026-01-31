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

func TestMain(m *testing.M) {
	// Set Gin to test mode once for all tests
	gin.SetMode(gin.TestMode)
	code := m.Run()
	os.Exit(code)
}

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

func runFallbackCaseTest(t *testing.T, tc fallbackCase, mock redismock.ClientMock) {
	if tc.redisNil {
		mock.ExpectGet("session:" + tc.token).RedisNil()
	} else {
		mock.ExpectGet("session:" + tc.token).SetVal(tc.redisValue)
	}

	db, user, _ := newTestDBWithUserSession(t, tc.params)
	w := runValidateLoginTokenRequest(db, tc.token, func(c *gin.Context) {
		tc.assert(t, c, user)
		c.Status(200)
	})

	if w.Code != tc.statusCode {
		t.Fatalf("expected %d when DB fallback succeeds, got %d", tc.statusCode, w.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Redis expectations were not met: %v", err)
	}
}

type fallbackCase struct {
	name       string
	token      string
	redisValue string
	redisNil   bool
	params     testSessionParams
	assert     func(t *testing.T, c *gin.Context, user model.User)
	statusCode int
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
	w := runValidateLoginTokenRequest(nil, "test-token", func(c *gin.Context) {
		c.Status(200)
	})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when database missing, got %d", w.Code)
	}
}

func TestValidateLoginToken_RedisSuccessfulParse(t *testing.T) {
	// Test successful Redis parse with valid uint values
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

func TestValidateLoginToken_RedisFallbackCases(t *testing.T) {
	cases := []fallbackCase{
		{
			name:       "non_numeric_value",
			token:      "malformed-token",
			redisValue: "abc:1",
			params:     testSessionParams{roleID: 1, token: "malformed-token"},
			assert: func(t *testing.T, c *gin.Context, user model.User) {
				assertUserContext(t, c, user, " from DB fallback")
			},
			statusCode: http.StatusOK,
		},
		{
			name:       "missing_colon",
			token:      "invalid-format-token",
			redisValue: "123",
			params:     testSessionParams{roleID: 2, token: "invalid-format-token"},
			assert: func(t *testing.T, c *gin.Context, user model.User) {
				assertUserIDContext(t, c, user, " from DB fallback")
			},
			statusCode: http.StatusOK,
		},
		{
			name:       "zero_user_id",
			token:      "zero-uid-token",
			redisValue: "0:1",
			params:     testSessionParams{roleID: 1, token: "zero-uid-token"},
			assert: func(t *testing.T, c *gin.Context, user model.User) {
				assertUserIDContext(t, c, user, " from DB fallback")
			},
			statusCode: http.StatusOK,
		},
		{
			name:       "bad_role_id",
			token:      "bad-rid-token",
			redisValue: "456:xyz",
			params:     testSessionParams{roleID: 3, token: "bad-rid-token"},
			assert: func(t *testing.T, c *gin.Context, user model.User) {
				assertUserContext(t, c, user, " from DB fallback")
			},
			statusCode: http.StatusOK,
		},
		{
			name:     "redis_nil",
			token:    "notfound-token",
			redisNil: true,
			params:   testSessionParams{roleID: 1, token: "notfound-token"},
			assert: func(t *testing.T, c *gin.Context, user model.User) {
				assertUserIDContext(t, c, user, " from DB fallback")
			},
			statusCode: http.StatusOK,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mock := setupRedisMock(t)
			runFallbackCaseTest(t, tc, mock)
		})
	}
}

func TestValidateLoginToken_RedisNotAvailable_DBFallback(t *testing.T) {
	// Test fallback to DB when Redis is not available
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
