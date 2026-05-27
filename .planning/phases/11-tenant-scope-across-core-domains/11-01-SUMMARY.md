---
phase: 11-tenant-scope-across-core-domains
plan: 01
status: complete
completed_at: 2026-05-28T04:40:00+08:00
requirements: [TENANT-04, TENANT-05]
commits:
  - 6435e3e feat(11): add active organization session scope
  - 75d104b feat(11): tenant scope chat knowledge console
  - 34dd33e test(relay): stabilize weighted load balancer test
  - 8d60ede feat(11): tenant scope agent memory mcp
  - fd33cd3 feat(11): tenant scope quota billing records
  - 4fba2d9 feat(11): tenant scope marketplace publisher data
---

# Phase 11 Plan 01 Summary

## Result

Implemented tenant scoping across the v04 core domains and proved representative cross-tenant reads and writes fail under DB-backed HTTP tests.

Delivered:
- `0027_tenant_scope_core_domains.sql` with `organization_id` additions, legacy backfill, NOT NULL promotion, tenant indexes, quota/idempotency uniqueness updates, and audit backfill.
- Session-derived active organization context through Auth and Tenant scope resolution.
- Organization-scoped Chat, Knowledge, Console usage, Agent, Memory, MCP, Quota, billing-session, subscription, top-up, Marketplace publisher, install, review, analytics, and Admin audit paths.
- DB-backed cross-tenant HTTP tests covering representative read/write denial for Chat, Knowledge, Console, Agent, Memory, MCP, Quota, Marketplace publisher data, and Admin audit organization visibility.
- Audit logs now carry `organization_id` for tenant-owned organization mutations and Marketplace review actions while platform-global Admin channel/route/plan resources remain global.

## Verification

Environment:
- PostgreSQL test container: `oblivious-phase11-postgres`
- `TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32770/oblivious_test?sslmode=disable`
- Go proxy override: `GOPROXY=https://mirrors.aliyun.com/goproxy/,direct`
- Go checksum DB override: `GOSUMDB=sum.golang.google.cn`

Passed:
- `cd src/server && go test ./internal/marketplace ./internal/http -run 'CrossTenant.*Marketplace|Marketplace|Publisher' -count=1`
- `cd src/server && TEST_DATABASE_URL=... go test ./internal/http -run 'CrossTenant.*Marketplace' -count=1`
- `cd src/server && go test ./internal/marketplace ./internal/admin ./internal/http -run 'CrossTenant|TenantScope|Marketplace|Publisher|Audit|RouteSurface' -count=1`
- `cd src/server && TEST_DATABASE_URL=... go test ./internal/marketplace ./internal/admin ./internal/http -run 'CrossTenant|TenantScope|Marketplace|Publisher|Audit|RouteSurface|OrganizationInvitationLifecycle' -count=1`
- `cd src/server && go test ./internal/marketplace -count=1`
- `cd src/server && go test ./internal/admin -count=1`
- `cd src/server && go test ./cmd/migrate ./internal/... -count=1`
- `cd src/server && TEST_DATABASE_URL=... go test ./cmd/migrate ./internal/... -count=1`
- `GOPROXY=... GOSUMDB=... TEST_DATABASE_URL=... bash scripts/test.sh all`
- `GOPROXY=... GOSUMDB=... TEST_DATABASE_URL=... bash scripts/check.sh all`
- `git diff --check`

Additional focused evidence:
- Manual SQL smoke during Task 4 confirmed `0027_tenant_scope_core_domains.sql` merges duplicate legacy user quota rows into one organization quota row.
- `scripts/test.sh all` passed 32 frontend test files / 110 frontend tests, then server unit and DB-backed HTTP integration tests.
- `scripts/check.sh all` passed release-asset checks, docs/env consistency checks, web production build, and server release checks.

## Requirement Closure

| Requirement | Evidence |
|-------------|----------|
| TENANT-04 | Migration and code carry organization identity across Chat, Agent, Knowledge, Memory, MCP, Quota, Console usage, Admin tenant audit data, and Marketplace publisher/install/review/analytics data |
| TENANT-05 | DB-backed HTTP tests prove representative cross-tenant reads and writes are denied and denied writes leave other-tenant rows unchanged |

## Residual Commercial Work

- Phase 11 does not complete v05 Relay Billing Completeness: `/v1/*` endpoint classification, production fail-closed unsupported endpoints, per-endpoint billing/rate-limit/audit behavior, and direct-provider bypass CI checks remain future work.
- Phase 11 does not complete v06 Billing And Marketplace Operations: Stripe production routes/webhooks, subscription lifecycle, invoices/refunds/top-ups, Marketplace settlement, payout state, and moderation remain future work.
- Phase 11 does not complete v07 Production Operations: Kubernetes/equivalent orchestration proof, backup/restore smoke, logs, metrics, tracing, alerts, dashboards, runbooks, and rollback remain future work.
- Phase 11 does not complete v08 Product Completeness: real or disabled built-in MCP tools, durable Agent workflows, Knowledge behavior matching product copy, commercial UX/docs/onboarding/pricing/operator guides, and full commercial journeys remain future work.
- Phase 12 still owns CI and commercial gate evidence so future milestones cannot claim commercial readiness without current repository evidence, DB-backed verification, and runtime smoke where applicable.

## Next Phase Readiness

Phase 12 can now plan the reproducible v04 evidence gate on top of:
- first-class organizations and migration ledger from Phase 9;
- membership/auth security controls from Phase 10;
- tenant-scoped core data and cross-tenant denial tests from Phase 11.

---

*Summary written: 2026-05-28*
