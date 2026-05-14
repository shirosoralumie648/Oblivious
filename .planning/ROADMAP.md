# Roadmap: Oblivious

## Milestones

- ✅ **v03.2 Quality and Release** — Phase 4 (shipped 2026-05-14; Docker compose runtime validated 2026-05-12)
- ✅ **v03.1 Admin and Marketplace UI** — Phase 03.1 (shipped 2026-05-02)
- ✅ **Foundation through Admin/Marketplace Backend** — Phase 1, Phase 2, Phase 3a (completed 2026-04-27 through 2026-04-29)

## Current Status

Milestone v03.2 is archived. The next workflow step is `$gsd:new-milestone` to define the next version's goals, requirements, and roadmap.

## Archived Milestone Details

<details>
<summary>✅ v03.2 Quality and Release — SHIPPED 2026-05-14</summary>

**Goal:** 补齐集成测试、E2E、API 文档和部署发布能力。

**Requirements:** TEST-01, TEST-02, DOC-01, DEPLOY-01.

**Plans:** 4/4 complete.

- [x] 04-01: Backend release test gate — `go test ./... -count=1` covers Admin, Marketplace, Relay, Agent, Memory, and Quota boundaries.
- [x] 04-02: Admin and Marketplace E2E gate — Playwright covers Admin navigation, Marketplace browse/detail/install, publish, and my-agents flows.
- [x] 04-03: API documentation and RC checklist — API docs, system contracts, README links, and quality-gate assertions aligned.
- [x] 04-04: Deployment configuration and smoke path — Docker compose build/start/smoke passed against the real `/healthz` endpoint with documented restricted-network overrides.

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

**Plans:** 7/7 complete.

**Verification:** Go handler suite, 12 focused Vitest files / 32 tests, and `tsc --noEmit` passed. UAT, security (`threats_open: 0`), Nyquist validation, and milestone audit are complete.

**Archive:** `.planning/milestones/v03.1-ROADMAP.md`

</details>

<details>
<summary>✅ Foundation through Admin/Marketplace Backend — COMPLETED 2026-04-27 to 2026-04-29</summary>

- [x] Phase 1 Relay/Chat/Agent/MCP foundation — RELAY-01~07, CHAT-01~05, AGENT-01~10, MCP-01~07.
- [x] Phase 2 Agent 与 Memory 增强 — EXEC-01~03, MEM-01~03, QUOTA-01.
- [x] Phase 3a Admin 与 Marketplace 后端 — ADMIN-01~03, MARKET-01.

These phases remain available in `.planning/phases/` and the v03.2 roadmap archive for historical context.

</details>

## Progress

| Milestone | Scope | Plans | Requirements | Status | Completed |
|-----------|-------|-------|--------------|--------|-----------|
| v03.2 Quality and Release | Phase 4 | 4/4 | TEST-01, TEST-02, DOC-01, DEPLOY-01 | Shipped | 2026-05-14 |
| v03.1 Admin and Marketplace UI | Phase 03.1 | 7/7 | ADMIN-04, MARKET-02 | Shipped | 2026-05-02 |
| Foundation through Backend | Phases 1, 2, 3a | Historical | RELAY, CHAT, AGENT, MCP, MEM, EXEC, QUOTA, ADMIN, MARKET | Complete | 2026-04-29 |

## Backlog

### Phase 999.1: Follow-up — Phase 01 incomplete plan artifacts (BACKLOG)

**Goal**: 补齐 Phase 01 缺失的执行总结工件，避免后续阶段缺少完成记录
**Source phase:** `01-relay-integration`
**Deferred at:** `2026-04-27` during `$gsd-next` advancement to `02-agent-memory-enhancement`
**Plans:**
- [ ] `01-relay-integration/PLAN.md` ran to verification completion but has no matching `SUMMARY.md`
**Notes:**
- [ ] Reconstruct a concise `SUMMARY.md` from `PLAN.md` + `VERIFICATION.md`
- [ ] Confirm whether missing P0/P1 tests should be promoted into an active follow-up phase

### Phase 999.2: Follow-up — Phase 03.1 accepted cleanup debt (BACKLOG)

**Goal:** Clean up non-blocking debt accepted at v03.1 milestone close
**Source phase:** `03.1-admin-marketplace-ui`
**Deferred at:** `2026-05-02` during `$gsd-next` milestone completion
**Items:**
- [ ] Decide whether to delete, migrate, or document `src/web/src/routes/workspace/MarketplacePage.tsx`, which is no longer routed by `/marketplace`
- [ ] Decide whether future milestone completion should reset `.planning/REQUIREMENTS.md` or keep it as cross-phase context in this repo

---
*Roadmap updated: 2026-05-14 after v03.2 milestone archive*
