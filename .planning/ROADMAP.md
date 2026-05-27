# Roadmap: Oblivious

## Milestones

- 🚧 **v04 Commercial Foundation** — Phase 9 through Phase 12 (planning started 2026-05-27)
- ✅ **v03.3 Mainline Consolidation** — Phase 5 through Phase 8 plus 999.1/999.2 follow-ups (completed 2026-05-27)
- ✅ **v03.2 Quality and Release** — Phase 4 (shipped 2026-05-14; Docker compose runtime validated 2026-05-12)
- ✅ **v03.1 Admin and Marketplace UI** — Phase 03.1 (shipped 2026-05-02)
- ✅ **Foundation through Admin/Marketplace Backend** — Phase 1, Phase 2, Phase 3a (completed 2026-04-27 through 2026-04-29)

## Current Status

Milestone v04 has been initialized from `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`.

**Next workflow step:** execute Phase 12 Commercial Gate CI and Evidence Plan 01.

## Current Milestone: v04 Commercial Foundation

**Goal:** Establish the SaaS tenant, security, migration, and CI foundation required before commercial Relay billing, Marketplace settlement, production operations, and final product completeness work.

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| Phase 9 | Tenant Model and Migration Ledger | TENANT-01, MIGR-01 | Complete |
| Phase 10 | Membership, Roles, and Auth Security | TENANT-02, TENANT-03, SEC-01, SEC-02, SEC-03 | Complete |
| Phase 11 | Tenant Scope Across Core Domains | TENANT-04, TENANT-05 | Complete |
| Phase 12 | Commercial Gate CI and Evidence | CI-01, DOC-03 | Ready to execute |

### Phase 9: Tenant Model and Migration Ledger

**Goal:** Add first-class organization tenancy and an append-only migration ledger without changing later billing or product surfaces prematurely.

**Requirements:** TENANT-01, MIGR-01

**Success criteria:**
1. New migrations create `schema_migrations` or equivalent migration ledger tables and organization tenant tables.
2. Migration execution records applied migration identity and is safe to rerun.
3. Admin/server code can create, list, inspect, update, and deactivate organizations through explicit service boundaries.
4. Organization identity is available to downstream handlers/services without relying on ad hoc user-only context.
5. Targeted DB-backed tests prove ledger idempotency and organization CRUD behavior.

**Likely verification:**
- `cd src/server && go test ./internal/... -run 'Migration|Organization|Tenant' -count=1`
- `cd src/server && TEST_DATABASE_URL=<postgres test db> go test ./... -count=1`
- `bash scripts/check.sh all`

**Completion evidence:**
- Implementation commits: `52b552f`, `25521ef`, `4293403`, `ef59081`, `b973106`
- Summary: `.planning/phases/09-tenant-model-and-migration-ledger/09-01-SUMMARY.md`
- DB-backed verification: migration ledger and organization lifecycle tests passed against PostgreSQL on `127.0.0.1:32768`
- Broad gates: `bash scripts/test.sh all` and `bash scripts/check.sh all` passed with `TEST_DATABASE_URL` and restricted-network Go proxy overrides

### Phase 10: Membership, Roles, and Auth Security

**Goal:** Make organization membership and production auth controls enforceable before tenant-scoped domain data migration.

**Requirements:** TENANT-02, TENANT-03, SEC-01, SEC-02, SEC-03

**Success criteria:**
1. Users can belong to multiple organizations with member/admin/owner roles.
2. Invitation, acceptance, removal, ownership transfer, and role changes are audited.
3. Cookie-authenticated mutating routes reject missing/invalid CSRF tokens.
4. Login, registration, password reset, and sensitive admin actions have enforced rate limits.
5. Password policy and session rotation are covered by targeted tests.

**Completion evidence:**
- Implementation commits: `991b17b`, `31dc646`, `79ad6cf`, `01e6e50`, `e1d6854`
- Build-gate fix commit: `67c729a`
- Summary: `.planning/phases/10-membership-roles-and-auth-security/10-01-SUMMARY.md`
- DB-backed verification: membership, invitation revoke/accept, role/ownership, audit, CSRF, rate-limit, password reset, and session rotation/revocation tests passed against PostgreSQL on `127.0.0.1:32769`
- Broad gates: `bash scripts/test.sh all` and `bash scripts/check.sh all` passed with `TEST_DATABASE_URL` and restricted-network Go proxy overrides

