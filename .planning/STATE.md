# STATE.md

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-04)

**Core value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

**Current focus:** Milestone v03.2 Quality and Release is blocked on DEPLOY-01 runtime validation. TEST-01, TEST-02, and DOC-01 are complete; deployment config exists but real Docker/Kubernetes startup evidence is missing.

## Current Status

**Milestone v03.2: Quality and Release — BLOCKED**

## Current Position

| Field | Value |
|-------|-------|
| Phase | Phase 4: 质量与发布 |
| Plan | 04-01 through 04-03 complete; 04-04 config complete but runtime validation blocked |
| Status | Blocked on Docker daemon / kubectl availability |
| Progress | 0/1 phases, 3/4 requirements complete, 1 blocked |
| Last activity | 2026-05-04 — completion audit found DEPLOY-01 lacks real Docker/Kubernetes startup evidence |

## Current Scope

| Requirement | Target |
|-------------|--------|
| TEST-01 | Complete — broad backend release gate and boundary tests |
| TEST-02 | Complete — E2E tests for Admin and Marketplace browser workflows |
| DOC-01 | Complete — API documentation and release checklist |
| DEPLOY-01 | Blocked — config and smoke path exist; real stack startup/healthcheck not verified |

## Previous Milestone Baseline

**Milestone v03.1: Admin and Marketplace UI — COMPLETE**

| Area | Status | Evidence |
|------|--------|----------|
| Admin API/UI | Complete | `.planning/phases/03.1-admin-marketplace-ui/03.1-UAT.md` tests 1-6 |
| Marketplace API/UI | Complete | `.planning/phases/03.1-admin-marketplace-ui/03.1-UAT.md` test 7 |
| Security | Verified | `.planning/phases/03.1-admin-marketplace-ui/03.1-SECURITY.md` (`threats_open: 0`) |
| Validation | Compliant | `.planning/phases/03.1-admin-marketplace-ui/03.1-VALIDATION.md` (`nyquist_compliant: true`) |
| Milestone audit | Complete with tech debt | `.planning/v03.1-MILESTONE-AUDIT.md` |

## Verification Results

Latest v03.2 completion audit:

```bash
bash scripts/check.sh docs
docker compose config
bash scripts/check.sh all
bash scripts/test.sh all
BASE_URL=http://127.0.0.1:18080 bash scripts/deploy-smoke.sh
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e
bash -n scripts/deploy-validate.sh
bash scripts/deploy-validate.sh
docker build -f Dockerfile.server -t oblivious-server:local .
kubectl version --client
```

Result: docs quality gate passed; `docker compose config` passed; full check gate passed in the approved non-sandbox path; full test gate passed in the approved non-sandbox path with 32 web test files / 110 tests and server `go test ./... -count=1`; Admin/Marketplace Playwright E2E passed 3/3; DB-backed HTTP integration tests skipped explicitly because `TEST_DATABASE_URL` was not set; deployment smoke script passed against a temporary local `/healthz` stub; `scripts/deploy-validate.sh` passed shell syntax validation and fails clearly when Docker daemon access is unavailable.

Blocked checks: Docker image builds could not run because the local Docker daemon socket denied access; starting a temporary `dockerd` failed because root privileges are required; Docker Desktop build socket/buildx also report permission errors; Kubernetes apply/dry-run could not run because `kubectl` is not installed. These are recorded in `.planning/phases/04-quality-release/04-COMPLETION-AUDIT.md`.

Latest v03.2 TEST-02 gate:

```bash
COREPACK_HOME=.tmp/corepack pnpm --dir src/web install --frozen-lockfile
COREPACK_HOME=.tmp/corepack pnpm --dir src/web exec playwright install chromium
COREPACK_HOME=.tmp/corepack pnpm --dir src/web exec playwright test --list
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e
COREPACK_HOME=.tmp/corepack pnpm --dir src/web test
COREPACK_HOME=.tmp/corepack pnpm --dir src/web build
```

Result: all commands passed. Playwright listed 3 Admin/Marketplace tests; E2E passed 3/3; Web Vitest passed 32 files / 110 tests; Web build passed.

Latest v03.2 DOC-01 gate:

```bash
rg -n "## Admin Endpoints|## Marketplace Endpoints|## Relay /v1 Endpoints" docs/API.md
rg -n "/api/v1/admin/stats|/api/v1/marketplace/featured|/marketplace/my-agents|COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e|lobehub/|new-api/" docs/architecture/current-system-contracts.md
rg -n "## Admin Endpoints|## Marketplace Endpoints|## Relay /v1 Endpoints|TEST_DATABASE_URL|pnpm --dir src/web test:e2e" scripts/verify-quality-gates.sh
bash scripts/check.sh docs
```

Result: all commands passed. API docs, current system contracts, RC checklist, README release links, and docs quality-gate assertions are aligned for DOC-01.

Latest v03.2 TEST-01 gate:

```bash
bash scripts/check.sh server
bash scripts/test.sh server
cd src/server && go test ./internal/http ./internal/admin ./internal/marketplace ./internal/relay ./internal/agent ./internal/memory ./internal/quota -count=1
```

Result: all commands passed. `scripts/test.sh server` skipped DB-backed HTTP integration tests explicitly because `TEST_DATABASE_URL` was not set.

Latest v03.1 gate:

