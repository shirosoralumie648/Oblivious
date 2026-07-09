package http

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"oblivious/server/internal/config"
	"oblivious/server/internal/observability"
	"oblivious/server/internal/relay/handler"
	"oblivious/server/internal/relay/types"
)

type captureRouteAuditAlertSink struct {
	events []observability.AlertEvent
	err    error
}

func (s *captureRouteAuditAlertSink) Notify(_ context.Context, event observability.AlertEvent) error {
	s.events = append(s.events, event)
	return s.err
}

func TestBuildRelayConfigInjectsRouteAuditSinkFromRequestLogSink(t *testing.T) {
	sink := &captureMiddlewareRequestLogSink{}
	restore := setRequestLogSink(sink)
	defer restore()

	relayConfig := buildRelayConfig(config.Config{Env: "production"}, nil, nil, nil, nil, nil, nil)
	if relayConfig.RouteAuditSink == nil {
		t.Fatal("expected HTTP server relay config to include route audit sink")
	}

	createdAt := time.Date(2026, 7, 2, 8, 0, 0, 0, time.UTC)
	relayConfig.RouteAuditSink.RecordRelayRouteDecision(context.Background(), handler.RouteAuditEvent{
		Method:         "POST",
		Path:           "/v1/files",
		APIType:        types.APITypeFiles,
		Class:          handler.CommercialSupportedBilled,
		Result:         handler.RouteAuditResultAllowed,
		UserID:         "user_1",
		OrganizationID: "org_1",
		RequestID:      "req_1",
		CreatedAt:      createdAt,
	})

	if len(sink.rows) != 1 {
		t.Fatalf("expected one route audit request log row, got %+v", sink.rows)
	}
	row := sink.rows[0]
	if row.Service != "relay" || row.Endpoint != "/v1/files" || row.Method != "POST" || row.StatusCode != 200 {
		t.Fatalf("unexpected route audit request log row: %+v", row)
	}
	if !row.Timestamp.Equal(createdAt) {
		t.Fatalf("expected createdAt timestamp, got %s", row.Timestamp)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(row.Metadata), &metadata); err != nil {
		t.Fatalf("decode route audit metadata: %v", err)
	}
	if metadata["relay_api_type"] != "files" || metadata["relay_route_class"] != string(handler.CommercialSupportedBilled) || metadata["relay_route_result"] != string(handler.RouteAuditResultAllowed) {
		t.Fatalf("unexpected route audit metadata: %+v", metadata)
	}
}

func TestRelayRouteAuditSinkAlertsWhenRequestLogWriteFails(t *testing.T) {
	sink := &captureMiddlewareRequestLogSink{err: errors.New("clickhouse route audit unavailable")}
	alertSink := &captureRouteAuditAlertSink{}
	restoreAlert := setHTTPAlertSink(alertSink)
	defer restoreAlert()

	createdAt := time.Date(2026, 7, 5, 4, 5, 0, 0, time.UTC)
	auditSink := newRelayRouteAuditRequestLogSink(sink)
	if auditSink == nil {
		t.Fatal("expected route audit sink")
	}

	auditSink.RecordRelayRouteDecision(context.Background(), handler.RouteAuditEvent{
		Method:         "POST",
		Path:           "/v1/chat/completions",
		APIType:        types.APITypeChat,
		Class:          handler.CommercialSupportedBilled,
		Result:         handler.RouteAuditResultAllowed,
		UserID:         "user_1",
		OrganizationID: "org_1",
		RequestID:      "req_route_audit_failure",
		CreatedAt:      createdAt,
	})

	if len(alertSink.events) != 1 {
		t.Fatalf("expected request-log failure alert, got %+v", alertSink.events)
	}
	event := alertSink.events[0]
	if event.Component != observability.ComponentObservability || event.Severity != observability.AlertSeverityCritical {
		t.Fatalf("unexpected alert classification: %+v", event)
	}
	if event.Fields["source"] != "http.request_log" || event.Fields["failure_kind"] != "request_log_sink" {
		t.Fatalf("unexpected alert fields: %+v", event.Fields)
	}
	if event.Fields["route"] != "/v1/chat/completions" || event.Fields["request_id"] != "req_route_audit_failure" {
		t.Fatalf("expected route/request id in alert, got %+v", event.Fields)
	}
}
