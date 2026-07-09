# Relay Realtime Usage Capture

Date: 2026-07-05

## Scope

This slice adds repository-local Realtime usage capture for the guarded commercial lifecycle path. The active Realtime WebSocket handler now parses upstream usage events such as `response.done.response.usage.input_tokens`, `output_tokens`, and `total_tokens`, maps them into Relay `types.Usage`, and returns that usage to `RouteWithBilling` for final usage logging and quota settlement. If the upstream Realtime stream closes without any usage, the handler now fails closed with `realtime_usage_missing` instead of returning an empty usage object that could settle as a successful zero-token call.

`GET /v1/realtime` remains disabled by default in production. This slice improves the opt-in lifecycle path but does not prove final commercial Realtime readiness by itself.

## Changed Files

- `src/server/internal/relay/handler/realtime.go`
- `src/server/internal/relay/handler/realtime_test.go`
- `src/server/internal/relay/router.go`
- `src/server/internal/relay/router_test.go`

## Verification

- RED: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay/handler -run 'TestRealtimeCapturesUpstreamUsageForBillingSettlement' -count=1 -v`
  - Failed because the Realtime billing response still had nil usage.
- RED: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay/handler -run 'TestRealtimeFailsClosedWhenUpstreamClosesWithoutUsage' -count=1 -v`
  - Failed because the Realtime billing callback still succeeded when the upstream stream closed without usage.
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay/handler -run 'TestRealtimeFailsClosedWhenUpstreamClosesWithoutUsage|TestRealtimeCapturesUpstreamUsageForBillingSettlement|TestRealtimeUsesRouteWithBillingForLifecycle|TestRealtimeRejectsMissingModelBeforeUpstreamDial|TestRealtimeUsesAdapterRealtimeEndpointWithoutDuplicatePath' -count=1 -v`
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/config ./internal/relay/handler ./internal/relay ./internal/http ./cmd/relay -count=1`
- RED: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay -run '^TestRouterRouteWithBillingFinalizesStreamingAbortUsageAndRequestLogScope$' -count=1 -v`
  - First exposed missing Realtime price setup in the test, then failed with one request leaving multiple pending/error records after retrying channels instead of replacing pending usage with one terminal error record.
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay -run '^TestRouterRouteWithBillingFinalizesStreamingAbortUsageAndRequestLogScope$' -count=1 -v`

## Remaining Boundary

Realtime is still not a final commercial runtime capability from this local slice alone. Later 2026-07-05 slices added repository-local origin-policy proof, API-token query-model authorization proof, and streaming abort pending-usage replacement plus request-log metadata proof. Production enablement still requires prebill configured with production prices, target `realtime_usage_missing` abort behavior, target request-log joins, target-runtime evidence, and final no-skip release evidence. `relayRealtime.mode=disabled_until_commercial_lifecycle` is now RC or negative evidence only; a final commercial release claim must prove `relayRealtime.mode=commercial_lifecycle_enabled` with target ledger and artifact-body evidence.
