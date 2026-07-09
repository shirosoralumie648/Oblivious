package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/observability"
	relaytypes "oblivious/server/internal/relay/types"
)

type captureMiddlewareRequestLogSink struct {
	rows []observability.RequestLogRow
	err  error
}

func (s *captureMiddlewareRequestLogSink) InsertRequestLog(_ context.Context, row observability.RequestLogRow) error {
	s.rows = append(s.rows, row)
	return s.err
}

type captureMiddlewareAlertSink struct {
	channel observability.AlertDeliveryChannel
	events  []observability.AlertEvent
	err     error
}

func (s *captureMiddlewareAlertSink) Channel() observability.AlertDeliveryChannel {
	return s.channel
}

func (s *captureMiddlewareAlertSink) Deliver(_ context.Context, event observability.AlertEvent) error {
	s.events = append(s.events, event)
	return s.err
}

func assertUUIDValue(t *testing.T, label string, value any) {
	t.Helper()

	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		t.Fatalf("expected %s to be a UUID string, got %#v", label, value)
	}
	if _, err := uuid.Parse(text); err != nil {
		t.Fatalf("expected %s to be a UUID, got %q: %v", label, text, err)
	}
}

func TestWithCORSAllowsConfiguredOrigin(t *testing.T) {
	handler := withCORS([]string{"http://localhost:5173"})(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusNoContent)
	}))

	request := httptest.NewRequest(stdhttp.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("expected allow origin header, got %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
	if recorder.Header().Get("Vary") != "Origin" {
		t.Fatalf("expected Vary Origin header, got %q", recorder.Header().Get("Vary"))
	}
	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected status %d, got %d", stdhttp.StatusNoContent, recorder.Code)
	}
}

func TestWithCORSHandlesPreflightRequest(t *testing.T) {
	handler := withCORS([]string{"http://localhost:5173"})(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		t.Fatal("preflight request should not reach next handler")
	}))

	request := httptest.NewRequest(stdhttp.MethodOptions, "/api/v1/auth/login", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", stdhttp.MethodPost)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected status %d, got %d", stdhttp.StatusNoContent, recorder.Code)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("expected allow origin header, got %q", recorder.Header().Get("Access-Control-Allow-Origin"))
	}
	if recorder.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("expected allow methods header to be set")
	}
}

func TestRequestIDFromContextReturnsRequestIDGeneratedByMiddleware(t *testing.T) {
	var gotRequestID string
	handler := withRequestID(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		gotRequestID = requestIDFromContext(r.Context())
		w.WriteHeader(stdhttp.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/healthz", nil)

	handler.ServeHTTP(recorder, request)

	if gotRequestID == "" {
		t.Fatal("expected requestIDFromContext to return a request ID")
	}
	if _, err := uuid.Parse(gotRequestID); err != nil {
		t.Fatalf("expected request ID to be a UUID for ClickHouse request_logs, got %q: %v", gotRequestID, err)
	}
	if recorder.Header().Get(requestIDHeader) != gotRequestID {
		t.Fatalf("expected response request id %q, got %q", gotRequestID, recorder.Header().Get(requestIDHeader))
	}
}

func TestWithLoggingWritesStructuredRequestEvent(t *testing.T) {
	var output bytes.Buffer
	restoreLogger := setObservabilityLoggerForTest(observability.NewJSONLogger(&output))
	defer restoreLogger()

	handler := withRequestID(withLogging(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		attachSessionToObservabilityScope(r, auth.Session{
			OrganizationID: "org_123",
			User:           auth.User{ID: "user_123"},
		})
		w.WriteHeader(stdhttp.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})))

	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/agents/agent_123?api_key=sk-secret", strings.NewReader(`{"prompt":"secret"}`))
	request.Header.Set("Authorization", "Bearer sk-secret")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatalf("expected structured JSON log, got error: %v\n%s", err, output.String())
	}
	if record["event"] != "http.request" {
		t.Fatalf("expected http.request event, got %#v", record["event"])
	}
	if record["component"] != "agent" {
		t.Fatalf("expected agent component, got %#v", record["component"])
	}
	if record["request_id"] == "" {
		t.Fatalf("expected request_id in log: %#v", record)
	}
	assertUUIDValue(t, "request_id", record["request_id"])
	assertUUIDValue(t, "trace_id", record["trace_id"])
	assertUUIDValue(t, "span_id", record["span_id"])
	if record["organization_id"] != "org_123" {
		t.Fatalf("expected organization scope, got %#v", record["organization_id"])
	}
	if record["user_id"] != "user_123" {
		t.Fatalf("expected user scope, got %#v", record["user_id"])
	}
	if record["route"] != "/api/v1/app/agents/:id" {
		t.Fatalf("expected normalized route, got %#v", record["route"])
	}
	if record["status"] != float64(stdhttp.StatusCreated) {
		t.Fatalf("expected status 201, got %#v", record["status"])
	}
	if _, ok := record["latency_ms"]; !ok {
		t.Fatalf("expected latency_ms in log: %#v", record)
	}
	forbiddenText := output.String()
	for _, forbidden := range []string{"sk-secret", "secret", "Authorization", "api_key"} {
		if strings.Contains(forbiddenText, forbidden) {
			t.Fatalf("log leaked forbidden value %q: %s", forbidden, forbiddenText)
		}
	}
}

