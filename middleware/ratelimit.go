package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

const (
	// Rate limiting defaults
	defaultRateLimit  = 5                // 5 attempts
	defaultRateWindow = 15 * time.Minute // per 15 minutes
)

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	Limit  int
	Window time.Duration
}

// RateLimiter creates a rate limiting middleware
func RateLimiter(config RateLimitConfig) gin.HandlerFunc {
	if config.Limit == 0 {
		config.Limit = defaultRateLimit
	}
	if config.Window == 0 {
		config.Window = defaultRateWindow
	}

	return func(c *gin.Context) {
		// Get client identifier (IP address)
		clientIP := c.ClientIP()
		endpoint := c.Request.URL.Path

		// Create rate limit key
		key := fmt.Sprintf("ratelimit:%s:%s", endpoint, clientIP)

		// Check rate limit
		allowed, err := checkRateLimit(key, config.Limit, config.Window)
		if err != nil {
			// If rate limiting fails, log the error but allow the request
			// to prevent denial of service due to Redis unavailability
			util.LogSecurityEvent(util.SecurityEvent{
				EventType: util.EventSuspiciousActivity,
				IP:        clientIP,
				Message:   fmt.Sprintf("Rate limit check failed: %v", err),
			})
			c.Next()
			return
		}

		if !allowed {
			// Log rate limit exceeded
			util.LogRateLimitExceeded(util.RateLimitParams{Email: "", IP: clientIP, Endpoint: endpoint})

			util.CallUserError(c, util.APIErrorParams{
				Msg: "Too many requests. Please try again later.",
				Err: fmt.Errorf("rate limit exceeded"),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkRateLimit checks if a request is within rate limits
// Returns true if allowed, false if rate limit exceeded
func checkRateLimit(key string, limit int, window time.Duration) (bool, error) {
	rdb := config.GetRedisClient()
	if rdb == nil {
		// If Redis is not available, allow the request
		// In production, you might want to implement a local in-memory rate limiter
		return true, nil
	}

	ctx := context.Background()

	// Use Lua script for atomic rate limit check and increment
	// This prevents race conditions between multiple concurrent requests
	luaScript := `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		
		local current = redis.call('GET', key)
		if current == false then
			redis.call('SET', key, 1, 'EX', window)
			return 1
		end
		
		current = tonumber(current)
		if current >= limit then
			return 0
		end
		
		redis.call('INCR', key)
		return 1
	`

	result, err := rdb.Eval(ctx, luaScript, []string{key}, limit, int(window.Seconds())).Int()
	if err != nil {
		return false, fmt.Errorf("failed to check rate limit: %w", err)
	}

	return result == 1, nil
}

// ResetRateLimit resets the rate limit for a given key (useful for testing or admin operations)
func ResetRateLimit(clientIP, endpoint string) error {
	rdb := config.GetRedisClient()
	if rdb == nil {
		return fmt.Errorf("redis not available")
	}

	key := fmt.Sprintf("ratelimit:%s:%s", endpoint, clientIP)
	ctx := context.Background()

	return rdb.Del(ctx, key).Err()
}
