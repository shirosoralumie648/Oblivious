# Relay Realtime Missing Model Fail Fast

Date: 2026-07-04

## Runtime Claim

Active Relay Realtime now rejects `GET /v1/realtime` requests that omit `model` before dialing the upstream WebSocket provider, converts adapter HTTP(S) endpoints to WS(S), uses the adapter-provided `/v1/realtime` path without appending a duplicate suffix, and avoids passing duplicate `Upgrade`/`Connection` headers to the gorilla WebSocket dialer. This closes the old default-model/path/handshake fallback paths without enabling Realtime as a commercial production route.

## Reference Inputs

```text
reference/CLIProxyAPI/README.md - OpenAI-compatible proxy surface used as route-compatibility target.
reference/CliRelay/README.md - Relay-style OpenAI-compatible gateway reference; used for Realtime surface parity framing, not copied as lifecycle proof.
docs/audit/stub-hardcoded-todo-report.md - Live local entry identifying the hardcoded Realtime default model.
docs/release/relay-route-table.md - Local route-policy authority for keeping Realtime production-disabled.
```

## Oblivious Files Changed

```text
src/server/internal/relay/handler/realtime.go
src/server/internal/relay/handler/realtime_test.go
docs/audit/stub-hardcoded-todo-report.md
docs/audit/agent-evidence/2026-07-04-relay-realtime-missing-model-fail-fast.md
```

## Contract Changes

None for production release policy. Non-production direct handler calls now fail with `400` and `realtime_model_required` when `model` is missing instead of silently using `gpt-4o-realtime-preview`. Realtime upstream dialing now uses a WebSocket URL derived from the adapter endpoint and relies on the dialer to set reserved handshake headers.

## Verification Commands

```text
command: cd src/server && go test ./internal/relay/handler -run 'TestRealtimeRejectsMissingModelBeforeUpstreamDial' -count=1
result: RED before implementation; failed with status 502 and body {"error":"upstream connection failed"}.

command: cd src/server && go test ./internal/relay/handler -run 'TestRealtimeRejectsMissingModelBeforeUpstreamDial|TestRealtimePolicyDeclaresCommercialReleaseBlockers|TestProductionBatchAndRealtimeRoutesFailClosedBeforeHandler' -count=1
result: GREEN after implementation; ok oblivious/server/internal/relay/handler 0.009s.

command: cd src/server && go test ./internal/relay/handler -run 'TestRealtimeUsesAdapterRealtimeEndpointWithoutDuplicatePath' -count=1
result: RED before endpoint/handshake implementation; failed with status 502 and `websocket: bad handshake`.

command: cd src/server && go test ./internal/relay/handler -run 'TestRealtimeUsesAdapterRealtimeEndpointWithoutDuplicatePath|TestRealtimeRejectsMissingModelBeforeUpstreamDial|TestRealtimePolicyDeclaresCommercialReleaseBlockers|TestProductionBatchAndRealtimeRoutesFailClosedBeforeHandler' -count=1
result: GREEN after endpoint/handshake implementation; ok oblivious/server/internal/relay/handler 0.011s.
```

## Runtime Evidence IDs

```text
api_type: realtime
route: GET /v1/realtime
error_code: realtime_model_required
upstream_path: /v1/realtime
```

## Failure Evidence

The missing-model regression test failed before implementation because a missing-model Realtime request attempted an upstream WebSocket dial and returned `502 upstream connection failed`. The endpoint regression test then failed because the active handler could not complete an upstream WebSocket handshake against the adapter-provided `/v1/realtime` route. These failures prove the tests cover the old default-model, duplicate-path/scheme, and duplicate-handshake-header behavior.

## Unsupported / Deferred Surfaces

- Realtime production enablement remains deferred.
- Later 2026-07-05 slices added repository-local origin policy, usage capture, and API-token query-model authorization proof. Production prebill, abort settlement, request-log linkage, target-runtime proof, and final no-skip release evidence remain required before the route policy can change.

## Known Residual Risk

Realtime must remain disabled in production until production prebill, abort settlement, provider/client disconnect handling, request-log linkage, target release evidence, and final no-skip release evidence are implemented and verified.
