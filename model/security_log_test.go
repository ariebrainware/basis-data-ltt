package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSecurityLogTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&SecurityLog{})
	assert.NoError(t, err)

	return db
}

func TestSecurityLogModel_Create(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	log := SecurityLog{
		EventType: "LOGIN_SUCCESS",
		UserID:    "123",
		Email:     "test@test.com",
		IP:        "192.168.1.1",
		Message:   "User logged in successfully",
	}

	err := db.Create(&log).Error
	assert.NoError(t, err)
	assert.NotZero(t, log.ID)
}

func TestSecurityLogModel_Read(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	log := SecurityLog{
		EventType: "LOGIN_FAILURE",
		Email:     "fail@test.com",
		IP:        "192.168.1.2",
	}
	db.Create(&log)

	var found SecurityLog
	err := db.First(&found, log.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, "LOGIN_FAILURE", found.EventType)
}

func TestSecurityLogModel_AllFields(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	log := SecurityLog{
		EventType: "UNAUTHORIZED_ACCESS",
		UserID:    "456",
		Email:     "user@test.com",
		IP:        "10.0.0.1",
		UserAgent: "Mozilla/5.0",
		Location:  "Jakarta, Indonesia",
		Message:   "Unauthorized access attempt",
		Details:   []byte(`{"reason":"admin_panel_access"}`),
	}

	err := db.Create(&log).Error
	assert.NoError(t, err)

	var found SecurityLog
	db.First(&found, log.ID)
	assert.Equal(t, "UNAUTHORIZED_ACCESS", found.EventType)
	assert.Equal(t, "456", found.UserID)
	assert.Equal(t, "user@test.com", found.Email)
	assert.Equal(t, "10.0.0.1", found.IP)
	assert.Equal(t, "Mozilla/5.0", found.UserAgent)
	assert.Equal(t, "Jakarta, Indonesia", found.Location)
	assert.Equal(t, "Unauthorized access attempt", found.Message)
	assert.NotNil(t, found.Details)
}

func TestSecurityLogModel_ListByEventType(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	// Create multiple logs
	for i := 0; i < 3; i++ {
		log := SecurityLog{
			EventType: "LOGIN_SUCCESS",
			IP:        "192.168.1." + string(rune(i+1)),
		}
		db.Create(&log)
	}

	for i := 0; i < 2; i++ {
		log := SecurityLog{
			EventType: "LOGIN_FAILURE",
			IP:        "192.168.2." + string(rune(i+1)),
		}
		db.Create(&log)
	}

	var successLogs []SecurityLog
	err := db.Where("event_type = ?", "LOGIN_SUCCESS").Find(&successLogs).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(successLogs), 3)
}

func TestSecurityLogModel_ListByUserID(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	// Create logs for specific user
	for i := 0; i < 5; i++ {
		log := SecurityLog{
			EventType: "LOGIN_SUCCESS",
			UserID:    "user123",
			IP:        "192.168.1.1",
		}
		db.Create(&log)
	}

	var userLogs []SecurityLog
	err := db.Where("user_id = ?", "user123").Find(&userLogs).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(userLogs), 5)
}

func TestSecurityLogModel_ListByIP(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	log := SecurityLog{
		EventType: "SUSPICIOUS_ACTIVITY",
		IP:        "10.0.0.100",
	}
	db.Create(&log)

	var ipLogs []SecurityLog
	err := db.Where("ip = ?", "10.0.0.100").Find(&ipLogs).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(ipLogs), 1)
}

func TestSecurityLogModel_Timestamps(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	log := SecurityLog{
		EventType: "TEST_EVENT",
		IP:        "127.0.0.1",
	}
	db.Create(&log)

	assert.NotZero(t, log.CreatedAt)
}

func TestSecurityLogModel_OrderByTimestamp(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	// Create multiple logs
	for i := 0; i < 3; i++ {
		log := SecurityLog{
			EventType: "EVENT_" + string(rune(i)),
			IP:        "192.168.1.1",
		}
		db.Create(&log)
	}

	var logs []SecurityLog
	err := db.Order("created_at DESC").Find(&logs).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(logs), 3)

	// Verify ordering
	if len(logs) >= 2 {
		assert.True(t, logs[0].CreatedAt.After(logs[1].CreatedAt) || logs[0].CreatedAt.Equal(logs[1].CreatedAt))
	}
}

func TestSecurityLogModel_OptionalFields(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	// Create log with minimal fields
	log := SecurityLog{
		EventType: "MINIMAL_EVENT",
		IP:        "127.0.0.1",
	}
	err := db.Create(&log).Error
	assert.NoError(t, err)

	var found SecurityLog
	db.First(&found, log.ID)
	assert.Equal(t, "", found.UserID)
	assert.Equal(t, "", found.Email)
	assert.Equal(t, "", found.UserAgent)
	assert.Equal(t, "", found.Location)
}

func TestSecurityLogModel_LongMessage(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	longMessage := "This is a very long message that contains detailed information about the security event. " +
		"It includes multiple sentences and provides context about what happened, why it happened, and " +
		"what actions were taken in response to the event."

	log := SecurityLog{
		EventType: "DETAILED_EVENT",
		IP:        "192.168.1.1",
		Message:   longMessage,
	}

	err := db.Create(&log).Error
	assert.NoError(t, err)

	var found SecurityLog
	db.First(&found, log.ID)
	assert.Equal(t, longMessage, found.Message)
}

func TestSecurityLogModel_SearchByEmail(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	log := SecurityLog{
		EventType: "SEARCH_TEST",
		Email:     "searchable@test.com",
		IP:        "192.168.1.1",
	}
	db.Create(&log)

	var found []SecurityLog
	err := db.Where("email LIKE ?", "%searchable%").Find(&found).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(found), 1)
}

func TestSecurityLogModel_CountByEventType(t *testing.T) {
	db := setupSecurityLogTestDB(t)

	// Create multiple logs of same type
	for i := 0; i < 7; i++ {
		log := SecurityLog{
			EventType: "COUNT_TEST",
			IP:        "192.168.1.1",
		}
		db.Create(&log)
	}

	var count int64
	err := db.Model(&SecurityLog{}).Where("event_type = ?", "COUNT_TEST").Count(&count).Error
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(7))
}
