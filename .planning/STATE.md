# STATE.md

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-02)

**Core value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

**Current focus:** Milestone v03.2 Quality and Release; Phase 4 execution in progress. Plan 04-01 backend release gate complete; 04-02 E2E next.

## Current Status

**Milestone v03.2: Quality and Release — IN PROGRESS**

## Current Position

| Field | Value |
|-------|-------|
| Phase | Phase 4: 质量与发布 |
| Plan | 04-01 complete; 04-02 next |
| Status | Executing quality/release plans |
| Progress | 0/1 phases, 1/4 requirements complete |
| Last activity | 2026-05-02 — `$gsd-next` completed 04-01 backend release gate |

## Current Scope

| Requirement | Target |
|-------------|--------|
| TEST-01 | Complete — broad backend release gate and boundary tests |
| TEST-02 | E2E tests for Admin and Marketplace browser workflows |
| DOC-01 | API documentation and release checklist |
| DEPLOY-01 | Docker/Kubernetes startup and validation path |

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

## Deferred Items

Items acknowledged and deferred at v03.1 milestone close on 2026-05-02:

| Category | Item | Status |
|----------|------|--------|
| planning | Phase 01 missing SUMMARY artifact | Backlog 999.1 |
| cleanup | Legacy workspace MarketplacePage no longer routed by `/marketplace` | Backlog 999.2 |
| workflow | Decide whether `.planning/REQUIREMENTS.md` should be reset on future milestone closes in this repo | Backlog 999.2 |

## Next Suggested Step

Run `$gsd-execute-phase 4` to continue with plan 04-02 E2E coverage.

## Context Files

- Project: `.planning/PROJECT.md`
- Requirements: `.planning/REQUIREMENTS.md`
- Roadmap: `.planning/ROADMAP.md`
- Phase 4 context: `.planning/phases/04-quality-release/04-CONTEXT.md`
- Phase 4 plans: `.planning/phases/04-quality-release/04-01-PLAN.md` through `04-04-PLAN.md`
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

---
*State updated: 2026-05-02 starting v03.2 milestone*
