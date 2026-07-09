package relay

import (
	"strings"
	"sync"

	"oblivious/server/internal/relay/types"
)

// ChannelPool 渠道池（内存缓存）
type ChannelPool struct {
	mu       sync.RWMutex
	channels map[string]*types.Channel      // channel_id -> channel
	routes   map[string]*types.ModelRoute   // model -> route
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

func (p *ChannelPool) GetChannelForOrganization(id, organizationID string) (*types.Channel, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ch, ok := p.channels[id]
	if !ok || !channelMatchesOrganization(ch, organizationID) {
		return nil, false
	}
	return ch, true
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

func (p *ChannelPool) ReplaceConfig(channels []*types.Channel, routes []*types.ModelRoute) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.channels = make(map[string]*types.Channel, len(channels))
	p.routes = make(map[string]*types.ModelRoute, len(routes))
	p.stats = make(map[string]*types.ChannelStats, len(channels))
	for _, ch := range channels {
		if ch == nil || ch.ID == "" {
			continue
		}
		p.channels[ch.ID] = ch
		p.stats[ch.ID] = &types.ChannelStats{ChannelID: ch.ID}
	}
	for _, route := range routes {
		if route == nil {
			continue
		}
		p.routes[route.Model] = route
	}
}

// ListChannels 列出所有渠道
func (p *ChannelPool) ListChannels() []*types.Channel {
	return p.ListChannelsForOrganization("")
}

func (p *ChannelPool) ListChannelsForOrganization(organizationID string) []*types.Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*types.Channel, 0, len(p.channels))
	for _, ch := range p.channels {
		if !channelMatchesOrganization(ch, organizationID) {
			continue
		}
		result = append(result, ch)
	}
	return result
}

func channelMatchesOrganization(ch *types.Channel, organizationID string) bool {
	organizationID = strings.TrimSpace(organizationID)
	if organizationID == "" {
		return true
	}
	if ch == nil {
		return false
	}
	channelOrganizationID := strings.TrimSpace(ch.OrganizationID)
	return channelOrganizationID == "" || channelOrganizationID == organizationID
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

// AddChannel 添加渠道路由（用于测试和动态注册）
func (p *ChannelPool) AddChannel(ch *types.Channel, weight int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.channels[ch.ID] = ch
	if p.stats[ch.ID] == nil {
		p.stats[ch.ID] = &types.ChannelStats{ChannelID: ch.ID}
	}
	// 同时注册到 default 路由
	if p.routes[""] == nil {
		p.routes[""] = &types.ModelRoute{Model: "", Strategy: "weighted"}
	}
	p.routes[""].Channels = append(p.routes[""].Channels, types.RouteChannel{
		Channel:            ch,
		ChannelID:          ch.ID,
		Weight:             weight,
		Healthy:            ch.Enabled,
		Enabled:            ch.Enabled,
		EstimatedCostPer1K: ch.EstimatedCostPer1K,
		CostMultiplier:     ch.CostMultiplier,
	})
}
