package relay

import (
	"sync"
	"time"
)

type TokenBucket struct {
	mu            sync.Mutex
	rpmTokens     int
	tpmTokens     int
	rpmLimit      int
	tpmLimit      int
	rpmLastRefill time.Time
	tpmLastRefill time.Time
}

func NewTokenBucket(rpm, tpm int) *TokenBucket {
	now := time.Now()
	return &TokenBucket{
		rpmTokens:    rpm,
		tpmTokens:    tpm,
		rpmLimit:     rpm,
		tpmLimit:     tpm,
		rpmLastRefill: now,
		tpmLastRefill: now,
	}
}

func (tb *TokenBucket) TryAcquire(dimension string) (bool, int) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	switch dimension {
	case "rpm":
		tb.refill(&tb.rpmTokens, &tb.rpmLastRefill, tb.rpmLimit)
		if tb.rpmTokens <= 0 {
			return false, 0
		}
		tb.rpmTokens--
		return true, tb.rpmTokens
	case "tpm":
		tb.refill(&tb.tpmTokens, &tb.tpmLastRefill, tb.tpmLimit)
		if tb.tpmTokens <= 0 {
			return false, 0
		}
		tb.tpmTokens--
		return true, tb.tpmTokens
	default:
		return true, 0
	}
}

func (tb *TokenBucket) refill(tokens *int, lastRefill *time.Time, limit int) {
	now := time.Now()
	elapsed := now.Sub(*lastRefill)
	// Refill: limit tokens per minute, proportionally for elapsed seconds
	refill := int(elapsed.Seconds() * float64(limit) / 60.0)
	if refill > 0 {
		*tokens = minInt(limit, *tokens+refill)
		*lastRefill = now
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (tb *TokenBucket) Available(dimension string) int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	switch dimension {
	case "rpm":
		tb.refill(&tb.rpmTokens, &tb.rpmLastRefill, tb.rpmLimit)
		return tb.rpmTokens
	case "tpm":
		tb.refill(&tb.tpmTokens, &tb.tpmLastRefill, tb.tpmLimit)
		return tb.tpmTokens
	}
	return 0
}
