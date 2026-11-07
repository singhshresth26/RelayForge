package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	ErrInvalidCiphertext = errors.New("ciphertext is invalid or corrupt")
	ErrInvalidMasterKey  = errors.New("master key must be exactly 32 bytes for AES-256")
)

type SecretCryptor struct {
	key []byte
}

func NewSecretCryptor(masterKeyHex string) (*SecretCryptor, error) {
	keyBytes := []byte(masterKeyHex)
	// If 32 chars ASCII or 64 hex characters, normalize:
	if len(keyBytes) != 32 {
		return nil, ErrInvalidMasterKey
	}
	return &SecretCryptor{key: keyBytes}, nil
}

// Encrypt encrypts plaintext using AES-GCM and returns standard Base64 string [nonce + ciphertext + tag]
func (c *SecretCryptor) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("failed to create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts standard Base64 encoded [nonce + ciphertext + tag]
func (c *SecretCryptor) Decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to base64 decode: %w", err)
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("failed to create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt ciphertext: %w", err)
	}

	return string(plaintext), nil
}

// GenerateWebhookSecret generates a secure random signing secret like `whsec_...`
func GenerateWebhookSecret() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "whsec_" + base64.RawURLEncoding.EncodeToString(b), nil
}

