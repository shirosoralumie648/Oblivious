package relay

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"oblivious/server/internal/relay/types"
)

const defaultModelRoute = ""

type LoadBalancer struct {
	pool           *ChannelPool
	strategy       string
	mu             sync.Mutex
	random         loadBalancerRandom
	weightedCursor int
}

type loadBalancerRandom interface {
	Float64() float64
	Intn(n int) int
}

func NewLoadBalancer(pool *ChannelPool, strategy string) *LoadBalancer {
	return &LoadBalancer{
		pool:     pool,
		strategy: strategy,
		random:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (lb *LoadBalancer) Select(apiType string) *types.RouteChannel {
	return lb.SelectModel(apiType, defaultModelRoute)
}

func (lb *LoadBalancer) SelectModel(apiType, model string) *types.RouteChannel {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	candidates := lb.filterHealthy(apiType, model)
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
	case "adaptive":
		return lb.adaptiveSelect(candidates)
	default:
		return lb.weightedSelect(candidates)
	}
}

func (lb *LoadBalancer) SelectExcluding(apiType string, excluded map[string]bool) *types.RouteChannel {
	return lb.SelectModelExcluding(apiType, defaultModelRoute, excluded)
}

func (lb *LoadBalancer) SelectModelExcluding(apiType, model string, excluded map[string]bool) *types.RouteChannel {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	candidates := lb.filterHealthy(apiType, model)
	if len(excluded) > 0 {
		filtered := candidates[:0]
		for _, ch := range candidates {
			if !excluded[routeChannelID(ch)] {
				filtered = append(filtered, ch)
			}
		}
		candidates = filtered
	}
	if len(candidates) == 0 {
		return nil
	}
	return lb.selectFromCandidates(candidates)
}

func (lb *LoadBalancer) SelectExcludingWithWeights(apiType string, excluded map[string]bool, adjuster func(*types.RouteChannel) int) *types.RouteChannel {
	return lb.SelectModelExcludingWithWeights(apiType, defaultModelRoute, excluded, adjuster)
}

func (lb *LoadBalancer) SelectModelExcludingWithWeights(apiType, model string, excluded map[string]bool, adjuster func(*types.RouteChannel) int) *types.RouteChannel {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	candidates := lb.filterHealthy(apiType, model)
	if len(excluded) > 0 {
		filtered := candidates[:0]
		for _, ch := range candidates {
			if !excluded[routeChannelID(ch)] {
				filtered = append(filtered, ch)
			}
		}
		candidates = filtered
	}
	if len(candidates) == 0 {
		return nil
	}
	candidates = adjustedCandidateWeights(candidates, adjuster)
	return lb.selectFromCandidates(candidates)
}

func (lb *LoadBalancer) SelectChannelByID(apiType, channelID string) *types.RouteChannel {
	return lb.SelectModelChannelByID(apiType, defaultModelRoute, channelID)
}

func (lb *LoadBalancer) SelectModelChannelByID(apiType, model, channelID string) *types.RouteChannel {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for _, ch := range lb.filterHealthy(apiType, model) {
		if routeChannelID(ch) == channelID {
			return ch
		}
	}
	return nil
}

func (lb *LoadBalancer) filterHealthy(apiType, model string) []*types.RouteChannel {
	_ = apiType
	routeChannels := lb.modelRouteChannels(model)
	if len(routeChannels) == 0 && model != defaultModelRoute {
		routeChannels = lb.modelRouteChannels(defaultModelRoute)
	}
	if len(routeChannels) == 0 {
		// Fall back to listing all channels and filtering
		channels := lb.pool.ListChannels()
		for _, ch := range channels {
			if ch.Enabled {
				routeChannels = append(routeChannels, &types.RouteChannel{
					Channel:   ch,
					ChannelID: ch.ID,
					Weight:    1,
					Enabled:   ch.Enabled,
				})
			}
		}
	}
	var result []*types.RouteChannel
	now := time.Now()
	for _, ch := range routeChannels {
		stats, _ := lb.pool.GetStats(routeChannelID(ch))
		if stats != nil && !stats.RateLimitedUntil.IsZero() && now.Before(stats.RateLimitedUntil) {
			continue
		}
		if stats != nil && stats.Invalid {
			continue
		}
		if stats != nil && stats.Forbidden {
			continue
		}
		if ch.Healthy {
			result = append(result, ch)
		}
	}
	return result
}

func (lb *LoadBalancer) modelRouteChannels(model string) []*types.RouteChannel {
	routeChannels := lb.pool.GetChannelsByModel(model)
	if len(routeChannels) == 0 {
		return nil
	}
	result := make([]*types.RouteChannel, 0, len(routeChannels))
	for _, ch := range routeChannels {
		if ch == nil {
			continue
		}
		if ch.Channel == nil && ch.ChannelID != "" {
			if channel, ok := lb.pool.GetChannel(ch.ChannelID); ok {
				copyCh := *ch
				copyCh.Channel = channel
				ch = &copyCh
			}
		}
		result = append(result, ch)
	}
	return result
}

func adjustedCandidateWeights(candidates []*types.RouteChannel, adjuster func(*types.RouteChannel) int) []*types.RouteChannel {
	if adjuster == nil || len(candidates) == 0 {
		return candidates
	}
	adjusted := make([]*types.RouteChannel, 0, len(candidates))
	for _, ch := range candidates {
		if ch == nil {
			continue
		}
		copyCh := *ch
		copyCh.Weight = adjuster(ch)
		adjusted = append(adjusted, &copyCh)
	}
	return adjusted
}

func (lb *LoadBalancer) selectFromCandidates(candidates []*types.RouteChannel) *types.RouteChannel {
	switch lb.strategy {
	case "weighted":
		return lb.weightedSelect(candidates)
	case "priority":
		return lb.prioritySelect(candidates)
	case "cost_aware":
		return lb.costAwareSelect(candidates)
	case "adaptive":
		return lb.adaptiveSelect(candidates)
	default:
		return lb.weightedSelect(candidates)
	}
}

func (lb *LoadBalancer) weightedSelect(channels []*types.RouteChannel) *types.RouteChannel {
	totalWeight := 0
	for _, ch := range channels {
		if ch.Weight > 0 {
			totalWeight += ch.Weight
		}
	}
	if totalWeight <= 0 {
		if lb.weightedCursor >= len(channels) {
			lb.weightedCursor = 0
		}
		ch := channels[lb.weightedCursor]
		lb.weightedCursor++
		return ch
	}

	position := lb.weightedCursor % totalWeight
	lb.weightedCursor++
	cumulative := 0
	for _, ch := range channels {
		if ch.Weight <= 0 {
			continue
		}
		cumulative += ch.Weight
		if position < cumulative {
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
	r := lb.random.Float64() * totalInverse
	cumulative := 0.0
	for i, ch := range channels {
		cumulative += weights[i]
		if r < cumulative {
			return ch
		}
	}
	return channels[len(channels)-1]
}

func (lb *LoadBalancer) adaptiveSelect(channels []*types.RouteChannel) *types.RouteChannel {
	totalWeight := 0.0
	weights := make([]float64, len(channels))
	for i, ch := range channels {
		weight := lb.adaptiveWeight(ch)
		weights[i] = weight
		totalWeight += weight
	}
	if totalWeight <= 0 {
		return lb.weightedSelect(channels)
	}
	r := lb.random.Float64() * totalWeight
	cumulative := 0.0
	for i, ch := range channels {
		cumulative += weights[i]
		if r < cumulative {
			return ch
		}
	}
	return channels[len(channels)-1]
}

func (lb *LoadBalancer) adaptiveWeight(ch *types.RouteChannel) float64 {
	staticWeight := ch.Weight
	if staticWeight <= 0 {
		staticWeight = 1
	}
	healthScore := 100.0
	avgLatencyMs := 100.0
	errorRate := 0.0
	stats, ok := lb.pool.GetStats(routeChannelID(ch))
	if ok && stats != nil {
		windowTotal, windowFailures, windowLatencyUs := adaptiveRuntimeWindowStats(stats, time.Now().UTC())
		if windowTotal > 0 {
			errorRate = float64(windowFailures) / float64(windowTotal)
			healthScore = 100 * (1 - errorRate)
			avgLatencyMs = float64(windowLatencyUs) / float64(windowTotal) / 1000
		} else {
			total := stats.SuccessCount + stats.FailureCount
			if total > 0 {
				errorRate = float64(stats.FailureCount) / float64(total)
				healthScore = 100 * (1 - errorRate)
			}
			if stats.LatencyCount > 0 {
				avgLatencyMs = float64(stats.LatencySumUs) / float64(stats.LatencyCount) / 1000
			}
		}
		if avgLatencyMs > 0 {
			healthScore *= math.Min(1, 200/avgLatencyMs)
		}
	}
	logLatency := math.Log2(avgLatencyMs + 1)
	if logLatency <= 0 {
		logLatency = 1
	}
	weight := float64(staticWeight) * (healthScore / 100) * (1 - errorRate) / logLatency
	if weight < 0 {
		return 0
	}
	return weight
}

func adaptiveRuntimeWindowStats(stats *types.ChannelStats, now time.Time) (total int64, failures int64, latencyUs int64) {
	if stats == nil {
		return 0, 0, 0
	}
	cutoff := now.Add(-adaptiveRuntimeWindow)
	for _, sample := range stats.RuntimeSamples {
		if sample.At.IsZero() || !sample.At.After(cutoff) {
			continue
		}
		total++
		if !sample.Success {
			failures++
		}
		if sample.LatencyUs >= 0 {
			latencyUs += sample.LatencyUs
		}
	}
	return total, failures, latencyUs
}
