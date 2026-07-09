# WebSocket Origin And Hijacker Hardening Evidence - 2026-07-02

## Claim

The application WebSocket endpoint no longer accepts arbitrary browser origins, and the HTTP middleware stack preserves WebSocket upgrades through the production router.

## Runtime Surface

- `/api/v1/ws` still requires an authenticated session before upgrade.
- Browser `Origin` values are accepted only when they are same-host or configured in `CORSAllowedOrigins`.
- Requests without `Origin` remain valid for non-browser/server clients.
- Wildcard origin configuration is not treated as permission to accept arbitrary browser origins.
- Relay `/v1/realtime` remains disabled by the production relay route policy until realtime auth, origin policy, prebill, abort settlement, and usage capture are implemented.

## Changed Files

- `src/server/internal/ws/handler.go`
- `src/server/internal/ws/handler_test.go`
- `src/server/internal/http/router.go`
- `src/server/internal/http/middleware.go`
- `src/server/internal/http/websocket_origin_test.go`
- `src/server/internal/relay/handler/realtime.go`
- `src/server/internal/relay/handler/chat.go`
- `src/server/internal/relay/handler_new/realtime.go`
- `src/server/internal/relay/handler_new/chat.go`

## Verification

```bash
cd src/server
export PATH="/c/Program Files/Go/bin:$PATH"
go test ./internal/ws -count=1 -v
go test ./internal/http -run "TestWebSocketHandshake|TestRouteSurfaceWebSocket" -count=1 -v
go test ./internal/relay/handler -run "TestProductionBatchAndRealtimeRoutesFailClosedBeforeHandler|TestRoutePolicyCommercialClassifications" -count=1 -v
```

All commands passed on 2026-07-02.

Additional static check:

```bash
rg -n "CheckOrigin:\s*func\([^)]*\)\s*bool\s*\{\s*return true\s*\}|CheckOrigin:.*return true|return true \}," src/server/internal/relay src/server/internal/ws src/server/internal/http
```

The scan returned no matches.

## Failure Prevented

Before this change, `/api/v1/ws` reached `gorilla/websocket` through an HTTP logging wrapper that did not expose `http.Hijacker`, causing authenticated WebSocket handshakes to fail with HTTP 500. The same endpoint also used an upgrader with `CheckOrigin: true`, allowing any browser origin once a session cookie was present.

## Residual Risk

Realtime relay WebSocket support is intentionally not a commercial runtime capability yet. The production route policy still fails closed for `GET /v1/realtime`; enabling it later still requires audited auth, origin policy, prebill, abort settlement, and usage-capture evidence.
