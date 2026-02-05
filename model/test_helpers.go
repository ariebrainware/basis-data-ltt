package model

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite database for testing with the specified model.
// It automatically runs AutoMigrate on the provided model and returns the database connection.
// The database name is uniquified using the current Unix nanosecond timestamp to prevent
// cross-test contamination when tests run in the same process.
func setupTestDB(t *testing.T, modelName string, models ...interface{}) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:testdb_%s_%d?mode=memory&cache=shared", modelName, time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if len(models) > 0 {
		if err := db.AutoMigrate(models...); err != nil {
			t.Fatalf("failed to auto-migrate models: %v", err)
		}
	}

	return db
}
