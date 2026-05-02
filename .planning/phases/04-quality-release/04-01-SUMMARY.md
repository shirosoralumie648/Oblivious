---
phase: 04-quality-release
plan: 01
subsystem: testing
tags: [go, release-gate, admin, marketplace, relay, quota, agent, memory]

requires:
  - phase: 03.1-admin-marketplace-ui
    provides: Admin and Marketplace release surfaces to validate
  - phase: 02-agent-memory-enhancement
    provides: Relay billing, Agent, Memory, and Quota collaboration paths
provides:
  - Broad backend release gate through `go test ./... -count=1`
  - Admin and Marketplace HTTP/service boundary regression tests
  - Relay billing idempotency and Agent gateway regression assertions
affects: [04-quality-release, TEST-01, release-candidate]

tech-stack:
  added: []
  patterns:
    - Repo-local Go caches for release test scripts
    - Fake-store boundary tests for Admin, Marketplace, Relay, Agent, Memory, and Quota

key-files:
  created:
    - src/server/internal/marketplace/service_test.go
  modified:
    - scripts/check.sh
    - scripts/test.sh
    - src/server/internal/http/admin_marketplace_handler_test.go
    - src/server/internal/relay/billing_test.go
    - src/server/internal/agent/service_test.go

key-decisions:
  - "Treat `go test ./... -count=1` as the server release gate for Phase 4."
  - "Keep database-backed HTTP integration tests explicit behind `TEST_DATABASE_URL`."
  - "Do not add direct CI `go test` commands that diverge from the local scripts."

patterns-established:
  - "Release scripts own broad backend verification; CI calls those scripts."
  - "Marketplace service tests use in-memory fake stores instead of provider credentials."

requirements-completed: [TEST-01]

duration: 35min
completed: 2026-05-02
---

# Phase 04 Plan 01: Backend Release Test Gate Summary

**Backend release checks now run the broad Go test suite and cover Admin, Marketplace, Relay, Agent, Memory, and Quota boundaries without live provider credentials.**

## Performance

- **Duration:** 35 min
- **Started:** 2026-05-02T12:20:00Z
- **Completed:** 2026-05-02T12:55:13Z
- **Tasks:** 4 completed
- **Files modified:** 6

## Accomplishments

- `scripts/check.sh server` and `scripts/test.sh server` now run `go test ./... -count=1` with repo-local Go caches.
- Admin handler tests now assert `requireAdmin` rejects non-admin sessions and accepts admin sessions, and cover the release list surfaces.
- Marketplace service tests cover publish, install, review create/update, and my-agents pagination behavior with fake stores.
- Relay billing tests now assert duplicate idempotency keys do not double-call quota `PreConsume`.
- Agent tests now assert replies use the configured Relay-compatible gateway instead of bypassing it.

## Task Commits

1. **Task 1: Broaden backend release scripts** - `79694ca` (`test`)
2. **Task 2: Add Admin and Marketplace boundary tests** - `7480ec6` (`test`)
3. **Task 3: Protect Relay billing, Agent, Memory, and Quota paths** - `3d93772` (`test`)
4. **Task 4: Align CI server job with local release gate** - validated existing CI script calls; no content change required

## Files Created/Modified

- `scripts/check.sh` - Server checks now run the broad backend release gate.
- `scripts/test.sh` - Server tests now run the same broad gate and preserve the explicit `TEST_DATABASE_URL` integration skip.
- `src/server/internal/http/admin_marketplace_handler_test.go` - Adds Admin role-gate coverage and Marketplace public/protected route assertions.
- `src/server/internal/marketplace/service_test.go` - Adds fake-store service coverage for publish, install, review, and my-agents flows.
- `src/server/internal/relay/billing_test.go` - Adds quota preconsume idempotency protection.
- `src/server/internal/agent/service_test.go` - Adds configured gateway call assertions for tool and plain paths.

## Verification

Passed:

```bash
bash scripts/check.sh server
bash scripts/test.sh server
cd src/server && go test ./internal/http ./internal/admin ./internal/marketplace ./internal/relay ./internal/agent ./internal/memory ./internal/quota -count=1
```

Additional acceptance checks passed:

```bash
rg -n "requireAdmin|/api/v1/admin/channels|/api/v1/marketplace/search" src/server/internal/http/admin_marketplace_handler_test.go
rg -n "PreConsume|Refund" src/server/internal/relay/billing_test.go
rg -n "Relay" src/server/internal/memory/embedder_test.go
rg -n "insufficient|Insufficient" src/server/internal/quota/service_test.go
rg -n "bash scripts/check.sh server|bash scripts/test.sh server|OPENAI_API_KEY|STRIPE_SECRET" .github/workflows/ci.yml
```

Integration skip reason:

```text
[test] Skipping server integration tests: TEST_DATABASE_URL not set.
```

## Decisions Made

- Existing `.github/workflows/ci.yml` already calls `bash scripts/check.sh server` and `bash scripts/test.sh server`, so the CI gate stayed aligned without a CI content change.
- Public Marketplace search/list endpoints were checked for "no session required" semantics without introducing a database-backed search fixture in this plan.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. The broad backend gate passed after dependency downloads completed.

## User Setup Required

None - no external service configuration required.

## Self-Check: PASSED

- `04-01-SUMMARY.md` exists.
- Requirement `TEST-01` is complete for this release gate.
- All plan-level verification commands passed.

## Next Phase Readiness

Ready for `04-02` E2E execution. Remaining Phase 4 work is TEST-02, DOC-01, and DEPLOY-01.

---
*Phase: 04-quality-release*
*Completed: 2026-05-02*
