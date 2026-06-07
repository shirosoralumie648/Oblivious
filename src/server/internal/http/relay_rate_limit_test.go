package http

import (
	"context"
	"testing"

	"oblivious/server/internal/config"
	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/relay/ratelimit"
	"oblivious/server/internal/relay/types"
)

func TestBuildRelayRateLimiterCreatesMemoryLimiterByDefault(t *testing.T) {
	limiter, closeLimiter := buildRelayRateLimiter(config.Config{})
	if _, ok := limiter.(*ratelimit.InMemoryRateLimiter); !ok {
		t.Fatalf("expected in-memory rate limiter by default, got %T", limiter)
	}
	if closeLimiter != nil {
		t.Fatal("expected nil close function for in-memory limiter")
	}
}

func TestBuildRelayRateLimiterDisabledWhenBackendIsNone(t *testing.T) {
	limiter, closeLimiter := buildRelayRateLimiter(config.Config{RelayRateLimitBackend: "none"})
	if limiter != nil {
		t.Fatalf("expected nil limiter when backend is none, got %T", limiter)
	}
	if closeLimiter != nil {
		t.Fatal("expected nil close function for disabled limiter")
	}
}

func TestBuildRelayRateLimiterCreatesMemoryLimiter(t *testing.T) {
	limiter, closeLimiter := buildRelayRateLimiter(config.Config{RelayRateLimitBackend: "memory"})
	if _, ok := limiter.(*ratelimit.InMemoryRateLimiter); !ok {
		t.Fatalf("expected in-memory rate limiter, got %T", limiter)
	}
	if closeLimiter != nil {
		t.Fatal("expected nil close function for in-memory limiter")
	}
}

func TestBuildRelayRateLimiterCreatesRedisLimiter(t *testing.T) {
	limiter, closeLimiter := buildRelayRateLimiter(config.Config{
		RelayRateLimitBackend:        "redis",
		RedisAddr:                    "127.0.0.1:6380",
		RedisPassword:                "redis-secret",
		RedisDB:                      2,
		RelayRateLimitRedisKeyPrefix: "tenant:relay:limit",
	})
	if _, ok := limiter.(*ratelimit.RedisRateLimiter); !ok {
		t.Fatalf("expected redis rate limiter, got %T", limiter)
	}
	if closeLimiter == nil {
		t.Fatal("expected redis limiter close function")
	}
	if err := closeLimiter(); err != nil {
		t.Fatalf("close redis limiter: %v", err)
	}
}

func TestBuildRelayUsageLimitResolverUsesResolvedQuotaScope(t *testing.T) {
	service := &fakeRelayUsageLimitService{
		limit: quota.UsageLimit{
			OrganizationID:        "org_1",
			MaxConcurrentRequests: 2,
			MaxTokensPerWindow:    500,
		},
	}
	resolver := buildRelayUsageLimitResolver(service)
	ctx := types.WithTrustedOrganizationID(types.WithTrustedUserID(context.Background(), "user_2"), "org_1")

	resolution := resolver(ctx, nil, "gpt-4o", &types.Usage{PromptTokens: 25})

	if service.organizationID != "org_1" || service.userID != "user_2" {
		t.Fatalf("expected resolver to use trusted identity, got org=%q user=%q", service.organizationID, service.userID)
	}
	if resolution.Limits.MaxConcurrent != 2 || resolution.Limits.TPM != 500 {
		t.Fatalf("unexpected resolver limits: %+v", resolution.Limits)
	}
	if resolution.Key != (ratelimit.Key{ChannelID: "quota", Model: "org_1", TokenID: "org_1"}) {
		t.Fatalf("expected organization-scoped quota key, got %+v", resolution.Key)
	}

	service.limit.UserID = "user_2"
	resolution = resolver(ctx, nil, "gpt-4o", nil)
	if resolution.Key != (ratelimit.Key{ChannelID: "quota", Model: "org_1", TokenID: "user_2"}) {
		t.Fatalf("expected user-scoped quota key, got %+v", resolution.Key)
	}
}

func TestBuildRelayUsageLimitResolverKeepsChannelRateLimits(t *testing.T) {
	service := &fakeRelayUsageLimitService{
		limit: quota.UsageLimit{
			OrganizationID:        "org_1",
			MaxConcurrentRequests: 2,
			MaxTokensPerWindow:    500,
		},
	}
	resolver := buildRelayUsageLimitResolver(service)
	ctx := types.WithTrustedOrganizationID(types.WithTrustedUserID(context.Background(), "user_2"), "org_1")
	channel := &types.RouteChannel{
		ChannelID: "channel_1",
		Channel: &types.Channel{
			RPMLimit: 60,
			TPMLimit: 1000,
		},
	}

	resolution := resolver(ctx, channel, "gpt-4o", &types.Usage{TotalTokens: 25})

	if resolution.Key != (ratelimit.Key{ChannelID: "quota", Model: "org_1", TokenID: "org_1"}) {
		t.Fatalf("expected quota-scoped primary key, got %+v", resolution.Key)
	}
	if resolution.Limits.MaxConcurrent != 2 || resolution.Limits.TPM != 500 || resolution.Limits.RPM != 0 {
		t.Fatalf("unexpected quota-scoped primary limits: %+v", resolution.Limits)
	}
	if len(resolution.Additional) != 1 {
		t.Fatalf("expected one channel-scoped additional limit check, got %d", len(resolution.Additional))
	}
	channelCheck := resolution.Additional[0]
	if channelCheck.Key != (ratelimit.Key{ChannelID: "channel_1", Model: "gpt-4o"}) {
		t.Fatalf("expected channel-scoped key, got %+v", channelCheck.Key)
	}
	if channelCheck.Limits.RPM != 60 || channelCheck.Limits.TPM != 1000 || channelCheck.Limits.MaxConcurrent != 0 {
		t.Fatalf("unexpected channel-scoped limits: %+v", channelCheck.Limits)
	}
	if channelCheck.Usage.Tokens != 25 {
		t.Fatalf("expected channel usage tokens 25, got %d", channelCheck.Usage.Tokens)
	}
}

type fakeRelayUsageLimitService struct {
	limit          quota.UsageLimit
	organizationID string
	userID         string
}

func (s *fakeRelayUsageLimitService) ResolveUsageLimit(ctx context.Context, organizationID, userID string) (quota.UsageLimit, error) {
	s.organizationID = organizationID
	s.userID = userID
	return s.limit, nil
}

var _ relayUsageLimitService = (*fakeRelayUsageLimitService)(nil)
var _ relay.RateLimitResolver = buildRelayUsageLimitResolver((*fakeRelayUsageLimitService)(nil))
