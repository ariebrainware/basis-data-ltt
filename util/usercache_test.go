package util

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInitUserEmailCache(t *testing.T) {
	// Test with default capacity
	InitUserEmailCache(0)
	if userCache == nil {
		t.Fatal("Expected userCache to be initialized")
	}
	if userCache.capacity != 1000 {
		t.Errorf("Expected default capacity 1000, got %d", userCache.capacity)
	}

	// Test with specific capacity
	InitUserEmailCache(50)
	if userCache.capacity != 50 {
		t.Errorf("Expected capacity 50, got %d", userCache.capacity)
	}
}

func TestUserEmailCacheGetSet(t *testing.T) {
	InitUserEmailCache(3)

	// Test cache miss
	email, ok := UserEmailCacheGet(1)
	if ok {
		t.Error("Expected cache miss for non-existent key")
	}
	if email != "" {
		t.Errorf("Expected empty email, got %q", email)
	}

	// Test cache set and get
	UserEmailCacheSet(1, "user1@example.com")
	email, ok = UserEmailCacheGet(1)
	if !ok {
		t.Error("Expected cache hit")
	}
	if email != "user1@example.com" {
		t.Errorf("Expected user1@example.com, got %q", email)
	}

	// Test cache update
	UserEmailCacheSet(1, "updated@example.com")
	email, ok = UserEmailCacheGet(1)
	if !ok {
		t.Error("Expected cache hit after update")
	}
	if email != "updated@example.com" {
		t.Errorf("Expected updated@example.com, got %q", email)
	}
}

func TestUserEmailCacheEviction(t *testing.T) {
	InitUserEmailCache(3)

	// Fill cache to capacity
	UserEmailCacheSet(1, "user1@example.com")
	UserEmailCacheSet(2, "user2@example.com")
	UserEmailCacheSet(3, "user3@example.com")

	// Verify all are in cache
	if _, ok := UserEmailCacheGet(1); !ok {
		t.Error("Expected user 1 in cache")
	}
	if _, ok := UserEmailCacheGet(2); !ok {
		t.Error("Expected user 2 in cache")
	}
	if _, ok := UserEmailCacheGet(3); !ok {
		t.Error("Expected user 3 in cache")
	}

	// Add one more, should evict least recently used (user 1)
	UserEmailCacheSet(4, "user4@example.com")

	// User 1 should be evicted
	if _, ok := UserEmailCacheGet(1); ok {
		t.Error("Expected user 1 to be evicted")
	}

	// Others should still be present
	if _, ok := UserEmailCacheGet(2); !ok {
		t.Error("Expected user 2 still in cache")
	}
	if _, ok := UserEmailCacheGet(3); !ok {
		t.Error("Expected user 3 still in cache")
	}
	if _, ok := UserEmailCacheGet(4); !ok {
		t.Error("Expected user 4 in cache")
	}
}

func TestUserEmailCacheLRUOrdering(t *testing.T) {
	InitUserEmailCache(3)

	// Add items
	UserEmailCacheSet(1, "user1@example.com")
	UserEmailCacheSet(2, "user2@example.com")
	UserEmailCacheSet(3, "user3@example.com")

	// Access user 1 to make it recently used
	UserEmailCacheGet(1)

	// Add user 4, should evict user 2 (least recently used)
	UserEmailCacheSet(4, "user4@example.com")

	if _, ok := UserEmailCacheGet(1); !ok {
		t.Error("Expected user 1 still in cache (recently accessed)")
	}
	if _, ok := UserEmailCacheGet(2); ok {
		t.Error("Expected user 2 to be evicted")
	}
	if _, ok := UserEmailCacheGet(3); !ok {
		t.Error("Expected user 3 still in cache")
	}
	if _, ok := UserEmailCacheGet(4); !ok {
		t.Error("Expected user 4 in cache")
	}
}

func TestGetUserEmail_WithCache(t *testing.T) {
	InitUserEmailCache(10)

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create users table
	err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)").Error
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	// Insert test user
	err = db.Exec("INSERT INTO users (id, email) VALUES (1, 'test@example.com')").Error
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	// Test cache miss and DB lookup
	email := GetUserEmail(db, 1)
	if email != "test@example.com" {
		t.Errorf("Expected test@example.com, got %q", email)
	}

	// Verify it's now in cache
	cachedEmail, ok := UserEmailCacheGet(1)
	if !ok {
		t.Error("Expected email to be cached after DB lookup")
	}
	if cachedEmail != "test@example.com" {
		t.Errorf("Expected cached email test@example.com, got %q", cachedEmail)
	}

	// Test cache hit (remove from DB to verify cache is used)
	err = db.Exec("DELETE FROM users WHERE id = 1").Error
	if err != nil {
		t.Fatalf("Failed to delete test user: %v", err)
	}

	email = GetUserEmail(db, 1)
	if email != "test@example.com" {
		t.Errorf("Expected cached email test@example.com, got %q", email)
	}
}

func TestGetUserEmail_EdgeCases(t *testing.T) {
	InitUserEmailCache(10)

	// Test with userID 0
	email := GetUserEmail(nil, 0)
	if email != "" {
		t.Errorf("Expected empty string for userID 0, got %q", email)
	}

	// Test with nil DB
	email = GetUserEmail(nil, 1)
	if email != "" {
		t.Errorf("Expected empty string with nil DB, got %q", email)
	}

	// Test with non-existent user
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)").Error
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	email = GetUserEmail(db, 999)
	if email != "" {
		t.Errorf("Expected empty string for non-existent user, got %q", email)
	}
}

func TestUserEmailCache_NilCache(t *testing.T) {
	// Test operations when cache is nil
	userCache = nil

	email, ok := UserEmailCacheGet(1)
	if ok {
		t.Error("Expected false when cache is nil")
	}
	if email != "" {
		t.Errorf("Expected empty string when cache is nil, got %q", email)
	}

	// Should not panic
	UserEmailCacheSet(1, "test@example.com")
}

func TestInitUserEmailCacheFromEnv(t *testing.T) {
	// Test will use default capacity when env var is not set
	// Just verify it doesn't panic
	InitUserEmailCacheFromEnv()
	if userCache == nil {
		t.Fatal("Expected userCache to be initialized")
	}
}
