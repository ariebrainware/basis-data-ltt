package util

import "testing"

func TestHashPasswordDeterministic(t *testing.T) {
	SetJWTSecret("secret1")
	h1 := HashPassword("password")
	h2 := HashPassword("password")
	if h1 != h2 {
		t.Fatalf("expected same hash for same secret, got %s vs %s", h1, h2)
	}
}

func TestHashPasswordDifferentSecrets(t *testing.T) {
	SetJWTSecret("secretA")
	h1 := HashPassword("password")
	SetJWTSecret("secretB")
	h2 := HashPassword("password")
	if h1 == h2 {
		t.Fatalf("expected different hashes for different secrets, both %s", h1)
	}
}
