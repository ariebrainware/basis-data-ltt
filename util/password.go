package util

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

var (
	jwtSecretByte []byte
	jwtMutex      sync.RWMutex
)

const (
	// Argon2id parameters (exceeds minimum cost of 12)
	argon2Time    = 3         // Number of iterations
	argon2Memory  = 64 * 1024 // 64 MB
	argon2Threads = 4         // Number of threads
	argon2KeyLen  = 32        // Length of the derived key
	saltLength    = 16        // Length of the salt in bytes
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

// GenerateSalt generates a cryptographically secure random salt
func GenerateSalt() (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	return base64.RawStdEncoding.EncodeToString(salt), nil
}

// HashPasswordArgon2 hashes a password using Argon2id with a unique salt
// Returns the encoded hash in the format: argon2id$base64(salt)$base64(hash)
func HashPasswordArgon2(password, salt string) (string, error) {
	saltBytes, err := base64.RawStdEncoding.DecodeString(salt)
	if err != nil {
		return "", fmt.Errorf("failed to decode salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), saltBytes, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	// Format: argon2id$salt$hash
	return fmt.Sprintf("argon2id$%s$%s", salt, encodedHash), nil
}

// VerifyPasswordArgon2 verifies a password against an Argon2id hash
func VerifyPasswordArgon2(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false, fmt.Errorf("invalid hash format")
	}

	salt := parts[1]
	expectedHash := parts[2]

	saltBytes, err := base64.RawStdEncoding.DecodeString(salt)
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), saltBytes, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	actualHash := base64.RawStdEncoding.EncodeToString(hash)

	// Use constant-time comparison to prevent timing attacks
	expectedHashBytes := []byte(expectedHash)
	actualHashBytes := []byte(actualHash)

	if len(expectedHashBytes) != len(actualHashBytes) {
		return false, nil
	}

	return subtle.ConstantTimeCompare(expectedHashBytes, actualHashBytes) == 1, nil
}

// HashPassword is deprecated - use HashPasswordArgon2 for new passwords
// This is kept for backward compatibility with existing passwords
func HashPassword(password string) (hashedPassword string) {
	secretByte := GetJWTSecretByte()
	h := hmac.New(sha256.New, secretByte)
	h.Write([]byte(password))
	hashedPassword = hex.EncodeToString(h.Sum(nil))
	return
}

// VerifyPassword verifies a password against either Argon2id or legacy HMAC hash
// The salt parameter is only used for future extensibility and is currently unused
// because Argon2id embeds the salt in the hash string. For legacy HMAC hashes,
// no separate salt was used.
func VerifyPassword(password, hash, salt string) (bool, error) {
	// Check if it's an Argon2id hash (salt is embedded in the hash)
	if strings.HasPrefix(hash, "argon2id$") {
		return VerifyPasswordArgon2(password, hash)
	}

	// Legacy HMAC verification with constant-time comparison (no separate salt)
	expectedHash := HashPassword(password)
	expectedBytes := []byte(expectedHash)
	actualBytes := []byte(hash)

	if len(expectedBytes) != len(actualBytes) {
		return false, nil
	}

	return subtle.ConstantTimeCompare(expectedBytes, actualBytes) == 1, nil
}

// SetJWTSecret allows tests or runtime code to update the JWT secret used
// for token signing. This function is thread-safe and can be called concurrently.
// Tests using this should avoid parallel execution if they need deterministic secret values.
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
