package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
)

var (
	jwtSecretByte []byte
	jwtMutex      sync.RWMutex
)

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	return value
}

// InitJWTSecretFromEnv initializes the in-memory JWT secret from the `JWTSECRET` environment variable.
// Call this early in application startup (e.g. in main) to ensure the secret is set.
func InitJWTSecretFromEnv() {
	SetJWTSecret(getEnv("JWTSECRET", ""))
}

// ValidateJWTSecret returns an error when the JWT secret is empty.
// The caller can decide whether to treat an empty secret as fatal (recommended for non-test envs).
func ValidateJWTSecret() error {
	secret := GetJWTSecretByte()
	if len(secret) == 0 {
		return fmt.Errorf("JWTSECRET is not configured")
	}
	return nil
}

func HashPassword(password string) (hashedPassword string) {
	secretByte := GetJWTSecretByte()
	h := hmac.New(sha256.New, secretByte)
	h.Write([]byte(password))
	hashedPassword = hex.EncodeToString(h.Sum(nil))
	return
}

// SetJWTSecret allows tests or runtime code to update the JWT secret used
// for both token signing and password hashing. This function is thread-safe
// and can be called concurrently. Tests using this should avoid parallel execution
// if they need deterministic secret values.
func SetJWTSecret(secret string) {
	jwtMutex.Lock()
	defer jwtMutex.Unlock()
	jwtSecretByte = []byte(secret)
}

// GetJWTSecretByte returns a copy of the current JWT secret bytes in a thread-safe manner.
func GetJWTSecretByte() []byte {
	jwtMutex.RLock()
	defer jwtMutex.RUnlock()
	// Return a copy to prevent external modifications using idiomatic Go pattern
	return append([]byte(nil), jwtSecretByte...)
}
