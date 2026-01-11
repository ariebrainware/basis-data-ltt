package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
)

var JWTSecret = getEnv("JWTSECRET", "")
var JWTSecretByte = []byte(getEnv("JWTSECRET", ""))

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	return value
}

func HashPassword(password string) (hashedPassword string) {
	h := hmac.New(sha256.New, JWTSecretByte)
	h.Write([]byte(password))
	hashedPassword = hex.EncodeToString(h.Sum(nil))
	return
}

// SetJWTSecret allows tests or runtime code to update the JWT secret used
// for both token signing and password hashing.
func SetJWTSecret(secret string) {
	JWTSecret = secret
	JWTSecretByte = []byte(secret)
}
