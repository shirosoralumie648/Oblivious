# Roadmap: Oblivious

## Milestones

- 🚧 **v03.3 Mainline Consolidation** — Phase 5 through Phase 8 (planning started 2026-05-14)
- ✅ **v03.2 Quality and Release** — Phase 4 (shipped 2026-05-14; Docker compose runtime validated 2026-05-12)
- ✅ **v03.1 Admin and Marketplace UI** — Phase 03.1 (shipped 2026-05-02)
- ✅ **Foundation through Admin/Marketplace Backend** — Phase 1, Phase 2, Phase 3a (completed 2026-04-27 through 2026-04-29)

## Current Status

Milestone v03.3 has completed Phase 7 frontend/E2E/deployment alignment. Frontend/API types, Playwright coverage, CI workflow, Dockerfiles, compose, env templates, and restricted-network deployment validation now have runtime-backed proof. Phase 8 is ready to discuss and should reconcile docs plus final release-verification evidence before the consolidated mainline is committed further.

**Next workflow step:** `$gsd:discuss-phase 8`

## Current Milestone: v03.3 Mainline Consolidation

**Goal:** Make the current uncommitted mainline work coherent, verified, documented, and ready for clean commits.

| Phase | Name | Requirements | Status |
|-------|------|--------------|--------|
| Phase 5 | Dirty Worktree Triage and Commit Boundary | CONS-01 | Complete |
| Phase 6 | Backend Mainline Integration Hardening | ROUTE-01, CHAT-06, AUTH-01 | Complete |
| Phase 7 | Frontend, E2E, and Deployment Gate Alignment | DEPLOY-02 | Complete |
| Phase 8 | Contract Docs and Release Verification | DOC-02, VERIFY-01 | Ready to discuss |

### Phase 5: Dirty Worktree Triage and Commit Boundary

**Goal:** Classify the existing uncommitted work into coherent slices before implementation or verification continues.

**Requirements:** CONS-01

**Success criteria:**
1. Maintainer can see which changed files belong to backend integration, frontend/E2E, deployment/CI, documentation, or historical/reference material.
2. Planning commits remain separate from source-code commits.
3. Obvious generated/cache artifacts stay ignored or excluded without deleting user-owned source.
4. Follow-up phase scopes can reference an explicit commit-boundary inventory.

**Starting evidence:**
- Current status includes tracked changes across CI, Docker, docs, server auth/chat/config/http/userprefs, web package/theme/types/tailwind/vite files.
- Current status includes untracked route/service additions under `src/server/internal/*`, migrations `0013`-`0019`, Playwright files, and several web route pages.

### Phase 6: Backend Mainline Integration Hardening

**Goal:** Verify backend route registration, service boundaries, Relay-first Chat/Agent behavior, and auth/session contracts.

**Requirements:** ROUTE-01, CHAT-06, AUTH-01

**Plans:** 4/4 complete

**Waves:**

- Wave 1: Plans 01-02 — route surface / auth guard coverage and notification ownership hardening
- Wave 2: Plan 03 — Relay metadata propagation, structured tool calls, and production fallback policy
- Wave 3: Plan 04 — auth/session/userprefs response consistency and final verification

**Cross-cutting constraints:**

- Keep session gating and `requireAdmin` intact across route and notification work.
- Preserve Relay metadata, structured tool-call handling, and usage recording separation in Chat/Agent flows.
- Keep auth response shaping and user preference defaults deterministic across register, login, and `/api/v1/auth/me`.

**Success criteria:**
1. Agent, Memory, MCP, Notification, Quota, and WebSocket routes are registered intentionally with the expected middleware/auth boundaries.
2. Chat and Agent gateway paths preserve Relay metadata, structured tool call handling, streaming behavior, and usage accounting hooks.
3. User/session data exposes name and role consistently across auth store/service/middleware without weakening admin checks.
4. Targeted Go tests cover the changed route/service contracts and fail on route/auth regressions.

**Likely verification:**
- `cd src/server && go test ./internal/http ./internal/auth ./internal/chat ./internal/userprefs ./internal/relay ./internal/agent ./internal/memory ./internal/quota -count=1`
- Broaden to `cd src/server && go test ./... -count=1` before closing the phase.

**Completion evidence:**
- Backend implementation commit: `ef81374` (`feat(06): harden backend mainline integration`)
- Phase summaries: `06-01-SUMMARY.md` through `06-04-SUMMARY.md`
- Verification: `06-VERIFICATION.md`
- Final DB-backed backend gate: `TEST_DATABASE_URL=postgres://oblivious:oblivious@127.0.0.1:32768/oblivious_test?sslmode=disable go test ./... -count=1`

