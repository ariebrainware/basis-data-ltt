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

type UserCreateOpts struct {
	Name   string
	Email  string
	RoleID uint32
}

type UserWithRoleOpts struct {
	Name     string
	Email    string
	RoleName string
}

func mustCreateUser(db *gorm.DB, t *testing.T, opts UserCreateOpts) User {
	t.Helper()
	user := User{
		Name:     opts.Name,
		Email:    opts.Email,
		Password: "hash",
		RoleID:   opts.RoleID,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

// mustCreateDefaultUser creates a user with a unique email and the "User" role.
func mustCreateDefaultUser(db *gorm.DB, t *testing.T) User {
	t.Helper()
	// ensure unique email per call
	email := fmt.Sprintf("test+%d@example.com", time.Now().UnixNano())
	return mustCreateUserWithRole(db, t, UserWithRoleOpts{Name: "Test User", Email: email, RoleName: "User"})
}

// mustCreateUserWithRole creates a role (if needed) and a user with that role.
func mustCreateUserWithRole(db *gorm.DB, t *testing.T, opts UserWithRoleOpts) User {
	t.Helper()
	role := Role{Name: opts.RoleName}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("failed to create role: %v", err)
	}
	return mustCreateUser(db, t, UserCreateOpts{Name: opts.Name, Email: opts.Email, RoleID: uint32(role.ID)})
}

// SessionCreateOpts groups parameters for creating a test session to reduce
// the number of function arguments and improve readability.
type SessionCreateOpts struct {
	UserID   uint
	Token    string
	Expires  time.Time
	ClientIP string
	Browser  string
}

func mustCreateSession(db *gorm.DB, t *testing.T, opts SessionCreateOpts) Session {
	t.Helper()
	s := Session{
		UserID:       opts.UserID,
		SessionToken: opts.Token,
		ExpiresAt:    opts.Expires,
		ClientIP:     opts.ClientIP,
		Browser:      opts.Browser,
	}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	return s
}

// mustCreateMultipleSessions creates `count` sessions for a given user using
// a token prefix and an expiry offset (relative to now). Tokens will be
// generated as prefix + index.
type MultipleSessionsOpts struct {
	UserID        uint
	Count         int
	Prefix        string
	ExpiresOffset time.Duration
}

func mustCreateMultipleSessions(db *gorm.DB, t *testing.T, opts MultipleSessionsOpts) {
	t.Helper()
	for i := 0; i < opts.Count; i++ {
		token := fmt.Sprintf("%s%d", opts.Prefix, i)
		mustCreateSession(db, t, SessionCreateOpts{
			UserID:  opts.UserID,
			Token:   token,
			Expires: time.Now().Add(opts.ExpiresOffset),
		})
	}
}

func TestSessionModel_Create(t *testing.T) {
	db := setupSessionTestDB(t)

	// Create user first
	user := mustCreateDefaultUser(db, t)

	s := mustCreateSession(db, t, SessionCreateOpts{UserID: user.ID, Token: "token123", Expires: time.Now().Add(time.Hour)})
	assert.NotZero(t, s.ID)
}

func TestSessionModel_Read(t *testing.T) {
	db := setupSessionTestDB(t)
	user := mustCreateDefaultUser(db, t)

	s := mustCreateSession(db, t, SessionCreateOpts{UserID: user.ID, Token: "read-token", Expires: time.Now().Add(time.Hour)})

	var found Session
	err := db.First(&found, s.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "read-token", found.SessionToken)
	assert.Equal(t, user.ID, found.UserID)
}

func TestSessionModel_Delete(t *testing.T) {
	db := setupSessionTestDB(t)
	user := mustCreateDefaultUser(db, t)

	s := mustCreateSession(db, t, SessionCreateOpts{UserID: user.ID, Token: "delete-token", Expires: time.Now().Add(time.Hour)})

	err := db.Delete(&s).Error
	assert.NoError(t, err)

	var found Session
	err = db.First(&found, s.ID).Error
	assert.Error(t, err) // Should be soft deleted
}

func TestSessionModel_FindByToken(t *testing.T) {
	db := setupSessionTestDB(t)
	user := mustCreateDefaultUser(db, t)

	_ = mustCreateSession(db, t, SessionCreateOpts{UserID: user.ID, Token: "find-by-token", Expires: time.Now().Add(time.Hour)})

	var found Session
	err := db.Where("session_token = ?", "find-by-token").First(&found).Error
	assert.NoError(t, err)
	assert.Equal(t, user.ID, found.UserID)
}

func TestSessionModel_ExpiredSession(t *testing.T) {
	db := setupSessionTestDB(t)
	user := mustCreateDefaultUser(db, t)

	// Create expired session
	_ = mustCreateSession(db, t, SessionCreateOpts{UserID: user.ID, Token: "expired-token", Expires: time.Now().Add(-time.Hour)})

	// Query for active sessions (not expired)
	var activeSessions []Session
	err := db.Where("user_id = ? AND expires_at > ?", user.ID, time.Now()).Find(&activeSessions).Error
	assert.NoError(t, err)
	assert.Equal(t, 0, len(activeSessions))
}

func TestSessionModel_ValidSession(t *testing.T) {
	db := setupSessionTestDB(t)
	user := mustCreateDefaultUser(db, t)

	_ = mustCreateSession(db, t, SessionCreateOpts{UserID: user.ID, Token: "valid-token", Expires: time.Now().Add(time.Hour)})

	// Query for active sessions
	var activeSessions []Session
	err := db.Where("user_id = ? AND expires_at > ?", user.ID, time.Now()).Find(&activeSessions).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(activeSessions), 1)
}

