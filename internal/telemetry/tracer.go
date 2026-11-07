package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
)

const (
	TraceParentHeader = "traceparent"
)

// GenerateTraceID generates a 16-byte random hex string (W3C trace-id)
func GenerateTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateSpanID generates an 8-byte random hex string (W3C parent-id/span-id)
func GenerateSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// InjectTraceHeader injects W3C traceparent header into HTTP request:
// format: 00-<trace_id>-<span_id>-01
func InjectTraceHeader(req *http.Request, traceID, spanID string) {
	if traceID == "" {
		traceID = GenerateTraceID()
	}
	if spanID == "" {
		spanID = GenerateSpanID()
	}
	req.Header.Set(TraceParentHeader, fmt.Sprintf("00-%s-%s-01", traceID, spanID))
}

