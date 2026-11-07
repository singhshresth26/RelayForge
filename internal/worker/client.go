package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/relayforge/relayforge/internal/fanout"
	"github.com/relayforge/relayforge/internal/security"
)

const (
	MaxResponseBodyLogSize = 4096 // Truncate response body log to 4KB
	DefaultUserAgent       = "RelayForge-Delivery-Worker/1.0"
)

type DeliveryResult struct {
	StatusCode      *int32
	LatencyMS       int32
	RequestHeaders  []byte
	ResponseHeaders []byte
	ResponseBody    *string
	ErrorMessage    *string
	IsSuccess       bool
}

type HTTPDeliveryClient struct {
	client  *http.Client
	cryptor *security.SecretCryptor
	ssrf    *security.SSRFValidator
}

func NewHTTPDeliveryClient(cryptor *security.SecretCryptor, ssrf *security.SSRFValidator) *HTTPDeliveryClient {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DialContext:         ssrf.SafeDialContext(),
		ForceAttemptHTTP2:   true,
	}

	return &HTTPDeliveryClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
		},
		cryptor: cryptor,
		ssrf:    ssrf,
	}
}

// ExecuteDelivery sends HTTP POST to destination with HMAC signing and SSRF checks
func (c *HTTPDeliveryClient) ExecuteDelivery(ctx context.Context, job *fanout.DeliveryJob) *DeliveryResult {
	start := time.Now()

	// 1. Decrypt endpoint secret
	secret := ""
	if c.cryptor != nil && job.SecretEncrypted != "" {
		decrypted, err := c.cryptor.Decrypt(job.SecretEncrypted)
		if err == nil {
			secret = decrypted
		}
	}

	// 2. Timeout context
	timeout := 10 * time.Second
	if job.TimeoutMS > 0 {
		timeout = time.Duration(job.TimeoutMS) * time.Millisecond
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 3. Create HTTP Request
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, job.URL, bytes.NewReader(job.Payload))
	if err != nil {
		errMsg := fmt.Sprintf("failed to create http request: %v", err)
		return &DeliveryResult{
			LatencyMS:    int32(time.Since(start).Milliseconds()),
			ErrorMessage: &errMsg,
			IsSuccess:    false,
		}
	}

	// 4. Setup Headers
	timestamp := time.Now().Unix()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("X-RelayForge-Delivery", job.DeliveryID.String())
	req.Header.Set("X-RelayForge-Event", job.EventID.String())
	req.Header.Set("X-RelayForge-Timestamp", fmt.Sprintf("%d", timestamp))

	if secret != "" {
		signature := security.SignPayload(secret, timestamp, job.Payload)
		req.Header.Set("X-RelayForge-Signature", signature)
	}

	// 5. Execute Request
	resp, err := c.client.Do(req)
	latency := int32(time.Since(start).Milliseconds())

	if err != nil {
		errMsg := err.Error()
		return &DeliveryResult{
			LatencyMS:    latency,
			ErrorMessage: &errMsg,
			IsSuccess:    false,
		}
	}
	defer resp.Body.Close()

	statusCode := int32(resp.StatusCode)
	isSuccess := statusCode >= 200 && statusCode < 300

	// Read response body safely (max 4KB)
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBodyLogSize))
	bodyStr := string(bodyBytes)

	return &DeliveryResult{
		StatusCode:   &statusCode,
		LatencyMS:    latency,
		ResponseBody: &bodyStr,
		IsSuccess:    isSuccess,
	}
}

