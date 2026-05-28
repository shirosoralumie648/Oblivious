package http

import (
	"bytes"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/observability"
)

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
	if record["component"] != "http" {
		t.Fatalf("expected http component, got %#v", record["component"])
	}
	if record["request_id"] == "" {
		t.Fatalf("expected request_id in log: %#v", record)
	}
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
