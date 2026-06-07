package balancer

import (
	"math/rand"
	"sync"

	"oblivious/server/internal/relay/types"
)

// WeightedRoundRobin 加权轮询负载均衡器
// 根据 channel 的 Weight 字段分配流量，权重越高被选中概率越大
type WeightedRoundRobin struct {
	mu           sync.Mutex
	totalWeight  int
	channels     []*types.WeightedChannel
}

// NewWeightedRoundRobin 创建加权轮询负载均衡器
func NewWeightedRoundRobin(channels []*types.WeightedChannel) *WeightedRoundRobin {
	wrr := &WeightedRoundRobin{}
	wrr.channels = make([]*types.WeightedChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.Enabled && ch.Healthy {
			wrr.channels = append(wrr.channels, ch)
			wrr.totalWeight += ch.StaticWeight
		}
	}
	return wrr
}

// Select 根据加权随机选择渠道
func (w *WeightedRoundRobin) Select() *types.WeightedChannel {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.channels) == 0 {
		return nil
	}

	// 如果所有权重为 0，均匀随机
	if w.totalWeight == 0 {
		return w.channels[rand.Intn(len(w.channels))]
	}

	r := rand.Intn(w.totalWeight)
	cumulative := 0
	for _, ch := range w.channels {
		cumulative += ch.StaticWeight
		if r < cumulative {
			return ch
		}
	}

	return w.channels[len(w.channels)-1]
}

// UpdateChannels 更新渠道列表（例如健康状态变化后）
func (w *WeightedRoundRobin) UpdateChannels(channels []*types.WeightedChannel) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.channels = make([]*types.WeightedChannel, 0, len(channels))
	w.totalWeight = 0
	for _, ch := range channels {
		if ch.Enabled && ch.Healthy {
			w.channels = append(w.channels, ch)
			w.totalWeight += ch.StaticWeight
		}
	}
}

// Channels 返回当前可用渠道
func (w *WeightedRoundRobin) Channels() []*types.WeightedChannel {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make([]*types.WeightedChannel, len(w.channels))
	copy(result, w.channels)
	return result
}

// TotalWeight 返回当前总权重
func (w *WeightedRoundRobin) TotalWeight() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.totalWeight
}
