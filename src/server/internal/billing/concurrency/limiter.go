package concurrency

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrConcurrencyLimitExceeded = errors.New("concurrency: limit exceeded")

// Limiter 分布式并发控制器，基于 Redis 计数器
type Limiter struct {
	rdb        *redis.Client
	keyPrefix  string
	maxConcur  int
	ttl        time.Duration
}

// NewLimiter 创建并发控制器
// keyPrefix: Redis key 前缀
// maxConcur: 最大并发数
// ttl: 计数器过期时间（防止泄漏）
func NewLimiter(rdb *redis.Client, keyPrefix string, maxConcur int, ttl time.Duration) *Limiter {
	return &Limiter{
		rdb:       rdb,
		keyPrefix: keyPrefix,
		maxConcur: maxConcur,
		ttl:       ttl,
	}
}

func (l *Limiter) key(ownerID string) string {
	return fmt.Sprintf("%s:concurrency:%s", l.keyPrefix, ownerID)
}

// Acquire 获取许可，返回释放函数
// 如果超过并发限制返回 ErrConcurrencyLimitExceeded
func (l *Limiter) Acquire(ctx context.Context, ownerID string) (func() error, error) {
	key := l.key(ownerID)

	// Lua 脚本：原子性检查并递增
	script := redis.NewScript(`
		local key = KEYS[1]
		local max = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])
		local current = tonumber(redis.call('GET', key) or '0')
		if current >= max then
			return -1
		end
		redis.call('INCR', key)
		redis.call('EXPIRE', key, ttl)
		return current + 1
	`)

	result, err := script.Run(ctx, l.rdb, []string{key}, l.maxConcur, int(l.ttl.Seconds())).Int()
	if err != nil {
		return nil, fmt.Errorf("concurrency: acquire failed: %w", err)
	}
	if result == -1 {
		return nil, ErrConcurrencyLimitExceeded
	}

	released := false
	release := func() error {
		if released {
			return nil
		}
		released = true
		decrScript := redis.NewScript(`
			local key = KEYS[1]
			local current = tonumber(redis.call('GET', key) or '0')
			if current > 0 then
				redis.call('DECR', key)
			end
			return current
		`)
		return decrScript.Run(ctx, l.rdb, []string{key}).Err()
	}

	return release, nil
}

// Release 手动释放许可（供不使用 defer 的场景）
func (l *Limiter) Release(ctx context.Context, ownerID string) error {
	key := l.key(ownerID)
	decrScript := redis.NewScript(`
		local key = KEYS[1]
		local current = tonumber(redis.call('GET', key) or '0')
		if current > 0 then
			redis.call('DECR', key)
		end
		return current
	`)
	return decrScript.Run(ctx, l.rdb, []string{key}).Err()
}

// CurrentCount 获取当前并发数
func (l *Limiter) CurrentCount(ctx context.Context, ownerID string) (int, error) {
	key := l.key(ownerID)
	val, err := l.rdb.Get(ctx, key).Int()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return val, nil
}
