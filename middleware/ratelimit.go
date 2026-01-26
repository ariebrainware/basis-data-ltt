package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	// Rate limiting defaults
	defaultRateLimit   = 5             // 5 attempts
	defaultRateWindow  = 15 * time.Minute // per 15 minutes
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
			util.LogRateLimitExceeded("", clientIP, endpoint)
			
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
	
	// Get current count
	count, err := rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		// Key doesn't exist, this is the first request
		// Use SET with EX for atomic set-with-expiry
		err = rdb.Set(ctx, key, 1, window).Err()
		if err != nil {
			return false, fmt.Errorf("failed to set rate limit: %w", err)
		}
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to get rate limit: %w", err)
	}
	
	// Check if limit exceeded
	if count >= int64(limit) {
		return false, nil
	}
	
	// Increment counter
	newCount, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to increment rate limit: %w", err)
	}
	
	return newCount <= int64(limit), nil
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
