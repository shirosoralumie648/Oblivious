# Relay Realtime Pricing Prebill Gate

Date: 2026-07-05

## Scope

This slice adds repository-local Realtime prebill prerequisite evidence for the guarded commercial lifecycle path. Production Relay startup now fails closed when `RealtimeCommercialLifecycleEnabled` is true and no enabled channel model has an active Realtime `total_tokens` price. When active Realtime pricing is configured for an enabled channel model, startup succeeds and the opt-in lifecycle flag remains enabled.

`GET /v1/realtime` remains disabled by default in production. This slice proves startup-time pricing coverage for the opt-in lifecycle path only; it does not prove target provider pricing freshness or final commercial readiness.

## Changed Files

- `src/server/internal/relay/relay.go`
- `src/server/internal/relay/relay_test.go`
- `scripts/verify-commercial-db-evidence.sh`
- `scripts/verify-commercial-db-evidence-profiles.sh`

## Verification

- RED: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay -run '^TestNewRelayProductionRealtimeCommercialLifecycleRequiresRealtimePricing$' -count=1 -v`
  - Failed because production Relay startup accepted `RealtimeCommercialLifecycleEnabled=true` with `NewPricingStoreWithDefaults()` even though the default catalog had no Realtime prices.
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay -run '^(TestNewRelayProductionRealtimeCommercialLifecycleRequiresRealtimePricing|TestNewRelayProductionRealtimeCommercialLifecycleAcceptsActiveRealtimePricing)$' -count=1 -v`

## Remaining Boundary

Realtime is still not a final commercial runtime capability from this local slice alone. Production enablement still requires target price-source freshness, target `realtime_usage_missing` abort behavior, target ClickHouse request-log to usage/billing join proof, target-runtime provider evidence, and final no-skip release evidence. `relayRealtime.mode=disabled_until_commercial_lifecycle` is now RC or negative evidence only; a final commercial release claim must prove `relayRealtime.mode=commercial_lifecycle_enabled` with target ledger and artifact-body evidence.
