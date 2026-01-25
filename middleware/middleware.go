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
	origin := os.Getenv("CORSALLOWORIGIN")
	if origin == "" {
		origin = "http://localhost:3000"
	}
	c.Writer.Header().Set("Access-Control-Allow-Origin", origin)

	methods := os.Getenv("CORSALLOWMETHODS")
	if methods == "" {
		methods = "POST, PUT, GET, OPTIONS, DELETE, PATCH"
	}
	c.Writer.Header().Set("Access-Control-Allow-Methods", methods)

	headers := os.Getenv("CORSALLOWHEADERS")
	if headers == "" {
		headers = "X-Requested-With, Content-Type, Authorization, session-token"
	}
	c.Writer.Header().Set("Access-Control-Allow-Headers", headers)

	maxAge := os.Getenv("CORSMAXAGE")
	if maxAge == "" {
		maxAge = "86400"
	}
	c.Writer.Header().Set("Access-Control-Max-Age", maxAge)

	credentials := os.Getenv("CORSALLOWCREDENTIALS")
	if credentials == "" {
		credentials = "true"
	}
	c.Writer.Header().Set("Access-Control-Allow-Credentials", credentials)

	contentType := os.Getenv("CORSCONTENTTYPE")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Writer.Header().Set("Content-Type", contentType)
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
		roleID, exists := GetRoleID(c)
		if !exists {
			util.CallUserNotAuthorized(c, util.APIErrorParams{
				Msg: "Role information not available",
				Err: fmt.Errorf("role information not available in context"),
			})
			c.Abort()
			return
		}

		// Check if user's role is in the allowed roles
		roleAllowed := false
		for _, allowedRole := range allowedRoles {
			if roleID == allowedRole {
				roleAllowed = true
				break
			}
		}

		if !roleAllowed {
			util.CallUserNotAuthorized(c, util.APIErrorParams{
				Msg: "Insufficient permissions to access this resource",
				Err: fmt.Errorf("user role %d not authorized", roleID),
			})
			c.Abort()
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
		if roleID, exists := GetRoleID(c); exists {
			for _, allowed := range allowedRoles {
				if roleID == allowed {
					c.Next()
					return
				}
			}
		}

		// Otherwise check ownership: compare context user id with URL param `id`
		userID, ok := GetUserID(c)
		if !ok {
			util.CallUserNotAuthorized(c, util.APIErrorParams{
				Msg: "User not authenticated",
				Err: fmt.Errorf("user id not found in context"),
			})
			c.Abort()
			return
		}

		idParam := c.Param("id")
		if idParam == "" {
			util.CallUserNotAuthorized(c, util.APIErrorParams{
				Msg: "Resource id required",
				Err: fmt.Errorf("resource id parameter missing"),
			})
			c.Abort()
			return
		}

		// Parse id param to unsigned integer, constrained to platform uint size
		uid, err := strconv.ParseUint(idParam, 10, 0)
		if err != nil {
			util.CallUserNotAuthorized(c, util.APIErrorParams{
				Msg: "Invalid resource id",
				Err: err,
			})
			c.Abort()
			return
		}

		// Ensure the parsed id fits into the platform-dependent uint type to avoid overflow
		maxUint := ^uint(0)
		if uid > uint64(maxUint) {
			util.CallUserNotAuthorized(c, util.APIErrorParams{
				Msg: "Invalid resource id",
				Err: fmt.Errorf("resource id out of range"),
			})
			c.Abort()
			return
		}
		if uint(uid) == userID {
			c.Next()
			return
		}

		util.CallUserNotAuthorized(c, util.APIErrorParams{
			Msg: "Insufficient permissions to access this resource",
			Err: fmt.Errorf("user %d not owner nor allowed role", userID),
		})
		c.Abort()
		return
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
			util.CallUserNotAuthorized(c, util.APIErrorParams{
				Msg: "Session token not provided",
				Err: fmt.Errorf("session token not provided"),
			})
			c.Abort()
			return
		}

		// First try Redis for fast session validation: key session:<token> -> "userID:roleID"
		if rdb := config.GetRedisClient(); rdb != nil {
			val, err := rdb.Get(context.Background(), fmt.Sprintf("session:%s", sessionToken)).Result()
			if err == nil {
				parts := strings.Split(val, ":")
				if len(parts) == 2 {
					// Parse user ID and role ID from Redis value. Parse user ID
					// using the platform's native int size and perform explicit
					// bounds checks before converting to narrower types to
					// avoid integer overflow vulnerabilities.
					uid64, errUID := strconv.ParseUint(parts[0], 10, strconv.IntSize)
					rid64, errRID := strconv.ParseUint(parts[1], 10, 32)

					// Fail fast on parse errors.
					if errUID != nil || errRID != nil {
						// malformed value -> fallback to DB validation.
					} else {
						// Ensure uid is non-zero.
						if uid64 == 0 {
							// invalid user id -> fallback to DB validation.
						} else {
							// Verify uid64 fits into native `uint` on this platform.
							var maxUint uint64 = uint64(^uint(0))
							if uid64 <= maxUint {
								c.Set(UserIDKey, uint(uid64))
								c.Set(RoleIDKey, uint32(rid64))
								c.Next()
								return
							}
							// If uid is out of range, fallthrough to DB validation.
						}
					}
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
			util.CallUserNotAuthorized(c, util.APIErrorParams{
				Msg: "Invalid or expired session token",
				Err: fmt.Errorf("failed to validate session: %w", err),
			})
			c.Abort()
			return
		}

		if result.UserID == 0 {
			util.CallUserNotAuthorized(c, util.APIErrorParams{
				Msg: "Invalid or expired session token",
				Err: fmt.Errorf("no active session found for provided token"),
			})
			c.Abort()
			return
		}

		// Store user_id and role_id in context for use in handlers
		c.Set(UserIDKey, result.UserID)
		c.Set(RoleIDKey, result.RoleID)
		c.Next()
	}
}
