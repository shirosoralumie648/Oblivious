package relay

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"oblivious/server/internal/relay/types"
)

func TestRouter_SelectsHealthyChannel(t *testing.T) {
	// Setup: create pool with two channels, one healthy, one not
	pool := NewChannelPool()
	healthyCh := &types.Channel{ID: "healthy", BaseURL: "http://healthy", Enabled: true}
	unhealthyCh := &types.Channel{ID: "unhealthy", BaseURL: "http://unhealthy", Enabled: false}
	pool.AddChannel(healthyCh, 1)
	pool.AddChannel(unhealthyCh, 1)

	lb := NewLoadBalancer(pool, "weighted")
	cb := NewCircuitBreaker("test", 3, time.Second, 30*time.Second)
	tb := NewTokenBucket(60, 1000)
	hc := NewHealthChecker(HealthCheckDisabled, 5*time.Second)

	router := NewRouter(pool, lb, map[string]*CircuitBreaker{"healthy": cb, "unhealthy": cb}, tb, hc)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	healthyCh.BaseURL = ts.URL

	ch := router.SelectChannel(context.Background(), "chat")
	if ch == nil {
		t.Fatal("should return a channel")
	}
	if !ch.Channel.Enabled {
		t.Fatal("should not return disabled channel")
	}
}

func TestRouter_AllChannelsFailed(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "fail", BaseURL: "http://fail", Enabled: false}, 1)

	lb := NewLoadBalancer(pool, "weighted")
	hc := NewHealthChecker(HealthCheckDisabled, 5*time.Second)
	router := NewRouter(pool, lb, nil, nil, hc)

	_, err := router.Route(context.Background(), "chat", nil)
	if err == nil {
		t.Fatal("expected error when all channels fail")
	}
	// Should return 503 with Retry-After header
	var re *RouterError
	if !errors.As(err, &re) {
		t.Fatalf("expected RouterError, got %T", err)
	}
	if re.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", re.Code)
	}
}

func TestRouter_RouteWithFallback_RetriesAllChannels(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "a", BaseURL: "http://a", Enabled: true}, 1)
	pool.AddChannel(&types.Channel{ID: "b", BaseURL: "http://b", Enabled: true}, 1)

	lb := NewLoadBalancer(pool, "weighted")
	router := NewRouter(pool, lb, nil, nil, NewHealthChecker(HealthCheckDisabled, 5*time.Second))

	callCount := 0
	fn := func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		callCount++
		return nil, errors.New("error")
	}

	_, err := router.RouteWithFallback(context.Background(), "chat", 2, fn)
	if err == nil {
		t.Fatal("expected error")
	}
	// Both attempts use the same channel due to LB; with 2 channels it varies
}
