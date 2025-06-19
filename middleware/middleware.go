package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
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

func ValidateLoginToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionToken := c.GetHeader("session-token")
		if sessionToken == "" {
			util.CallUserNotAuthorized(c, util.APIErrorParams{
				Msg: "Session token not provided",
				Err: fmt.Errorf("session token not provided"),
			})
			c.Abort()
			return
		}

		// Connect to the database
		db, err := config.ConnectMySQL()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to MySQL"})
			c.Abort()
			return
		}

		// Find the session record in the database based on sessionToken
		var session model.Session
		if err := db.Where("session_token = ? AND expires_at > ? AND deleted_at IS NULL", sessionToken, time.Now()).First(&session).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session not found"})
			c.Abort()
			return
		}
		c.Next()
	}
}
