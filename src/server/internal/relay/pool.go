package relay

import (
	"sync"

	"oblivious/server/internal/relay/channel"
)

// ChannelPool 渠道池（内存缓存）
type ChannelPool struct {
	mu       sync.RWMutex
	channels map[string]*channel.Channel       // channel_id -> channel
	routes   map[string]*channel.ModelRoute    // model -> route
	stats    map[string]*channel.ChannelStats  // channel_id -> runtime stats
}

// NewChannelPool 创建空池
func NewChannelPool() *ChannelPool {
	return &ChannelPool{
		channels: make(map[string]*channel.Channel),
		routes:   make(map[string]*channel.ModelRoute),
		stats:    make(map[string]*channel.ChannelStats),
	}
}

// GetChannel 根据 ID 获取渠道
func (p *ChannelPool) GetChannel(id string) (*channel.Channel, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ch, ok := p.channels[id]
	return ch, ok
}

// GetChannelsByModel 根据模型获取渠道路由
func (p *ChannelPool) GetChannelsByModel(model string) []*channel.RouteChannel {
	p.mu.RLock()
	defer p.mu.RUnlock()
	route, ok := p.routes[model]
	if !ok {
		return nil
	}
	result := make([]*channel.RouteChannel, len(route.Channels))
	for i := range route.Channels {
		result[i] = &route.Channels[i]
	}
	return result
}

// GetStats 获取渠道运行时统计
func (p *ChannelPool) GetStats(channelID string) (*channel.ChannelStats, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	stats, ok := p.stats[channelID]
	return stats, ok
}

// UpdateChannel 更新渠道配置
func (p *ChannelPool) UpdateChannel(ch *channel.Channel) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.channels[ch.ID] = ch
	if p.stats[ch.ID] == nil {
		p.stats[ch.ID] = &channel.ChannelStats{ChannelID: ch.ID}
	}
}

// UpdateRoute 更新模型路由
func (p *ChannelPool) UpdateRoute(route *channel.ModelRoute) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.routes[route.Model] = route
}

// ListChannels 列出所有渠道
func (p *ChannelPool) ListChannels() []*channel.Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*channel.Channel, 0, len(p.channels))
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
		for _, c := range rc.Channels {
			if c.ChannelID == channelID {
				c.Healthy = healthy
			}
		}
	}
}

// GetAllStats 获取所有渠道统计
func (p *ChannelPool) GetAllStats() map[string]*channel.ChannelStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make(map[string]*channel.ChannelStats, len(p.stats))
	for k, v := range p.stats {
		result[k] = v
	}
	return result
}
