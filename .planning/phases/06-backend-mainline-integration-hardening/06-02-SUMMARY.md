---
phase: 06-backend-mainline-integration-hardening
plan: 02
subsystem: notifications
tags: [notifications, ownership, authz, regression-tests]

requires:
  - phase: 06-01-route-surface-and-auth-guard
    provides: Session-gated notification route coverage
provides:
  - User-owned notification read helper
  - Owner checks for notification mark-read and delete mutations
  - Handler and service regression tests for cross-user mutation denial
affects: [ROUTE-01, Phase 7, Phase 8]

tech-stack:
  added: []
  patterns:
    - Load notification ownership before mutation
    - Return `403 Forbidden` for valid-session cross-user mutation attempts

key-files:
  created:
    - src/server/internal/notification/service.go
    - src/server/internal/notification/service_test.go
    - src/server/internal/http/notification_handler.go
    - src/server/internal/http/notification_handler_test.go
  modified: []

key-decisions:
  - "Notification mutation authorization is enforced before write operations, not inferred from route session checks alone."
  - "Owner and non-owner outcomes are covered at handler level with real HTTP sessions."

patterns-established:
  - "Mutation handlers for user-owned resources should load the record and compare `UserID` before mutating."

requirements-completed: [ROUTE-01]
requirements-blocked: []
status: complete

duration: 15min
completed: 2026-05-14
---

# Phase 06 Plan 02: Notification Ownership Summary

**Notification mark-read and delete paths now deny cross-user mutations while preserving owner success behavior.**

## Accomplishments

- Added `notification.Service.Get` so handlers can inspect record ownership before mutation.
- Hardened notification `PATCH` and `DELETE` handlers to return `404` for missing records and `403` for non-owner access.
- Added service and HTTP regression tests proving owner success and non-owner denial for notification mutations.
- Kept list, unread count, mark-all-read, and create behavior unchanged.

## Task Commits

- Notification ownership hardening: `ef81374` (`feat(06): harden backend mainline integration`)

## Verification

```bash
cd src/server && go test ./internal/http ./internal/notification -count=1
env TEST_DATABASE_URL='postgres://oblivious:oblivious@127.0.0.1:32768/oblivious_test?sslmode=disable' go test ./internal/http -v -count=1
```

Both commands passed. The DB-backed run proved `TestNotificationMutationRoutesEnforceOwnership` with `403` for non-owner mutation attempts and `200` for owner mutation attempts.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Next Phase Readiness

Notification route and ownership behavior is ready for frontend state/API type alignment in Phase 7 and documentation reconciliation in Phase 8.