func TestWithLoggingWritesRequestEventToRequestLogSink(t *testing.T) {
	var output bytes.Buffer
	restoreLogger := setObservabilityLoggerForTest(observability.NewJSONLogger(&output))
	defer restoreLogger()
	sink := &captureMiddlewareRequestLogSink{}
	restoreSink := setRequestLogSinkForTest(sink)
	defer restoreSink()

	handler := withRequestID(withLogging(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		attachSessionToObservabilityScope(r, auth.Session{
			OrganizationID: "750e8400-e29b-41d4-a716-446655440000",
			User:           auth.User{ID: "850e8400-e29b-41d4-a716-446655440000"},
		})
		w.WriteHeader(stdhttp.StatusAccepted)
	})))

	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations/conversation_123/messages", strings.NewReader(`{"prompt":"secret"}`))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusAccepted {
		t.Fatalf("expected handler response to survive request log sink, got %d", recorder.Code)
	}
	if len(sink.rows) != 1 {
		t.Fatalf("expected one request log row, got %+v", sink.rows)
	}
	row := sink.rows[0]
	if row.Service != "chat" ||
		row.Endpoint != "/api/v1/app/conversations/:id/messages" ||
		row.Method != stdhttp.MethodPost ||
		row.StatusCode != stdhttp.StatusAccepted ||
		row.OrganizationID != "750e8400-e29b-41d4-a716-446655440000" ||
		row.UserID != "850e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected request log row: %+v", row)
	}
	if row.DurationMS == 0 {
		t.Fatalf("expected non-zero request duration, got %+v", row)
	}
	if row.ID == "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("expected request log id to preserve UUID request id, got %+v", row)
	}
	if row.TraceID == "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("expected request log trace id to be populated, got %+v", row)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(row.Metadata), &metadata); err != nil {
		t.Fatalf("expected JSON metadata, got %v: %s", err, row.Metadata)
	}
	if metadata["feature_type"] != "chat" {
		t.Fatalf("expected chat feature_type in request log metadata, got %+v", metadata)
	}
	assertUUIDValue(t, "metadata span_id", metadata["span_id"])
	if strings.Contains(row.Metadata, "secret") {
		t.Fatalf("request log metadata leaked request body: %s", row.Metadata)
	}
}

func TestWithLoggingClassifiesRequestLogsByFeature(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		service string
		feature string
	}{
		{
			name:    "workflow API",
			path:    "/api/v1/workflows/workflow_123/execute",
			service: "workflow",
			feature: "workflow",
		},
		{
			name:    "agent API",
			path:    "/api/v1/agent/runs/run_123/approve-tool",
			service: "agent",
			feature: "agent",
		},
		{
			name:    "knowledge RAG API",
			path:    "/api/v1/app/knowledge-bases/kb_123/retrieve",
			service: "rag",
			feature: "rag",
		},
		{
			name:    "relay image API",
			path:    "/v1/images/generations",
			service: "relay",
			feature: "image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &captureMiddlewareRequestLogSink{}
			restoreSink := setRequestLogSinkForTest(sink)
			defer restoreSink()

			handler := withRequestID(withLogging(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				w.WriteHeader(stdhttp.StatusNoContent)
			})))

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(stdhttp.MethodPost, tt.path, nil))

			if len(sink.rows) != 1 {
				t.Fatalf("expected one request log row, got %+v", sink.rows)
			}
			row := sink.rows[0]
			if row.Service != tt.service {
				t.Fatalf("service = %q, want %q; row=%+v", row.Service, tt.service, row)
			}
			var metadata map[string]any
			if err := json.Unmarshal([]byte(row.Metadata), &metadata); err != nil {
				t.Fatalf("expected JSON metadata, got %v: %s", err, row.Metadata)
			}
			if metadata["feature_type"] != tt.feature {
				t.Fatalf("feature_type = %#v, want %q; metadata=%+v", metadata["feature_type"], tt.feature, metadata)
			}
		})
	}
}

