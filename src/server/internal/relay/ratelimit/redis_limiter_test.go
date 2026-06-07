package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestRedisRateLimiterUsesChannelModelAndTokenNamespace(t *testing.T) {
	client := newFakeRedisRateLimitClient()
	limiter := NewRedisRateLimiter(client, RedisOptions{})
	ctx := context.Background()
	key := Key{ChannelID: "ch_1", Model: "gpt-5", TokenID: "tok_1"}

	if err := limiter.Allow(ctx, key, Limits{RPM: 10, TPM: 100}, Usage{Tokens: 12}); err != nil {
		t.Fatalf("allow should pass: %v", err)
	}
	if err := limiter.Begin(ctx, key, Limits{MaxConcurrent: 3}); err != nil {
		t.Fatalf("begin should pass: %v", err)
	}

	keys := client.keys()
	want := []string{
		"relay:ratelimit:rpm:ch_1:gpt-5:tok_1",
		"relay:ratelimit:tpm:ch_1:gpt-5:tok_1",
		"relay:ratelimit:concurrent:ch_1:gpt-5:tok_1",
	}
	for _, key := range want {
		if !slices.Contains(keys, key) {
			t.Fatalf("redis keys = %v, want %s", keys, key)
		}
	}
}

func TestRedisRateLimiterRPMReturnsRateLimitedDecision(t *testing.T) {
	client := newFakeRedisRateLimitClient()
	limiter := NewRedisRateLimiter(client, RedisOptions{})
	ctx := context.Background()
	key := Key{ChannelID: "ch_1", Model: "gpt-5", TokenID: "tok_1"}
	limits := Limits{RPM: 1}

	if err := limiter.Allow(ctx, key, limits, Usage{}); err != nil {
		t.Fatalf("first request should pass: %v", err)
	}
	err := limiter.Allow(ctx, key, limits, Usage{})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("second request err = %v, want ErrRateLimited", err)
	}
	var limitErr *LimitError
	if !errors.As(err, &limitErr) {
		t.Fatalf("second request err = %v, want LimitError", err)
	}
	if limitErr.Dimension != DimensionRPM || limitErr.Limit != 1 || limitErr.Current != 1 || limitErr.Remaining != 0 {
		t.Fatalf("limit decision = %#v, want RPM decision at limit", limitErr.Decision)
	}
}

func TestRedisRateLimiterRequestTokenLimitRejectsOversizedSingleRequestWithoutConsumingWindow(t *testing.T) {
	client := newFakeRedisRateLimitClient()
	limiter := NewRedisRateLimiter(client, RedisOptions{})
	ctx := context.Background()
	key := Key{ChannelID: "ch_1", Model: "gpt-5", TokenID: "tok_1"}
	limits := Limits{RPM: 1, TPM: 100, MaxTokensPerRequest: 20}

	err := limiter.Allow(ctx, key, limits, Usage{Tokens: 25})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("oversized request err = %v, want ErrRateLimited", err)
	}
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Dimension != DimensionRequestTokens {
		t.Fatalf("oversized request err = %#v, want request-token LimitError", err)
	}

	if err := limiter.Allow(ctx, key, limits, Usage{Tokens: 10}); err != nil {
		t.Fatalf("valid request should pass after oversized request was rejected without consuming RPM/TPM: %v", err)
	}
}

func TestRedisRateLimiterCheckReportsAllowedLocalUsage(t *testing.T) {
	client := newFakeRedisRateLimitClient()
	limiter := NewRedisRateLimiter(client, RedisOptions{})
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
	if err := limiter.Allow(ctx, key, limits, Usage{Tokens: 5}); err != nil {
		t.Fatalf("check should not consume RPM/TPM window before the next allow: %v", err)
	}
}

func TestRedisRateLimiterBeginEndManagesConcurrency(t *testing.T) {
	client := newFakeRedisRateLimitClient()
	limiter := NewRedisRateLimiter(client, RedisOptions{})
	ctx := context.Background()
	key := Key{ChannelID: "ch_1", Model: "gpt-5", TokenID: "tok_1"}
	limits := Limits{MaxConcurrent: 1}

	if err := limiter.Begin(ctx, key, limits); err != nil {
		t.Fatalf("first begin should pass: %v", err)
	}
	err := limiter.Begin(ctx, key, limits)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("second begin err = %v, want ErrRateLimited", err)
	}
	var limitErr *LimitError
	if !errors.As(err, &limitErr) || limitErr.Dimension != DimensionConcurrent {
		t.Fatalf("second begin err = %v, want concurrent LimitError", err)
	}

	if err := limiter.End(ctx, key); err != nil {
		t.Fatalf("end should pass: %v", err)
	}
	if err := limiter.Begin(ctx, key, limits); err != nil {
		t.Fatalf("begin after end should pass: %v", err)
	}
}

