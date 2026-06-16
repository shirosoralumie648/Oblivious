package relay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthChecker_ModelsAPI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	hc := NewHealthChecker(HealthCheckModelsAPI, 5*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	healthy, lat := hc.Check(ctx, ts.URL+"/v1/models", "fake-key")
	if !healthy {
		t.Fatal("models_api probe should succeed")
	}
	if lat < 0 {
		t.Fatalf("latency should be non-negative, got %dms", lat.Milliseconds())
	}
}

func TestHealthChecker_Disabled(t *testing.T) {
	hc := NewHealthChecker(HealthCheckDisabled, 5*time.Second)
	ctx := context.Background()
	healthy, _ := hc.Check(ctx, "http://fake", "fake-key")
	if !healthy {
		t.Fatal("disabled probe should always return healthy")
	}
}

func TestHealthChecker_RealtimeProbePostsChatCompletion(t *testing.T) {
	var called bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer fake-key" {
			t.Errorf("unexpected authorization header: %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("unexpected content-type header: %q", got)
		}

		var body struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
			Messages  []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode probe request: %v", err)
		}
		if body.Model == "" {
			t.Fatal("probe model should be present")
		}
		if body.MaxTokens <= 0 {
			t.Fatalf("probe max_tokens should be positive, got %d", body.MaxTokens)
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" || body.Messages[0].Content == "" {
			t.Fatalf("unexpected probe messages: %+v", body.Messages)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	hc := NewHealthChecker(HealthCheckRealtime, 5*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	healthy, lat := hc.Check(ctx, ts.URL, "fake-key")
	if !healthy {
		t.Fatal("realtime probe should succeed")
	}
	if !called {
		t.Fatal("realtime probe server was not called")
	}
	if lat < 0 {
		t.Fatalf("latency should be non-negative, got %dms", lat.Milliseconds())
	}
}

func TestHealthChecker_RealtimeProbeFailsOnServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	hc := NewHealthChecker(HealthCheckRealtime, 5*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	healthy, _ := hc.Check(ctx, ts.URL, "fake-key")
	if healthy {
		t.Fatal("realtime probe should fail on server error")
	}
}

func TestHealthChecker_UnknownStrategyFailsClosed(t *testing.T) {
	hc := NewHealthChecker(HealthCheckStrategy("something_else"), 5*time.Second)
	ctx := context.Background()

	healthy, _ := hc.Check(ctx, "http://fake", "fake-key")
	if healthy {
		t.Fatal("unknown health strategy should fail closed")
	}
}

func TestHealthChecker_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(10 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()

	hc := NewHealthChecker(HealthCheckModelsAPI, 50*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	healthy, _ := hc.Check(ctx, ts.URL+"/v1/models", "fake-key")
	if healthy {
		t.Fatal("probe should fail on timeout")
	}
}

func TestHealthChecker_ProbeErrorCounting(t *testing.T) {
	hc := NewHealthChecker(HealthCheckModelsAPI, 5*time.Second)
	cb := NewCircuitBreaker("test", 3, time.Second, 30*time.Second)

	// Record 3 failures
	for i := 0; i < 3; i++ {
		hc.RecordProbeResult(cb, false)
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected Open after 3 probe failures, got %s", cb.State())
	}
}