func TestWithLoggingEnrichesRelayRequestLogFromBillingScope(t *testing.T) {
	sink := &captureMiddlewareRequestLogSink{}
	restoreSink := setRequestLogSinkForTest(sink)
	defer restoreSink()

	handler := withRequestID(withLogging(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		scope, ok := relaytypes.RequestLogScopeFromContext(r.Context())
		if !ok {
			t.Fatal("expected relay request log scope in request context")
		}
		scope.Record(relaytypes.RequestLogMetadata{
			Model:                    "gpt-4o-mini",
			RequestedModel:           "gpt-4o-mini",
			ResolvedModel:            "gpt-4o-mini-2026-07",
			ChannelID:                "channel_primary",
			Provider:                 "openai",
			BillingSessionID:         "bill_relay_scope",
			PreauthorizedAmount:      0.023,
			TokenPreauthorizedAmount: 0.023,
			Cost:                     0.019,
			ChannelCost:              0.017,
			RequestTokens:            120,
			ResponseTokens:           30,
			TotalTokens:              150,
			Status:                   "success",
			PriceCurrency:            "quota",
			PriceSource:              "sql_catalog",
			PriceSnapshot:            map[string]any{"total_cost": 0.019},
		})
		w.WriteHeader(stdhttp.StatusOK)
	})))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodPost, "/v1/chat/completions", nil))

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected OK response, got %d", recorder.Code)
	}
	if len(sink.rows) != 1 {
		t.Fatalf("expected one request log row, got %+v", sink.rows)
	}
	row := sink.rows[0]
	if row.Service != "relay" ||
		row.Endpoint != "/v1/chat/completions" ||
		row.Model != "gpt-4o-mini" ||
		row.RequestTokens != 120 ||
		row.ResponseTokens != 30 ||
		row.CostUSD != 0.019 {
		t.Fatalf("relay request log did not carry usage/cost columns: %+v", row)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(row.Metadata), &metadata); err != nil {
		t.Fatalf("expected JSON metadata, got %v: %s", err, row.Metadata)
	}
	for key, want := range map[string]any{
		"provider":                   "openai",
		"channel_id":                 "channel_primary",
		"billing_session_id":         "bill_relay_scope",
		"requested_model":            "gpt-4o-mini",
		"resolved_model":             "gpt-4o-mini-2026-07",
		"relay_usage_status":         "success",
		"price_currency":             "quota",
		"price_source":               "sql_catalog",
		"preauthorized_amount":       float64(0.023),
		"token_preauthorized_amount": float64(0.023),
		"channel_cost":               float64(0.017),
		"total_tokens":               float64(150),
	} {
		if metadata[key] != want {
			t.Fatalf("metadata[%s] = %#v, want %#v; metadata=%+v", key, metadata[key], want, metadata)
		}
	}
	if _, ok := metadata["price_snapshot"]; !ok {
		t.Fatalf("expected price snapshot metadata, got %+v", metadata)
	}
}

func TestWithLoggingRequestLogSinkFailureDoesNotBreakResponse(t *testing.T) {
	sink := &captureMiddlewareRequestLogSink{err: errors.New("clickhouse unavailable")}
	restoreSink := setRequestLogSinkForTest(sink)
	defer restoreSink()

	handler := withRequestID(withLogging(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusNoContent)
	})))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/healthz", nil))

	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected request log sink error not to break response, got %d", recorder.Code)
	}
	if len(sink.rows) != 1 {
		t.Fatalf("expected sink to be attempted once, got %+v", sink.rows)
	}
}

