# Relay Requirement Audit - 2026-06-07

Scope:

- `docs/superpowers/specs/2026-06-04-functional-logic-details.md` sections 1.1-1.5.
- `docs/superpowers/specs/2026-06-04-complete-fusion-design.md` section 3.1.
- `docs/superpowers/specs/2026-06-04-complete-fusion-design-part3.md` section 8.3.1.

Status values:

- `Proven`: current code and focused tests prove the requirement.
- `Partial`: current code exists, but the requirement is only partly implemented or partly verified.
- `Gap`: current evidence contradicts or misses the requirement.

## 1.1 Load Balancing

| Requirement | Status | Evidence |
| --- | --- | --- |
| Supports weighted load balancing using each channel's configured weight. | Proven | `src/server/internal/relay/balancer/weighted.go` now implements deterministic weighted round-robin; `src/server/internal/relay/loadbalancer.go` uses the same deterministic weighted sequence for the production `weighted` strategy. `TestWeightedRoundRobin_SelectsDeterministicWeightedSequence` and `TestLoadBalancer_Weighted` assert `3:1` channel sequencing. |
| Supports adaptive load balancing. | Proven | `src/server/internal/relay/balancer/adaptive.go` and `src/server/internal/relay/loadbalancer.go` compute dynamic weights from static weight, health score, error rate, and average latency. `TestAdaptiveBalancer_HighErrorRateReducesWeight` and `TestLoadBalancer_AdaptiveUsesRuntimeHealthMetrics` verify degraded channels lose weight. |
| Adaptive formula uses health score, 5-minute error rate, and average latency. | Partial | Runtime stats include success/failure counts and average latency, and adaptive weights use `healthScore * (1-errorRate) / log2(latency+1)`. The current in-memory window is request-count based, not explicitly a 5-minute time window. |
| Health check and automatic removal from routing. | Proven | `src/server/internal/relay/healthchecker.go`, `src/server/internal/relay/relay.go`, and `src/server/internal/relay/loadbalancer.go` exclude unhealthy, invalid, and temporarily rate-limited channels. `TestRelayStartHealthChecksMarksUnhealthyChannelInvalid` and `TestRelayHealthChecksRouteChannelUnhealthyAlertAndResolveOnRecovery` cover health failure handling and recovery alert state. |

## 1.2 Channel Affinity

| Requirement | Status | Evidence |
| --- | --- | --- |
| Conversation-level sticky routing. | Proven | `Router.selectChannelForBilling` reads `conversation_id` from trusted context and prefers the saved affinity channel; `TestRouterRouteWithBillingUsesConversationAffinityBeforeLoadBalancer` proves sticky selection. |
| Initial successful channel is stored for the conversation. | Proven | `RouteWithBilling` saves `conversation_id -> selected_channel_id` after a terminal response; `src/server/internal/relay/store_test.go` covers persisted affinity updates, and router tests cover in-memory store behavior. |
| 5xx, 429, and network failure switch to a backup channel and update affinity. | Proven | `TestRouterRouteWithBillingRetries5xxAcrossChannelsAndUpdatesConversationAffinity`, `TestRouterRouteWithBillingRetriesBare502ProviderResponseAcrossChannels`, `TestRouterRouteWithBillingRetries429AcrossChannelsAndMarksRateLimitedUntil`, and `TestRouterRouteWithBillingRetriesNetworkErrorsAcrossChannels` cover failover and affinity updates. |
| Semantic cache is shared across channels. | Proven | `SemanticCacheQueryHash` includes model and query, not channel ID. `TestSemanticCacheQueryHashIncludesModelAndIgnoresChannelID`, `TestSemanticCacheStoreWritesClassifiedScopeWithPolicyTTL`, and `TestNewRelayProductionChatUsesSharedSemanticCacheOnSecondRequest` prove cross-channel cache keys and production cache reuse. |

## 1.3 RPM/TPM Limit Handling

