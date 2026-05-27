---
phase: 14-relay-provider-bypass-and-cost-abuse-guardrails
plan: 01
status: complete
completed_at: 2026-05-28T07:30:00+08:00
requirements: [RELAY-10, RELAY-11]
---

# Phase 14 Plan 01 Summary

## Result

Implemented the second v05 Relay Authority boundary.

Delivered:
- `scripts/verify-relay-security.sh` and `bash scripts/check.sh relay-security` now scan app service code for direct provider bypass patterns.
- GitHub Actions release gates run `bash scripts/check.sh relay-security`, so provider-bypass evidence is part of CI.
- `RoutePolicy` now carries `AuthPolicy`, `TenantIdentityRequired`, `RateLimitPolicy`, and `AuditPolicy`.
- Production-supported Relay routes require trusted internal auth plus user and organization headers before handler/provider execution.
- Route policy decisions emit injectable audit events for allowed and rejected production requests.
- Trusted user, organization, and request IDs are propagated into Relay context for all supported handlers.
- `chat.HTTPReplyGenerator` is demo-only; configured direct provider HTTP calls were removed from app service code.
- `memory.RelayEmbedder` forwards trusted internal Relay headers, and `memory.Service` seeds embedding calls from the active session.
- `docs/release/relay-route-table.md` documents route auth, tenant identity, rate-limit, audit, and billing policy columns.

## Verification

Passed:
- RED before implementation: `cd src/server && go test ./internal/relay/handler -run 'SupportedRoutePolicies|ProductionSupportedRoutesRequireTrustedIdentity|ProductionSupportedRoutesAttachTrustedIdentityAndAudit' -count=1`
  - Failed as expected because policy auth/rate/audit fields, audit sink, and production identity guard did not exist.
- RED before implementation: `cd src/server && go test ./internal/chat ./internal/memory -run 'HTTPReplyGenerator|RelayEmbedder|RelayIdentity' -count=1`
  - Failed as expected because `HTTPReplyGenerator` called `/chat/completions`, Memory did not forward trusted Relay headers, and trusted request ID context did not exist.
- RED before implementation: `bash scripts/check.sh relay-security`
  - Failed as expected on direct-provider bypass patterns in `chat/gateway.go` and `http/router.go`.
- GREEN: `cd src/server && go test ./internal/relay/handler -run 'SupportedRoutePolicies|ProductionSupportedRoutesRequireTrustedIdentity|ProductionSupportedRoutesAttachTrustedIdentityAndAudit|FailClosed|RoutePolicy' -count=1`
- GREEN: `cd src/server && go test ./internal/chat ./internal/memory -run 'HTTPReplyGenerator|RelayEmbedder|RelayIdentity' -count=1`
- GREEN: `bash scripts/check.sh relay-security`
- `cd src/server && go test ./internal/relay/... ./internal/http -count=1`
- `bash scripts/check.sh docs`
- `bash scripts/check.sh server`
- `git diff --check`

## Requirement Closure

| Requirement | Evidence |
|-------------|----------|
| RELAY-10 | `scripts/verify-relay-security.sh`, `scripts/check.sh relay-security`, and `.github/workflows/ci.yml` prove app service direct-provider bypass patterns fail CI |
| RELAY-11 | `policy.go`, `router.go`, policy/router tests, trusted Relay context helpers, and Memory metadata tests prove supported routes enforce tenant identity/auth/rate-limit/audit semantics before provider execution |

## Residual v05 Work

- BILL-01 remains required: exact quota preauthorization, idempotent settlement, and refund behavior.
- BILL-02 remains required: streaming/realtime, file, batch, and async settlement models or production disablement evidence.
- DOC-04 remains required: v05 closeout evidence and Relay Authority Gate completion.

## Next Phase Readiness

Phase 15 should plan and implement Relay billing settlement and refund semantics on top of the Phase 13-14 route policy registry.

---

*Summary written: 2026-05-28*
