package balancer

import (
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestWeightedRoundRobin_Select(t *testing.T) {
	channels := []*types.WeightedChannel{
		{ChannelID: "a", StaticWeight: 3, Healthy: true, Enabled: true},
		{ChannelID: "b", StaticWeight: 1, Healthy: true, Enabled: true},
	}

	wrr := NewWeightedRoundRobin(channels)

	counts := map[string]int{"a": 0, "b": 0}
	for i := 0; i < 400; i++ {
		ch := wrr.Select()
		if ch != nil {
			counts[ch.ChannelID]++
		}
	}

	// a should appear ~3x more than b (3:1 ratio)
	if counts["a"] < 200 || counts["a"] > 350 {
		t.Fatalf("expected ~300 selections for a, got %d", counts["a"])
	}
	if counts["b"] < 50 || counts["b"] > 150 {
		t.Fatalf("expected ~100 selections for b, got %d", counts["b"])
	}
}

func TestWeightedRoundRobin_SkipsUnhealthy(t *testing.T) {
	channels := []*types.WeightedChannel{
		{ChannelID: "a", StaticWeight: 1, Healthy: false, Enabled: true},
		{ChannelID: "b", StaticWeight: 1, Healthy: true, Enabled: true},
	}

	wrr := NewWeightedRoundRobin(channels)
	for i := 0; i < 10; i++ {
		ch := wrr.Select()
		if ch == nil {
			t.Fatal("should return channel b")
		}
		if ch.ChannelID != "b" {
			t.Fatalf("expected b, got %s", ch.ChannelID)
		}
	}
}

func TestWeightedRoundRobin_UpdateChannels(t *testing.T) {
	channels := []*types.WeightedChannel{
		{ChannelID: "a", StaticWeight: 1, Healthy: true, Enabled: true},
	}

	wrr := NewWeightedRoundRobin(channels)
	if wrr.TotalWeight() != 1 {
		t.Fatalf("expected weight 1, got %d", wrr.TotalWeight())
	}

	// Update
	newChannels := []*types.WeightedChannel{
		{ChannelID: "a", StaticWeight: 1, Healthy: true, Enabled: true},
		{ChannelID: "b", StaticWeight: 2, Healthy: true, Enabled: true},
	}
	wrr.UpdateChannels(newChannels)
	if wrr.TotalWeight() != 3 {
		t.Fatalf("expected weight 3, got %d", wrr.TotalWeight())
	}
}

func TestWeightedRoundRobin_Empty(t *testing.T) {
	wrr := NewWeightedRoundRobin(nil)
	if wrr.Select() != nil {
		t.Fatal("should return nil for empty channels")
	}
}

func TestWeightedRoundRobin_ZeroWeight(t *testing.T) {
	channels := []*types.WeightedChannel{
		{ChannelID: "a", StaticWeight: 0, Healthy: true, Enabled: true},
		{ChannelID: "b", StaticWeight: 0, Healthy: true, Enabled: true},
	}

	wrr := NewWeightedRoundRobin(channels)
	// Should still select (uniform random)
	ch := wrr.Select()
	if ch == nil {
		t.Fatal("should return a channel even with zero weight")
	}
}
