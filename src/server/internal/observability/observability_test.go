package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJSONLoggerWritesStructuredSanitizedEvent(t *testing.T) {
	var output bytes.Buffer
	logger := NewJSONLogger(&output)

	logger.Log(context.Background(), Event{
		Component:      "http",
		Event:          "http.request",
		RequestID:      "req_123",
		OrganizationID: "org_123",
		UserID:         "user_123",
		Method:         "POST",
		Route:          "/api/v1/app/agents/:id",
		Status:         201,
		Latency:        25 * time.Millisecond,
		Fields: map[string]any{
			"authorization": "Bearer sk-secret",
			"database_url":  "postgres://user:password@localhost/db",
			"prompt":        "customer prompt should not be logged",
			"safe_field":    "safe-value",
		},
	})

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatalf("expected JSON log record, got error: %v\n%s", err, output.String())
	}

	if record["component"] != "http" {
		t.Fatalf("expected component http, got %#v", record["component"])
	}
	if record["event"] != "http.request" {
		t.Fatalf("expected event http.request, got %#v", record["event"])
	}
	if record["request_id"] != "req_123" {
		t.Fatalf("expected request id req_123, got %#v", record["request_id"])
	}
	if record["organization_id"] != "org_123" {
		t.Fatalf("expected organization id org_123, got %#v", record["organization_id"])
	}
	if record["user_id"] != "user_123" {
		t.Fatalf("expected user id user_123, got %#v", record["user_id"])
	}
	if record["route"] != "/api/v1/app/agents/:id" {
		t.Fatalf("expected normalized route, got %#v", record["route"])
	}
	if record["latency_ms"] != float64(25) {
		t.Fatalf("expected latency_ms 25, got %#v", record["latency_ms"])
	}
	if record["safe_field"] != "safe-value" {
		t.Fatalf("expected safe custom field, got %#v", record["safe_field"])
	}

	logText := output.String()
	for _, forbidden := range []string{"sk-secret", "postgres://", "customer prompt", "authorization", "database_url"} {
		if strings.Contains(logText, forbidden) {
			t.Fatalf("log leaked forbidden value %q: %s", forbidden, logText)
		}
	}
}

func TestStartSpanIsNoopSafe(t *testing.T) {
	ctx, span := StartSpan(context.Background(), "http.request", String("component", "http"), Int("status", 200))
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if span == nil {
		t.Fatal("expected non-nil span")
	}

	span.End()
}

func TestMemoryReporterCapturesSanitizedErrorEvents(t *testing.T) {
	reporter := NewMemoryReporter()

	reporter.ReportError(context.Background(), Event{
		Component:     "billing",
		Event:         "quota.settlement_failed",
		RequestID:     "req_456",
		FailureReason: "quota_store_timeout",
		Fields: map[string]any{
			"stripe_payload": `{"api_key":"sk-secret"}`,
			"safe_field":     "safe-value",
		},
	})

	events := reporter.Events()
	if len(events) != 1 {
		t.Fatalf("expected one reported event, got %d", len(events))
	}
	if events[0].Component != "billing" || events[0].Event != "quota.settlement_failed" {
		t.Fatalf("unexpected reported event: %+v", events[0])
	}
	if events[0].Fields["safe_field"] != "safe-value" {
		t.Fatalf("expected safe field to survive sanitization, got %+v", events[0].Fields)
	}
	if _, ok := events[0].Fields["stripe_payload"]; ok {
		t.Fatalf("expected sensitive payload field to be removed: %+v", events[0].Fields)
	}
}
