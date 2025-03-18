package endpoint

import (
	"net/http"
	"time"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
)

func ValidateToken(c *gin.Context) {
	sessionToken := c.GetHeader("session-token")
	if sessionToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session token"})
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

	// Join sessions, users, and roles to retrieve the role name aliased as 'role'
	var result struct {
		model.Session
		Role string `json:"role"`
	}
	err = db.Table("sessions").
		Select("sessions.*, roles.name as role").
		Joins("JOIN users ON sessions.user_id = users.id").
		Joins("JOIN roles ON users.role_id = roles.id").
		Where("session_token = ? AND expires_at > ? AND sessions.deleted_at IS NULL", sessionToken, time.Now()).
		First(&result).Error
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Session not found"})
		c.Abort()
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg:  "Valid session token",
		Data: result,
	})
}
