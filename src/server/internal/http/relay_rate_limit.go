package http

import (
	"context"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"

	"oblivious/server/internal/config"
	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/relay/ratelimit"
	"oblivious/server/internal/relay/types"
)

type relayUsageLimitService interface {
	ResolveUsageLimit(ctx context.Context, organizationID, userID string) (quota.UsageLimit, error)
}

func buildRelayRateLimiter(cfg config.Config) (ratelimit.RateLimiter, func() error) {
	switch strings.ToLower(strings.TrimSpace(cfg.RelayRateLimitBackend)) {
	case "", "memory":
		return ratelimit.NewInMemoryRateLimiter(ratelimit.InMemoryOptions{}), nil
	case "redis":
		addr := strings.TrimSpace(cfg.RedisAddr)
		if addr == "" {
			addr = "localhost:6379"
		}
		client := redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		limiter := ratelimit.NewRedisRateLimiter(client, ratelimit.RedisOptions{
			KeyPrefix: cfg.RelayRateLimitRedisKeyPrefix,
		})
		return limiter, client.Close
	default:
		return nil, nil
	}
}

func buildRelayUsageLimitResolver(service relayUsageLimitService) relay.RateLimitResolver {
	return func(ctx context.Context, channel *types.RouteChannel, model string, usage *types.Usage) relay.RateLimitResolution {
		channelCheck := relayChannelRateLimitCheck(channel, model, usage)
		if service == nil {
			return relayRateLimitResolutionFromCheck(channelCheck)
		}
		organizationID, _ := types.TrustedOrganizationIDFromContext(ctx)
		userID, _ := types.TrustedUserIDFromContext(ctx)
		organizationID = strings.TrimSpace(organizationID)
		userID = strings.TrimSpace(userID)
		if organizationID == "" {
			return relayRateLimitResolutionFromCheck(channelCheck)
		}

		limit, err := service.ResolveUsageLimit(ctx, organizationID, userID)
		if err != nil {
			log.Printf("warning: failed to resolve relay usage limit for organization %q user %q: %v", organizationID, userID, err)
			return relayRateLimitResolutionFromCheck(channelCheck)
		}
		resolution := relay.RateLimitResolution{
			Limits: ratelimit.Limits{
				MaxConcurrent: limit.MaxConcurrentRequests,
				TPM:           limit.MaxTokensPerWindow,
			},
			Key: relayUsageLimitRateKey(limit, organizationID),
		}
		if !relayRateLimitCheckEmpty(channelCheck) {
			resolution.Additional = append(resolution.Additional, channelCheck)
		}
		return resolution
	}
}

func relayChannelRateLimitCheck(channel *types.RouteChannel, model string, usage *types.Usage) relay.RateLimitCheck {
	limits := ratelimit.Limits{}
	if channel != nil && channel.Channel != nil {
		limits.RPM = channel.Channel.RPMLimit
		limits.TPM = channel.Channel.TPMLimit
	}
	tokens := 0
	if usage != nil {
		tokens = usage.TotalTokens
	}
	return relay.RateLimitCheck{
		Key: ratelimit.Key{
			ChannelID: relayRateLimitChannelID(channel),
			Model:     model,
		},
		Limits: limits,
		Usage:  ratelimit.Usage{Tokens: tokens},
	}
}

func relayRateLimitResolutionFromCheck(check relay.RateLimitCheck) relay.RateLimitResolution {
	return relay.RateLimitResolution{
		Key:    check.Key,
		Limits: check.Limits,
		Usage:  check.Usage,
	}
}

func relayRateLimitCheckEmpty(check relay.RateLimitCheck) bool {
	return check.Limits.RPM <= 0 && check.Limits.TPM <= 0 && check.Limits.MaxConcurrent <= 0
}

func relayRateLimitChannelID(channel *types.RouteChannel) string {
	if channel == nil {
		return ""
	}
	if strings.TrimSpace(channel.ChannelID) != "" {
		return strings.TrimSpace(channel.ChannelID)
	}
	if channel.Channel != nil {
		return strings.TrimSpace(channel.Channel.ID)
	}
	return ""
}

func relayUsageLimitRateKey(limit quota.UsageLimit, fallbackOrganizationID string) ratelimit.Key {
	organizationID := strings.TrimSpace(limit.OrganizationID)
	if organizationID == "" {
		organizationID = strings.TrimSpace(fallbackOrganizationID)
	}
	tokenID := strings.TrimSpace(limit.UserID)
	if tokenID == "" {
		tokenID = organizationID
	}
	return ratelimit.Key{
		ChannelID: "quota",
		Model:     organizationID,
		TokenID:   tokenID,
	}
}
