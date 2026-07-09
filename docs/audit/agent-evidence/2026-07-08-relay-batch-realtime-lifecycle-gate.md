# Relay Batch And Realtime Lifecycle Gate Evidence

Date: 2026-07-08

## Scope

Commercial readiness gap addressed: Batch and Realtime handlers must not reach upstream providers or billing routers unless their commercial lifecycle is explicitly enabled.

## Runtime Contract

- `src/server/internal/relay/handler/batch.go` and `src/server/internal/relay/handler/realtime.go` now default to `501 unsupported_api` before upstream calls, billing router calls, or polling registration.
- `src/server/internal/relay/handler_new/batch.go` and `src/server/internal/relay/handler_new/realtime.go` use the same default fail-closed behavior.
- `src/server/internal/relay/relay.go` wires `BatchCommercialLifecycleEnabled` and `RealtimeCommercialLifecycleEnabled` into the concrete handlers, matching the existing route policy options.
- Existing lifecycle behavior remains available only when tests or runtime config call `WithCommercialLifecycleEnabled(true)`.

## Verification

Command:

```powershell
$repo=(Resolve-Path '..\..').Path; New-Item -ItemType Directory -Force -Path (Join-Path $repo '.tmp\go-build'),(Join-Path $repo '.tmp\go-mod') | Out-Null; $env:GOCACHE=Join-Path $repo '.tmp\go-build'; $env:GOMODCACHE=Join-Path $repo '.tmp\go-mod'; & 'C:\Program Files\Go\bin\go.exe' test ./internal/relay/handler ./internal/relay/handler_new ./internal/relay -run 'Test(Batch|Realtime|NewRelay)' -count=1 -v
```

Result: passed.

Covered checks include:

- Batch submit/list/get default disabled path returns `501 unsupported_api`.
- Disabled Batch does not call upstream, `RouteWithBilling`, or the polling registrar.
- Realtime default disabled path returns `501 unsupported_api`.
- Disabled Realtime does not dial upstream or call `RouteWithBilling`.
- Explicitly enabled Batch still routes through billing and registers polling tasks.
- Explicitly enabled Realtime still routes through streaming billing and captures upstream usage.
- `NewRelay` propagates the Batch/Realtime commercial lifecycle flags into the handlers.

## Remaining Commercial Blockers

This closes the route-policy bypass risk for direct Batch/Realtime handler invocation. It does not prove these APIs are commercially complete. Batch still needs target-runtime polling worker evidence, completion settlement/refund/audit proof, and provider usage reconciliation. Realtime still needs target-runtime streaming prebill, abort settlement, request-log linkage, and production provider proof.