### Phase 7: Frontend, E2E, and Deployment Gate Alignment

**Goal:** Align frontend/API types, Playwright coverage, CI workflow, Dockerfiles, compose, and deployment validation with the consolidated backend surface.

**Requirements:** DEPLOY-02

**Plans:** 3/3 complete

**Waves:**

- Wave 1: Plans 01-02 — frontend/API/workspace alignment and Playwright/E2E gate stabilization
- Wave 2: Plan 03 — CI, Docker, compose, env, lockfile, and deployment validation alignment after web scripts/E2E are stable

**Cross-cutting constraints:**

- Preserve Phase 5 commit boundaries; do not stage frontend/deployment work with planning artifacts or Phase 8 contract docs.
- Preserve the v03.2 restricted-network deployment command:
  `OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh`
- Keep generated/cache artifacts such as `.tmp/`, `dist/`, `test-results/`, and `playwright-report/` out of Git.
- Treat the Phase 6 backend route surface as the frontend/deployment baseline.

**Success criteria:**
1. Frontend API types and app pages match the backend contracts introduced or changed in Phase 6.
2. Playwright configuration and E2E scripts are runnable without pulling generated reports or cache directories into Git.
3. CI workflow and package-lock updates reflect the actual test/build commands.
4. Dockerfiles, compose, env templates, and `scripts/deploy-validate.sh` preserve the v03.2 restricted-network override path.

**Likely verification:**
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test`
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e`
- `COREPACK_HOME=.tmp/corepack pnpm --dir src/web build`
- `docker compose config`
- `OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh`

**Completion evidence:**
- Frontend/workspace contract commit: `e6a2bf9` (`feat(07-01): align frontend workspace contracts`)
- Web/E2E gate commit: `292bc1a` (`test(07-02): stabilize web e2e gates`)
- CI/deployment gate commit: `e7fb740` (`chore(07-03): align ci and deployment gates`)
- Phase summaries: `07-01-SUMMARY.md` through `07-03-SUMMARY.md`
- Code review: `07-REVIEW.md`
- Verification: `07-VERIFICATION.md`
- Runtime proof: restricted-network `scripts/deploy-validate.sh` built images, started the compose stack, passed `/healthz`, and cleaned up.

### Phase 8: Contract Docs and Release Verification

**Goal:** Reconcile docs with live routes and commands, then run the targeted release verification needed to commit the consolidated mainline safely.

**Requirements:** DOC-02, VERIFY-01

**Success criteria:**
1. `docs/API.md`, current system contracts, release docs, README, and env examples describe the same route surface and commands as the code.
2. Verification evidence identifies which checks passed, which were skipped, and why.
3. The final worktree can be split into clear commits that do not mix planning docs, backend integration, frontend/deployment gates, and historical reference files.
4. Remaining external prerequisites, if any, are recorded as blockers instead of being treated as passed gates.

**Likely verification:**
- `bash scripts/check.sh docs`
- `bash scripts/check.sh all`
- `bash scripts/test.sh all`
- Targeted `rg` checks against docs for new route groups and restricted-network deployment commands.

## Traceability

| Requirement | Phase | Coverage |
|-------------|-------|----------|
| CONS-01 | Phase 5 | Complete — dirty worktree inventory and commit-boundary artifacts |
| ROUTE-01 | Phase 6 | Complete — backend routes/services/auth boundaries verified |
| CHAT-06 | Phase 6 | Complete — Relay-first Chat/Agent behavior verified |
| AUTH-01 | Phase 6 | Complete — user/session role/name contract verified |
| DEPLOY-02 | Phase 7 | Complete — frontend/E2E/CI/Docker/deployment gates verified with runtime smoke |
| DOC-02 | Phase 8 | API, architecture, release, README reconciliation |
| VERIFY-01 | Phase 8 | Targeted verification suite and commit-readiness evidence |

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
| v03.3 Mainline Consolidation | Phases 5-8 | 8/8 planned steps complete through Phase 7; Phase 8 unplanned | CONS-01, ROUTE-01, CHAT-06, AUTH-01, DEPLOY-02 complete; DOC-02, VERIFY-01 pending | Phase 8 ready to discuss | Phase 5 and Phase 6 complete 2026-05-14; Phase 7 complete 2026-05-17 |
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
*Roadmap updated: 2026-05-17 after Phase 7 execution and verification*
