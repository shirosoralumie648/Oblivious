# Relay Realtime Origin Policy

Date: 2026-07-05

## Scope

This slice adds repository-local Realtime WebSocket origin enforcement to the active Relay handler. `GET /v1/realtime` now rejects disallowed browser `Origin` values before `RouteWithBilling` and before any upstream WebSocket dial. The Relay config also forwards `CORS_ALLOWED_ORIGINS` into both standalone Relay and server-built Relay instances so the same origin policy governs Realtime upgrades.

A later 2026-07-05 slice also proves that production API-token Realtime requests pass the query-string `model` into Relay token authorization before the stream handler runs.

`GET /v1/realtime` remains disabled by default in production. This slice closes the local origin-policy proof only; it does not make Realtime a final commercial runtime capability.

## Changed Files

- `src/server/internal/relay/handler/realtime.go`
- `src/server/internal/relay/handler/realtime_test.go`
- `src/server/internal/relay/relay.go`
- `src/server/internal/http/server.go`
- `src/server/internal/http/server_test.go`
- `src/server/cmd/relay/main.go`
- `src/server/cmd/relay/main_test.go`
- `scripts/verify-commercial-db-evidence.sh`
- `scripts/verify-commercial-db-evidence-profiles.sh`

## Verification

- RED: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay/handler -run '^TestRealtimeRejectsDisallowedOriginBeforeBillingAndUpstreamDial$' -count=1 -v`
  - Failed with `RouteWithBilling calls = 1, want 0 before origin is allowed`, proving disallowed origins reached billing before this fix.
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay/handler -run '^(TestRealtimeRejectsDisallowedOriginBeforeBillingAndUpstreamDial|TestRealtimeUsesRouteWithBillingForLifecycle|TestRealtimeCapturesUpstreamUsageForBillingSettlement|TestRealtimeFailsClosedWhenUpstreamClosesWithoutUsage)$' -count=1 -v`
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run '^TestBuildRelayConfigWiresCommercialLifecycleFlags$' -count=1 -v`
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./cmd/relay -run '^TestBuildStandaloneRelayConfigWiresBatchCommercialLifecycle$' -count=1 -v`

## Remaining Boundary

Realtime is still not a final commercial runtime capability from this local slice alone. Production enablement still requires prebill configured with production prices, explicit target `realtime_usage_missing` abort settlement proof, request-log linkage, target-runtime evidence, and final no-skip release evidence. `relayRealtime.mode=disabled_until_commercial_lifecycle` is now RC or negative evidence only; a final commercial release claim must prove `relayRealtime.mode=commercial_lifecycle_enabled` with target ledger and artifact-body evidence.
