package health

import (
	"context"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"oblivious/server/internal/relay/types"
)

// Checker 定期健康检查器
// 定期检查所有渠道健康状态，自动摘除/恢复，维护 health_score
type Checker struct {
	mu              sync.RWMutex
	channels        map[string]*channelHealth  // channel_id -> health
	httpClient      *http.Client
	checkInterval   time.Duration
	unhealthyThresh int           // 连续失败多少次后摘除
	recoveryThresh  int           // 连续成功多少次后恢复
	onRemove        func(channelID string)
	onRecover       func(channelID string)
	stopCh          chan struct{}
}

type channelHealth struct {
	channel         *types.Channel
	healthScore     *types.HealthScore
	consecutiveFail int
	consecutiveOK   int
	removed         bool
}

// NewChecker 创建健康检查器
func NewChecker(checkInterval time.Duration, unhealthyThresh, recoveryThresh int) *Checker {
	return &Checker{
		channels:        make(map[string]*channelHealth),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: true,
				MaxIdleConns:      1,
			},
		},
		checkInterval:   checkInterval,
		unhealthyThresh: unhealthyThresh,
		recoveryThresh:  recoveryThresh,
		stopCh:          make(chan struct{}),
	}
}

// SetOnRemove 设置摘除回调
func (c *Checker) SetOnRemove(fn func(channelID string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onRemove = fn
}

// SetOnRecover 设置恢复回调
func (c *Checker) SetOnRecover(fn func(channelID string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onRecover = fn
}

// Register 注册渠道
func (c *Checker) Register(ch *types.Channel) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.channels[ch.ID] = &channelHealth{
		channel: ch,
		healthScore: &types.HealthScore{
			ChannelID: ch.ID,
			Score:     100.0,
		},
	}
}

// Unregister 注销渠道
func (c *Checker) Unregister(channelID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.channels, channelID)
}

// Start 启动定期健康检查
func (c *Checker) Start(ctx context.Context) {
	go c.run(ctx)
}

// Stop 停止健康检查
func (c *Checker) Stop() {
	close(c.stopCh)
}

// GetHealthScore 获取渠道健康分
func (c *Checker) GetHealthScore(channelID string) *types.HealthScore {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ch, ok := c.channels[channelID]
	if !ok {
		return nil
	}
	// 返回副本
	score := *ch.healthScore
	return &score
}

// GetAllHealthScores 获取所有渠道健康分
func (c *Checker) GetAllHealthScores() map[string]*types.HealthScore {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*types.HealthScore, len(c.channels))
	for id, ch := range result {
		result[id] = ch
	}
	// 重新填充
	for id, ch := range c.channels {
		score := *ch.healthScore
		result[id] = &score
	}
	return result
}

// RecordRequestResult 记录请求结果（被动健康更新）
func (c *Checker) RecordRequestResult(channelID string, latencyMs float64, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch, ok := c.channels[channelID]
	if !ok {
		return
	}

	hs := ch.healthScore
	hs.TotalProbes++

	// 更新移动平均延迟（指数移动平均 alpha=0.3）
	if hs.AvgLatencyMs == 0 {
		hs.AvgLatencyMs = latencyMs
	} else {
		hs.AvgLatencyMs = 0.7*hs.AvgLatencyMs + 0.3*latencyMs
	}

	if success {
		hs.LastHealthy = true
		ch.consecutiveFail = 0
		ch.consecutiveOK++

		// 恢复检查
		if ch.removed && ch.consecutiveOK >= c.recoveryThresh {
			ch.removed = false
			ch.healthScore.RemovedAt = time.Time{}
			if c.onRecover != nil {
				go c.onRecover(channelID)
			}
		}
	} else {
		hs.FailedProbes++
		hs.LastHealthy = false
		ch.consecutiveOK = 0
		ch.consecutiveFail++

		// 摘除检查
		if !ch.removed && ch.consecutiveFail >= c.unhealthyThresh {
			ch.removed = true
			ch.healthScore.RemovedAt = time.Now()
			if c.onRemove != nil {
				go c.onRemove(channelID)
			}
		}
	}

	hs.LastProbeTime = time.Now()

	// 重新计算健康分
	hs.Score = c.computeHealthScore(hs)
}

// IsRemoved 检查渠道是否已被摘除
func (c *Checker) IsRemoved(channelID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ch, ok := c.channels[channelID]
	if !ok {
		return false
	}
	return ch.removed
}

// ForceRecover 强制恢复渠道
func (c *Checker) ForceRecover(channelID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch, ok := c.channels[channelID]
	if !ok {
		return
	}

	ch.removed = false
	ch.consecutiveFail = 0
	ch.consecutiveOK = 0
	ch.healthScore.RemovedAt = time.Time{}
	ch.healthScore.Score = 50.0 // 恢复后初始分 50
}

func (c *Checker) run(ctx context.Context) {
	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.checkAll(ctx)
		}
	}
}

func (c *Checker) checkAll(ctx context.Context) {
	c.mu.RLock()
	snapshot := make(map[string]*channelHealth, len(c.channels))
	for id, ch := range c.channels {
		snapshot[id] = ch
	}
	c.mu.RUnlock()

	for _, ch := range snapshot {
		if ch.channel.HealthCheckStrategy == "disabled" {
			continue
		}

		probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		healthy, latency := c.probe(probeCtx, ch.channel)
		cancel()

		latencyMs := float64(latency.Milliseconds())
		c.RecordRequestResult(ch.channel.ID, latencyMs, healthy)
	}
}

func (c *Checker) probe(ctx context.Context, ch *types.Channel) (bool, time.Duration) {
	if ch.BaseURL == "" {
		return false, 0
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ch.BaseURL+"/v1/models", nil)
	if err != nil {
		return false, 0
	}
	if ch.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+ch.APIKey)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		log.Printf("health probe error for %s: %v", ch.ID, err)
		return false, latency
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, latency
}

// computeHealthScore 计算健康分
// score = 100 * (1 - error_rate) * latency_factor
// latency_factor = min(1, 500 / latency_ms)
func (c *Checker) computeHealthScore(hs *types.HealthScore) float64 {
	if hs.TotalProbes == 0 {
		return 100.0
	}

	errorRate := float64(hs.FailedProbes) / float64(hs.TotalProbes)

	latencyFactor := 1.0
	if hs.AvgLatencyMs > 0 {
		latencyFactor = math.Min(1.0, 500.0/hs.AvgLatencyMs)
	}

	score := 100.0 * (1.0 - errorRate) * latencyFactor

	// 最低分 0，最高分 100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}
