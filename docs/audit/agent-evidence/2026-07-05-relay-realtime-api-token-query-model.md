# Relay Realtime API Token Query Model Authorization

Date: 2026-07-05

## Scope

This slice adds repository-local Realtime API-token authorization evidence for the guarded commercial lifecycle path. Production `GET /v1/realtime?model=...` requests authenticated with `Authorization: Bearer ...` now pass the query-string `model` into the Relay API-token authenticator before the Realtime stream handler runs. This closes the WebSocket-specific gap where model-scoped token policy could not evaluate the requested Realtime model because the route has no JSON body.

`GET /v1/realtime` remains disabled by default in production. This slice proves the local enabled-path token/model authorization input only; it does not make Realtime a final commercial runtime capability.

## Changed Files

- `src/server/internal/relay/handler/policy.go`
- `src/server/internal/relay/handler/policy_test.go`
- `src/server/internal/relay/handler/router_test.go`
- `scripts/verify-commercial-db-evidence.sh`
- `scripts/verify-commercial-db-evidence-profiles.sh`

## Verification

- RED: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay/handler -run '^TestProductionRealtimeAPITokenUsesQueryModelBeforeStreamHandler$' -count=1 -v`
  - Failed with `authenticator saw token="obv_realtime" model="" apiType=realtime`, proving the production API-token authenticator did not see the requested query model.
- GREEN: `cd src/server && GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay/handler -run '^(TestProductionRealtimeAPITokenUsesQueryModelBeforeStreamHandler|TestRealtimePolicyDeclaresCommercialReleaseBlockers|TestProductionSupportedRoutesAcceptRelayAPIToken|TestProductionSupportedRoutesRequireTrustedIdentityBeforeHandler|TestProductionSupportedRoutesAttachTrustedIdentityAndAudit)$' -count=1 -v`

## Remaining Boundary

Realtime is still not a final commercial runtime capability from this local slice alone. Production enablement still requires production prebill configured with active prices, explicit target `realtime_usage_missing` abort settlement proof, request-log linkage, target-runtime evidence, and final no-skip release evidence. `relayRealtime.mode=disabled_until_commercial_lifecycle` is now RC or negative evidence only; a final commercial release claim must prove `relayRealtime.mode=commercial_lifecycle_enabled` with target ledger and artifact-body evidence.
