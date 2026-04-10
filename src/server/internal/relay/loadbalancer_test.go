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

	counts := map[string]int{"a": 0, "b": 0}
	for i := 0; i < 40; i++ {
		ch := lb.Select("chat")
		if ch != nil {
			counts[ch.Channel.ID]++
		}
	}

	// a should appear ~3x more than b
	if counts["a"] < 20 || counts["a"] > 35 {
		t.Fatalf("expected ~30 selections for a, got %d", counts["a"])
	}
	if counts["b"] < 5 || counts["b"] > 15 {
		t.Fatalf("expected ~10 selections for b, got %d", counts["b"])
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
