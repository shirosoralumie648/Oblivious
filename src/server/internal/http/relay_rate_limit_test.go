package http

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/config"
	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/relay/ratelimit"
	"oblivious/server/internal/relay/types"
)

func TestBuildRelayRateLimiterCreatesMemoryLimiterByDefault(t *testing.T) {
	limiter, closeLimiter, err := buildRelayRateLimiter(config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := limiter.(*ratelimit.InMemoryRateLimiter); !ok {
		t.Fatalf("expected in-memory rate limiter by default, got %T", limiter)
	}
	if closeLimiter != nil {
		t.Fatal("expected nil close function for in-memory limiter")
	}
}

func TestBuildRelayRateLimiterDisabledWhenBackendIsNone(t *testing.T) {
	limiter, closeLimiter, err := buildRelayRateLimiter(config.Config{RelayRateLimitBackend: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if limiter != nil {
		t.Fatalf("expected nil limiter when backend is none, got %T", limiter)
	}
	if closeLimiter != nil {
		t.Fatal("expected nil close function for disabled limiter")
	}
}

func TestBuildRelayRateLimiterCreatesMemoryLimiter(t *testing.T) {
	limiter, closeLimiter, err := buildRelayRateLimiter(config.Config{RelayRateLimitBackend: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := limiter.(*ratelimit.InMemoryRateLimiter); !ok {
		t.Fatalf("expected in-memory rate limiter, got %T", limiter)
	}
	if closeLimiter != nil {
		t.Fatal("expected nil close function for in-memory limiter")
	}
}

func TestBuildRelayRateLimiterCreatesRedisLimiter(t *testing.T) {
	limiter, closeLimiter, err := buildRelayRateLimiter(config.Config{
		RelayRateLimitBackend:        "redis",
		RedisAddr:                    "127.0.0.1:6380",
		RedisPassword:                "redis-secret",
		RedisDB:                      2,
		RelayRateLimitRedisKeyPrefix: "tenant:relay:limit",
	})
	if err != nil {
		t.Fatal(err)
	}
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

func TestBuildRelayRateLimiterRedisTransportContract(t *testing.T) {
	testRelayRateLimiterRedisTransport(t, buildRelayRateLimiter)
}

func testRelayRateLimiterRedisTransport(
	t *testing.T,
	build func(config.Config) (ratelimit.RateLimiter, func() error, error),
) {
	t.Helper()
	for _, test := range []struct {
		name      string
		tls       bool
		wantFirst byte
	}{
		{name: "plain Redis sends RESP", wantFirst: '*'},
		{name: "rediss sends TLS ClientHello", tls: true, wantFirst: 0x16},
	} {
		t.Run(test.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			firstByte := make(chan byte, 1)
			go func() {
				connection, acceptErr := listener.Accept()
				if acceptErr != nil {
					return
				}
				defer connection.Close()
				buffer := []byte{0}
				if _, readErr := connection.Read(buffer); readErr == nil {
					firstByte <- buffer[0]
				}
			}()

			limiter, closeLimiter, err := build(config.Config{
				RelayRateLimitBackend: "redis",
				RedisAddr:             listener.Addr().String(),
				RedisTLS:              test.tls,
			})
			if err != nil {
				t.Fatal(err)
			}
			if closeLimiter == nil {
				t.Fatal("Redis limiter did not return a close function")
			}
			defer closeLimiter()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			result := make(chan error, 1)
			go func() {
				result <- limiter.Allow(ctx, ratelimit.Key{ChannelID: "transport"}, ratelimit.Limits{RPM: 1}, ratelimit.Usage{})
			}()
			select {
			case got := <-firstByte:
				if got != test.wantFirst {
					t.Fatalf("first transport byte = 0x%02x, want 0x%02x", got, test.wantFirst)
				}
			case <-time.After(time.Second):
				t.Fatal("Redis limiter did not connect")
			}
			cancel()
			select {
			case <-result:
			case <-time.After(time.Second):
				t.Fatal("Redis limiter did not stop after cancellation")
			}
		})
	}

	const malformed = "redis-secret.invalid-address"
	limiter, closeLimiter, err := build(config.Config{RelayRateLimitBackend: "redis", RedisAddr: malformed, RedisTLS: true})
	if err == nil || limiter != nil || closeLimiter != nil || strings.Contains(err.Error(), malformed) || strings.Contains(err.Error(), "redis-secret") {
		t.Fatalf("invalid TLS limiter result limiter=%T close=%v error=%v", limiter, closeLimiter != nil, err)
	}
}

func TestBuildRelayUsageLimitResolverUsesResolvedQuotaScope(t *testing.T) {
	service := &fakeRelayUsageLimitService{
		limit: quota.UsageLimit{
			OrganizationID:        "org_1",
			MaxConcurrentRequests: 2,
			MaxTokensPerWindow:    500,
			MaxTokensPerRequest:   100,
		},
	}
	resolver := buildRelayUsageLimitResolver(service)
	ctx := types.WithTrustedOrganizationID(types.WithTrustedUserID(context.Background(), "user_2"), "org_1")

	resolution := resolver(ctx, nil, "gpt-4o", &types.Usage{PromptTokens: 25})

	if service.organizationID != "org_1" || service.userID != "user_2" {
		t.Fatalf("expected resolver to use trusted identity, got org=%q user=%q", service.organizationID, service.userID)
	}
	if resolution.Limits.MaxConcurrent != 2 || resolution.Limits.TPM != 500 || resolution.Limits.MaxTokensPerRequest != 100 {
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
			MaxTokensPerRequest:   100,
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
	if resolution.Limits.MaxConcurrent != 2 || resolution.Limits.TPM != 500 || resolution.Limits.MaxTokensPerRequest != 100 || resolution.Limits.RPM != 0 {
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
