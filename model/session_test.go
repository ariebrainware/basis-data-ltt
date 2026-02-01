package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSessionTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:testdb_session_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&Session{}, &User{}, &Role{})
	assert.NoError(t, err)

	return db
}

func TestSessionModel_Create(t *testing.T) {
	db := setupSessionTestDB(t)

	// Create user first
	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := Session{
		UserID:       user.ID,
		SessionToken: "token123",
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	err := db.Create(&session).Error
	assert.NoError(t, err)
	assert.NotZero(t, session.ID)
}

func TestSessionModel_Read(t *testing.T) {
	db := setupSessionTestDB(t)

	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := Session{
		UserID:       user.ID,
		SessionToken: "read-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)

	var found Session
	err := db.First(&found, session.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "read-token", found.SessionToken)
	assert.Equal(t, user.ID, found.UserID)
}

func TestSessionModel_Delete(t *testing.T) {
	db := setupSessionTestDB(t)

	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := Session{
		UserID:       user.ID,
		SessionToken: "delete-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)

	err := db.Delete(&session).Error
	assert.NoError(t, err)

	var found Session
	err = db.First(&found, session.ID).Error
	assert.Error(t, err) // Should be soft deleted
}

func TestSessionModel_FindByToken(t *testing.T) {
	db := setupSessionTestDB(t)

	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := Session{
		UserID:       user.ID,
		SessionToken: "find-by-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)

	var found Session
	err := db.Where("session_token = ?", "find-by-token").First(&found).Error
	assert.NoError(t, err)
	assert.Equal(t, user.ID, found.UserID)
}

func TestSessionModel_ExpiredSession(t *testing.T) {
	db := setupSessionTestDB(t)

	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	// Create expired session
	session := Session{
		UserID:       user.ID,
		SessionToken: "expired-token",
		ExpiresAt:    time.Now().Add(-time.Hour), // Past time
	}
	db.Create(&session)

	// Query for active sessions (not expired)
	var activeSessions []Session
	err := db.Where("user_id = ? AND expires_at > ?", user.ID, time.Now()).Find(&activeSessions).Error
	assert.NoError(t, err)
	assert.Equal(t, 0, len(activeSessions))
}

func TestSessionModel_ValidSession(t *testing.T) {
	db := setupSessionTestDB(t)

	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := Session{
		UserID:       user.ID,
		SessionToken: "valid-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)

	// Query for active sessions
	var activeSessions []Session
	err := db.Where("user_id = ? AND expires_at > ?", user.ID, time.Now()).Find(&activeSessions).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(activeSessions), 1)
}

func TestSessionModel_WithClientInfo(t *testing.T) {
	db := setupSessionTestDB(t)

	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := Session{
		UserID:       user.ID,
		SessionToken: "client-info-token",
		ExpiresAt:    time.Now().Add(time.Hour),
		ClientIP:     "192.168.1.1",
		Browser:      "Mozilla/5.0",
	}

	err := db.Create(&session).Error
	assert.NoError(t, err)

	var found Session
	db.First(&found, session.ID)
	assert.Equal(t, "192.168.1.1", found.ClientIP)
	assert.Equal(t, "Mozilla/5.0", found.Browser)
}

func TestSessionModel_MultipleSessionsPerUser(t *testing.T) {
	db := setupSessionTestDB(t)

	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	// Create multiple sessions for same user
	for i := 0; i < 3; i++ {
		session := Session{
			UserID:       user.ID,
			SessionToken: "multi-token-" + string(rune(i)),
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		db.Create(&session)
	}

	var sessions []Session
	err := db.Where("user_id = ?", user.ID).Find(&sessions).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(sessions), 3)
}

func TestSessionModel_Timestamps(t *testing.T) {
	db := setupSessionTestDB(t)

	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := Session{
		UserID:       user.ID,
		SessionToken: "timestamp-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)

	assert.NotZero(t, session.CreatedAt)
	assert.NotZero(t, session.UpdatedAt)
}

func TestSessionModel_DeleteExpiredSessions(t *testing.T) {
	db := setupSessionTestDB(t)

	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	// Create expired sessions
	for i := 0; i < 5; i++ {
		session := Session{
			UserID:       user.ID,
			SessionToken: "cleanup-token-" + string(rune(i)),
			ExpiresAt:    time.Now().Add(-time.Hour),
		}
		db.Create(&session)
	}

	// Delete expired sessions
	err := db.Where("expires_at < ?", time.Now()).Delete(&Session{}).Error
	assert.NoError(t, err)

	var expiredSessions []Session
	db.Unscoped().Where("user_id = ? AND expires_at < ?", user.ID, time.Now()).Find(&expiredSessions)
	// Sessions should be soft deleted
	for _, s := range expiredSessions {
		assert.NotNil(t, s.DeletedAt)
	}
}

func TestSessionModel_CountUserSessions(t *testing.T) {
	db := setupSessionTestDB(t)

	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	// Create sessions
	for i := 0; i < 4; i++ {
		session := Session{
			UserID:       user.ID,
			SessionToken: "count-token-" + string(rune(i)),
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		db.Create(&session)
	}

	var count int64
	err := db.Model(&Session{}).Where("user_id = ?", user.ID).Count(&count).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(4))
}

func TestSessionModel_UpdateExpiry(t *testing.T) {
	db := setupSessionTestDB(t)

	role := Role{Name: "User"}
	db.Create(&role)

	user := User{
		Name:     "Test User",
		Email:    "test@test.com",
		Password: "hash",
		RoleID:   uint32(role.ID),
	}
	db.Create(&user)

	session := Session{
		UserID:       user.ID,
		SessionToken: "update-expiry-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	db.Create(&session)

	// Update expiry time
	newExpiry := time.Now().Add(2 * time.Hour)
	session.ExpiresAt = newExpiry
	err := db.Save(&session).Error
	assert.NoError(t, err)

	var updated Session
	db.First(&updated, session.ID)
	assert.True(t, updated.ExpiresAt.After(time.Now().Add(time.Hour)))
}