**Planning evidence:**
- Context: `.planning/phases/10-membership-roles-and-auth-security/10-CONTEXT.md`
- Discussion log: `.planning/phases/10-membership-roles-and-auth-security/10-DISCUSSION-LOG.md`
- Plan: `.planning/phases/10-membership-roles-and-auth-security/10-01-PLAN.md`

### Phase 11: Tenant Scope Across Core Domains

**Goal:** Apply tenant identity to core data domains and prove cross-tenant reads/writes fail.

**Requirements:** TENANT-04, TENANT-05

**Success criteria:**
1. Chat, Agent, Knowledge, Memory, MCP, Quota, Console, Admin, and Marketplace publisher data have tenant ownership semantics.
2. Handlers and services derive tenant scope from authenticated context instead of client-controlled identifiers.
3. Cross-tenant reads are denied for representative list/detail paths.
4. Cross-tenant writes are denied for representative create/update/delete paths.
5. Regression tests cover the representative core domains and fail if tenant filters are removed.

**Completion evidence:**
- Implementation commits: `6435e3e`, `75d104b`, `34dd33e`, `8d60ede`, `fd33cd3`, `4fba2d9`
- Summary: `.planning/phases/11-tenant-scope-across-core-domains/11-01-SUMMARY.md`
- DB-backed verification: cross-tenant HTTP tests passed for Chat, Knowledge, Console, Agent, Memory, MCP, Quota, Marketplace publisher data, and Admin audit organization visibility against PostgreSQL on `127.0.0.1:32770`
- Broad gates: `bash scripts/test.sh all` and `bash scripts/check.sh all` passed with `TEST_DATABASE_URL` and restricted-network Go proxy overrides

**Planning evidence:**
- Context: `.planning/phases/11-tenant-scope-across-core-domains/11-CONTEXT.md`
- Patterns: `.planning/phases/11-tenant-scope-across-core-domains/11-PATTERNS.md`
- Plan: `.planning/phases/11-tenant-scope-across-core-domains/11-01-PLAN.md`

### Phase 12: Commercial Gate CI and Evidence

**Goal:** Make v04 evidence reproducible and define the commercial gate contract for later v05-v08 milestones.

**Requirements:** CI-01, DOC-03

**Success criteria:**
1. CI server jobs run DB-backed HTTP integration tests and fail loudly when the database is unavailable.
2. Commercial gate documentation maps tenant, Relay, billing, product, security, operations, and verification gates to required evidence.
3. v04 verification records exact commands, environment class, DB migration status, passed checks, skipped checks, and accepted residual debt.
4. `bash scripts/check.sh all` and `bash scripts/test.sh all` are documented as v04 release gates with DB-backed coverage expectations.

**Likely verification:**
- CI workflow dry checks or local equivalent showing DB-backed test commands are wired.
- `bash scripts/check.sh all`
- `bash scripts/test.sh all`
- Targeted `rg` checks proving commercial gate docs no longer describe v04 as MVP/RC-only.

**Planning evidence:**
- Context: `.planning/phases/12-commercial-gate-ci-and-evidence/12-CONTEXT.md`
- Plan: `.planning/phases/12-commercial-gate-ci-and-evidence/12-01-PLAN.md`

## Traceability

| Requirement | Phase | Coverage |
|-------------|-------|----------|
| TENANT-01 | Phase 9 | Complete — organization tenant model and admin/service boundaries |
| MIGR-01 | Phase 9 | Complete — append-only migration ledger |
| TENANT-02 | Phase 10 | Complete — multi-organization membership and owner/admin/member roles |
| TENANT-03 | Phase 10 | Complete — invitation create/accept/revoke, removal, ownership transfer, and audit |
| SEC-01 | Phase 10 | Complete — CSRF protection for cookie-auth mutating routes |
| SEC-02 | Phase 10 | Complete — rate limits for auth/admin/organization sensitive actions |
| SEC-03 | Phase 10 | Complete — password policy and session rotation/revocation |
| TENANT-04 | Phase 11 | Complete — tenant scope across core domains |
| TENANT-05 | Phase 11 | Complete — cross-tenant denial tests |
| CI-01 | Phase 12 | Ready to execute — DB-backed CI integration guarantee |
| DOC-03 | Phase 12 | Ready to execute — commercial gate documentation |

