# Relay Unsupported Lifecycle Handler Fail-Closed Evidence

Date: 2026-07-08

## Scope

Commercial readiness gap addressed: fine-tuning, Assistants, Threads, and Runs must not reach upstream providers until their lifecycle billing, governance, audit, and request-log contracts are implemented.

## Runtime Contract

- `src/server/internal/relay/handler/policy.go` marks fine-tuning, Assistants, Threads, and Runs route policies disabled.
- `src/server/internal/relay/handler/assistants.go` and `src/server/internal/relay/handler/fine_tuning.go` now return `501 unsupported_api` for recognized endpoints instead of calling upstream.
- `src/server/internal/relay/handler_new/assistants.go` and `src/server/internal/relay/handler_new/fine_tuning.go` have the same fail-closed behavior.
- HTTP Relay aliases do not broaden the supported surface to these disabled endpoints.

## Verification

Commands:

```powershell
$repo=(Resolve-Path '..\..').Path; New-Item -ItemType Directory -Force -Path (Join-Path $repo '.tmp\go-build'),(Join-Path $repo '.tmp\go-mod') | Out-Null; $env:GOCACHE=Join-Path $repo '.tmp\go-build'; $env:GOMODCACHE=Join-Path $repo '.tmp\go-mod'; & 'C:\Program Files\Go\bin\go.exe' test ./internal/relay/handler -run 'Test(AssistantsHandlerFailsClosedWithoutUpstreamPassthrough|FineTuningHandlerFailsClosedWithoutUpstreamPassthrough|AllRegisteredRoutesHaveCommercialPolicy|PolicyRegistryDoesNotContainUnknownRegisteredRoutes|InitialCommercialPolicyClassifiesCurrentSurface|InitialBillingSettlementPolicyClassifiesCurrentSurface)' -count=1 -v
```

```powershell
$repo=(Resolve-Path '..\..').Path; New-Item -ItemType Directory -Force -Path (Join-Path $repo '.tmp\go-build'),(Join-Path $repo '.tmp\go-mod') | Out-Null; $env:GOCACHE=Join-Path $repo '.tmp\go-build'; $env:GOMODCACHE=Join-Path $repo '.tmp\go-mod'; & 'C:\Program Files\Go\bin\go.exe' test ./internal/http -run 'TestCombineHandlersDoesNotBroadenRelayAliasesToUnsupportedSurfaces' -count=1 -v
```

```powershell
$repo=(Resolve-Path '..\..').Path; New-Item -ItemType Directory -Force -Path (Join-Path $repo '.tmp\go-build'),(Join-Path $repo '.tmp\go-mod') | Out-Null; $env:GOCACHE=Join-Path $repo '.tmp\go-build'; $env:GOMODCACHE=Join-Path $repo '.tmp\go-mod'; & 'C:\Program Files\Go\bin\go.exe' test ./internal/relay/handler_new -count=1
```

Result: passed.

## Remaining Commercial Blockers

This closes the unsupported-surface fail-closed contract. It does not implement fine-tuning, Assistants, Threads, or Runs. Commercial enablement still requires durable lifecycle state, billing settlement/refund behavior, request-log evidence, governance/audit rules, and target-runtime proof.
