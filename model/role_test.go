package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSeedRolesCreatesRoles(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory DB: %v", err)
	}

	if err := db.AutoMigrate(&Role{}); err != nil {
		t.Fatalf("failed to aut migrate: %v", err)
	}

	if err := SeedRoles(db); err != nil {
		t.Fatalf("SeedRoles returned error: %v", err)
	}

	var count int64
	if err := db.Model(&Role{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count roles: %v", err)
	}
	if count < 3 {
		t.Fatalf("expected at least 3 seeded roles, got %d", count)
	}
}