| Requirement | Status | Evidence |
| --- | --- | --- |
| Local counters enforce channel RPM and TPM. | Proven | `src/server/internal/relay/ratelimit/limiter.go` enforces RPM and TPM by channel/model/token key; `relayChannelRateLimitCheck` maps `Channel.RPMLimit` and `Channel.TPMLimit`; `TestInMemoryRateLimiterRPMUsesSlidingWindow`, `TestInMemoryRateLimiterTPMUsesSixtySecondAccumulator`, and `TestBuildRelayUsageLimitResolverKeepsChannelRateLimits` cover behavior. |
| Redis-backed counters are available. | Proven | `buildRelayRateLimiter` supports `RelayRateLimitBackend=redis`; `src/server/internal/relay/ratelimit/redis_limiter.go` implements the Redis limiter; `TestBuildRelayRateLimiterCreatesRedisLimiter` verifies wiring. |
| Local counter rejects before upstream call. | Proven | `TestRouterRouteWithBillingRejectsRateLimitedRequestBeforeUpstream` proves a 429 `relay_rate_limited` router error is returned and the upstream callback is not called. |
| Passive upstream 429 marks channel `rate_limited`, parses `Retry-After`, removes it temporarily, and retries another channel. | Proven | `markRetryableProviderFailure` sets `RateLimitedUntil` from `Retry-After`; `LoadBalancer.filterHealthy` excludes rate-limited channels; `TestRouterRouteWithBillingRetries429AcrossChannelsAndMarksRateLimitedUntil` proves retry and temporary removal. |
| Approaching 90% of local limit lowers channel weight before hard reject. | Gap | No current limiter decision or load-balancer weight path uses a 90% soft-threshold signal. Existing behavior enforces the hard local limit and handles upstream 429 passively. |

## 1.4 Failover Rules

| Requirement | Status | Evidence |
| --- | --- | --- |
| 500/502/503/504 retry across channels. | Proven | `IsRetryable` includes 500, 502, 503, and 504. Billing route tests cover 5xx and 502 cross-channel retry. |
| 429 retry across channels and mark rate-limited. | Proven | `IsRetryable` includes 429; `TestRouterRouteWithBillingRetries429AcrossChannelsAndMarksRateLimitedUntil` covers `Retry-After` parsing and backup channel selection. |
| Network errors retry across channels. | Proven | `RouteWithBilling` treats upstream callback errors as retryable and excludes the failed channel; `TestRouterRouteWithBillingRetriesNetworkErrorsAcrossChannels` proves backup channel retry. |
| 401 and 403 do not retry and mark channel invalid/forbidden. | Proven | 401 and 403 are non-retryable via `IsRetryable`; `TestRouterRouteWithBillingDoesNotRetryUnauthorizedProviderResponse` proves 401 marks the channel invalid and does not retry; `TestRouterRouteWithBillingDoesNotRetryForbiddenProviderResponse` proves 403 marks a distinct forbidden runtime state without retrying; `TestLoadBalancer_SkipsForbiddenChannel` proves forbidden channels are removed from subsequent selection. |
| 400 returns directly to the user without retry. | Proven | `IsRetryable` excludes 400 and `RouteWithBilling` returns non-retryable provider responses directly. |
| Maximum retry count is 3 cross-channel retries. | Proven | `maxRouteBillingAttempts` is 4 total attempts; `TestRouterRouteWithBillingAllowsThreeCrossChannelRetries` proves primary plus three retry channels. |
| Backoff is immediate, 1s, then 3s. | Proven | `routeRetryBackoff` returns `0`, `1s`, and `3s`; billing failover path uses `sleepBeforeRetry`. |
| Legacy `RouteWithFallback` follows the same backoff contract. | Proven | `RouteWithFallback` now uses `sleepBeforeRetry`, the same helper as the production billing path. `TestRouterRouteWithFallbackRetriesFirstRetryableProviderResponseImmediately` proves the first retryable provider response uses the immediate production backoff instead of the old quadratic delay. |

