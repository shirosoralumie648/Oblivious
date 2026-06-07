package balancer

import (
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestWeightedRoundRobin_SelectsDeterministicWeightedSequence(t *testing.T) {
	channels := []*types.WeightedChannel{
		{ChannelID: "a", StaticWeight: 3, Healthy: true, Enabled: true},
		{ChannelID: "b", StaticWeight: 1, Healthy: true, Enabled: true},
	}

	wrr := NewWeightedRoundRobin(channels)

	got := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		ch := wrr.Select()
		if ch != nil {
			got = append(got, ch.ChannelID)
		}
	}

	want := []string{"a", "a", "a", "b", "a", "a", "a", "b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("weighted round-robin sequence = %v, want %v", got, want)
		}
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
