package middleware

import (
	"fmt"
	"time"

	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

// EndpointCallLogger logs each HTTP request as a security/endpoint event.
// It relies on the DatabaseMiddleware having already set DB in context and
// util.SetSecurityLoggerDB having been called during startup so events
// will be persisted to the SecurityLog table.
func EndpointCallLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		status := c.Writer.Status()

		userID, _ := GetUserID(c)
		roleID, _ := GetRoleID(c)

		details := map[string]interface{}{
			"method":      c.Request.Method,
			"path":        c.FullPath(),
			"raw_path":    c.Request.URL.Path,
			"status":      status,
			"duration_ms": duration.Milliseconds(),
			"query":       c.Request.URL.RawQuery,
		}
		if userID != 0 {
			details["user_id"] = userID
		}
		if roleID != 0 {
			details["role_id"] = roleID
		}

		util.LogSecurityEvent(util.SecurityEvent{
			EventType: util.EventEndpointCall,
			UserID:    fmt.Sprintf("%d", userID),
			Email:     "",
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			Message:   fmt.Sprintf("%s %s -> %d", c.Request.Method, c.Request.URL.Path, status),
			Details:   details,
		})
	}
}
