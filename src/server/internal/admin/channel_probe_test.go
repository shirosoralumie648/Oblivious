package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestRelayChannelProbeUsesAPIKeyAndReturnsModels(t *testing.T) {
	var gotPaths []string
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"text-embedding-3-small"}]}`))
	}))
	t.Cleanup(upstream.Close)

	result := testRelayChannel(context.Background(), &types.Channel{
		ID:       "ch_probe",
		Name:     "Probe",
		Provider: "openai",
		BaseURL:  upstream.URL,
		APIKey:   "sk-probe",
	})

	if !result.Success {
		t.Fatalf("expected probe success, got %+v", result)
	}
	if !containsString(gotPaths, "/v1/models") {
		t.Fatalf("paths = %+v, want /v1/models", gotPaths)
	}
	if gotAuth != "Bearer sk-probe" {
		t.Fatalf("authorization = %q, want real channel API key", gotAuth)
	}
	if len(result.Models) != 2 || result.Models[0] != "gpt-4o-mini" || result.Models[1] != "text-embedding-3-small" {
		t.Fatalf("models not parsed: %+v", result.Models)
	}
}

func TestRelayChannelProbeReturnsBalanceAndHealthPayload(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
		case "/v1/dashboard/billing/credit_grants":
			_, _ = w.Write([]byte(`{"total_available":12.5}`))
		default:
			t.Fatalf("unexpected probe path: %s", r.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	result := testRelayChannel(context.Background(), &types.Channel{
		ID:       "ch_probe_balance",
		Name:     "Probe Balance",
		Provider: "openai",
		BaseURL:  upstream.URL,
		APIKey:   "sk-balance",
	})

	if !result.Success {
		t.Fatalf("expected probe success, got %+v", result)
	}

	payload := marshalTestResult(t, result)
	balance, ok := payload["balance"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected balance payload, got %+v", payload)
	}
	if amount, ok := balance["amount"].(float64); !ok || amount != 12.5 {
		t.Fatalf("balance amount = %+v, want 12.5", balance["amount"])
	}
	if source, ok := balance["source"].(string); !ok || source == "" {
		t.Fatalf("balance source missing: %+v", balance)
	}
	health, ok := payload["health"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected health payload, got %+v", payload)
	}
	if status, ok := health["status"].(string); !ok || status != "online" {
		t.Fatalf("health status = %+v, want online", health["status"])
	}
}

func TestRelayChannelProbeRedactsAPIKeyFromProviderErrors(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad credential sk-secret-leak"}}`))
	}))
	t.Cleanup(upstream.Close)

	result := testRelayChannel(context.Background(), &types.Channel{
		ID:       "ch_probe_redact",
		Name:     "Probe Redact",
		Provider: "openai",
		BaseURL:  upstream.URL,
		APIKey:   "sk-secret-leak",
	})

	if result.Success {
		t.Fatalf("expected probe failure, got %+v", result)
	}
	if strings.Contains(result.Error, "sk-secret-leak") {
		t.Fatalf("probe error leaked API key: %q", result.Error)
	}
	if !strings.Contains(result.Error, "[redacted]") {
		t.Fatalf("probe error should show redaction marker, got %q", result.Error)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func marshalTestResult(t *testing.T, result *ChannelTestResult) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return payload
}
