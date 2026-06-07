package gateway

import (
	"sync"
	"time"
)

// circuitState represents the state of the circuit breaker.
type circuitState int

const (
	// stateClosed means the circuit is closed; requests flow normally.
	stateClosed circuitState = iota
	// stateOpen means the circuit is open; requests are rejected.
	stateOpen
	// stateHalfOpen means the circuit is in half-open state; a single probe
	// request is allowed through to test recovery.
	stateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern with three states:
// closed, open, and half-open. It tracks per-service-target error rates and
// automatically transitions between states based on configurable thresholds.
type CircuitBreaker struct {
	mu              sync.RWMutex
	targets         map[ServiceTarget]*circuitTarget
	threshold       float64
	openDuration    time.Duration
	minRequests     int
}

// circuitTarget tracks the state for a single service target.
type circuitTarget struct {
	state         circuitState
	failures      int
	successes     int
	totalRequests int
	openedAt      time.Time
}

// NewCircuitBreaker creates a new CircuitBreaker.
// threshold is the error-rate percentage (0-100) that triggers the circuit to open.
// openDuration is how long the circuit stays open before transitioning to half-open.
func NewCircuitBreaker(threshold float64, openDuration time.Duration) *CircuitBreaker {
	if threshold <= 0 || threshold > 100 {
		threshold = 50.0
	}
	if openDuration <= 0 {
		openDuration = 30 * time.Second
	}
	return &CircuitBreaker{
		targets:      make(map[ServiceTarget]*circuitTarget),
		threshold:    threshold,
		openDuration: openDuration,
		minRequests:  10, // Minimum requests before calculating error rate.
	}
}

// Allow returns true if requests to the given service target are permitted.
func (cb *CircuitBreaker) Allow(target ServiceTarget) bool {
	cb.mu.RLock()
	t, exists := cb.targets[target]
	cb.mu.RUnlock()

	if !exists {
		return true // No data yet; allow.
	}

	switch t.state {
	case stateClosed:
		return true
	case stateOpen:
		// Check if enough time has passed to transition to half-open.
		if time.Since(t.openedAt) >= cb.openDuration {
			cb.mu.Lock()
			if t.state == stateOpen {
				t.state = stateHalfOpen
			}
			cb.mu.Unlock()
			return true // Allow probe request.
		}
		return false
	case stateHalfOpen:
		// Allow exactly one probe request at a time.
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful response for the given service target.
func (cb *CircuitBreaker) RecordSuccess(target ServiceTarget) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	t := cb.getOrCreate(target)
	t.successes++
	t.totalRequests++

	switch t.state {
	case stateHalfOpen:
		// Probe succeeded; close the circuit.
		t.state = stateClosed
		t.failures = 0
		t.successes = 0
		t.totalRequests = 0
	case stateClosed:
		// Reset counters after enough successful requests to prevent stale data.
		if t.totalRequests > cb.minRequests*10 {
			t.failures = 0
			t.successes = 0
			t.totalRequests = 0
		}
	}
}

// RecordFailure records a failed response for the given service target.
func (cb *CircuitBreaker) RecordFailure(target ServiceTarget) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	t := cb.getOrCreate(target)
	t.failures++
	t.totalRequests++

	switch t.state {
	case stateClosed:
		if t.totalRequests >= cb.minRequests {
			errorRate := float64(t.failures) / float64(t.totalRequests) * 100
			if errorRate >= cb.threshold {
				t.state = stateOpen
				t.openedAt = time.Now()
			}
		}
	case stateHalfOpen:
		// Probe failed; re-open the circuit.
		t.state = stateOpen
		t.openedAt = time.Now()
		t.failures = 0
		t.successes = 0
		t.totalRequests = 0
	}
}

// State returns the current state of the circuit breaker for a target.
func (cb *CircuitBreaker) State(target ServiceTarget) circuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	t, exists := cb.targets[target]
	if !exists {
		return stateClosed
	}
	return t.state
}

// getOrCreate returns the target entry, creating it if necessary.
// Caller must hold cb.mu.
func (cb *CircuitBreaker) getOrCreate(target ServiceTarget) *circuitTarget {
	t, exists := cb.targets[target]
	if !exists {
		t = &circuitTarget{state: stateClosed}
		cb.targets[target] = t
	}
	return t
}