## Archived Milestone Details

<details>
<summary>✅ v03.3 Mainline Consolidation — COMPLETE 2026-05-27</summary>

**Goal:** Make the current uncommitted mainline work coherent, verified, documented, and ready for clean commits.

**Requirements:** CONS-01, ROUTE-01, CHAT-06, AUTH-01, DEPLOY-02, DOC-02, VERIFY-01.

**Delivered:**
- Phase 5 dirty worktree triage and commit-boundary inventory.
- Phase 6 backend route/service/auth/Relay hardening.
- Phase 7 frontend, E2E, CI, Docker, and deployment gate alignment.
- Phase 8 contract docs and release verification.
- Phase 999.1 missing Phase 01 summary reconstruction.
- Phase 999.2 obsolete workspace MarketplacePage cleanup and living requirements close policy verification.

**Archive snapshots:**
- `.planning/milestones/v03.3-ROADMAP.md`
- `.planning/milestones/v03.3-REQUIREMENTS.md`
- `.planning/milestones/v03.3-STATE.md`

</details>

<details>
<summary>✅ v03.2 Quality and Release — SHIPPED 2026-05-14</summary>

**Goal:** 补齐集成测试、E2E、API 文档和部署发布能力。

**Requirements:** TEST-01, TEST-02, DOC-01, DEPLOY-01.

**Primary verification:**
- `bash scripts/check.sh all`
- `bash scripts/test.sh all`
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e`
- `docker compose config`
- `OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh`

**Archive:** `.planning/milestones/v03.2-ROADMAP.md`

</details>

<details>
<summary>✅ v03.1 Admin and Marketplace UI — SHIPPED 2026-05-02</summary>

**Goal:** 实现 Admin 管理面板 UI 和 Marketplace 前端页面（8 Admin + 4 Marketplace 页面合同）。

**Requirements:** ADMIN-04, MARKET-02.

**Verification:** Go handler suite, 12 focused Vitest files / 32 tests, and `tsc --noEmit` passed.

**Archive:** `.planning/milestones/v03.1-ROADMAP.md`

</details>

<details>
<summary>✅ Foundation through Admin/Marketplace Backend — COMPLETED 2026-04-27 to 2026-04-29</summary>

- [x] Phase 1 Relay/Chat/Agent/MCP foundation — RELAY-01~07, CHAT-01~05, AGENT-01~10, MCP-01~07.
- [x] Phase 2 Agent 与 Memory 增强 — EXEC-01~03, MEM-01~03, QUOTA-01.
- [x] Phase 3a Admin 与 Marketplace 后端 — ADMIN-01~03, MARKET-01.

</details>

## Progress

| Milestone | Scope | Plans | Requirements | Status | Completed |
|-----------|-------|-------|--------------|--------|-----------|
| v04 Commercial Foundation | Phases 9-12 | 3/4 phases planned, 2/4 complete | 7/11 requirements complete | Active | Phase 10 complete 2026-05-28 |
| v03.3 Mainline Consolidation | Phases 5-8 plus backlog 999.1 and 999.2 | 12/12 steps complete | 7/7 requirements complete | Complete | 2026-05-27 |
| v03.2 Quality and Release | Phase 4 | 4/4 | TEST-01, TEST-02, DOC-01, DEPLOY-01 | Shipped | 2026-05-14 |
| v03.1 Admin and Marketplace UI | Phase 03.1 | 7/7 | ADMIN-04, MARKET-02 | Shipped | 2026-05-02 |
| Foundation through Backend | Phases 1, 2, 3a | Historical | RELAY, CHAT, AGENT, MCP, MEM, EXEC, QUOTA, ADMIN, MARKET | Complete | 2026-04-29 |

---
*Roadmap updated: 2026-05-28 after planning Phase 12 commercial gate CI and evidence*
