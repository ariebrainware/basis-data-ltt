package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	DBKey     = "db"
	UserIDKey = "user_id"
	RoleIDKey = "role_id"
)

// getenvOrDefault returns the environment value for key or the provided default.
func getenvOrDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

// setHSTSHeader sets the Strict-Transport-Security header when appropriate.
func setHSTSHeader(c *gin.Context) {
	hstsMaxAge := getenvOrDefault("HSTS_MAX_AGE", "31536000")
	includeSubDomains := os.Getenv("HSTS_INCLUDE_SUBDOMAINS")
	hstsValue := fmt.Sprintf("max-age=%s", hstsMaxAge)
	if includeSubDomains == "true" {
		hstsValue += "; includeSubDomains"
	}
	c.Writer.Header().Set("Strict-Transport-Security", hstsValue)
}

// unauthorizedSession logs and returns a standardized unauthorized session response.
func unauthorizedSession(c *gin.Context, msg, logMsg string, err error) {
	util.LogUnauthorizedAccess("", "", c.ClientIP(), c.Request.URL.Path, logMsg)
	unauthorizedAbort(c, msg, err)
}

// unauthorizedAbort calls the standardized unauthorized response and aborts the context.
func unauthorizedAbort(c *gin.Context, msg string, err error) {
	util.CallUserNotAuthorized(c, util.APIErrorParams{Msg: msg, Err: err})
	c.Abort()
}

// isRoleAllowed checks whether the current request's role is among allowedRoles.
// It returns (allowed, roleID, exists).
func isRoleAllowed(c *gin.Context, allowedRoles ...uint32) (bool, uint32, bool) {
	roleID, exists := GetRoleID(c)
	if !exists {
		return false, 0, false
	}
	for _, allowed := range allowedRoles {
		if roleID == allowed {
			return true, roleID, true
		}
	}
	return false, roleID, true
}

// tryParseRedisSession parses a Redis session value of the form "userID:roleID".
// It returns parsed userID, roleID and whether parsing succeeded.
func tryParseRedisSession(val string) (uint, uint32, bool) {
	parts := strings.Split(val, ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	uid64, errUID := strconv.ParseUint(parts[0], 10, strconv.IntSize)
	rid64, errRID := strconv.ParseUint(parts[1], 10, 32)
	if errUID != nil || errRID != nil {
		return 0, 0, false
	}
	if uid64 == 0 {
		return 0, 0, false
	}
	maxUint := uint64(^uint(0))
	if uid64 > maxUint {
		return 0, 0, false
	}
	return uint(uid64), uint32(rid64), true
}

func tokenValidator(c *gin.Context, expectedToken string) bool {
	if c.Request.Method == http.MethodOptions {
		return true
	}
	token := strings.TrimSpace(c.GetHeader("Authorization"))
	if token != expectedToken {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API token"})
		return false
	}
	return true
}

func setCorsHeaders(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", getenvOrDefault("CORSALLOWORIGIN", "http://localhost:3000"))
	c.Writer.Header().Set("Access-Control-Allow-Methods", getenvOrDefault("CORSALLOWMETHODS", "POST, PUT, GET, OPTIONS, DELETE, PATCH"))
	c.Writer.Header().Set("Access-Control-Allow-Headers", getenvOrDefault("CORSALLOWHEADERS", "X-Requested-With, Content-Type, Authorization, session-token"))
	c.Writer.Header().Set("Access-Control-Max-Age", getenvOrDefault("CORSMAXAGE", "86400"))
	c.Writer.Header().Set("Access-Control-Allow-Credentials", getenvOrDefault("CORSALLOWCREDENTIALS", "true"))
	c.Writer.Header().Set("Content-Type", getenvOrDefault("CORSCONTENTTYPE", "application/json"))

	// Add HSTS header for HTTPS security. Only set when TLS is present or explicitly enabled
	if c.Request.TLS != nil || os.Getenv("ENABLE_HSTS") == "true" {
		setHSTSHeader(c)
	}
}

// CORSMiddleware configures CORS headers for incoming requests.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set CORS headers
		setCorsHeaders(c)

		// For preflight requests, simply return after setting CORS headers.
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		// Skip token validation for Swagger documentation routes
		if strings.HasPrefix(c.Request.URL.Path, "/swagger/") {
			c.Next()
			return
		}

		// Call tokenValidator after handling preflight.
		if !tokenValidator(c, fmt.Sprintf("Bearer %s", os.Getenv("APITOKEN"))) {
			return
		}

		c.Next()
	}
}

// DatabaseMiddleware injects the database connection into the context
func DatabaseMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(DBKey, db)
		c.Next()
	}
}

// GetDB retrieves the database from the Gin context
func GetDB(c *gin.Context) *gorm.DB {
	db, exists := c.Get(DBKey)
	if !exists {
		return nil
	}
	return db.(*gorm.DB)
}

// GetUserID retrieves the user ID from the Gin context
func GetUserID(c *gin.Context) (uint, bool) {
	userID, exists := c.Get(UserIDKey)
	if !exists {
		return 0, false
	}
	id, ok := userID.(uint)
	if !ok {
		return 0, false
	}
	return id, true
}

