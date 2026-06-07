package balancer

import (
	"sync"

	"oblivious/server/internal/relay/types"
)

// WeightedRoundRobin 加权轮询负载均衡器
// 根据 channel 的 Weight 字段分配流量，权重越高被选中概率越大
type WeightedRoundRobin struct {
	mu           sync.Mutex
	totalWeight  int
	channels     []*types.WeightedChannel
	schedule     []*types.WeightedChannel
	nextSchedule int
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
	wrr.schedule = weightedChannelSchedule(wrr.channels)
	return wrr
}

// Select 根据加权轮询选择渠道
func (w *WeightedRoundRobin) Select() *types.WeightedChannel {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.schedule) == 0 {
		return nil
	}

	if w.nextSchedule >= len(w.schedule) {
		w.nextSchedule = 0
	}
	ch := w.schedule[w.nextSchedule]
	w.nextSchedule++
	return ch
}

// UpdateChannels 更新渠道列表（例如健康状态变化后）
func (w *WeightedRoundRobin) UpdateChannels(channels []*types.WeightedChannel) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.channels = make([]*types.WeightedChannel, 0, len(channels))
	w.totalWeight = 0
	w.nextSchedule = 0
	for _, ch := range channels {
		if ch.Enabled && ch.Healthy {
			w.channels = append(w.channels, ch)
			w.totalWeight += ch.StaticWeight
		}
	}
	w.schedule = weightedChannelSchedule(w.channels)
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

func weightedChannelSchedule(channels []*types.WeightedChannel) []*types.WeightedChannel {
	totalWeight := 0
	for _, ch := range channels {
		if ch.StaticWeight > 0 {
			totalWeight += ch.StaticWeight
		}
	}

	if totalWeight <= 0 {
		schedule := make([]*types.WeightedChannel, len(channels))
		copy(schedule, channels)
		return schedule
	}

	schedule := make([]*types.WeightedChannel, 0, totalWeight)
	for _, ch := range channels {
		for i := 0; i < ch.StaticWeight; i++ {
			schedule = append(schedule, ch)
		}
	}
	return schedule
}
