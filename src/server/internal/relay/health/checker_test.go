package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"oblivious/server/internal/relay/types"
)

func TestChecker_RegisterAndGetHealthScore(t *testing.T) {
	c := NewChecker(time.Hour, 3, 2)

	c.Register(&types.Channel{ID: "ch1", BaseURL: "http://test", Enabled: true})

	hs := c.GetHealthScore("ch1")
	if hs == nil {
		t.Fatal("should find health score for registered channel")
	}
	if hs.Score != 100.0 {
		t.Fatalf("initial score should be 100, got %.1f", hs.Score)
	}
}

func TestChecker_RecordRequestResult(t *testing.T) {
	c := NewChecker(time.Hour, 3, 2)

	c.Register(&types.Channel{ID: "ch1", BaseURL: "http://test", Enabled: true})

	// Record success
	c.RecordRequestResult("ch1", 50.0, true)
	hs := c.GetHealthScore("ch1")
	if hs.Score <= 0 {
		t.Fatal("score should be positive after success")
	}
	if hs.AvgLatencyMs != 50.0 {
		t.Fatalf("avg latency should be 50, got %.1f", hs.AvgLatencyMs)
	}
}

func TestChecker_AutoRemove(t *testing.T) {
	removed := make(chan string, 1)
	c := NewChecker(time.Hour, 3, 2)
	c.SetOnRemove(func(channelID string) {
		removed <- channelID
	})

	c.Register(&types.Channel{ID: "ch1", BaseURL: "http://test", Enabled: true})

	// Record 3 consecutive failures
	for i := 0; i < 3; i++ {
		c.RecordRequestResult("ch1", 500.0, false)
	}

	// Check auto-removed
	select {
	case ch := <-removed:
		if ch != "ch1" {
			t.Fatalf("expected ch1 removed, got %s", ch)
		}
	case <-time.After(time.Second):
		t.Fatal("onRemove callback not called")
	}

	if !c.IsRemoved("ch1") {
		t.Fatal("ch1 should be marked as removed")
	}
}

func TestChecker_AutoRecover(t *testing.T) {
	recovered := make(chan string, 1)
	c := NewChecker(time.Hour, 3, 2)
	c.SetOnRecover(func(channelID string) {
		recovered <- channelID
	})

	c.Register(&types.Channel{ID: "ch1", BaseURL: "http://test", Enabled: true})

	// Remove first
	for i := 0; i < 3; i++ {
		c.RecordRequestResult("ch1", 500.0, false)
	}
	if !c.IsRemoved("ch1") {
		t.Fatal("ch1 should be removed")
	}

	// Recover
	for i := 0; i < 2; i++ {
		c.RecordRequestResult("ch1", 50.0, true)
	}

	select {
	case ch := <-recovered:
		if ch != "ch1" {
			t.Fatalf("expected ch1 recovered, got %s", ch)
		}
	case <-time.After(time.Second):
		t.Fatal("onRecover callback not called")
	}

	if c.IsRemoved("ch1") {
		t.Fatal("ch1 should NOT be removed after recovery")
	}
}

func TestChecker_ForceRecover(t *testing.T) {
	c := NewChecker(time.Hour, 3, 2)
	c.Register(&types.Channel{ID: "ch1", BaseURL: "http://test", Enabled: true})

	// Remove
	for i := 0; i < 3; i++ {
		c.RecordRequestResult("ch1", 500.0, false)
	}

	c.ForceRecover("ch1")
	if c.IsRemoved("ch1") {
		t.Fatal("ch1 should be recovered after ForceRecover")
	}

	hs := c.GetHealthScore("ch1")
	if hs.Score != 50.0 {
		t.Fatalf("score should be 50 after forced recovery, got %.1f", hs.Score)
	}
}

func TestChecker_GetAllHealthScores(t *testing.T) {
	c := NewChecker(time.Hour, 3, 2)
	c.Register(&types.Channel{ID: "ch1", BaseURL: "http://test", Enabled: true})
	c.Register(&types.Channel{ID: "ch2", BaseURL: "http://test2", Enabled: true})

	scores := c.GetAllHealthScores()
	if len(scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(scores))
	}
}

func TestChecker_Probe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewChecker(time.Hour, 3, 2)
	c.Register(&types.Channel{ID: "ch1", BaseURL: ts.URL, Enabled: true})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c.checkAll(ctx)

	hs := c.GetHealthScore("ch1")
	if !hs.LastHealthy {
		t.Fatal("probe should succeed against test server")
	}
}

func TestChecker_Unregister(t *testing.T) {
	c := NewChecker(time.Hour, 3, 2)
	c.Register(&types.Channel{ID: "ch1", BaseURL: "http://test", Enabled: true})

	c.Unregister("ch1")

	hs := c.GetHealthScore("ch1")
	if hs != nil {
		t.Fatal("should not find unregistered channel")
	}
}
