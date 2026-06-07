package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultWindow = time.Minute

var ErrRateLimited = errors.New("relay rate limit exceeded")

type Dimension string

const (
	DimensionRPM        Dimension = "rpm"
	DimensionTPM        Dimension = "tpm"
	DimensionConcurrent Dimension = "concurrent"
)

type Key struct {
	ChannelID string
	Model     string
	TokenID   string
}

type Limits struct {
	RPM           int
	TPM           int
	MaxConcurrent int
}

type Usage struct {
	Tokens int
}

type Decision struct {
	Allowed    bool
	Dimension  Dimension
	Limit      int
	Current    int
	Remaining  int
	RetryAfter time.Duration
}

type LimitError struct {
	Decision
	Key Key
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("relay rate limit exceeded: %s", e.Dimension)
}

func (e *LimitError) Unwrap() error {
	return ErrRateLimited
}

type RateLimiter interface {
	Allow(ctx context.Context, key Key, limits Limits, usage Usage) error
	Begin(ctx context.Context, key Key, limits Limits) error
	End(ctx context.Context, key Key) error
	Check(ctx context.Context, key Key, limits Limits, usage Usage) Decision
}

type WindowCounter interface {
	Add(ctx context.Context, key string, amount int, limit int, window time.Duration) (Decision, error)
	Current(ctx context.Context, key string, window time.Duration) (int, error)
}

type InMemoryOptions struct {
	Clock func() time.Time
}

type InMemoryRateLimiter struct {
	mu         sync.Mutex
	clock      func() time.Time
	rpmEvents  map[string][]time.Time
	tpmWindows map[string]fixedWindow
	concurrent map[string]int
}

type fixedWindow struct {
	start time.Time
	total int
}

func NewInMemoryRateLimiter(opts InMemoryOptions) *InMemoryRateLimiter {
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	return &InMemoryRateLimiter{
		clock:      clock,
		rpmEvents:  make(map[string][]time.Time),
		tpmWindows: make(map[string]fixedWindow),
		concurrent: make(map[string]int),
	}
}

func (l *InMemoryRateLimiter) Allow(ctx context.Context, key Key, limits Limits, usage Usage) error {
	_ = ctx
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock().UTC()
	if limits.RPM > 0 {
		rpmKey := counterKey("rpm", key)
		events := pruneEvents(l.rpmEvents[rpmKey], now, defaultWindow)
		if len(events)+1 > limits.RPM {
			l.rpmEvents[rpmKey] = events
			return limitError(key, DimensionRPM, limits.RPM, len(events), 0, retryAfter(events, now, defaultWindow))
		}
		l.rpmEvents[rpmKey] = append(events, now)
	}

	tokens := usage.Tokens
	if tokens < 0 {
		tokens = 0
	}
	if limits.TPM > 0 && tokens > 0 {
		tpmKey := counterKey("tpm", key)
		window := l.tpmWindows[tpmKey]
		if window.start.IsZero() || !now.Before(window.start.Add(defaultWindow)) {
			window = fixedWindow{start: now}
		}
		if window.total+tokens > limits.TPM {
			l.tpmWindows[tpmKey] = window
			return limitError(key, DimensionTPM, limits.TPM, window.total, maxInt(0, limits.TPM-window.total), window.start.Add(defaultWindow).Sub(now))
		}
		window.total += tokens
		l.tpmWindows[tpmKey] = window
	}

	return nil
}

func (l *InMemoryRateLimiter) Begin(ctx context.Context, key Key, limits Limits) error {
	_ = ctx
	if limits.MaxConcurrent <= 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	concurrentKey := counterKey("concurrent", key)
	current := l.concurrent[concurrentKey]
	if current+1 > limits.MaxConcurrent {
		return limitError(key, DimensionConcurrent, limits.MaxConcurrent, current, 0, 0)
	}
	l.concurrent[concurrentKey] = current + 1
	return nil
}

func (l *InMemoryRateLimiter) End(ctx context.Context, key Key) error {
	_ = ctx
	l.mu.Lock()
	defer l.mu.Unlock()

	concurrentKey := counterKey("concurrent", key)
	if l.concurrent[concurrentKey] <= 1 {
		delete(l.concurrent, concurrentKey)
		return nil
	}
	l.concurrent[concurrentKey]--
	return nil
}

func (l *InMemoryRateLimiter) Check(ctx context.Context, key Key, limits Limits, usage Usage) Decision {
	_ = ctx
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock().UTC()
	if limits.RPM > 0 {
		events := pruneEvents(l.rpmEvents[counterKey("rpm", key)], now, defaultWindow)
		if len(events)+1 > limits.RPM {
			return Decision{Allowed: false, Dimension: DimensionRPM, Limit: limits.RPM, Current: len(events), RetryAfter: retryAfter(events, now, defaultWindow)}
		}
	}
	if limits.TPM > 0 && usage.Tokens > 0 {
		window := l.tpmWindows[counterKey("tpm", key)]
		current := 0
		retry := time.Duration(0)
		if !window.start.IsZero() && now.Before(window.start.Add(defaultWindow)) {
			current = window.total
			retry = window.start.Add(defaultWindow).Sub(now)
		}
		if current+usage.Tokens > limits.TPM {
			return Decision{Allowed: false, Dimension: DimensionTPM, Limit: limits.TPM, Current: current, Remaining: maxInt(0, limits.TPM-current), RetryAfter: retry}
		}
	}
	if limits.MaxConcurrent > 0 {
		current := l.concurrent[counterKey("concurrent", key)]
		if current+1 > limits.MaxConcurrent {
			return Decision{Allowed: false, Dimension: DimensionConcurrent, Limit: limits.MaxConcurrent, Current: current}
		}
	}
	return Decision{Allowed: true}
}

func pruneEvents(events []time.Time, now time.Time, window time.Duration) []time.Time {
	cutoff := now.Add(-window)
	first := 0
	for first < len(events) && !events[first].After(cutoff) {
		first++
	}
	return events[first:]
}

func retryAfter(events []time.Time, now time.Time, window time.Duration) time.Duration {
	if len(events) == 0 {
		return 0
	}
	retry := events[0].Add(window).Sub(now)
	if retry < 0 {
		return 0
	}
	return retry
}

func limitError(key Key, dimension Dimension, limit, current, remaining int, retryAfter time.Duration) *LimitError {
	return &LimitError{
		Key: key,
		Decision: Decision{
			Allowed:    false,
			Dimension:  dimension,
			Limit:      limit,
			Current:    current,
			Remaining:  remaining,
			RetryAfter: retryAfter,
		},
	}
}

func counterKey(prefix string, key Key) string {
	return strings.Join([]string{prefix, key.ChannelID, key.Model, key.TokenID}, ":")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
