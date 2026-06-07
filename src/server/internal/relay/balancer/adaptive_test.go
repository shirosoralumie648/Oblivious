package balancer

import (
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestAdaptiveBalancer_Select(t *testing.T) {
	channels := []*types.WeightedChannel{
		{ChannelID: "a", StaticWeight: 3, Healthy: true, Enabled: true},
		{ChannelID: "b", StaticWeight: 1, Healthy: true, Enabled: true},
	}

	ab := NewAdaptiveBalancer(channels)
	ch := ab.Select()
	if ch == nil {
		t.Fatal("should return a channel")
	}
}

func TestAdaptiveBalancer_RecordResult_UpdatesMetrics(t *testing.T) {
	channels := []*types.WeightedChannel{
		{ChannelID: "a", StaticWeight: 1, Healthy: true, Enabled: true},
	}

	ab := NewAdaptiveBalancer(channels)

	// Record some results
	ab.RecordResult("a", 50.0, true)
	ab.RecordResult("a", 100.0, true)
	ab.RecordResult("a", 200.0, false) // error

	metrics := ab.GetChannelMetrics()
	m := metrics["a"]
	if m.ErrorRate <= 0 {
		t.Fatal("error rate should be > 0 after a failure")
	}
	if m.AvgLatencyMs <= 0 {
		t.Fatal("avg latency should be > 0")
	}
}

func TestAdaptiveBalancer_HighErrorRateReducesWeight(t *testing.T) {
	channels := []*types.WeightedChannel{
		{ChannelID: "good", StaticWeight: 1, Healthy: true, Enabled: true},
		{ChannelID: "bad", StaticWeight: 1, Healthy: true, Enabled: true},
	}

	ab := NewAdaptiveBalancer(channels)

	// Make "bad" channel have high error rate
	for i := 0; i < 20; i++ {
		ab.RecordResult("bad", 500.0, false)
	}
	// "good" channel succeeds
	for i := 0; i < 20; i++ {
		ab.RecordResult("good", 50.0, true)
	}

	metrics := ab.GetChannelMetrics()
	goodWeight := metrics["good"].DynamicWeight
	badWeight := metrics["bad"].DynamicWeight

	if badWeight >= goodWeight {
		t.Fatalf("bad channel weight (%.2f) should be less than good (%.2f)", badWeight, goodWeight)
	}
}

func TestAdaptiveBalancer_SetChannelHealth(t *testing.T) {
	channels := []*types.WeightedChannel{
		{ChannelID: "a", StaticWeight: 1, Healthy: true, Enabled: true},
	}

	ab := NewAdaptiveBalancer(channels)
	ab.SetChannelHealth("a", false)

	ch := ab.Select()
	if ch != nil {
		t.Fatal("should return nil when all channels unhealthy")
	}
}

func TestAdaptiveBalancer_Reset(t *testing.T) {
	channels := []*types.WeightedChannel{
		{ChannelID: "a", StaticWeight: 1, Healthy: true, Enabled: true},
	}

	ab := NewAdaptiveBalancer(channels)
	for i := 0; i < 10; i++ {
		ab.RecordResult("a", 500.0, false)
	}

	metrics := ab.GetChannelMetrics()
	if metrics["a"].ErrorRate <= 0 {
		t.Fatal("error rate should be > 0")
	}

	ab.Reset("a")
	metrics = ab.GetChannelMetrics()
	if metrics["a"].ErrorRate != 0 {
		t.Fatal("error rate should be 0 after reset")
	}
	if metrics["a"].HealthScore != 100.0 {
		t.Fatal("health score should be 100 after reset")
	}
}

func TestComputeHealthScore(t *testing.T) {
	// Perfect health
	score := computeHealthScore(50.0, 0.0)
	if score != 100.0 {
		t.Fatalf("expected 100, got %.1f", score)
	}

	// High latency reduces score
	score = computeHealthScore(1000.0, 0.0)
	if score >= 100.0 {
		t.Fatalf("high latency should reduce score, got %.1f", score)
	}

	// High error rate reduces score
	score = computeHealthScore(50.0, 0.5)
	if score > 50.0 {
		t.Fatalf("50%% error should reduce score to 50 or below, got %.1f", score)
	}
}
