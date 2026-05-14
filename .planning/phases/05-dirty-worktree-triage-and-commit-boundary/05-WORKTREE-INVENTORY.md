# Phase 5 Worktree Inventory

**Captured:** 2026-05-14
**Phase:** 05 - Dirty Worktree Triage and Commit Boundary
**Requirement:** CONS-01

## Source Snapshot

This inventory is derived from these non-destructive commands:

```bash
git status --short
git diff --name-status
git ls-files --others --exclude-standard
```

Source worktree counts before Phase 5 execution artifacts:

| Status | Count | Notes |
|--------|-------|-------|
| Tracked modified files | 26 | Existing source, frontend, deployment, and docs edits |
| Untracked files | 64 | Existing source additions, docs, E2E files, and historical/reference material |
| Generated/cache artifacts | 0 | No `.tmp/`, `node_modules/`, `dist/`, `coverage/`, `test-results/`, or `playwright-report/` paths observed |

Phase 5 treats all existing dirty worktree entries as user-owned inputs. This
inventory does not delete, revert, or clean any source file.

## Backend integration

Backend integration contains the route, service, store, WebSocket, Relay,
auth/session/userprefs, and SQL migration work that Phase 6 should harden and
verify.

### Tracked modified files

- `src/server/internal/auth/service.go`
- `src/server/internal/auth/store.go`
- `src/server/internal/chat/gateway.go`
- `src/server/internal/chat/service.go`
- `src/server/internal/config/config.go`
- `src/server/internal/http/auth_middleware.go`
- `src/server/internal/http/router.go`
- `src/server/internal/userprefs/service.go`
- `src/server/internal/userprefs/store.go`

### Untracked files

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

### Handoff

Phase 6 owns backend route/service hardening. It should verify that
`src/server/internal/http/router.go` and the `routes_*.go` files agree on the
intended route registry and auth/admin boundaries.

## Frontend/E2E

Frontend/E2E contains web route pages, API types, theme/config changes, package
updates, and Playwright configuration/specs that Phase 7 should align with the
backend contract.

### Tracked modified files

- `src/web/package.json`
- `src/web/src/theme/tokens.css`
- `src/web/src/types/api.ts`
- `src/web/tailwind.config.ts`
- `src/web/vite.config.ts`

### Untracked files

- `src/web/e2e/admin-marketplace.spec.ts`
- `src/web/e2e/fixtures/adminMarketplace.ts`
- `src/web/playwright.config.ts`
- `src/web/src/routes/workspace/KnowledgePageView.tsx`
- `src/web/src/routes/workspace/MarketplacePage.tsx`
- `src/web/src/routes/workspace/SoloPageView.tsx`

### Handoff

Phase 7 owns frontend, Playwright, E2E, and build verification. The legacy
workspace `MarketplacePage.tsx` remains visible here as user-owned input; Phase
7 should decide whether it is active frontend work or accepted cleanup debt.

## Deployment/CI

Deployment/CI contains the CI, Docker, compose, env template, root package/lock,
and deployment validation changes that must preserve the v03.2 restricted-network
runtime proof.

### Tracked modified files

- `.github/workflows/ci.yml`
- `Dockerfile.server`
- `Dockerfile.web`
- `config/.env.example`
- `docker-compose.yml`
- `package.json`
- `pnpm-lock.yaml`
- `scripts/deploy-validate.sh`

### Required deployment baseline

Later phases must preserve this validated command:

```bash
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

### Handoff

Phase 7 owns CI, Docker, compose, Playwright, and deployment-gate alignment.

## Contract docs

Contract docs contain current API, architecture, release, README, and decision
documentation that Phase 8 should reconcile against live routes and commands.

### Tracked modified files

- `README.md`
- `docs/architecture/current-system-contracts.md`
- `docs/release/deployment-runtime-remediation.md`
- `docs/release/rc-checklist.md`

### Untracked files

- `docs/API.md`
- `docs/architecture/knowledge-evolution-decision.md`
- `docs/architecture/solo-runtime-decision.md`

### Handoff

Phase 8 owns API, architecture, release, README, env, and verification evidence
reconciliation. Contract docs should not be staged with backend or frontend
implementation commits unless a later phase deliberately records that coupling.

## Historical/reference

Historical/reference docs are useful inputs but should not be mixed into
functional implementation commits by default.

### Root historical/reference docs

- `ARCHAEOLOGY_REPORT.md`
- `CLAUDE.md`
- `CURRENT_STATUS.md`
- `ROADMAP.md`

### Reports

- `docs/reports/2026-04-04-codebase-analysis.md`
- `docs/reports/2026-04-04-explicit-markers.md`
- `docs/reports/2026-04-04-todo-tracker.md`
- `docs/reports/2026-04-05-explicit-markers.md`
- `docs/reports/2026-04-05-technical-audit.md`
- `docs/reports/2026-04-05-todo-tracker.md`

### Superpowers plans/specs

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

### Handoff

Historical/reference files need explicit promotion before they enter any
implementation or contract-docs commit.

## Planning-only

Planning-only artifacts are the only paths Phase 5 should stage during this
execution:

- `.planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-WORKTREE-INVENTORY.md`
- `.planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-COMMIT-BOUNDARIES.md`
- `.planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-01-SUMMARY.md`
- `.planning/STATE.md`

Existing Phase 5 planning references:

- `.planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-CONTEXT.md`
- `.planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-PATTERNS.md`
- `.planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-01-PLAN.md`

## Excluded generated/cache artifacts

None observed in `git status --short`.

If later verification creates generated or cache paths under `.tmp/`,
`node_modules/`, `dist/`, `coverage/`, `test-results/`, or
`playwright-report/`, do not stage them by default. Prefer `.gitignore` or a
separate explicit cleanup phase.
