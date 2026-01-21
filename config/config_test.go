package config

import (
	"os"
	"testing"
)

// Test that LoadConfig returns a non-nil config and respects APPENV=test
func TestLoadConfigAndConnectMySQL_TestEnv(t *testing.T) {
	// Ensure APPENV=test so ConnectMySQL uses in-memory sqlite
	t.Setenv("APPENV", "test")

	cfg := LoadConfig()
	if cfg == nil {
		t.Fatalf("expected non-nil config")
	}

	db, err := ConnectMySQL()
	if err != nil {
		t.Fatalf("ConnectMySQL failed in test env: %v", err)
	}
	if db == nil {
		t.Fatalf("expected non-nil DB connection")
	}

	// cleanup environment (t.Setenv will restore automatically in Go 1.17+)
	_ = os.Unsetenv("APPENV")
}
