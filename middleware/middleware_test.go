package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
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
