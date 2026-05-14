---
gsd_state_version: 1.0
milestone: v03.3
milestone_name: Mainline Consolidation
status: planning
last_updated: "2026-05-14T05:59:12.705Z"
last_activity: 2026-05-14 — Phase 5 context gathered
progress:
  total_phases: 6
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
---

# STATE.md

## Project Reference

See: `.planning/PROJECT.md` (updated 2026-05-14)

**Core value:** 统一的多渠道 LLM 调用层 — 所有 AI 调用必须经过 Relay

**Current focus:** Milestone v03.3 Mainline Consolidation — consolidate and verify the current uncommitted mainline work.

## Current Status

**Milestone v03.3: Mainline Consolidation — PLANNING**

The repository has substantial uncommitted source, docs, deployment, and frontend test changes. v03.3 is scoped to classifying that work, hardening the backend/frontend/deployment contracts, reconciling docs, and producing clean commit boundaries.

## Current Position

Phase: 5 — Dirty Worktree Triage and Commit Boundary
Plan: —
Status: Ready to plan Phase 5
Last activity: 2026-05-14 — Phase 5 context gathered

## Current Scope

| Requirement | Target |
|-------------|--------|
| CONS-01 | Classify current uncommitted source/docs into coherent work slices |
| ROUTE-01 | Register Agent, Memory, MCP, Notification, Quota, and WebSocket routes/services with explicit auth boundaries and tests |
| CHAT-06 | Preserve Relay-first Chat/Agent structured tool calls, streaming behavior, metadata, and usage hooks |
| AUTH-01 | Keep user name/role/session contracts consistent while preserving admin boundaries |
| DEPLOY-02 | Align Docker, compose, CI, and Playwright with v03.2 restricted-network runtime proof |
| DOC-02 | Reconcile API, architecture, release, README, and env docs against live routes and commands |
| VERIFY-01 | Document and run targeted verification before committing consolidated work |

## Worktree Context

Current mainline changes are already present before Phase 5 starts. Treat them as inputs to triage, not as already-accepted milestone output.

Observed change areas:

- Tracked CI, Docker, compose, env, release docs, server auth/chat/config/http/userprefs, and web package/theme/types/tailwind/vite changes.
- Untracked backend route handlers and route split files for Agent, Memory, MCP, Notification, Quota, WebSocket, Relay store, migrations `0013`-`0019`, Playwright config/E2E files, and new workspace pages.
- Root historical/reference docs such as `CURRENT_STATUS.md` and `ROADMAP.md` should not override `.planning` state unless deliberately incorporated.

## Previous Milestone Baseline

**Milestone v03.2: Quality and Release — COMPLETE**

| Area | Status | Evidence |
|------|--------|----------|
| Backend release gate | Complete | `bash scripts/check.sh all`, `bash scripts/test.sh all`, server `go test ./... -count=1` |
| Browser E2E | Complete | `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e` passed 3/3 Admin/Marketplace tests |
| Docs and release checklist | Complete | API docs, system contracts, README release links, and docs gate aligned |
| Runtime deployment path | Complete | Docker compose build/start/smoke passed with restricted-network overrides |

Validated Docker runtime command:

```bash
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

Result: Docker compose built server/web images, started PostgreSQL, Redis, `oblivious-server`, and `oblivious-web`, passed `/healthz` smoke against `http://127.0.0.1:8080/healthz`, and cleaned up the stack.

Environment notes:

- Direct Docker Hub access from the Docker daemon remains unreliable in this host environment.
- Default Go module download paths were unreliable during image builds.
- `kubectl` is unavailable, so Kubernetes remains an unexecuted alternate deployment path.

## Completed Work

| Milestone | Completed | Requirements |
|-----------|-----------|--------------|
| Phase 1 Relay/Chat/Agent/MCP foundation | 2026-04-27 | RELAY-01~07, CHAT-01~05, AGENT-01~10, MCP-01~07 |
| Phase 2 Agent 与 Memory 增强 | 2026-04-28 | EXEC-01~03, MEM-01~03, QUOTA-01 |
| Phase 3a Admin 与 Marketplace 后端 | 2026-04-29 | ADMIN-01~03, MARKET-01 |
| v03.1 Admin 与 Marketplace UI | 2026-05-02 | ADMIN-04, MARKET-02 |
| v03.2 Quality and Release | 2026-05-14 | TEST-01, TEST-02, DOC-01, DEPLOY-01 |

## Deferred Items

Items acknowledged before v03.3:

| Category | Item | Status |
|----------|------|--------|
| planning | Phase 01 missing SUMMARY artifact | Backlog 999.1 |
| cleanup | Legacy workspace MarketplacePage no longer routed by `/marketplace` | Backlog 999.2 |
| workflow | Decide whether `.planning/REQUIREMENTS.md` should be reset on future milestone closes in this repo | Backlog 999.2 |

## Next Suggested Step

Run `$gsd:plan-phase 5` to create the execution plan for Phase 5: Dirty Worktree Triage and Commit Boundary.

## Context Files

- Project: `.planning/PROJECT.md`
- Requirements: `.planning/REQUIREMENTS.md`
- Roadmap: `.planning/ROADMAP.md`
- Phase 5 context: `.planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-CONTEXT.md`
- Milestones: `.planning/MILESTONES.md`
- Milestone archive: `.planning/milestones/v03.2-ROADMAP.md`
- Requirements archive: `.planning/milestones/v03.2-REQUIREMENTS.md`
- Milestone audit: `.planning/milestones/v03.2-MILESTONE-AUDIT.md`
- Previous phase artifacts: `.planning/phases/`
- Codebase Map: `.planning/codebase/`

## Key Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-27 | 从 Phase 1 开始执行 | Relay 集成是所有后续工作的基础 |
| 2026-05-02 | 完成 v03.1 时保留 living REQUIREMENTS.md | 当前仓库仍依赖该文件承载跨阶段上下文，直接删除风险高 |
| 2026-05-02 | 将 obsolete workspace MarketplacePage 作为 backlog 清理项 | 避免在脏工作树里删除可能仍被用户保留的旧入口 |
| 2026-05-02 | v03.2 跳过新领域研究 | 质量/发布工作由已交付代码和既有 Phase 4 需求限定 |
| 2026-05-12 | DEPLOY-01 通过 Docker compose 真实运行验证 | 使用 documented registry / Go proxy overrides 后，`scripts/deploy-validate.sh` 完成镜像构建、compose 启动和 `/healthz` smoke |
| 2026-05-14 | v03.3 聚焦 Mainline Consolidation | 工作树已有大批主线改动，下一步应先分类、验证、对齐文档和提交边界 |
| 2026-05-14 | v03.3 跳过新研究 | 当前范围来自本地代码/文档状态，不是新产品方向探索 |

---
*State updated: 2026-05-14 after v03.3 milestone initialization*
