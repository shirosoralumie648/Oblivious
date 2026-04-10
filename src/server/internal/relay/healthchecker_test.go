package relay

import (
	"context"
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

func TestHealthChecker_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
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
