package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"
)

var (
	jwtSecretValue = getEnv("JWTSECRET", "")
	jwtSecret      = jwtSecretValue
	jwtSecretByte  = []byte(jwtSecretValue)
	jwtMutex       sync.RWMutex
)

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	return value
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
	jwtSecret = secret
	jwtSecretByte = []byte(secret)
}

// GetJWTSecretByte returns a copy of the current JWT secret bytes in a thread-safe manner.
func GetJWTSecretByte() []byte {
	jwtMutex.RLock()
	defer jwtMutex.RUnlock()
	// Return a copy to prevent external modifications using idiomatic Go pattern
	return append([]byte(nil), jwtSecretByte...)
}
