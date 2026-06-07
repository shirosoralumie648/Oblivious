package relay

import (
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestLoadBalancer_Weighted(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "a", BaseURL: "http://a", Enabled: true}, 3)
	pool.AddChannel(&types.Channel{ID: "b", BaseURL: "http://b", Enabled: true}, 1)

	lb := NewLoadBalancer(pool, "weighted")

	got := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		ch := lb.Select("chat")
		if ch == nil {
			t.Fatal("channel should not be nil")
		}
		got = append(got, ch.Channel.ID)
	}

	want := []string{"a", "a", "a", "b", "a", "a", "a", "b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("weighted round-robin sequence = %v, want %v", got, want)
		}
	}
}

func TestLoadBalancer_Priority(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "a", BaseURL: "http://a", Enabled: true, Priority: 1}, 1)
	pool.AddChannel(&types.Channel{ID: "b", BaseURL: "http://b", Enabled: true, Priority: 2}, 1)

	lb := NewLoadBalancer(pool, "priority")

	// Should always pick the lowest priority number (highest priority)
	for i := 0; i < 10; i++ {
		ch := lb.Select("chat")
		if ch == nil {
			t.Fatal("channel should not be nil")
		}
		if ch.Channel.ID != "a" {
			t.Fatalf("expected priority channel a, got %s", ch.Channel.ID)
		}
	}
}

func TestLoadBalancer_CostAware(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "cheap", BaseURL: "http://cheap", Enabled: true}, 1)
	pool.AddChannel(&types.Channel{ID: "expensive", BaseURL: "http://expensive", Enabled: true}, 1)

	// Manually set RouteChannel cost for cost-aware selection
	routes := pool.GetChannelsByModel("")
	if len(routes) >= 2 {
		for _, rc := range routes {
			if rc.Channel.ID == "cheap" {
				rc.EstimatedCostPer1K = 0.5
			} else if rc.Channel.ID == "expensive" {
				rc.EstimatedCostPer1K = 5.0
			}
		}
	}

	lb := NewLoadBalancer(pool, "cost_aware")

	counts := map[string]int{"cheap": 0, "expensive": 0}
	for i := 0; i < 20; i++ {
		ch := lb.Select("chat")
		if ch != nil {
			counts[ch.Channel.ID]++
		}
	}

	// cheap should be selected more often
	if counts["cheap"] <= counts["expensive"] {
		t.Fatalf("expected cheap selected more, got cheap=%d expensive=%d", counts["cheap"], counts["expensive"])
	}
}

func TestLoadBalancer_AdaptiveUsesRuntimeHealthMetrics(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "healthy", BaseURL: "http://healthy", Enabled: true}, 1)
	pool.AddChannel(&types.Channel{ID: "degraded", BaseURL: "http://degraded", Enabled: true}, 100)

	healthyStats, ok := pool.GetStats("healthy")
	if !ok {
		t.Fatal("healthy channel stats should exist")
	}
	healthyStats.SuccessCount = 100
	healthyStats.FailureCount = 0
	healthyStats.LatencySumUs = 50_000 * 100
	healthyStats.LatencyCount = 100

	degradedStats, ok := pool.GetStats("degraded")
	if !ok {
		t.Fatal("degraded channel stats should exist")
	}
	degradedStats.SuccessCount = 1
	degradedStats.FailureCount = 99
	degradedStats.LatencySumUs = 2_000_000 * 100
	degradedStats.LatencyCount = 100

	lb := NewLoadBalancer(pool, "adaptive")

	counts := map[string]int{"healthy": 0, "degraded": 0}
	for i := 0; i < 80; i++ {
		ch := lb.Select("chat")
		if ch == nil {
			t.Fatal("channel should not be nil")
		}
		counts[ch.Channel.ID]++
	}

	if counts["healthy"] <= counts["degraded"] {
		t.Fatalf("adaptive strategy should prefer healthy runtime metrics, got healthy=%d degraded=%d", counts["healthy"], counts["degraded"])
	}
}

func TestLoadBalancer_AllHealthy(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "a", BaseURL: "http://a", Enabled: true}, 1)
	pool.AddChannel(&types.Channel{ID: "b", BaseURL: "http://b", Enabled: true}, 1)

	lb := NewLoadBalancer(pool, "weighted")
	ch := lb.Select("chat")
	if ch == nil {
		t.Fatal("should return a channel")
	}
}

func TestLoadBalancer_SkipsUnhealthy(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "a", BaseURL: "http://a", Enabled: false}, 1)
	pool.AddChannel(&types.Channel{ID: "b", BaseURL: "http://b", Enabled: true}, 1)

	lb := NewLoadBalancer(pool, "weighted")
	ch := lb.Select("chat")
	if ch == nil {
		t.Fatal("should return healthy channel b")
	}
	if ch.Channel.ID != "b" {
		t.Fatalf("expected b, got %s", ch.Channel.ID)
	}
}

func TestLoadBalancer_SkipsForbiddenChannel(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "forbidden", BaseURL: "http://forbidden", Enabled: true}, 100)
	pool.AddChannel(&types.Channel{ID: "backup", BaseURL: "http://backup", Enabled: true}, 1)
	stats, ok := pool.GetStats("forbidden")
	if !ok {
		t.Fatal("forbidden channel stats should exist")
	}
	stats.Forbidden = true

	lb := NewLoadBalancer(pool, "weighted")
	for i := 0; i < 5; i++ {
		ch := lb.Select("chat")
		if ch == nil {
			t.Fatal("should return backup channel")
		}
		if ch.Channel.ID != "backup" {
			t.Fatalf("expected backup when forbidden channel is marked forbidden, got %s", ch.Channel.ID)
		}
	}
}

type sequenceRandom struct {
	ints     []int
	floats   []float64
	intPos   int
	floatPos int
}

func (r *sequenceRandom) Intn(n int) int {
	if len(r.ints) == 0 {
		return 0
	}
	value := r.ints[r.intPos%len(r.ints)]
	r.intPos++
	return value % n
}

func (r *sequenceRandom) Float64() float64 {
	if len(r.floats) == 0 {
		return 0
	}
	value := r.floats[r.floatPos%len(r.floats)]
	r.floatPos++
	return value
}
