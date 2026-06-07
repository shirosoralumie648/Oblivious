package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInMemoryRateLimiterRPMUsesSlidingWindow(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	clock := newManualClock(now)
	limiter := NewInMemoryRateLimiter(InMemoryOptions{Clock: clock.Now})
	ctx := context.Background()
	limits := Limits{RPM: 2}
	key := Key{ChannelID: "ch_1", Model: "gpt-5", TokenID: "tok_1"}

	if err := limiter.Allow(ctx, key, limits, Usage{Tokens: 1}); err != nil {
		t.Fatalf("first request should pass: %v", err)
	}
	clock.Advance(30 * time.Second)
	if err := limiter.Allow(ctx, key, limits, Usage{Tokens: 1}); err != nil {
		t.Fatalf("second request should pass: %v", err)
	}
	clock.Advance(29 * time.Second)
	err := limiter.Allow(ctx, key, limits, Usage{Tokens: 1})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("third request inside sliding window err = %v, want ErrRateLimited", err)
	}
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Dimension != DimensionRPM {
		t.Fatalf("err = %v, want RPM LimitError", err)
	}

	clock.Advance(time.Second)
	if err := limiter.Allow(ctx, key, limits, Usage{Tokens: 1}); err != nil {
		t.Fatalf("request should pass after oldest event slides out: %v", err)
	}
}

func TestInMemoryRateLimiterTPMUsesSixtySecondAccumulator(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	clock := newManualClock(now)
	limiter := NewInMemoryRateLimiter(InMemoryOptions{Clock: clock.Now})
	ctx := context.Background()
	limits := Limits{TPM: 10}
	key := Key{ChannelID: "ch_1", Model: "gpt-5", TokenID: "tok_1"}

	if err := limiter.Allow(ctx, key, limits, Usage{Tokens: 6}); err != nil {
		t.Fatalf("first token reservation should pass: %v", err)
	}
	clock.Advance(30 * time.Second)
	err := limiter.Allow(ctx, key, limits, Usage{Tokens: 5})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("second token reservation err = %v, want ErrRateLimited", err)
	}
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Dimension != DimensionTPM || limitErr.Remaining != 4 {
		t.Fatalf("err = %#v, want TPM LimitError with 4 remaining", err)
	}

	clock.Advance(30 * time.Second)
	if err := limiter.Allow(ctx, key, limits, Usage{Tokens: 5}); err != nil {
		t.Fatalf("reservation should pass after 60s accumulator resets: %v", err)
	}
}

func TestInMemoryRateLimiterRequestTokenLimitRejectsOversizedSingleRequestWithoutConsumingWindow(t *testing.T) {
	limiter := NewInMemoryRateLimiter(InMemoryOptions{})
	ctx := context.Background()
	limits := Limits{RPM: 1, TPM: 100, MaxTokensPerRequest: 20}
	key := Key{ChannelID: "ch_1", Model: "gpt-5", TokenID: "tok_1"}

	err := limiter.Allow(ctx, key, limits, Usage{Tokens: 25})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("oversized request err = %v, want ErrRateLimited", err)
	}
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Dimension != DimensionRequestTokens {
		t.Fatalf("err = %#v, want request-token LimitError", err)
	}
	if limitErr.Limit != 20 || limitErr.Current != 25 || limitErr.Remaining != 0 {
		t.Fatalf("limit decision = %#v, want current request tokens without remaining budget", limitErr.Decision)
	}

	if err := limiter.Allow(ctx, key, limits, Usage{Tokens: 10}); err != nil {
		t.Fatalf("valid request should pass after oversized request was rejected without consuming RPM/TPM: %v", err)
	}
}

func TestInMemoryRateLimiterCheckReportsAllowedLocalUsage(t *testing.T) {
	limiter := NewInMemoryRateLimiter(InMemoryOptions{})
	ctx := context.Background()
	key := Key{ChannelID: "ch_1", Model: "gpt-5", TokenID: "tok_1"}
	limits := Limits{RPM: 10, TPM: 100}

	for i := 0; i < 9; i++ {
		if err := limiter.Allow(ctx, key, limits, Usage{Tokens: 5}); err != nil {
			t.Fatalf("seed request %d should pass: %v", i+1, err)
		}
	}

	decision := limiter.Check(ctx, key, limits, Usage{Tokens: 5})
	if !decision.Allowed || decision.Dimension != DimensionRPM || decision.Limit != 10 || decision.Current != 9 || decision.Remaining != 1 {
		t.Fatalf("RPM check decision = %+v, want allowed current usage", decision)
	}
}

func TestInMemoryRateLimiterConcurrentBeginEnd(t *testing.T) {
	limiter := NewInMemoryRateLimiter(InMemoryOptions{})
	ctx := context.Background()
	limits := Limits{MaxConcurrent: 1}
	key := Key{ChannelID: "ch_1", Model: "gpt-5", TokenID: "tok_1"}

	if err := limiter.Begin(ctx, key, limits); err != nil {
		t.Fatalf("first begin should pass: %v", err)
	}
	err := limiter.Begin(ctx, key, limits)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("second begin err = %v, want ErrRateLimited", err)
	}
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Dimension != DimensionConcurrent {
		t.Fatalf("err = %v, want concurrent LimitError", err)
	}

	if err := limiter.End(ctx, key); err != nil {
		t.Fatalf("end should pass: %v", err)
	}
	if err := limiter.Begin(ctx, key, limits); err != nil {
		t.Fatalf("begin should pass after end: %v", err)
	}
}

func TestInMemoryRateLimiterSeparatesChannelModelAndToken(t *testing.T) {
	limiter := NewInMemoryRateLimiter(InMemoryOptions{})
	ctx := context.Background()
	limits := Limits{RPM: 1}
	key := Key{ChannelID: "ch_1", Model: "gpt-5", TokenID: "tok_1"}

	if err := limiter.Allow(ctx, key, limits, Usage{}); err != nil {
		t.Fatalf("initial request should pass: %v", err)
	}
	if err := limiter.Allow(ctx, Key{ChannelID: "ch_2", Model: "gpt-5", TokenID: "tok_1"}, limits, Usage{}); err != nil {
		t.Fatalf("different channel should have separate RPM bucket: %v", err)
	}
	if err := limiter.Allow(ctx, Key{ChannelID: "ch_1", Model: "gpt-4", TokenID: "tok_1"}, limits, Usage{}); err != nil {
		t.Fatalf("different model should have separate RPM bucket: %v", err)
	}
	if err := limiter.Allow(ctx, Key{ChannelID: "ch_1", Model: "gpt-5", TokenID: "tok_2"}, limits, Usage{}); err != nil {
		t.Fatalf("different token should have separate RPM bucket: %v", err)
	}
	if err := limiter.Allow(ctx, key, limits, Usage{}); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("same channel/model/token err = %v, want ErrRateLimited", err)
	}
}

type manualClock struct {
	now time.Time
}

func newManualClock(now time.Time) *manualClock {
	return &manualClock{now: now}
}

func (c *manualClock) Now() time.Time {
	return c.now
}

func (c *manualClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}