## 1.5 Semantic Cache

| Requirement | Status | Evidence |
| --- | --- | --- |
| Public queries use global namespace and 24-hour TTL. | Proven | `SemanticCachePolicy` returns `global:cache:{model}:{query_hash}` and `DefaultSemanticCacheGlobalTTL`; `TestSemanticCachePolicyClassifiesScopeAndTTL` and `TestSemanticCacheStoreWritesClassifiedScopeWithPolicyTTL`. |
| Sensitive or user-scoped queries use organization namespace and 1-hour TTL. | Proven | `isSensitiveSemanticCacheQuery` checks email, phone, organization name, user name/id, pronouns, and `UserScoped`; `TestSemanticCachePolicyClassifiesScopeAndTTL` and `TestSemanticCacheLookupSensitiveQueriesNeverCheckGlobal`. |
| Lookup order checks global first, then org cache for public org requests. | Proven | `SemanticCache.Lookup` builds `[global, org]` lookup keys for public org requests; `TestSemanticCacheLookupOrderUsesGlobalThenOrgForPublicQueries` proves order. |
| Cache key excludes channel ID. | Proven | `TestSemanticCacheQueryHashIncludesModelAndIgnoresChannelID`. |
| Vector/text similarity lookup is available. | Partial | In-memory store supports text similarity and embedding similarity. SQL backend can store vectors and configures an embedder for SQL cache. Current thresholds are `0.7` text and `0.8` embedding, not the `0.85` threshold referenced elsewhere in the functional logic details. |

## Fusion Design 3.1 and API Contract

| Requirement | Status | Evidence |
| --- | --- | --- |
| OpenAI-compatible relay routes for chat, embeddings, images, audio, and models. | Proven | `src/server/internal/relay/handler/router.go` registers `/v1/chat/completions`, `/v1/embeddings`, `/v1/images/generations`, `/v1/audio/transcriptions`, `/v1/models`; `relayAliasTargetPath` maps `/api/v1/relay/*` aliases; `TestCombineHandlersRelayAliasesRouteToOpenAICompatiblePaths` and `TestCombineHandlersRelayAliasesReachProductionRelayPolicy`. |
| Streaming/SSE response support. | Partial | Chat, responses, and realtime handlers have stream paths and route strategies. This audit did not run an end-to-end SSE compatibility proof against the fusion spec. |
| Provider adapters cover OpenAI, Claude, Gemini, Vertex, Bedrock, and expanded OpenAI-compatible providers. | Proven | `AdapterForChannel` supports OpenAI-compatible providers plus native Claude, Gemini, Vertex, and Bedrock; provider tests cover URL construction for these adapters. |
| 100+ provider support. | Partial | `SupportedProviders()` returns more than 100 catalog entries, but many are explicitly `planned` and fail closed until an adapter is wired. This proves catalog coverage, not full callable adapter parity for every provider. |
| Billing, usage logging, semantic cache, and metrics are unified through Relay. | Proven for production chat path | `RouteWithBilling` performs pre-authorization, API-token quota, usage logging, cache lookup/store, metrics, and settlement; production tests cover quota failure, semantic cache reuse, and settlement success/failure paths. |

## Current Conclusion

The repository-owned Relay path now proves the core Functional Logic 1.1-1.5 behavior for weighted round-robin, adaptive selection, conversation affinity, cross-channel failover, RPM/TPM local enforcement, passive 429 handling, and public/private semantic cache policy.

The row remains `Partial`, not `Proven`, because two requirements still need work or explicit boundary decisions:

1. Add a 90% local RPM/TPM soft-threshold signal that lowers channel weight before hard rejection.
2. Decide whether semantic cache similarity thresholds must be changed to the spec's `0.85`, and whether all 100+ provider entries must be callable adapters rather than planned catalog entries.
