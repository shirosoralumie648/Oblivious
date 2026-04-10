package relay

import (
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := NewCircuitBreaker("test-provider", 5, 10*time.Second, 60*time.Second)

	// 5 failures should trip the breaker
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected Open after 5 failures, got %s", cb.State())
	}
	if !cb.CanTry() {
		t.Fatal("CanTry should be true when Open (pre-flight probe)")
	}
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker("test-provider", 5, 10*time.Millisecond, 60*time.Second)
	cb.failureCount = 5
	_ = cb.State() // trigger state computation

	// Advance past probe interval
	time.Sleep(15 * time.Millisecond)
	cb.probeAt = time.Now().Add(-time.Second) // force into probe window

	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HalfOpen after probe interval, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	cb := NewCircuitBreaker("test-provider", 5, 10*time.Millisecond, 60*time.Second)
	cb.state = StateHalfOpen
	cb.successCount = 3

	if cb.State() != StateClosed {
		t.Fatalf("expected Closed after 3 successes in HalfOpen, got %s", cb.State())
	}
}

func TestCircuitBreaker_Escalation(t *testing.T) {
	cb := NewCircuitBreaker("test-provider", 5, 10*time.Second, 60*time.Second)
	// Call RecordFailure 10 times to trigger escalation beyond failureLimit=5
	for i := 0; i < 10; i++ {
		cb.RecordFailure()
	}

	// 5 extra failures beyond limit → probeInterval doubles to 20s, then 40s, capped at 60s
	if cb.probeInterval != 60*time.Second {
		t.Fatalf("expected escalated probeInterval 60s, got %v", cb.probeInterval)
	}
}
