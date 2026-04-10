package relay

import (
	"sync"
	"time"
)

type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	}
	return "unknown"
}

type CircuitBreaker struct {
	mu               sync.Mutex
	provider         string
	failureLimit    int
	successLimit    int
	failureCount    int
	successCount    int
	state           CircuitState
	openedAt        time.Time
	probeAt         time.Time
	probeInterval   time.Duration
	maxProbeInterval time.Duration
}

func NewCircuitBreaker(provider string, failureLimit int, probeInterval, maxProbeInterval time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		provider:         provider,
		failureLimit:     failureLimit,
		successLimit:     3,
		failureCount:     0,
		successCount:     0,
		state:            StateClosed,
		probeInterval:    probeInterval,
		maxProbeInterval: maxProbeInterval,
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.recompute()
	return cb.state
}

func (cb *CircuitBreaker) recompute() {
	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.failureLimit {
			cb.state = StateOpen
			cb.openedAt = time.Now()
			cb.probeAt = cb.openedAt.Add(cb.probeInterval)
		}
	case StateOpen:
		if time.Now().After(cb.probeAt) {
			cb.state = StateHalfOpen
			cb.successCount = 0
		}
	case StateHalfOpen:
		if cb.successCount >= cb.successLimit {
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
			cb.probeInterval = cb.probeInterval / 2 // reset to base
		}
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	if cb.failureCount > cb.failureLimit {
		cb.probeInterval = minDuration(cb.probeInterval*2, cb.maxProbeInterval)
	}
	if cb.state == StateHalfOpen {
		cb.state = StateOpen
		cb.probeAt = time.Now().Add(cb.probeInterval)
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == StateHalfOpen {
		cb.successCount++
	}
	cb.failureCount = 0
}

func (cb *CircuitBreaker) CanTry() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state == StateOpen || cb.state == StateHalfOpen
}

func (cb *CircuitBreaker) Provider() string {
	return cb.provider
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
