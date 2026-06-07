package ratelimit

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultRedisKeyPrefix     = "relay:ratelimit"
	defaultRedisConcurrentTTL = 10 * time.Minute
)

type RedisClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
}

type RedisOptions struct {
	KeyPrefix     string
	Window        time.Duration
	ConcurrentTTL time.Duration
}

type RedisRateLimiter struct {
	client        RedisClient
	keyPrefix     string
	window        time.Duration
	concurrentTTL time.Duration
}

func NewRedisRateLimiter(client RedisClient, opts RedisOptions) *RedisRateLimiter {
	keyPrefix := strings.Trim(strings.TrimSpace(opts.KeyPrefix), ":")
	if keyPrefix == "" {
		keyPrefix = defaultRedisKeyPrefix
	}
	window := opts.Window
	if window <= 0 {
		window = defaultWindow
	}
	concurrentTTL := opts.ConcurrentTTL
	if concurrentTTL <= 0 {
		concurrentTTL = defaultRedisConcurrentTTL
	}
	return &RedisRateLimiter{
		client:        client,
		keyPrefix:     keyPrefix,
		window:        window,
		concurrentTTL: concurrentTTL,
	}
}

func (l *RedisRateLimiter) Allow(ctx context.Context, key Key, limits Limits, usage Usage) error {
	if err := l.ensureClient(); err != nil {
		return err
	}
	requestTokens := requestTokensFromUsage(usage)
	if limits.MaxTokensPerRequest > 0 && requestTokens > limits.MaxTokensPerRequest {
		return limitError(key, DimensionRequestTokens, limits.MaxTokensPerRequest, requestTokens, 0, 0)
	}

	if limits.RPM > 0 {
		decision, err := l.runDecisionScript(ctx, redisRPMScript, DimensionRPM, limits.RPM, l.redisKey("rpm", key), limits.RPM, millis(l.window), uniqueRedisMember())
		if err != nil {
			return err
		}
		if !decision.Allowed {
			return &LimitError{Key: key, Decision: decision}
		}
	}

	tokens := usage.Tokens
	if tokens < 0 {
		tokens = 0
	}
	if limits.TPM > 0 && tokens > 0 {
		decision, err := l.runDecisionScript(ctx, redisTPMScript, DimensionTPM, limits.TPM, l.redisKey("tpm", key), limits.TPM, tokens, millis(l.window))
		if err != nil {
			return err
		}
		if !decision.Allowed {
			return &LimitError{Key: key, Decision: decision}
		}
	}
	return nil
}

func (l *RedisRateLimiter) Begin(ctx context.Context, key Key, limits Limits) error {
	if err := l.ensureClient(); err != nil {
		return err
	}
	if limits.MaxConcurrent <= 0 {
		return nil
	}
	decision, err := l.runDecisionScript(ctx, redisConcurrencyBeginScript, DimensionConcurrent, limits.MaxConcurrent, l.redisKey("concurrent", key), limits.MaxConcurrent, millis(l.concurrentTTL))
	if err != nil {
		return err
	}
	if !decision.Allowed {
		return &LimitError{Key: key, Decision: decision}
	}
	return nil
}

func (l *RedisRateLimiter) End(ctx context.Context, key Key) error {
	if err := l.ensureClient(); err != nil {
		return err
	}
	_, err := l.runDecisionScript(ctx, redisConcurrencyReleaseScript, DimensionConcurrent, 0, l.redisKey("concurrent", key))
	return err
}

