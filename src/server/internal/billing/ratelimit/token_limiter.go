package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrRateLimitExceeded = errors.New("ratelimit: token rate limit exceeded")

// TokenRateLimiter 基于 Redis 滑动窗口的 Token 速率限制器
type TokenRateLimiter struct {
	rdb       *redis.Client
	keyPrefix string
	maxTokens int
	window    time.Duration
}

// NewTokenRateLimiter 创建 Token 速率限制器
// keyPrefix: Redis key 前缀
// maxTokens: 窗口内最大 token 数
// window: 滑动窗口大小
func NewTokenRateLimiter(rdb *redis.Client, keyPrefix string, maxTokens int, window time.Duration) *TokenRateLimiter {
	return &TokenRateLimiter{
		rdb:       rdb,
		keyPrefix: keyPrefix,
		maxTokens: maxTokens,
		window:    window,
	}
}

func (l *TokenRateLimiter) key(ownerID string) string {
	return fmt.Sprintf("%s:token_limit:%s", l.keyPrefix, ownerID)
}

// Allow 检查消耗指定 token 数是否被允许
// 返回 true 表示允许，false 表示超出限制
func (l *TokenRateLimiter) Allow(ctx context.Context, ownerID string, tokens int) (bool, error) {
	key := l.key(ownerID)
	now := time.Now().UnixMicro()
	windowStart := now - l.window.Microseconds()

	// Lua 滑动窗口脚本
	// 1. 清除窗口外的旧数据
	// 2. 统计窗口内总 token
	// 3. 如果未超限则添加新记录
	script := redis.NewScript(`
		local key = KEYS[1]
		local window_start = tonumber(ARGV[1])
		local now = tonumber(ARGV[2])
		local tokens = tonumber(ARGV[3])
		local max_tokens = tonumber(ARGV[4])

		redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

		local current = 0
		local members = redis.call('ZRANGE', key, 0, -1, 'WITHSCORES')
		for i = 2, #members, 2 do
			current = current + tonumber(members[i])
		end

		if current + tokens > max_tokens then
			return -1
		end

		redis.call('ZADD', key, now, now .. ':' .. tokens)
		redis.call('EXPIRE', key, math.ceil((now - window_start) / 1000000) + 1)
		return current + tokens
	`)

	result, err := script.Run(ctx, l.rdb, []string{key},
		windowStart, now, tokens, l.maxTokens,
	).Int()
	if err != nil {
		return false, fmt.Errorf("ratelimit: allow check failed: %w", err)
	}
	if result == -1 {
		return false, ErrRateLimitExceeded
	}

	return true, nil
}

// CurrentUsage 获取当前窗口内的 token 使用量
func (l *TokenRateLimiter) CurrentUsage(ctx context.Context, ownerID string) (int, error) {
	key := l.key(ownerID)
	now := time.Now().UnixMicro()
	windowStart := now - l.window.Microseconds()

	// 清除旧数据
	if err := l.rdb.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", windowStart)).Err(); err != nil {
		return 0, err
	}

	members, err := l.rdb.ZRangeWithScores(ctx, key, 0, -1).Result()
	if err != nil {
		return 0, err
	}

	var total int
	for _, m := range members {
		total += int(m.Score)
	}
	return total, nil
}
