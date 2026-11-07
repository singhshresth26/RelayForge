package security_test

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/relayforge/relayforge/internal/security"
)

func TestAPIKeyGenerationAndVerification(t *testing.T) {
	// 1. Generate live key
	liveToken, prefix, hash, err := security.GenerateAPIKey(true)
	if err != nil {
		t.Fatalf("failed to generate live api key: %v", err)
	}
	if !strings.HasPrefix(liveToken, security.APIKeyLivePrefix) {
		t.Errorf("expected live key to have prefix %s, got %s", security.APIKeyLivePrefix, liveToken)
	}
	if prefix != security.APIKeyLivePrefix {
		t.Errorf("expected prefix %s, got %s", security.APIKeyLivePrefix, prefix)
	}
	if len(hash) != 64 { // SHA-256 hex string length
		t.Errorf("expected hash length 64, got %d", len(hash))
	}

	// 2. Verify valid key
	if !security.VerifyAPIKey(liveToken, hash) {
		t.Errorf("expected api key verification to succeed for valid token")
	}

	// 3. Verify invalid key
	if security.VerifyAPIKey(liveToken+"invalid", hash) {
		t.Errorf("expected api key verification to fail for tampered token")
	}

	// 4. Generate test key
	testToken, testPrefix, _, err := security.GenerateAPIKey(false)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	if !strings.HasPrefix(testToken, security.APIKeyTestPrefix) {
		t.Errorf("expected test key to have prefix %s, got %s", security.APIKeyTestPrefix, testToken)
	}
	if testPrefix != security.APIKeyTestPrefix {
		t.Errorf("expected prefix %s, got %s", security.APIKeyTestPrefix, testPrefix)
	}
}

func TestSecretCryptor(t *testing.T) {
	masterKey := "01234567890123456789012345678901" // 32 bytes
	cryptor, err := security.NewSecretCryptor(masterKey)
	if err != nil {
		t.Fatalf("failed to create cryptor: %v", err)
	}

	plaintext := "whsec_test_secret_key_123456789"
	encrypted, err := cryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}
	if encrypted == plaintext {
		t.Errorf("encrypted string should not equal plaintext")
	}

	decrypted, err := cryptor.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("expected decrypted text %s, got %s", plaintext, decrypted)
	}

	// Corrupt ciphertext test
	_, err = cryptor.Decrypt("invalid_base64_!!")
	if err == nil {
		t.Errorf("expected error for invalid base64")
	}
}

func TestHMACSignature(t *testing.T) {
	secret := "whsec_super_secret_signing_key"
	payload := []byte(`{"event":"order.created","amount":100}`)
	timestamp := time.Now().Unix()

	sig := security.SignPayload(secret, timestamp, payload)
	if !strings.HasPrefix(sig, "v1=") {
		t.Errorf("expected signature to start with v1=, got %s", sig)
	}

	// Valid verification
	if !security.VerifySignature(secret, sig, timestamp, payload, 5*time.Minute) {
		t.Errorf("expected signature verification to succeed")
	}

	// Tampered payload
	tamperedPayload := []byte(`{"event":"order.created","amount":999}`)
	if security.VerifySignature(secret, sig, timestamp, tamperedPayload, 5*time.Minute) {
		t.Errorf("expected signature verification to fail on tampered payload")
	}

	// Expired timestamp
	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	oldSig := security.SignPayload(secret, oldTimestamp, payload)
	if security.VerifySignature(secret, oldSig, oldTimestamp, payload, 5*time.Minute) {
		t.Errorf("expected signature verification to fail on expired timestamp")
	}
}

func TestSSRFValidator(t *testing.T) {
	validator := security.NewSSRFValidator(false, false)

	// Test private IP detection
	privateIPs := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.5",
		"192.168.1.100",
		"169.254.169.254", // Cloud metadata
		"::1",
	}

	for _, ipStr := range privateIPs {
		ip := net.ParseIP(ipStr)
		if !security.IsPrivateIP(ip) {
			t.Errorf("expected %s to be recognized as private IP", ipStr)
		}
	}

	publicIP := net.ParseIP("8.8.8.8")
	if security.IsPrivateIP(publicIP) {
		t.Errorf("expected 8.8.8.8 to be recognized as public IP")
	}

	// Test URL validation
	if err := validator.ValidateURL("ftp://example.com"); err == nil {
		t.Errorf("expected error for non-http scheme")
	}

	if err := validator.ValidateURL("http://127.0.0.1:8080/webhook"); err == nil {
		t.Errorf("expected error for loopback IP URL")
	}

	if err := validator.ValidateURL("http://169.254.169.254/latest/meta-data"); err == nil {
		t.Errorf("expected error for AWS metadata IP URL")
	}
}
