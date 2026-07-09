# Relay Realtime Abort Settlement and Request Log Linkage

Date: 2026-07-05

## Scope

This slice adds repository-local Realtime abort settlement evidence for the guarded commercial lifecycle path. When a streaming Realtime provider callback fails after a pending usage record has been written, `RouteWithBilling` now replaces that pending record with one terminal coded Realtime error usage record, refunds quota and API-token preauthorization, avoids retrying another channel after the stream lifecycle has started, and records request-log metadata containing the request, user, organization, channel, provider, status, and error code needed for later request-log joins. The 2026-07-08 final-proof hardening requires the terminal usage error code to be explicit `realtime_usage_missing`; generic `upstream_error` is no longer release-proof evidence for Realtime abort settlement.

`GET /v1/realtime` remains disabled by default in production. This slice proves local abort lifecycle behavior only; it does not prove target Realtime provider behavior or final commercial readiness.

## Changed Files

- `src/server/internal/relay/router.go`
- `src/server/internal/relay/router_test.go`
- `scripts/verify-commercial-db-evidence.sh`
- `scripts/verify-commercial-db-evidence-profiles.sh`

## Verification

- RED: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay -run '^TestRouterRouteWithBillingFinalizesStreamingAbortUsageAndRequestLogScope$' -count=1 -v`
  - First exposed missing Realtime pricing setup in the test, then failed because one Realtime abort left multiple pending/error records across retries instead of replacing the pending usage record with one terminal error record.
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay -run '^TestRouterRouteWithBillingFinalizesStreamingAbortUsageAndRequestLogScope$' -count=1 -v`

## Remaining Boundary

Realtime is still not a final commercial runtime capability from this local slice alone. Production enablement still requires production Realtime prices, target `realtime_usage_missing` abort behavior, target ClickHouse request-log to usage/billing join proof, target-runtime provider evidence, and final no-skip release evidence. `relayRealtime.mode=disabled_until_commercial_lifecycle` is now RC or negative evidence only; a final commercial release claim must prove `relayRealtime.mode=commercial_lifecycle_enabled` with target ledger and artifact-body evidence.
