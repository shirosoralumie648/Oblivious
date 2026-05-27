# Phase 9 Discussion Log

**Date:** 2026-05-27
**Mode:** Auto-locked defaults from commercial complete spec and current codebase evidence.

## Locked Decisions

1. Phase 9 proves only TENANT-01 and MIGR-01.
2. Use `organizations` as the first-class tenant table.
3. Use `/api/v1/admin/organizations` for admin organization management.
4. Implement migration ledger in the runner, not as a docs-only convention.
5. Require DB-backed tests for both migration ledger idempotency and organization CRUD.
6. Defer memberships, invitations, ownership transfer, CSRF, rate limits, password policy, session rotation, and cross-domain tenant scoping to later v04 phases.

## Reasoning

The final goal is commercial complete SaaS, not MVP. Tenant identity and migration safety are prerequisites for billing, Marketplace settlement, production operations, and cross-tenant isolation, so Phase 9 must establish those foundations without prematurely touching downstream domains.

---

*Phase: 09-tenant-model-and-migration-ledger*
