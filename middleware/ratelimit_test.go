package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/gin-gonic/gin"
)

func TestRateLimiter_WithoutRedis(t *testing.T) {
	// Ensure no Redis client is available
	config.SetRedisClientForTesting(nil)
	defer config.SetRedisClientForTesting(nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	rateLimiter := RateLimiter(RateLimitConfig{
		Limit:  5,
		Window: 15 * time.Minute,
	})

	r.Use(rateLimiter)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Without Redis, all requests should be allowed
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected status 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimiter_DefaultConfig(t *testing.T) {
	// Ensure no Redis client is available
	config.SetRedisClientForTesting(nil)
	defer config.SetRedisClientForTesting(nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Use rate limiter with empty config to test defaults
	rateLimiter := RateLimiter(RateLimitConfig{})

	r.Use(rateLimiter)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestResetRateLimit_NoRedis(t *testing.T) {
	config.SetRedisClientForTesting(nil)
	defer config.SetRedisClientForTesting(nil)

	err := ResetRateLimit("192.168.1.1", "/test")
	if err == nil {
		t.Error("Expected error when Redis not available, got nil")
	}
}
