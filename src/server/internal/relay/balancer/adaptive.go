package balancer

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"oblivious/server/internal/relay/types"
)

// AdaptiveBalancer 自适应负载均衡器
// 根据 health_score, avg_latency_ms, error_rate 动态调整权重
// weight_dynamic = health_score * (1 - error_rate) / log2(latency_ms + 1)
type AdaptiveBalancer struct {
	mu       sync.Mutex
	channels []*adaptiveChannel
}

type adaptiveChannel struct {
	channel      *types.Channel
	channelID    string
	staticWeight int
	healthScore  float64 // 0-100
	avgLatencyMs float64 // 移动平均延迟
	errorRate    float64 // 0-1
	healthy      bool
	enabled      bool
	// 用于计算移动平均
	recentLatencies []float64
	recentErrors    int
	recentTotal     int
	windowSize      int
}

// NewAdaptiveBalancer 创建自适应负载均衡器
func NewAdaptiveBalancer(channels []*types.WeightedChannel) *AdaptiveBalancer {
	ab := &AdaptiveBalancer{}
	ab.channels = make([]*adaptiveChannel, 0, len(channels))
	for _, ch := range channels {
		ac := &adaptiveChannel{
			channel:         ch.Channel,
			channelID:       ch.ChannelID,
			staticWeight:    ch.StaticWeight,
			healthScore:     100.0, // 初始健康分
			avgLatencyMs:    100.0, // 初始延迟估算
			errorRate:       0.0,
			healthy:         ch.Healthy,
			enabled:         ch.Enabled,
			recentLatencies: make([]float64, 0, 20),
			windowSize:      20,
		}
		ab.channels = append(ab.channels, ac)
	}
	return ab
}

// Select 根据动态权重选择渠道
func (ab *AdaptiveBalancer) Select() *types.WeightedChannel {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	available := ab.getAvailable()
	if len(available) == 0 {
		return nil
	}

	// 计算动态权重
	weights := make([]float64, len(available))
	totalWeight := 0.0
	for i, ch := range available {
		w := ab.computeDynamicWeight(ch)
		weights[i] = w
		totalWeight += w
	}

	if totalWeight <= 0 {
		// 降级到均匀选择
		ch := available[rand.Intn(len(available))]
		return ab.toWeightedChannel(ch)
	}

	// 加权随机
	r := rand.Float64() * totalWeight
	cumulative := 0.0
	for i, ch := range available {
		cumulative += weights[i]
		if r < cumulative {
			return ab.toWeightedChannel(ch)
		}
	}

	return ab.toWeightedChannel(available[len(available)-1])
}

// RecordResult 记录请求结果，更新动态指标
func (ab *AdaptiveBalancer) RecordResult(channelID string, latencyMs float64, success bool) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	for _, ch := range ab.channels {
		if ch.channelID == channelID {
			// 更新滑动窗口
			if len(ch.recentLatencies) >= ch.windowSize {
				ch.recentLatencies = ch.recentLatencies[1:]
			}
			ch.recentLatencies = append(ch.recentLatencies, latencyMs)

			// 更新移动平均延迟
			sum := 0.0
			for _, lat := range ch.recentLatencies {
				sum += lat
			}
			ch.avgLatencyMs = sum / float64(len(ch.recentLatencies))

			// 更新错误率（滑动窗口）
			ch.recentTotal++
			if !success {
				ch.recentErrors++
			}
			if ch.recentTotal > ch.windowSize {
				// 简单衰减：每 windowSize 次请求衰减一半
				ch.recentTotal = ch.recentTotal / 2
				ch.recentErrors = ch.recentErrors / 2
			}
			if ch.recentTotal > 0 {
				ch.errorRate = float64(ch.recentErrors) / float64(ch.recentTotal)
			}

			// 更新健康分
			ch.healthScore = computeHealthScore(ch.avgLatencyMs, ch.errorRate)
			break
		}
	}
}