func TestWithLoggingRoutesRequestLogSinkFailureToAlertAndRecovery(t *testing.T) {
	sink := &captureMiddlewareRequestLogSink{err: errors.New("clickhouse unavailable")}
	restoreSink := setRequestLogSinkForTest(sink)
	defer restoreSink()

	store := observability.NewInMemoryAlertStateStore()
	inApp := &captureMiddlewareAlertSink{channel: observability.AlertDeliveryChannelInApp}
	dispatcher := observability.NewAlertDeliveryDispatcher(observability.AlertDeliveryDispatcherOptions{
		Policy: observability.DeliveryPolicy{
			Routes: map[observability.AlertSeverity][]observability.AlertDeliveryChannel{
				observability.AlertSeverityCritical: {observability.AlertDeliveryChannelInApp},
			},
		},
		Sinks:        []observability.AlertDeliverySink{inApp},
		HistoryStore: store,
	})
	router := observability.NewAlertRouter(observability.AlertRouterOptions{
		StateStore: store,
		NotifySink: dispatcher,
	})
	recovery := observability.NewRecoveryController(observability.RecoveryControllerOptions{
		StateStore: store,
		Policies: []observability.RecoveryPolicy{
			{
				Name:         "record-request-log-sink-failure",
				Severity:     observability.AlertSeverityCritical,
				Component:    observability.ComponentObservability,
				FieldMatches: map[string]string{"failure_kind": "request_log_sink"},
				ActionType:   observability.RecoveryActionFailover,
			},
		},
	})
	restoreAlertRouter := setHTTPAlertRouterForTest(router)
	defer restoreAlertRouter()
	restoreRecovery := setHTTPRecoveryControllerForTest(recovery)
	defer restoreRecovery()

	handler := withRequestID(withLogging(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusNoContent)
	})))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/v1/chat/completions", nil))

	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected request log sink error not to break response, got %d", recorder.Code)
	}
	const alertKey = "observability:request-log-sink"
	state, found, err := store.GetAlertState(context.Background(), alertKey)
	if err != nil {
		t.Fatalf("get request log sink alert state: %v", err)
	}
	if !found || state.Status != observability.AlertStatusOpen || state.Severity != observability.AlertSeverityCritical || state.Component != observability.ComponentObservability {
		t.Fatalf("expected open critical request log alert state, found=%v state=%+v", found, state)
	}
	if len(inApp.events) != 1 {
		t.Fatalf("expected one request log sink alert delivery, got %+v", inApp.events)
	}
	if inApp.events[0].Fields["failure_kind"] != "request_log_sink" || inApp.events[0].Fields["source"] != "http.request_log" {
		t.Fatalf("expected request log sink alert fields, got %+v", inApp.events[0].Fields)
	}
	actions, err := store.ListRecoveryActions(context.Background(), observability.RecoveryActionFilter{AlertKey: alertKey})
	if err != nil {
		t.Fatalf("list request log sink recovery actions: %v", err)
	}
	if len(actions) != 1 || actions[0].PolicyName != "record-request-log-sink-failure" || actions[0].Status != observability.RecoveryActionRecorded {
		t.Fatalf("expected recorded request log sink recovery action, got %+v", actions)
	}
}