type fakeRedisRateLimitClient struct {
	mu         sync.Mutex
	keysSeen   []string
	rpm        map[string]int
	tpm        map[string]int
	concurrent map[string]int
}

func newFakeRedisRateLimitClient() *fakeRedisRateLimitClient {
	return &fakeRedisRateLimitClient{
		rpm:        make(map[string]int),
		tpm:        make(map[string]int),
		concurrent: make(map[string]int),
	}
}

func (c *fakeRedisRateLimitClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()

	c.keysSeen = append(c.keysSeen, keys...)
	if len(keys) != 1 {
		return redis.NewCmdResult(nil, fmt.Errorf("expected one key, got %d", len(keys)))
	}
	limit := intArg(args, 0)
	key := keys[0]
	switch {
	case strings.Contains(script, "rpm sliding-window check"):
		if limit <= 0 {
			return redis.NewCmdResult([]interface{}{int64(1), int64(0), int64(0)}, nil)
		}
		current := c.rpm[key]
		if current+1 > limit {
			return redis.NewCmdResult([]interface{}{int64(0), int64(current), int64(0)}, nil)
		}
		return redis.NewCmdResult([]interface{}{int64(1), int64(current), int64(limit - current)}, nil)
	case strings.Contains(script, "rpm"):
		if limit <= 0 {
			return redis.NewCmdResult([]interface{}{int64(1), int64(0), int64(0)}, nil)
		}
		current := c.rpm[key]
		if current+1 > limit {
			return redis.NewCmdResult([]interface{}{int64(0), int64(current), int64(0)}, nil)
		}
		current++
		c.rpm[key] = current
		return redis.NewCmdResult([]interface{}{int64(1), int64(current), int64(limit - current)}, nil)
	case strings.Contains(script, "tpm fixed-window check"):
		if limit <= 0 {
			return redis.NewCmdResult([]interface{}{int64(1), int64(0), int64(0)}, nil)
		}
		tokens := intArg(args, 1)
		current := c.tpm[key]
		if current+tokens > limit {
			return redis.NewCmdResult([]interface{}{int64(0), int64(current), int64(limit - current)}, nil)
		}
		return redis.NewCmdResult([]interface{}{int64(1), int64(current), int64(limit - current)}, nil)
	case strings.Contains(script, "tpm"):
		if limit <= 0 {
			return redis.NewCmdResult([]interface{}{int64(1), int64(0), int64(0)}, nil)
		}
		tokens := intArg(args, 1)
		current := c.tpm[key]
		if current+tokens > limit {
			return redis.NewCmdResult([]interface{}{int64(0), int64(current), int64(limit - current)}, nil)
		}
		current += tokens
		c.tpm[key] = current
		return redis.NewCmdResult([]interface{}{int64(1), int64(current), int64(limit - current)}, nil)
	case strings.Contains(script, "concurrency-release"):
		if c.concurrent[key] <= 1 {
			delete(c.concurrent, key)
			return redis.NewCmdResult([]interface{}{int64(1), int64(0), int64(limit)}, nil)
		}
		c.concurrent[key]--
		return redis.NewCmdResult([]interface{}{int64(1), int64(c.concurrent[key]), int64(limit - c.concurrent[key])}, nil)
	case strings.Contains(script, "concurrency-begin"):
		if limit <= 0 {
			return redis.NewCmdResult([]interface{}{int64(1), int64(0), int64(0)}, nil)
		}
		current := c.concurrent[key]
		if current+1 > limit {
			return redis.NewCmdResult([]interface{}{int64(0), int64(current), int64(0)}, nil)
		}
		current++
		c.concurrent[key] = current
		return redis.NewCmdResult([]interface{}{int64(1), int64(current), int64(limit - current)}, nil)
	default:
		return redis.NewCmdResult(nil, fmt.Errorf("unknown script: %s", script))
	}
}

func (c *fakeRedisRateLimitClient) keys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.keysSeen)
}

func intArg(args []interface{}, index int) int {
	if len(args) <= index {
		return 0
	}
	switch value := args[index].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case string:
		parsed, _ := strconv.Atoi(value)
		return parsed
	default:
		return 0
	}
}