func (l *RedisRateLimiter) Check(ctx context.Context, key Key, limits Limits, usage Usage) Decision {
	if err := l.ensureClient(); err != nil {
		return Decision{Allowed: false}
	}
	requestTokens := requestTokensFromUsage(usage)
	if limits.MaxTokensPerRequest > 0 && requestTokens > limits.MaxTokensPerRequest {
		return Decision{Allowed: false, Dimension: DimensionRequestTokens, Limit: limits.MaxTokensPerRequest, Current: requestTokens}
	}

	bestAllowed := Decision{Allowed: true}
	if limits.RPM > 0 {
		decision, err := l.runDecisionScript(ctx, redisRPMCheckScript, DimensionRPM, limits.RPM, l.redisKey("rpm", key), limits.RPM, millis(l.window))
		if err != nil || !decision.Allowed {
			return decision
		}
		bestAllowed = highestProjectedUsage(bestAllowed, decision, 1)
	}

	tokens := usage.Tokens
	if tokens < 0 {
		tokens = 0
	}
	if limits.TPM > 0 && tokens > 0 {
		decision, err := l.runDecisionScript(ctx, redisTPMCheckScript, DimensionTPM, limits.TPM, l.redisKey("tpm", key), limits.TPM, tokens, millis(l.window))
		if err != nil || !decision.Allowed {
			return decision
		}
		bestAllowed = highestProjectedUsage(bestAllowed, decision, tokens)
	}

	if limits.MaxConcurrent > 0 {
		decision, err := l.runDecisionScript(ctx, redisConcurrencyCheckScript, DimensionConcurrent, limits.MaxConcurrent, l.redisKey("concurrent", key), limits.MaxConcurrent)
		if err != nil || !decision.Allowed {
			return decision
		}
	}
	return bestAllowed
}

func (l *RedisRateLimiter) runDecisionScript(ctx context.Context, script string, dimension Dimension, limit int, key string, args ...interface{}) (Decision, error) {
	result, err := l.client.Eval(ctx, script, []string{key}, args...).Result()
	if err != nil {
		return Decision{}, err
	}
	return redisDecision(result, dimension, limit)
}

func (l *RedisRateLimiter) redisKey(prefix string, key Key) string {
	return strings.Join([]string{l.keyPrefix, counterKey(prefix, key)}, ":")
}

func (l *RedisRateLimiter) ensureClient() error {
	if l == nil || l.client == nil {
		return errors.New("redis rate limiter: nil client")
	}
	return nil
}

func redisDecision(result interface{}, dimension Dimension, limit int) (Decision, error) {
	values, ok := result.([]interface{})
	if !ok {
		return Decision{}, fmt.Errorf("redis rate limiter: unexpected result type %T", result)
	}
	if len(values) < 3 {
		return Decision{}, fmt.Errorf("redis rate limiter: expected at least 3 result values, got %d", len(values))
	}
	allowed, err := redisResultInt(values[0])
	if err != nil {
		return Decision{}, err
	}
	current, err := redisResultInt(values[1])
	if err != nil {
		return Decision{}, err
	}
	remaining, err := redisResultInt(values[2])
	if err != nil {
		return Decision{}, err
	}
	retryAfter := time.Duration(0)
	if len(values) > 3 {
		retryMillis, err := redisResultInt(values[3])
		if err != nil {
			return Decision{}, err
		}
		if retryMillis > 0 {
			retryAfter = time.Duration(retryMillis) * time.Millisecond
		}
	}
	return Decision{
		Allowed:    allowed > 0,
		Dimension:  dimension,
		Limit:      limit,
		Current:    current,
		Remaining:  remaining,
		RetryAfter: retryAfter,
	}, nil
}

func redisResultInt(value interface{}) (int, error) {
	switch value := value.(type) {
	case int:
		return value, nil
	case int64:
		return int(value), nil
	case string:
		return strconv.Atoi(value)
	default:
		return 0, fmt.Errorf("redis rate limiter: unexpected integer result type %T", value)
	}
}

func uniqueRedisMember() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return fmt.Sprintf("%d:%016x", time.Now().UnixNano(), binary.BigEndian.Uint64(random[:]))
}

func millis(d time.Duration) int64 {
	return int64(d / time.Millisecond)
}

