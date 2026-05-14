---
phase: 06-backend-mainline-integration-hardening
plan: 01
subsystem: backend-http
tags: [routes, auth, middleware, regression-tests]

requires:
  - phase: 05-dirty-worktree-triage-and-commit-boundary
    provides: Backend integration commit boundary and route-surface handoff
provides:
  - Live route-surface regression coverage for app session gates
  - Admin-route regression coverage for `requireAdmin`
  - Public auth-route regression coverage
affects: [ROUTE-01, Phase 7, Phase 8]

tech-stack:
  added: []
  patterns:
    - Table-driven `httptest` route-surface checks over the live router
    - Direct middleware regression tests for admin authorization

key-files:
  created:
    - src/server/internal/http/route_surface_test.go
  modified:
    - src/server/internal/http/auth_middleware_test.go

key-decisions:
  - "Kept `src/server/internal/http/router.go` as the authoritative route composition root for Phase 6."
  - "Protected app route groups with session checks and admin route groups with `requireAdmin` coverage."

patterns-established:
  - "Route-surface tests should assert status contracts against the live router instead of duplicating route registration logic."

requirements-completed: [ROUTE-01]
requirements-blocked: []
status: complete

duration: 15min
completed: 2026-05-14
---

# Phase 06 Plan 01: Route Surface And Auth Guard Summary

**The live backend router now has regression coverage for session-required app routes, admin-only routes, and public auth entrypoints.**

## Accomplishments

- Added route-surface tests covering Agent, Memory, MCP, Notification, Quota, Console, Preferences, Knowledge, Task, and WebSocket app routes.
- Added admin route checks proving normal users receive `403 Forbidden` for Admin stats, channels, routes, plans, users, audit logs, and reviews.
- Preserved public `/api/v1/auth/register` and `/api/v1/auth/login` behavior.
- Added a direct `requireAdmin` middleware test proving the wrapped handler is not called for non-admin sessions.

## Task Commits

- Route and auth guard coverage: `ef81374` (`feat(06): harden backend mainline integration`)

## Verification

```bash
cd src/server && go test ./internal/http -count=1
env TEST_DATABASE_URL='postgres://oblivious:oblivious@127.0.0.1:32768/oblivious_test?sslmode=disable' go test ./internal/http -v -count=1
```

Both commands passed. The DB-backed run proved `TestRouteSurfaceRequiresSessionForAppRoutes`, `TestRouteSurfaceAdminRoutesRequireAdmin`, and `TestRouteSurfaceKeepsAuthRoutesPublic`.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Next Phase Readiness

ROUTE-01 route boundary coverage is ready for frontend/API contract reconciliation in later phases.