func TestWithLoggingRoutesHTTP5xxToAlertDeliveryAndRecovery(t *testing.T) {
	store := observability.NewInMemoryAlertStateStore()
	inApp := &captureMiddlewareAlertSink{channel: observability.AlertDeliveryChannelInApp}
	dispatcher := observability.NewAlertDeliveryDispatcher(observability.AlertDeliveryDispatcherOptions{
		Policy: observability.DeliveryPolicy{
			Routes: map[observability.AlertSeverity][]observability.AlertDeliveryChannel{
				observability.AlertSeverityWarning: {observability.AlertDeliveryChannelInApp},
			},
		},
		Sinks:        []observability.AlertDeliverySink{inApp},
		HistoryStore: store,
	})
	router := observability.NewAlertRouter(observability.AlertRouterOptions{
		StateStore: store,
		NotifySink: dispatcher,
	})
	recovery := observability.NewRecoveryController(observability.RecoveryControllerOptions{
		StateStore: store,
		Policies: []observability.RecoveryPolicy{
			{
				Name:       "record-http-5xx",
				Severity:   observability.AlertSeverityWarning,
				Component:  observability.ComponentHTTP,
				ActionType: observability.RecoveryActionRestart,
			},
		},
	})
	restoreAlertRouter := setHTTPAlertRouterForTest(router)
	defer restoreAlertRouter()
	restoreRecovery := setHTTPRecoveryControllerForTest(recovery)
	defer restoreRecovery()

	handler := withRequestID(withLogging(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream failed"))
	})))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/relay/chat/completions", strings.NewReader(`{"model":"gpt-5"}`))

	handler.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusInternalServerError {
		t.Fatalf("expected original 500 response to survive alert handling, got %d", recorder.Code)
	}
	const alertKey = "http:/api/v1/relay/chat/completions:500"
	state, found, err := store.GetAlertState(context.Background(), alertKey)
	if err != nil {
		t.Fatalf("get alert state: %v", err)
	}
	if !found || state.Status != observability.AlertStatusOpen || state.Severity != observability.AlertSeverityWarning || state.Component != observability.ComponentHTTP {
		t.Fatalf("expected open HTTP warning alert state, found=%v state=%+v", found, state)
	}
	if len(inApp.events) != 1 {
		t.Fatalf("expected in-app alert sink to receive one event, got %+v", inApp.events)
	}
	if inApp.events[0].Key != alertKey || inApp.events[0].Title != "HTTP 500 on /api/v1/relay/chat/completions" {
		t.Fatalf("unexpected alert event delivered: %+v", inApp.events[0])
	}
	attempts, err := store.ListDeliveryAttempts(context.Background(), observability.AlertDeliveryHistoryFilter{AlertKey: alertKey})
	if err != nil {
		t.Fatalf("list delivery attempts: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Channel != observability.AlertDeliveryChannelInApp || !attempts[0].Delivered {
		t.Fatalf("expected successful in-app delivery attempt, got %+v", attempts)
	}
	actions, err := store.ListRecoveryActions(context.Background(), observability.RecoveryActionFilter{AlertKey: alertKey})
	if err != nil {
		t.Fatalf("list recovery actions: %v", err)
	}
	if len(actions) != 1 || actions[0].PolicyName != "record-http-5xx" || actions[0].Status != observability.RecoveryActionRecorded {
		t.Fatalf("expected recorded recovery action, got %+v", actions)
	}
}

func TestWithLoggingRoutesLatencySLOToAlertDeliveryAndRecovery(t *testing.T) {
	restoreThreshold := setHTTPAlertLatencySLOThresholdForTest(time.Nanosecond)
	defer restoreThreshold()

	store := observability.NewInMemoryAlertStateStore()
	inApp := &captureMiddlewareAlertSink{channel: observability.AlertDeliveryChannelInApp}
	dispatcher := observability.NewAlertDeliveryDispatcher(observability.AlertDeliveryDispatcherOptions{
		Policy: observability.DeliveryPolicy{
			Routes: map[observability.AlertSeverity][]observability.AlertDeliveryChannel{
				observability.AlertSeverityWarning: {observability.AlertDeliveryChannelInApp},
			},
		},
		Sinks:        []observability.AlertDeliverySink{inApp},
		HistoryStore: store,
	})
	router := observability.NewAlertRouter(observability.AlertRouterOptions{
		StateStore: store,
		NotifySink: dispatcher,
	})
	recovery := observability.NewRecoveryController(observability.RecoveryControllerOptions{
		StateStore: store,
		Policies: []observability.RecoveryPolicy{
			{
				Name:         "record-http-latency-slo",
				Severity:     observability.AlertSeverityWarning,
				Component:    observability.ComponentHTTP,
				FieldMatches: map[string]string{"slo": "latency"},
				ActionType:   observability.RecoveryActionScaleOut,
			},
		},
	})
	restoreAlertRouter := setHTTPAlertRouterForTest(router)
	defer restoreAlertRouter()
	restoreRecovery := setHTTPRecoveryControllerForTest(recovery)
	defer restoreRecovery()

	handler := withRequestID(withLogging(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		time.Sleep(2 * time.Millisecond)
		w.WriteHeader(stdhttp.StatusNoContent)
	})))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations/conversation_123/messages", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusNoContent {
		t.Fatalf("expected original response to survive SLO alert handling, got %d", recorder.Code)
	}
	const alertKey = "http-slo:/api/v1/app/conversations/:id/messages:latency"
	state, found, err := store.GetAlertState(context.Background(), alertKey)
	if err != nil {
		t.Fatalf("get latency SLO alert state: %v", err)
	}
	if !found || state.Status != observability.AlertStatusOpen || state.Severity != observability.AlertSeverityWarning || state.Component != observability.ComponentHTTP {
		t.Fatalf("expected open HTTP latency SLO warning state, found=%v state=%+v", found, state)
	}
	if len(inApp.events) != 1 {
		t.Fatalf("expected one latency SLO alert delivery, got %+v", inApp.events)
	}
	fields := inApp.events[0].Fields
	if fields["slo"] != "latency" || fields["source"] != "http.slo" || fields["request_id"] == "" {
		t.Fatalf("expected latency SLO evidence fields, got %+v", fields)
	}
	if fields["threshold_ms"] == nil || fields["latency_ms"] == nil {
		t.Fatalf("expected threshold_ms and latency_ms evidence fields, got %+v", fields)
	}
	actions, err := store.ListRecoveryActions(context.Background(), observability.RecoveryActionFilter{AlertKey: alertKey})
	if err != nil {
		t.Fatalf("list latency SLO recovery actions: %v", err)
	}
	if len(actions) != 1 || actions[0].PolicyName != "record-http-latency-slo" || actions[0].Status != observability.RecoveryActionRecorded {
		t.Fatalf("expected recorded latency SLO recovery action, got %+v", actions)
	}
}