// GetRoleID retrieves the role ID from the Gin context
func GetRoleID(c *gin.Context) (uint32, bool) {
	roleID, exists := c.Get(RoleIDKey)
	if !exists {
		return 0, false
	}
	id, ok := roleID.(uint32)
	if !ok {
		return 0, false
	}
	return id, true
}

// RequireRole creates a middleware that checks if the user has one of the specified roles
func RequireRole(allowedRoles ...uint32) gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, roleID, exists := isRoleAllowed(c, allowedRoles...)
		if !exists {
			userID, _ := GetUserID(c)
			util.LogUnauthorizedAccess(fmt.Sprintf("%d", userID), "", c.ClientIP(), c.Request.URL.Path, "Role information not available")
			unauthorizedAbort(c, "Role information not available", fmt.Errorf("role information not available in context"))
			return
		}
		if !allowed {
			userID, _ := GetUserID(c)
			util.LogUnauthorizedAccess(fmt.Sprintf("%d", userID), "", c.ClientIP(), c.Request.URL.Path, fmt.Sprintf("Insufficient permissions (role %d)", roleID))
			unauthorizedAbort(c, "Insufficient permissions to access this resource", fmt.Errorf("user role %d not authorized", roleID))
			return
		}
		c.Next()
	}
}

// RequireRoleOrOwner allows access when the user's role is one of the
// allowedRoles OR when the authenticated user is the owner of the
// resource identified by the URL parameter `id`.
func RequireRoleOrOwner(allowedRoles ...uint32) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First allow if the user's role is one of the allowed roles
		if allowed, _, _ := isRoleAllowed(c, allowedRoles...); allowed {
			c.Next()
			return
		}

		// Otherwise check ownership: compare context user id with URL param `id`
		userID, ok := GetUserID(c)
		if !ok {
			unauthorizedAbort(c, "User not authenticated", fmt.Errorf("user id not found in context"))
			return
		}

		idParam := c.Param("id")
		if idParam == "" {
			unauthorizedAbort(c, "Resource id required", fmt.Errorf("resource id parameter missing"))
			return
		}

		// Parse id param to unsigned integer, constrained to platform uint size
		uid, err := strconv.ParseUint(idParam, 10, 0)
		if err != nil {
			unauthorizedAbort(c, "Invalid resource id", err)
			return
		}

		// Ensure the parsed id fits into the platform-dependent uint type to avoid overflow
		maxUint := ^uint(0)
		if uid > uint64(maxUint) {
			unauthorizedAbort(c, "Invalid resource id", fmt.Errorf("resource id out of range"))
			return
		}
		if uint(uid) == userID {
			c.Next()
			return
		}

		unauthorizedAbort(c, "Insufficient permissions to access this resource", fmt.Errorf("user %d not owner nor allowed role", userID))
	}
}

func ValidateLoginToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		db := GetDB(c)
		if db == nil {
			util.CallServerError(c, util.APIErrorParams{
				Msg: "Database connection not available in context",
				Err: fmt.Errorf("database connection not available in context"),
			})
			c.Abort()
			return
		}
		sessionToken := c.GetHeader("session-token")
		if sessionToken == "" {
			util.LogUnauthorizedAccess("", "", c.ClientIP(), c.Request.URL.Path, "Session token not provided")
			unauthorizedAbort(c, "Session token not provided", fmt.Errorf("session token not provided"))
			return
		}
		// First try Redis for fast session validation: key session:<token> -> "userID:roleID"
		if rdb := config.GetRedisClient(); rdb != nil {
			if val, err := rdb.Get(context.Background(), fmt.Sprintf("session:%s", sessionToken)).Result(); err == nil {
				if uid, rid, ok := tryParseRedisSession(val); ok {
					c.Set(UserIDKey, uid)
					c.Set(RoleIDKey, rid)
					c.Next()
					return
				}
			}
			// any Redis error, malformed value, or missing key -> fallback to DB
		}

		// Fallback to DB lookup when Redis doesn't have the session
		var result struct {
			UserID uint
			RoleID uint32
		}
		err := db.Table("sessions").
			Select("sessions.user_id, users.role_id").
			Joins("JOIN users ON users.id = sessions.user_id").
			Where("sessions.session_token = ? AND sessions.expires_at > ? AND sessions.deleted_at IS NULL AND users.deleted_at IS NULL", sessionToken, time.Now()).
			Take(&result).Error
		if err != nil {
			unauthorizedSession(c, "Invalid or expired session token", "Invalid or expired session token", fmt.Errorf("failed to validate session: %w", err))
			return
		}

		if result.UserID == 0 {
			unauthorizedSession(c, "Invalid or expired session token", "No active session found", fmt.Errorf("no active session found for provided token"))
			return
		}

		// Store user_id and role_id in context for use in handlers
		c.Set(UserIDKey, result.UserID)
		c.Set(RoleIDKey, result.RoleID)
		c.Next()
	}
}