```bash
cd src/server && go test ./internal/http -count=1 -run 'Test(AdminHandlerExposesPhase31Operations|MarketplaceHandlerExposesPublicAndSessionOperations)'
cd src/web && npx vitest run src/features/admin/api.test.ts src/features/marketplace/api.test.ts src/components/shared/shared-components.test.tsx src/features/layouts/AdminSidebar.test.tsx src/routes/admin/AdminHomePage.test.tsx src/routes/admin/AdminChannelsPage.test.tsx src/routes/admin/AdminRoutesPage.test.tsx src/routes/admin/AdminPlansPage.test.tsx src/routes/admin/AdminUsersPage.test.tsx src/routes/admin/AdminAuditLogPage.test.tsx src/routes/admin/AdminReviewsPage.test.tsx src/routes/marketplace/MarketplacePage.test.tsx
cd src/web && npx tsc --noEmit
```

Result: Go handler suite passed; Vitest targeted suite passed (12 files, 32 tests); TypeScript compile passed.

## Completed Work

| Milestone | Completed | Requirements |
|-----------|-----------|--------------|
| Phase 1 Relay/Chat/Agent/MCP foundation | 2026-04-27 | RELAY-01~07, CHAT-01~05, AGENT-01~10, MCP-01~07 |
| Phase 2 Agent 与 Memory 增强 | 2026-04-28 | EXEC-01~03, MEM-01~03, QUOTA-01 |
| Phase 3a Admin 与 Marketplace 后端 | 2026-04-29 | ADMIN-01~03, MARKET-01 |
| v03.1 Admin 与 Marketplace UI | 2026-05-02 | ADMIN-04, MARKET-02 |
| v03.2 Quality and Release | Blocked 2026-05-04 | TEST-01, TEST-02, DOC-01 complete; DEPLOY-01 runtime validation blocked |

## Deferred Items

Items acknowledged and deferred at v03.1 milestone close on 2026-05-02:

| Category | Item | Status |
|----------|------|--------|
| planning | Phase 01 missing SUMMARY artifact | Backlog 999.1 |
| cleanup | Legacy workspace MarketplacePage no longer routed by `/marketplace` | Backlog 999.2 |
| workflow | Decide whether `.planning/REQUIREMENTS.md` should be reset on future milestone closes in this repo | Backlog 999.2 |

## Next Suggested Step

Restore Docker daemon access or install/provide Kubernetes tooling, then run `bash scripts/deploy-validate.sh` or an equivalent real deployment smoke. Do not archive v03.2 until DEPLOY-01 has real stack startup evidence.

## Context Files

- Project: `.planning/PROJECT.md`
- Requirements: `.planning/REQUIREMENTS.md`
- Roadmap: `.planning/ROADMAP.md`
- Phase 4 context: `.planning/phases/04-quality-release/04-CONTEXT.md`
- Phase 4 plans: `.planning/phases/04-quality-release/04-01-PLAN.md` through `04-04-PLAN.md`
- Phase 4 summaries: `.planning/phases/04-quality-release/04-01-SUMMARY.md` through `04-04-SUMMARY.md`
- Phase 4 audit: `.planning/phases/04-quality-release/04-COMPLETION-AUDIT.md`
- Milestones: `.planning/MILESTONES.md`
- Milestone archive: `.planning/milestones/v03.1-ROADMAP.md`
- Requirements archive: `.planning/milestones/v03.1-REQUIREMENTS.md`
- Milestone audit: `.planning/milestones/v03.1-MILESTONE-AUDIT.md`
- Codebase Map: `.planning/codebase/`

## Key Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-27 | 从 Phase 1 开始执行 | Relay 集成是所有后续工作的基础 |
| 2026-05-02 | 完成 v03.1 时保留 living REQUIREMENTS.md | 当前仓库仍依赖该文件承载跨阶段上下文，直接删除风险高 |
| 2026-05-02 | 将 obsolete workspace MarketplacePage 作为 backlog 清理项 | 避免在脏工作树里删除可能仍被用户保留的旧入口 |
| 2026-05-02 | v03.2 跳过新领域研究 | 质量/发布工作由已交付代码和既有 Phase 4 需求限定 |
| 2026-05-02 | Phase 4 context 采用 auto 默认决策 | `$gsd-next` 零确认推进，质量/发布灰区可由现有代码和发布目标确定 |
| 2026-05-02 | Phase 4 拆成四个执行计划 | TEST-01、TEST-02、DOC-01、DEPLOY-01 各自有明确执行和验收边界 |
| 2026-05-02 | 04-01 将 server release gate 扩展为 `go test ./... -count=1` | 避免窄包集合掩盖 Admin、Marketplace、Relay、Agent、Memory、Quota 回归风险 |
| 2026-05-02 | 04-02 采用 Playwright + route fixtures | 浏览器 E2E 覆盖 Admin/Marketplace 真实路由，同时避免 live provider、Stripe 或外部服务依赖 |
| 2026-05-04 | 04-03 将 `docs/API.md` 作为 canonical API index | 文件已覆盖当前 live route surface；质量门禁改为防漂移而非重复重写 |
| 2026-05-04 | RC checklist 要求显式记录 `TEST_DATABASE_URL` skip | DB-backed integration tests 可以按脚本规则跳过，但 release evidence 必须说明原因 |
| 2026-05-04 | Kubernetes Namespace 门禁检查 `metadata.name` 而不是 `namespace` 字段 | Namespace 资源不应要求无效的 namespace-scoped 字段 |
| 2026-05-04 | Docker/kubectl 运行验证不能作为已完成项 | 当前 session 无 Docker daemon 权限且无 kubectl；compose parsing 与 stub smoke 不足以证明服务栈可启动 |

---
*State updated: 2026-05-04 after completion audit*
