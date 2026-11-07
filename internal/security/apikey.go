package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	APIKeyLivePrefix = "rf_live_"
	APIKeyTestPrefix = "rf_test_"
	APIKeyRandomBytes = 24
)

var (
	ErrInvalidAPIKeyFormat = errors.New("invalid api key format")
)

// GenerateAPIKey creates a new API key with the given prefix (e.g. rf_live_...)
// It returns the plaintext token (to be shown once to the user), the prefix, and the SHA-256 hash (to be stored).
func GenerateAPIKey(isLive bool) (plaintext string, prefix string, hash string, err error) {
	b := make([]byte, APIKeyRandomBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	tokenHex := hex.EncodeToString(b)
	if isLive {
		prefix = APIKeyLivePrefix
	} else {
		prefix = APIKeyTestPrefix
	}

	plaintext = prefix + tokenHex
	hash = HashAPIKey(plaintext)
	return plaintext, prefix, hash, nil
}

// HashAPIKey computes the SHA-256 hex digest of a raw API key token.
func HashAPIKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// VerifyAPIKey verifies a raw token against a stored hash in constant time.
func VerifyAPIKey(token, expectedHash string) bool {
	actualHash := HashAPIKey(token)
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}

// ExtractPrefix returns the prefix from a key string.
func ExtractPrefix(key string) (string, error) {
	if strings.HasPrefix(key, APIKeyLivePrefix) {
		return APIKeyLivePrefix, nil
	}
	if strings.HasPrefix(key, APIKeyTestPrefix) {
		return APIKeyTestPrefix, nil
	}
	return "", ErrInvalidAPIKeyFormat
}

