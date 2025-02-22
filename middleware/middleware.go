package middleware

import (
	"fmt"
	"net/http"
	"os"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware configures CORS headers for incoming requests.
func CORSMiddleware() gin.HandlerFunc {
	// create api token validation
	expectedToken := fmt.Sprintf("Bearer %s", os.Getenv("APITOKEN"))

	tokenValidator := func(c *gin.Context) bool {
		token := c.GetHeader("Authorization")
		if token != expectedToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API token"})
			return false
		}
		return true
	}

	return func(c *gin.Context) {
		// Call tokenValidator at the beginning of the returned handler.
		if !tokenValidator(c) {
			return
		}

		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "X-Requested-With, Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Content-Type", "application/json")

		// For preflight requests, respond with 204 and abort further processing.
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func ValidateLoginToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionToken := c.GetHeader("session_token")
		if sessionToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session token not provided"})
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
		if err := db.Where("session_token = ?", sessionToken).First(&session).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session not found"})
			c.Abort()
			return
		}

		// Delete the session record from the database
		if err := db.Where("session_token = ?", sessionToken).Delete(&session).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete session"})
			c.Abort()
			return
		}

		c.Next()
	}
}
