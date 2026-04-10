package relay

import (
	"math/rand"
	"sync"

	"oblivious/server/internal/relay/types"
)

type LoadBalancer struct {
	pool     *ChannelPool
	strategy string
	mu       sync.Mutex
}

func NewLoadBalancer(pool *ChannelPool, strategy string) *LoadBalancer {
	return &LoadBalancer{
		pool:     pool,
		strategy: strategy,
	}
}

func (lb *LoadBalancer) Select(apiType string) *types.RouteChannel {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	candidates := lb.filterHealthy(apiType)
	if len(candidates) == 0 {
		return nil
	}

	switch lb.strategy {
	case "weighted":
		return lb.weightedSelect(candidates)
	case "priority":
		return lb.prioritySelect(candidates)
	case "cost_aware":
		return lb.costAwareSelect(candidates)
	default:
		return lb.weightedSelect(candidates)
	}
}

func (lb *LoadBalancer) filterHealthy(apiType string) []*types.RouteChannel {
	// apiType maps to a model route, but all API types share channel pool
	// Use ListChannels and construct RouteChannel list from default route
	routeChannels := lb.pool.GetChannelsByModel("")
	if len(routeChannels) == 0 {
		// Fall back to listing all channels and filtering
		channels := lb.pool.ListChannels()
		for _, ch := range channels {
			if ch.Enabled {
				routeChannels = append(routeChannels, &types.RouteChannel{
					Channel:    ch,
					ChannelID: ch.ID,
					Weight:    1,
					Enabled:   ch.Enabled,
				})
			}
		}
	}
	var result []*types.RouteChannel
	for _, ch := range routeChannels {
		if ch.Healthy {
			result = append(result, ch)
		}
	}
	return result
}

func (lb *LoadBalancer) weightedSelect(channels []*types.RouteChannel) *types.RouteChannel {
	totalWeight := 0
	for _, ch := range channels {
		totalWeight += ch.Weight
	}
	if totalWeight == 0 {
		return channels[len(channels)-1]
	}
	r := rand.Intn(totalWeight)
	cumulative := 0
	for _, ch := range channels {
		cumulative += ch.Weight
		if r < cumulative {
			return ch
		}
	}
	return channels[len(channels)-1]
}

func (lb *LoadBalancer) prioritySelect(channels []*types.RouteChannel) *types.RouteChannel {
	best := channels[0]
	for _, ch := range channels {
		if ch.Channel.Priority < best.Channel.Priority {
			best = ch
		}
	}
	return best
}

func (lb *LoadBalancer) costAwareSelect(channels []*types.RouteChannel) *types.RouteChannel {
	// Inverse probability proportional to cost
	totalInverse := 0.0
	weights := make([]float64, len(channels))
	for i, ch := range channels {
		cost := ch.EstimatedCostPer1K
		if cost <= 0 {
			cost = 1.0
		}
		weights[i] = 1.0 / cost
		totalInverse += weights[i]
	}
	r := rand.Float64() * totalInverse
	cumulative := 0.0
	for i, ch := range channels {
		cumulative += weights[i]
		if r < cumulative {
			return ch
		}
	}
	return channels[len(channels)-1]
}
