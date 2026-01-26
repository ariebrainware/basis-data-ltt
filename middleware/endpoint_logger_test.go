package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEndpointCallLogger_BasicRequest(t *testing.T) {
	// Capture security log output
	var buf bytes.Buffer
	originalLogger := util.GetSecurityLoggerForTest()
	util.SetSecurityLoggerForTest(log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix))
	defer func() {
		if originalLogger != nil {
			util.SetSecurityLoggerForTest(originalLogger)
		}
	}()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Create test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	r.Use(DatabaseMiddleware(db))
	r.Use(EndpointCallLogger())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test?foo=bar", nil)
	req.RemoteAddr = "192.168.1.100:1234"
	req.Header.Set("User-Agent", "TestAgent/1.0")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify log output
	logOutput := buf.String()
	if !strings.Contains(logOutput, "Event=ENDPOINT_CALL") {
		t.Error("Expected log to contain Event=ENDPOINT_CALL")
	}
	if !strings.Contains(logOutput, "GET /test -> 200") {
		t.Error("Expected log to contain request method and status")
	}
	if !strings.Contains(logOutput, "192.168.1.100") {
		t.Error("Expected log to contain IP address")
	}
	if !strings.Contains(logOutput, "TestAgent/1.0") {
		t.Error("Expected log to contain User-Agent")
	}
}

func TestEndpointCallLogger_WithUserContext(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := util.GetSecurityLoggerForTest()
	util.SetSecurityLoggerForTest(log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix))
	defer func() {
		if originalLogger != nil {
			util.SetSecurityLoggerForTest(originalLogger)
		}
	}()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Create test database with users table
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create users table and insert test user
	err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)").Error
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}
	err = db.Exec("INSERT INTO users (id, email) VALUES (42, 'testuser@example.com')").Error
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	// Initialize user email cache
	util.InitUserEmailCache(10)

	r.Use(DatabaseMiddleware(db))
	r.Use(EndpointCallLogger())
	r.GET("/test", func(c *gin.Context) {
		// Set user context
		c.Set(UserIDKey, uint(42))
		c.Set(RoleIDKey, uint(2))
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:1234"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify log output contains user information
	logOutput := buf.String()
	if !strings.Contains(logOutput, "UserID=42") {
		t.Error("Expected log to contain UserID=42")
	}
	if !strings.Contains(logOutput, "testuser@example.com") {
		t.Error("Expected log to contain user email")
	}
}

func TestEndpointCallLogger_NoUserContext(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := util.GetSecurityLoggerForTest()
	util.SetSecurityLoggerForTest(log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix))
	defer func() {
		if originalLogger != nil {
			util.SetSecurityLoggerForTest(originalLogger)
		}
	}()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	r.Use(DatabaseMiddleware(db))
	r.Use(EndpointCallLogger())
	r.GET("/test", func(c *gin.Context) {
		// Don't set user context
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify log output
	logOutput := buf.String()
	if !strings.Contains(logOutput, "Event=ENDPOINT_CALL") {
		t.Error("Expected log to contain Event=ENDPOINT_CALL")
	}
	// UserID should be 0 when not set
	if !strings.Contains(logOutput, "UserID=0") {
		t.Error("Expected log to contain UserID=0")
	}
}

func TestEndpointCallLogger_ErrorStatus(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := util.GetSecurityLoggerForTest()
	util.SetSecurityLoggerForTest(log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix))
	defer func() {
		if originalLogger != nil {
			util.SetSecurityLoggerForTest(originalLogger)
		}
	}()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	r.Use(DatabaseMiddleware(db))
	r.Use(EndpointCallLogger())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	// Verify log output contains error status
	logOutput := buf.String()
	if !strings.Contains(logOutput, "GET /test -> 404") {
		t.Error("Expected log to contain status 404")
	}
}

func TestEndpointCallLogger_POSTRequest(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := util.GetSecurityLoggerForTest()
	util.SetSecurityLoggerForTest(log.New(&buf, "[SECURITY] ", log.LstdFlags|log.Lmsgprefix))
	defer func() {
		if originalLogger != nil {
			util.SetSecurityLoggerForTest(originalLogger)
		}
	}()

	gin.SetMode(gin.TestMode)
	r := gin.New()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	r.Use(DatabaseMiddleware(db))
	r.Use(EndpointCallLogger())
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"created": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{"data":"test"}`))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	// Verify log output
	logOutput := buf.String()
	if !strings.Contains(logOutput, "POST /test -> 201") {
		t.Error("Expected log to contain POST method and status 201")
	}
}
