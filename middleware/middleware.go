package middleware

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware configures CORS headers for incoming requests.
func CORSMiddleware() gin.HandlerFunc {
	// create api token validation
	expectedToken := fmt.Sprintf("Bearer %s", os.Getenv("API_TOKEN"))

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

		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
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