const redisRPMScript = `
-- rpm sliding-window counter
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local member = ARGV[3]
local now = redis.call("TIME")
local now_ms = now[1] * 1000 + math.floor(now[2] / 1000)
redis.call("ZREMRANGEBYSCORE", key, 0, now_ms - window_ms)
local current = tonumber(redis.call("ZCARD", key))
if current + 1 > limit then
  local oldest = redis.call("ZRANGE", key, 0, 0, "WITHSCORES")
  local retry_ms = 0
  if oldest[2] then
    retry_ms = tonumber(oldest[2]) + window_ms - now_ms
    if retry_ms < 0 then retry_ms = 0 end
  end
  redis.call("PEXPIRE", key, window_ms)
  return {0, current, math.max(0, limit - current), retry_ms}
end
redis.call("ZADD", key, now_ms, member)
redis.call("PEXPIRE", key, window_ms)
return {1, current + 1, math.max(0, limit - current - 1), 0}
`

const redisRPMCheckScript = `
-- rpm sliding-window check
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local now = redis.call("TIME")
local now_ms = now[1] * 1000 + math.floor(now[2] / 1000)
redis.call("ZREMRANGEBYSCORE", key, 0, now_ms - window_ms)
local current = tonumber(redis.call("ZCARD", key))
if current + 1 > limit then
  local oldest = redis.call("ZRANGE", key, 0, 0, "WITHSCORES")
  local retry_ms = 0
  if oldest[2] then
    retry_ms = tonumber(oldest[2]) + window_ms - now_ms
    if retry_ms < 0 then retry_ms = 0 end
  end
  return {0, current, math.max(0, limit - current), retry_ms}
end
return {1, current, math.max(0, limit - current), 0}
`

const redisTPMScript = `
-- tpm fixed-window accumulator
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local tokens = tonumber(ARGV[2])
local window_ms = tonumber(ARGV[3])
local current = tonumber(redis.call("GET", key) or "0")
if current + tokens > limit then
  local retry_ms = redis.call("PTTL", key)
  if retry_ms < 0 then retry_ms = window_ms end
  return {0, current, math.max(0, limit - current), retry_ms}
end
current = tonumber(redis.call("INCRBY", key, tokens))
if current == tokens then
  redis.call("PEXPIRE", key, window_ms)
end
local retry_ms = redis.call("PTTL", key)
if retry_ms < 0 then
  redis.call("PEXPIRE", key, window_ms)
  retry_ms = window_ms
end
return {1, current, math.max(0, limit - current), retry_ms}
`

const redisTPMCheckScript = `
-- tpm fixed-window check
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local tokens = tonumber(ARGV[2])
local window_ms = tonumber(ARGV[3])
local current = tonumber(redis.call("GET", key) or "0")
if current + tokens > limit then
  local retry_ms = redis.call("PTTL", key)
  if retry_ms < 0 then retry_ms = window_ms end
  return {0, current, math.max(0, limit - current), retry_ms}
end
return {1, current, math.max(0, limit - current), 0}
`

const redisConcurrencyBeginScript = `
-- concurrency-begin counter
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local ttl_ms = tonumber(ARGV[2])
local current = tonumber(redis.call("GET", key) or "0")
if current + 1 > limit then
  return {0, current, math.max(0, limit - current), 0}
end
current = tonumber(redis.call("INCR", key))
if ttl_ms > 0 then
  redis.call("PEXPIRE", key, ttl_ms)
end
return {1, current, math.max(0, limit - current), 0}
`

const redisConcurrencyReleaseScript = `
-- concurrency-release counter
local key = KEYS[1]
local current = tonumber(redis.call("GET", key) or "0")
if current <= 1 then
  redis.call("DEL", key)
  return {1, 0, 0, 0}
end
current = tonumber(redis.call("DECR", key))
return {1, current, 0, 0}
`

const redisConcurrencyCheckScript = `
-- concurrency-check counter
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local current = tonumber(redis.call("GET", key) or "0")
if current + 1 > limit then
  return {0, current, math.max(0, limit - current), 0}
end
return {1, current, math.max(0, limit - current), 0}
`
