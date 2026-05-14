# Phase 5 Pattern Mapping: Dirty Worktree Triage and Commit Boundary

**Mapped:** 2026-05-14
**Scope:** Planning-only classification for the current uncommitted worktree
**Requirement:** CONS-01

## Source Commands

Use these commands as the reproducible inventory source for Phase 5 execution:

```bash
git status --short
git diff --name-status
git ls-files --others --exclude-standard
```

These commands are intentionally non-destructive. Phase 5 must not clean,
revert, delete, or broadly stage files.

## Active Boundaries

The active mainline boundary comes from `.planning/codebase/STACK.md`,
`.planning/codebase/ARCHITECTURE.md`, and `.planning/codebase/CONVENTIONS.md`.

- Active backend: `src/server`
- Active frontend: `src/web`
- Active runtime and release config: `config`, `scripts`, `.github/workflows`, `Dockerfile.server`, `Dockerfile.web`, `docker-compose.yml`
- Active root workspace metadata: `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, `README.md`
- Reference-only trees: `lobehub/`, `new-api/`

`lobehub/` and `new-api/` are excluded from Phase 5 classification unless a
future phase explicitly targets them.

## Existing Patterns to Preserve

| Area | Existing pattern | Phase 5 implication |
|------|------------------|---------------------|
| Backend | `src/server/internal/http/router.go` remains the composition root while route split files are a partial refactor. | Backend integration must classify router edits and `routes_*.go` additions together, then defer wiring verification to Phase 6. |
| Backend domains | Handler -> service -> store packages under `src/server/internal/<domain>`. | New Agent, Memory, MCP, Notification, Quota, WebSocket, Relay, auth, chat, and userprefs files belong to one backend integration slice unless later execution finds a tighter split. |
| Migrations | SQL migrations under `src/server/migrations` are append-only and ordered lexically. | Migrations `0013` through `0019` must travel with backend integration, not with docs-only commits. |
| Frontend | React/Vite app under `src/web`, route pages under `src/web/src/routes`, E2E under `src/web/e2e`. | Web route pages, API types, Playwright config/specs/fixtures, Tailwind/Vite/package changes form the Frontend/E2E slice. |
| Deployment | Release stack uses Dockerfiles, compose, env template, CI, and `scripts/deploy-validate.sh`. | CI/Docker/compose/env/script/root lockfile updates form the Deployment/CI slice. |
| Docs | API, architecture, release docs, and README are current contract surfaces. | Contract docs must be staged separately from backend/frontend/deployment implementation. |
| Historical docs | Root `CURRENT_STATUS.md`, root `ROADMAP.md`, `ARCHAEOLOGY_REPORT.md`, and older `docs/superpowers/*` are historical/reference inputs. | Inventory these separately and do not mix them into functional implementation commits by default. |

## Current Worktree Classification

### Backend integration

Tracked backend files:

- `src/server/internal/auth/service.go`
- `src/server/internal/auth/store.go`
- `src/server/internal/chat/gateway.go`
- `src/server/internal/chat/service.go`
- `src/server/internal/config/config.go`
- `src/server/internal/http/auth_middleware.go`
- `src/server/internal/http/router.go`
- `src/server/internal/userprefs/service.go`
- `src/server/internal/userprefs/store.go`

Untracked backend files:

- `src/server/internal/admin/service_test.go`
- `src/server/internal/http/agent_handler.go`
- `src/server/internal/http/mcp_handler.go`
- `src/server/internal/http/memory_handler.go`
- `src/server/internal/http/notification_handler.go`
- `src/server/internal/http/quota_handler.go`
- `src/server/internal/http/routes_auth.go`
- `src/server/internal/http/routes_chat.go`
- `src/server/internal/http/routes_console.go`
- `src/server/internal/http/routes_knowledge.go`
- `src/server/internal/http/routes_preferences.go`
- `src/server/internal/http/routes_task.go`
- `src/server/internal/mcp/builtin.go`
- `src/server/internal/mcp/client.go`
- `src/server/internal/memory/chunker.go`
- `src/server/internal/memory/embedder.go`
- `src/server/internal/memory/service.go`
- `src/server/internal/notification/service.go`
- `src/server/internal/notification/service_test.go`
- `src/server/internal/quota/service.go`
- `src/server/internal/relay/store.go`
- `src/server/internal/ws/handler.go`
- `src/server/internal/ws/hub.go`
- `src/server/internal/ws/hub_test.go`
- `src/server/migrations/0013_channels.sql`
- `src/server/migrations/0014_agents.sql`
- `src/server/migrations/0015_mcp_servers.sql`
- `src/server/migrations/0016_pgvector.sql`
- `src/server/migrations/0017_quotas.sql`
- `src/server/migrations/0018_user_preferences_ext.sql`
- `src/server/migrations/0019_admin_role.sql`

### Frontend/E2E

Tracked frontend files:

- `src/web/package.json`
- `src/web/src/theme/tokens.css`
- `src/web/src/types/api.ts`
- `src/web/tailwind.config.ts`
- `src/web/vite.config.ts`

Untracked frontend files:

- `src/web/e2e/admin-marketplace.spec.ts`
- `src/web/e2e/fixtures/adminMarketplace.ts`
- `src/web/playwright.config.ts`
- `src/web/src/routes/workspace/KnowledgePageView.tsx`
- `src/web/src/routes/workspace/MarketplacePage.tsx`
- `src/web/src/routes/workspace/SoloPageView.tsx`

### Deployment/CI

Tracked deployment and workspace files:

- `.github/workflows/ci.yml`
- `Dockerfile.server`
- `Dockerfile.web`
- `config/.env.example`
- `docker-compose.yml`
- `package.json`
- `pnpm-lock.yaml`
- `scripts/deploy-validate.sh`

### Contract docs

Tracked contract docs:

- `README.md`
- `docs/architecture/current-system-contracts.md`
- `docs/release/deployment-runtime-remediation.md`
- `docs/release/rc-checklist.md`

Untracked contract docs:

- `docs/API.md`
- `docs/architecture/knowledge-evolution-decision.md`
- `docs/architecture/solo-runtime-decision.md`

### Historical/reference docs

Untracked historical or reference docs:

- `ARCHAEOLOGY_REPORT.md`
- `CLAUDE.md`
- `CURRENT_STATUS.md`
- `ROADMAP.md`
- `docs/reports/2026-04-04-codebase-analysis.md`
- `docs/reports/2026-04-04-explicit-markers.md`
- `docs/reports/2026-04-04-todo-tracker.md`
- `docs/reports/2026-04-05-explicit-markers.md`
- `docs/reports/2026-04-05-technical-audit.md`
- `docs/reports/2026-04-05-todo-tracker.md`
- `docs/superpowers/plans/2026-04-04-project-functional-completion.md`
- `docs/superpowers/plans/2026-04-06-console-operations-overview-implementation.md`
- `docs/superpowers/plans/2026-04-06-m0-baseline-freeze-closeout-implementation.md`
- `docs/superpowers/plans/2026-04-06-workspace-main-flow-implementation.md`
- `docs/superpowers/plans/2026-04-07-m1-mainline-runnable-closeout-implementation.md`
- `docs/superpowers/plans/2026-04-07-m2a-knowledge-beta-closeout-implementation.md`
- `docs/superpowers/plans/2026-04-08-mainline-engineering-governance-implementation.md`
- `docs/superpowers/plans/2026-04-09-relay-plan-a-skeleton.md`
- `docs/superpowers/plans/2026-04-09-relay-plan-b-handlers.md`
- `docs/superpowers/plans/2026-04-09-relay-plan-c-router-engine.md`
- `docs/superpowers/plans/2026-04-09-relay-plan-d-billing-hooks.md`
- `docs/superpowers/plans/2026-04-10-relay-handler-router-integration.md`
- `docs/superpowers/plans/2026-04-22-full-delivery-plan.md`
- `docs/superpowers/specs/2026-04-06-m0-baseline-freeze-closeout-design.md`

### Planning-only artifacts

Phase 5 may add planning artifacts under:

- `.planning/phases/05-dirty-worktree-triage-and-commit-boundary/`
- `.planning/STATE.md`
- `.planning/ROADMAP.md` only if route/status metadata needs correction

Planning-only commits must remain separate from source, deployment, frontend,
and historical/reference commits.

## Generated and Cache Artifacts

No generated/cache artifacts are visible in the current tracked/untracked
inventory from `git status --short`. If later commands surface paths under
`.tmp/`, `node_modules/`, `dist/`, `coverage/`, `test-results/`, or
`playwright-report/`, Phase 5 should record them as excluded artifacts and
leave cleanup to `.gitignore` or a later explicit cleanup task.

## Recommended Downstream Commit Boundaries

1. Planning-only: Phase 5 inventory, commit-boundary docs, summaries, and state.
2. Backend integration: server route/service/store/migration changes.
3. Frontend/E2E: web pages, API types, Playwright config/specs/fixtures, web package/config updates.
4. Deployment/CI: GitHub Actions, Dockerfiles, compose, env template, root package/lock, deploy validation.
5. Contract docs: API, architecture, release docs, README, docs assertions.
6. Historical/reference docs: root historical docs, reports, and superpowers plans/specs, only after explicit promotion.

## Non-Goals

- Do not run full Go, frontend, E2E, or Docker validation in Phase 5.
- Do not stage implementation files with planning artifacts.
- Do not delete untracked files.
- Do not use `git add .` or `git add -A`.
