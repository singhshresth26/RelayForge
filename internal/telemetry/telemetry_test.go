package telemetry_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/relayforge/relayforge/internal/telemetry"
)

func TestTraceHeaderGeneration(t *testing.T) {
	traceID := telemetry.GenerateTraceID()
	if len(traceID) != 32 {
		t.Errorf("expected 32-char hex trace ID, got %d chars: %s", len(traceID), traceID)
	}

	spanID := telemetry.GenerateSpanID()
	if len(spanID) != 16 {
		t.Errorf("expected 16-char hex span ID, got %d chars: %s", len(spanID), spanID)
	}

	req, _ := http.NewRequest("GET", "/test", nil)
	telemetry.InjectTraceHeader(req, traceID, spanID)

	header := req.Header.Get(telemetry.TraceParentHeader)
	if !strings.HasPrefix(header, "00-") {
		t.Errorf("expected traceparent to start with 00-, got %s", header)
	}
	if !strings.Contains(header, traceID) {
		t.Errorf("expected header to contain trace ID %s, got %s", traceID, header)
	}
}

func TestMetricsInitialization(t *testing.T) {
	telemetry.InitMetrics()

	if telemetry.HttpRequestsTotal == nil {
		t.Errorf("expected HttpRequestsTotal to be initialized")
	}
	if telemetry.HttpRequestDuration == nil {
		t.Errorf("expected HttpRequestDuration to be initialized")
	}
	if telemetry.EventsIngestedTotal == nil {
		t.Errorf("expected EventsIngestedTotal to be initialized")
	}
	if telemetry.DeliveriesTotal == nil {
		t.Errorf("expected DeliveriesTotal to be initialized")
	}
}

