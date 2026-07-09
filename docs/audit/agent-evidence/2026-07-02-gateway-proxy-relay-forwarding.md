# Gateway Proxy Relay Forwarding Evidence

Date: 2026-07-02

## Scope

`/api/v1/gateway/proxy/*` must not be an API-shaped placeholder. Authenticated console/API traffic should be forwarded into the same Relay engine used by `/v1/*`, with browser credentials stripped and trusted internal identity headers injected.

## Implementation

- `src/server/internal/http/gateway_handler.go` rewrites `/api/v1/gateway/proxy/<path>` to `/v1/<path>`.
- The proxy strips browser `Authorization`, injects `X-Oblivious-Internal-Auth`, user ID, organization ID, feature type `gateway_proxy`, and request ID, then calls the configured Relay handler.
- `src/server/internal/http/server.go` wires the real Relay engine as `GatewayRelayHandler`.
- Missing sessions fail with `401`; missing Relay handler fails with `503 relay_unavailable`.

## Verification

Command run from `src/server`:

```bash
go test ./internal/http -run "TestGatewayProxy" -count=1
```

The test suite verifies session enforcement, fail-closed missing Relay handler behavior, path/query/body forwarding, authorization stripping, internal auth injection, tenant identity headers, feature attribution, and response pass-through.

## Remaining Gap

The stub blocker is closed in code. Final commercial release still needs target-runtime provider evidence that joins Gateway proxy request ID to Relay route decision, usage, quota settlement, request log, and streaming behavior.