func TestSessionModel_WithClientInfo(t *testing.T) {
	db := setupSessionTestDB(t)
	user := mustCreateDefaultUser(db, t)

	s := mustCreateSession(db, t, SessionCreateOpts{UserID: user.ID, Token: "client-info-token", Expires: time.Now().Add(time.Hour), ClientIP: "192.168.1.1", Browser: "Mozilla/5.0"})

	var found Session
	db.First(&found, s.ID)
	assert.Equal(t, "192.168.1.1", found.ClientIP)
	assert.Equal(t, "Mozilla/5.0", found.Browser)
}

func TestSessionModel_MultipleSessionsPerUser(t *testing.T) {
	db := setupSessionTestDB(t)
	user := mustCreateDefaultUser(db, t)

	// Create multiple sessions for same user
	mustCreateMultipleSessions(db, t, MultipleSessionsOpts{UserID: user.ID, Count: 3, Prefix: "multi-token-", ExpiresOffset: time.Hour})

	var sessions []Session
	err := db.Where("user_id = ?", user.ID).Find(&sessions).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(sessions), 3)
}

func TestSessionModel_Timestamps(t *testing.T) {
	db := setupSessionTestDB(t)
	user := mustCreateDefaultUser(db, t)

	s := mustCreateSession(db, t, SessionCreateOpts{UserID: user.ID, Token: "timestamp-token", Expires: time.Now().Add(time.Hour)})

	assert.NotZero(t, s.CreatedAt)
	assert.NotZero(t, s.UpdatedAt)
}

func TestSessionModel_DeleteExpiredSessions(t *testing.T) {
	db := setupSessionTestDB(t)
	user := mustCreateDefaultUser(db, t)

	// Create expired sessions
	mustCreateMultipleSessions(db, t, MultipleSessionsOpts{UserID: user.ID, Count: 5, Prefix: "cleanup-token-", ExpiresOffset: -1 * time.Hour})

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
	user := mustCreateDefaultUser(db, t)

	// Create sessions
	mustCreateMultipleSessions(db, t, MultipleSessionsOpts{UserID: user.ID, Count: 4, Prefix: "count-token-", ExpiresOffset: time.Hour})

	var count int64
	err := db.Model(&Session{}).Where("user_id = ?", user.ID).Count(&count).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(4))
}

func TestSessionModel_UpdateExpiry(t *testing.T) {
	db := setupSessionTestDB(t)
	user := mustCreateDefaultUser(db, t)

	s := mustCreateSession(db, t, SessionCreateOpts{UserID: user.ID, Token: "update-expiry-token", Expires: time.Now().Add(time.Hour)})

	// Update expiry time
	newExpiry := time.Now().Add(2 * time.Hour)
	s.ExpiresAt = newExpiry
	err := db.Save(&s).Error
	assert.NoError(t, err)

	var updated Session
	db.First(&updated, s.ID)
	assert.True(t, updated.ExpiresAt.After(time.Now().Add(time.Hour)))
}