// SetChannelHealth 设置渠道健康状态
func (ab *AdaptiveBalancer) SetChannelHealth(channelID string, healthy bool) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	for _, ch := range ab.channels {
		if ch.channelID == channelID {
			ch.healthy = healthy
			break
		}
	}
}

// GetChannelMetrics 获取渠道指标快照
func (ab *AdaptiveBalancer) GetChannelMetrics() map[string]ChannelMetrics {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	result := make(map[string]ChannelMetrics, len(ab.channels))
	for _, ch := range ab.channels {
		result[ch.channelID] = ChannelMetrics{
			ChannelID:       ch.channelID,
			HealthScore:     ch.healthScore,
			AvgLatencyMs:    ch.avgLatencyMs,
			ErrorRate:       ch.errorRate,
			DynamicWeight:   ab.computeDynamicWeight(ch),
			Healthy:         ch.healthy,
		}
	}
	return result
}

// ChannelMetrics 渠道指标
type ChannelMetrics struct {
	ChannelID     string  `json:"channel_id"`
	HealthScore   float64 `json:"health_score"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	ErrorRate     float64 `json:"error_rate"`
	DynamicWeight float64 `json:"dynamic_weight"`
	Healthy       bool    `json:"healthy"`
}

// computeDynamicWeight 计算动态权重
// weight_dynamic = static_weight * health_score/100 * (1 - error_rate) / log2(avg_latency_ms + 1)
// 对数避免延迟过高时权重被过度惩罚
func (ab *AdaptiveBalancer) computeDynamicWeight(ch *adaptiveChannel) float64 {
	if !ch.healthy || !ch.enabled {
		return 0
	}

	logLatency := math.Log2(ch.avgLatencyMs + 1.0)
	if logLatency <= 0 {
		logLatency = 1.0
	}

	weight := float64(ch.staticWeight) * (ch.healthScore / 100.0) * (1.0 - ch.errorRate) / logLatency
	if weight < 0 {
		weight = 0
	}
	return weight
}

// computeHealthScore 计算健康分
// health_score = 100 * (1 - error_rate) * min(1, 200/latency_ms)
func computeHealthScore(avgLatencyMs, errorRate float64) float64 {
	latencyFactor := 1.0
	if avgLatencyMs > 0 {
		latencyFactor = math.Min(1.0, 200.0/avgLatencyMs)
	}
	score := 100.0 * (1.0 - errorRate) * latencyFactor
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

func (ab *AdaptiveBalancer) getAvailable() []*adaptiveChannel {
	var available []*adaptiveChannel
	for _, ch := range ab.channels {
		if ch.healthy && ch.enabled {
			available = append(available, ch)
		}
	}
	return available
}

func (ab *AdaptiveBalancer) toWeightedChannel(ac *adaptiveChannel) *types.WeightedChannel {
	return &types.WeightedChannel{
		Channel:       ac.channel,
		ChannelID:     ac.channelID,
		StaticWeight:  ac.staticWeight,
		DynamicWeight: ab.computeDynamicWeight(ac),
		Healthy:       ac.healthy,
		Enabled:       ac.enabled,
	}
}

// SetWindowSize 设置滑动窗口大小
func (ab *AdaptiveBalancer) SetWindowSize(size int) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	for _, ch := range ab.channels {
		ch.windowSize = size
	}
}

// Reset 清零指定渠道的计数器（用于恢复后重新统计）
func (ab *AdaptiveBalancer) Reset(channelID string) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	for _, ch := range ab.channels {
		if ch.channelID == channelID {
			ch.recentLatencies = ch.recentLatencies[:0]
			ch.recentErrors = 0
			ch.recentTotal = 0
			ch.errorRate = 0
			ch.healthScore = 100.0
			ch.avgLatencyMs = 100.0
			break
		}
	}
}

// unused but required for time import
var _ = time.Now
