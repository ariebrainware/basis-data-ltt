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
