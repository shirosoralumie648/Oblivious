---
phase: 13-relay-endpoint-authority-and-fail-closed
plan: 01
status: complete
completed_at: 2026-05-28T06:20:00+08:00
requirements: [RELAY-08, RELAY-09]
commits:
  - 3b9d4dd feat(13): enforce relay route authority
---

# Phase 13 Plan 01 Summary

## Result

Implemented the first v05 Relay Authority boundary.

Delivered:
- `src/server/internal/relay/handler/policy.go` defines the Relay commercial route policy registry.
- All 34 routes currently returned by `getOpenAIRoutes()` are classified as `CommercialSupportedBilled` or `DisabledInProduction`.
- Production-disabled routes return `endpoint_disabled_in_production` before handler, passthrough, file proxy, or provider execution.
- `relay.Config{Production: true}` is wired from `APP_ENV=production` through `src/server/internal/http/server.go`.
- `docs/release/relay-route-table.md` documents the full `/v1/*` route table with commercial class, production status, disabled reason, and future owner.
- Docs/API and commercial gate docs now point to the route table, and quality gates assert the file and class names remain present.

## Verification

Passed:
- RED check before implementation: `cd src/server && go test ./internal/relay/handler -run 'RoutePolicy|CommercialPolicy|RegisteredRoutes' -count=1`
  - Failed as expected because `PolicyForRoute`, `AllRoutePolicies`, and class constants did not exist.
- RED check before fail-closed implementation: `cd src/server && go test ./internal/relay/handler -run 'FailClosed|ProductionSupported|DevelopmentDisabled' -count=1`
  - Failed as expected because `RegisterRoutesWithOptions` and `RouteRegistrationOptions` did not exist.
- `cd src/server && go test ./internal/relay/handler -run 'FailClosed|ProductionSupported|DevelopmentDisabled|RoutePolicy|CommercialPolicy|RegisteredRoutes' -count=1`
- `cd src/server && go test ./internal/relay/handler -count=1`
- `cd src/server && go test ./internal/relay/... ./internal/http -count=1`
- `bash scripts/check.sh docs`
- `git diff --check`
- Targeted `rg` check for `CommercialSupportedBilled`, `DisabledInProduction`, `endpoint_disabled_in_production`, `relay-route-table.md`, `/v1/chat/completions`, `/v1/files`, `/v1/fine_tuning/jobs`, and `/v1/threads/:id/runs`.

## Requirement Closure

| Requirement | Evidence |
|-------------|----------|
| RELAY-08 | `policy.go`, `policy_test.go`, and `docs/release/relay-route-table.md` classify every currently registered `/v1/*` route |
| RELAY-09 | `RegisterRoutesWithOptions`, `RejectIfProductionDisabled`, and router tests prove disabled routes fail closed in production before handler execution |

## Residual v05 Work

- RELAY-10 remains required: provider-bypass CI checks.
- RELAY-11 remains required: endpoint auth, tenant identity, rate-limit, and audit semantics.
- BILL-01 remains required: exact quota preauthorization, idempotent settlement, and refund behavior.
- BILL-02 remains required: streaming/realtime, file, batch, and async settlement models or production disablement evidence.
- DOC-04 remains required: v05 closeout evidence and Relay Authority Gate completion.

## Next Phase Readiness

Phase 14 should plan and implement provider-bypass and cost-abuse guardrails. It should start from the Phase 13 route policy registry rather than rediscovering route classes.

---

*Summary written: 2026-05-28*
