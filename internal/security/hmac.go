package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	SignatureHeader = "X-RelayForge-Signature"
	TimestampHeader = "X-RelayForge-Timestamp"
)

// SignPayload generates an HMAC-SHA256 signature for the given timestamp and body payload.
// Signature format: v1=<hex_signature>
func SignPayload(secret string, timestamp int64, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	// Payload format: timestamp.payload
	tsStr := strconv.FormatInt(timestamp, 10)
	mac.Write([]byte(tsStr))
	mac.Write([]byte("."))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("v1=%s", signature)
}

// VerifySignature validates that the provided signature matches the payload and secret,
// and checks that the timestamp does not exceed the allowed tolerance (e.g. 5 minutes).
func VerifySignature(secret, signatureHeader string, timestamp int64, payload []byte, tolerance time.Duration) bool {
	if tolerance > 0 {
		now := time.Now().Unix()
		diff := now - timestamp
		if diff < 0 {
			diff = -diff
		}
		if diff > int64(tolerance.Seconds()) {
			return false
		}
	}

	expectedSig := SignPayload(secret, timestamp, payload)

	// signatureHeader can be either "v1=xxx" or "v1=xxx,v1=yyy"
	signatures := strings.Split(signatureHeader, ",")
	for _, sig := range signatures {
		sig = strings.TrimSpace(sig)
		if subtle.ConstantTimeCompare([]byte(sig), []byte(expectedSig)) == 1 {
			return true
		}
	}

	return false
}

