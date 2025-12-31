package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

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
	return userID.(uint), true
}

// GetRoleID retrieves the role ID from the Gin context
func GetRoleID(c *gin.Context) (uint32, bool) {
	roleID, exists := c.Get(RoleIDKey)
	if !exists {
		return 0, false
	}
	return roleID.(uint32), true
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

		// Find the session record and join with users to get role_id
		var result struct {
			UserID uint
			RoleID uint32
		}
		err := db.Table("sessions").
			Select("sessions.user_id, users.role_id").
			Joins("JOIN users ON users.id = sessions.user_id").
			Where("sessions.session_token = ? AND sessions.expires_at > ? AND sessions.deleted_at IS NULL", sessionToken, time.Now()).
			Scan(&result).Error
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session not found"})
			c.Abort()
			return
		}

		// Store user_id and role_id in context for use in handlers
		c.Set(UserIDKey, result.UserID)
		c.Set(RoleIDKey, result.RoleID)
		c.Next()
	}
}
