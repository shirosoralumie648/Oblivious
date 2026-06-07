package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SlidingWindowLimiter implements a distributed sliding-window rate limiter
// backed by Redis. Each key tracks request count within a rolling window.
type SlidingWindowLimiter struct {
	client *redis.Client
	cfg    SlidingWindowConfig
}

// NewSlidingWindowLimiter creates a new Redis-backed sliding window rate limiter.
func NewSlidingWindowLimiter(client *redis.Client, cfg SlidingWindowConfig) *SlidingWindowLimiter {
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = time.Minute
	}
	return &SlidingWindowLimiter{
		client: client,
		cfg:    cfg,
	}
}

// Allow checks whether a request for the given key is within the limit.
// It uses a Redis sorted set with timestamp scores to implement the sliding
// window algorithm. Returns nil if the request is allowed, or an error if
// the limit is exceeded.
func (l *SlidingWindowLimiter) Allow(ctx context.Context, key string, limit int) error {
	if limit <= 0 {
		return nil // no limit configured
	}

	now := time.Now()
	windowStart := now.Add(-l.cfg.WindowSize)
	redisKey := fmt.Sprintf("gw:ratelimit:%s", key)

	// Use a Lua script for atomicity: remove expired entries, count current
	// window, add the new entry if under limit.
	script := redis.NewScript(`
		local key = KEYS[1]
		local window_start = tonumber(ARGV[1])
		local now = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local ttl = tonumber(ARGV[4])

		-- Remove entries outside the window.
		redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

		-- Count current entries in the window.
		local count = redis.call('ZCARD', key)

		if count < limit then
			-- Add new entry with current timestamp as score and member.
			redis.call('ZADD', key, now, now .. '-' .. math.random(1000000))
			redis.call('EXPIRE', key, ttl)
			return 1
		else
			-- Over limit.
			redis.call('EXPIRE', key, ttl)
			return 0
		end
	`)

	ttlSeconds := int(l.cfg.WindowSize.Seconds()) + 1
	result, err := script.Run(ctx, l.client, []string{redisKey},
		windowStart.UnixMicro(),
		now.UnixMicro(),
		limit,
		ttlSeconds,
	).Int()

	if err != nil {
		// On Redis failure, fail open (allow the request) to avoid cascading outages.
		return nil
	}

	if result == 0 {
		return fmt.Errorf("rate limit exceeded for key %s", key)
	}

	return nil
}

// AllowWithTPM checks both RPM and TPM (tokens-per-minute) limits for a given
// base key. tokenCount is the number of tokens consumed by this request.
func (l *SlidingWindowLimiter) AllowWithTPM(ctx context.Context, baseKey string, rpmLimit int, tpmLimit int, tokenCount int) error {
	// Check RPM.
	if err := l.Allow(ctx, baseKey+":rpm", rpmLimit); err != nil {
		return err
	}

	// Check TPM: increment a counter key by tokenCount with expiry.
	if tpmLimit > 0 && tokenCount > 0 {
		now := time.Now()
		windowStart := now.Add(-l.cfg.WindowSize)
		redisKey := fmt.Sprintf("gw:ratelimit:%s:tpm", baseKey)

		script := redis.NewScript(`
			local key = KEYS[1]
			local window_start = tonumber(ARGV[1])
			local tokens = tonumber(ARGV[2])
			local limit = tonumber(ARGV[3])
			local ttl = tonumber(ARGV[4])

			-- Remove expired entries.
			redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

			-- Sum tokens in the current window.
			local members = redis.call('ZRANGEBYSCORE', key, window_start, '+inf', 'WITHSCORES')
			local total = 0
			for i = 2, #members, 2 do
				total = total + tonumber(members[i])
			end

			if total + tokens <= limit then
				redis.call('ZADD', key, tokens, tonumber(ARGV[5]) .. '-' .. math.random(1000000))
				redis.call('EXPIRE', key, ttl)
				return 1
			else
				redis.call('EXPIRE', key, ttl)
				return 0
			end
		`)

		ttlSeconds := int(l.cfg.WindowSize.Seconds()) + 1
		result, err := script.Run(ctx, l.client, []string{redisKey},
			windowStart.UnixMicro(),
			tokenCount,
			tpmLimit,
			ttlSeconds,
			now.UnixMicro(),
		).Int()

		if err != nil {
			// Fail open on Redis errors.
			return nil
		}

		if result == 0 {
			return fmt.Errorf("token rate limit exceeded for key %s", baseKey)
		}
	}

	return nil
}