func TestWithRecoverRoutesPanicToCriticalAlertAndRecovery(t *testing.T) {
	store := observability.NewInMemoryAlertStateStore()
	inApp := &captureMiddlewareAlertSink{channel: observability.AlertDeliveryChannelInApp}
	dispatcher := observability.NewAlertDeliveryDispatcher(observability.AlertDeliveryDispatcherOptions{
		Policy: observability.DeliveryPolicy{
			Routes: map[observability.AlertSeverity][]observability.AlertDeliveryChannel{
				observability.AlertSeverityCritical: {observability.AlertDeliveryChannelInApp},
			},
		},
		Sinks:        []observability.AlertDeliverySink{inApp},
		HistoryStore: store,
	})
	router := observability.NewAlertRouter(observability.AlertRouterOptions{
		StateStore: store,
		NotifySink: dispatcher,
	})
	recovery := observability.NewRecoveryController(observability.RecoveryControllerOptions{
		StateStore: store,
		Policies: []observability.RecoveryPolicy{
			{
				Name:         "record-http-panic",
				Severity:     observability.AlertSeverityCritical,
				Component:    observability.ComponentHTTP,
				FieldMatches: map[string]string{"failure_kind": "panic"},
				ActionType:   observability.RecoveryActionRestart,
			},
		},
	})
	restoreAlertRouter := setHTTPAlertRouterForTest(router)
	defer restoreAlertRouter()
	restoreRecovery := setHTTPRecoveryControllerForTest(recovery)
	defer restoreRecovery()

	handler := applyMiddleware(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		panic("simulated crash")
	}), withRecover, withRequestID, withLogging)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/admin/relay/routes", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusInternalServerError {
		t.Fatalf("expected panic to become 500, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "panic recovered") {
		t.Fatalf("expected panic recovery response body, got %s", recorder.Body.String())
	}
	const alertKey = "http:/api/v1/admin/relay/routes:panic"
	state, found, err := store.GetAlertState(context.Background(), alertKey)
	if err != nil {
		t.Fatalf("get panic alert state: %v", err)
	}
	if !found || state.Status != observability.AlertStatusOpen || state.Severity != observability.AlertSeverityCritical || state.Component != observability.ComponentHTTP {
		t.Fatalf("expected open critical HTTP panic alert state, found=%v state=%+v", found, state)
	}
	if len(inApp.events) != 1 {
		t.Fatalf("expected one critical in-app alert delivery, got %+v", inApp.events)
	}
	if inApp.events[0].Fields["failure_kind"] != "panic" || inApp.events[0].Fields["source"] != "http.recover" {
		t.Fatalf("expected panic alert fields, got %+v", inApp.events[0].Fields)
	}
	if requestID, ok := inApp.events[0].Fields["request_id"].(string); !ok || requestID == "" {
		t.Fatalf("expected panic alert to preserve request id evidence, got %+v", inApp.events[0].Fields)
	}
	actions, err := store.ListRecoveryActions(context.Background(), observability.RecoveryActionFilter{AlertKey: alertKey})
	if err != nil {
		t.Fatalf("list panic recovery actions: %v", err)
	}
	if len(actions) != 1 ||
		actions[0].PolicyName != "record-http-panic" ||
		actions[0].Type != observability.RecoveryActionRestart ||
		actions[0].Status != observability.RecoveryActionRecorded ||
		actions[0].Attempt != 1 {
		t.Fatalf("expected recorded panic restart recovery action, got %+v", actions)
	}
}
