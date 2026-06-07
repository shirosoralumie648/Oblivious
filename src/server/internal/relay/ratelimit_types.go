package relay

import (
	"context"

	"oblivious/server/internal/relay/ratelimit"
	"oblivious/server/internal/relay/types"
)

type RateLimitResolution struct {
	Key        ratelimit.Key
	Limits     ratelimit.Limits
	Usage      ratelimit.Usage
	Additional []RateLimitCheck
}

type RateLimitCheck struct {
	Key    ratelimit.Key
	Limits ratelimit.Limits
	Usage  ratelimit.Usage
}

type RateLimitResolver func(ctx context.Context, channel *types.RouteChannel, model string, usage *types.Usage) RateLimitResolution
