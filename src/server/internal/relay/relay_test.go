package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRelayRegistersCommercialChatRoute(t *testing.T) {
	relayInstance, err := NewRelay(&Config{Production: true})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if recorder.Code == http.StatusNotFound {
		t.Fatalf("commercial chat route must be registered, got 404 with body %s", recorder.Body.String())
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("production chat route should reject missing trusted identity with 401, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}
