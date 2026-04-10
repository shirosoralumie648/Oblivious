package relay

import (
	"sync"

	"oblivious/server/internal/relay/types"
)

// ChannelPool 渠道池（内存缓存）
type ChannelPool struct {
	mu       sync.RWMutex
	channels map[string]*types.Channel       // channel_id -> channel
	routes   map[string]*types.ModelRoute  // model -> route
	stats    map[string]*types.ChannelStats // channel_id -> runtime stats
}

// Compile-time interface check
var _ types.ChannelPoolInterface = (*ChannelPool)(nil)

// NewChannelPool 创建空池
func NewChannelPool() *ChannelPool {
	return &ChannelPool{
		channels: make(map[string]*types.Channel),
		routes:   make(map[string]*types.ModelRoute),
		stats:    make(map[string]*types.ChannelStats),
	}
}

// GetChannel 根据 ID 获取渠道
func (p *ChannelPool) GetChannel(id string) (*types.Channel, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ch, ok := p.channels[id]
	return ch, ok
}

// GetChannelsByModel 根据模型获取渠道路由
func (p *ChannelPool) GetChannelsByModel(model string) []*types.RouteChannel {
	p.mu.RLock()
	defer p.mu.RUnlock()
	route, ok := p.routes[model]
	if !ok {
		return nil
	}
	result := make([]*types.RouteChannel, len(route.Channels))
	for i := range route.Channels {
		result[i] = &route.Channels[i]
	}
	return result
}

// GetStats 获取渠道运行时统计
func (p *ChannelPool) GetStats(channelID string) (*types.ChannelStats, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	stats, ok := p.stats[channelID]
	return stats, ok
}

// UpdateChannel 更新渠道配置
func (p *ChannelPool) UpdateChannel(ch *types.Channel) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.channels[ch.ID] = ch
	if p.stats[ch.ID] == nil {
		p.stats[ch.ID] = &types.ChannelStats{ChannelID: ch.ID}
	}
}

// UpdateRoute 更新模型路由
func (p *ChannelPool) UpdateRoute(route *types.ModelRoute) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.routes[route.Model] = route
}

// ListChannels 列出所有渠道
func (p *ChannelPool) ListChannels() []*types.Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*types.Channel, 0, len(p.channels))
	for _, ch := range p.channels {
		result = append(result, ch)
	}
	return result
}

// SetChannelHealthy 设置渠道健康状态
func (p *ChannelPool) SetChannelHealthy(channelID string, healthy bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, rc := range p.routes {
		for i := range rc.Channels {
			if rc.Channels[i].ChannelID == channelID {
				rc.Channels[i].Healthy = healthy
			}
		}
	}
}

// GetAllStats 获取所有渠道统计
func (p *ChannelPool) GetAllStats() map[string]*types.ChannelStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make(map[string]*types.ChannelStats, len(p.stats))
	for k, v := range p.stats {
		result[k] = v
	}
	return result
}
