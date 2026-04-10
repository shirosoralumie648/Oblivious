package relay

import (
	"sync"
	"testing"
	"time"
)

func TestTokenBucket_RefillRPM(t *testing.T) {
	tb := NewTokenBucket(10, 100) // rpm: 10, tpm: 100

	// Exhaust RPM tokens
	for i := 0; i < 10; i++ {
		ok, _ := tb.TryAcquire("rpm")
		if !ok {
			t.Fatalf("token %d should be acquirable", i)
		}
	}

	// 11th should fail
	ok, remaining := tb.TryAcquire("rpm")
	if ok {
		t.Fatal("11th token should be rejected")
	}
	if remaining != 0 {
		t.Fatalf("remaining rpm should be 0, got %d", remaining)
	}

	// Advance time by 6s → should refill 1 token (10 rpm / 60s = 1/6 per second)
	time.Sleep(6100 * time.Millisecond)
	ok, remaining = tb.TryAcquire("rpm")
	if !ok {
		t.Fatal("token should be refillable after 6s")
	}
	if remaining != 0 {
		t.Fatalf("remaining should be 0 (used 1 of refill), got %d", remaining)
	}
}

func TestTokenBucket_RefillTPM(t *testing.T) {
	tb := NewTokenBucket(60, 20) // rpm: 60, tpm: 20

	// Exhaust TPM tokens
	for i := 0; i < 20; i++ {
		ok, _ := tb.TryAcquire("tpm")
		if !ok {
			t.Fatalf("token %d should be acquirable, left: %d", i, 20-i-1)
		}
	}

	ok, remaining := tb.TryAcquire("tpm")
	if ok {
		t.Fatal("21st token should be rejected")
	}
	if remaining != 0 {
		t.Fatalf("remaining tpm should be 0, got %d", remaining)
	}
}

func TestTokenBucket_Concurrent(t *testing.T) {
	tb := NewTokenBucket(100, 1000)
	var success int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _ := tb.TryAcquire("rpm")
			if ok {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if success != 100 {
		t.Fatalf("expected exactly 100 successes under concurrency, got %d", success)
	}
}
