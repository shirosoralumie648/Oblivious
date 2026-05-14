# Phase 5 Commit Boundaries

**Captured:** 2026-05-14
**Phase:** 05 - Dirty Worktree Triage and Commit Boundary
**Requirement:** CONS-01

## Rules

- Do not use `git add .`.
- Do not use `git add -A`.
- Do not run `git clean`, destructive checkout, or reset commands to remove user-owned input files.
- Stage only reviewed, explicit paths.
- Keep planning-only commits separate from source, frontend, deployment, contract docs, and historical/reference commits.
- Treat `lobehub/` and `new-api/` as reference trees unless a future phase explicitly targets them.

## Planning-only

Purpose: record Phase 5 inventory, commit boundaries, summary, and routing state.

Recommended paths:

```bash
git add -- \
  .planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-WORKTREE-INVENTORY.md \
  .planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-COMMIT-BOUNDARIES.md \
  .planning/phases/05-dirty-worktree-triage-and-commit-boundary/05-01-SUMMARY.md \
  .planning/STATE.md
```

Do not stage source, frontend, deployment, contract docs, or historical/reference
files with this group.

## Backend integration

Purpose: hand off route/service/store/migration work to Phase 6.

Recommended paths:

```bash
git add -- \
  src/server/internal/auth/service.go \
  src/server/internal/auth/store.go \
  src/server/internal/chat/gateway.go \
  src/server/internal/chat/service.go \
  src/server/internal/config/config.go \
  src/server/internal/http/auth_middleware.go \
  src/server/internal/http/router.go \
  src/server/internal/userprefs/service.go \
  src/server/internal/userprefs/store.go \
  src/server/internal/admin/service_test.go \
  src/server/internal/http/agent_handler.go \
  src/server/internal/http/mcp_handler.go \
  src/server/internal/http/memory_handler.go \
  src/server/internal/http/notification_handler.go \
  src/server/internal/http/quota_handler.go \
  src/server/internal/http/routes_auth.go \
  src/server/internal/http/routes_chat.go \
  src/server/internal/http/routes_console.go \
  src/server/internal/http/routes_knowledge.go \
  src/server/internal/http/routes_preferences.go \
  src/server/internal/http/routes_task.go \
  src/server/internal/mcp/builtin.go \
  src/server/internal/mcp/client.go \
  src/server/internal/memory/chunker.go \
  src/server/internal/memory/embedder.go \
  src/server/internal/memory/service.go \
  src/server/internal/notification/service.go \
  src/server/internal/notification/service_test.go \
  src/server/internal/quota/service.go \
  src/server/internal/relay/store.go \
  src/server/internal/ws/handler.go \
  src/server/internal/ws/hub.go \
  src/server/internal/ws/hub_test.go \
  src/server/migrations/0013_channels.sql \
  src/server/migrations/0014_agents.sql \
  src/server/migrations/0015_mcp_servers.sql \
  src/server/migrations/0016_pgvector.sql \
  src/server/migrations/0017_quotas.sql \
  src/server/migrations/0018_user_preferences_ext.sql \
  src/server/migrations/0019_admin_role.sql
```

Phase 6 should verify backend tests before committing this group.

## Frontend/E2E

Purpose: hand off frontend route/API type/config and browser coverage work to
Phase 7.

Recommended paths:

```bash
git add -- \
  src/web/package.json \
  src/web/src/theme/tokens.css \
  src/web/src/types/api.ts \
  src/web/tailwind.config.ts \
  src/web/vite.config.ts \
  src/web/e2e/admin-marketplace.spec.ts \
  src/web/e2e/fixtures/adminMarketplace.ts \
  src/web/playwright.config.ts \
  src/web/src/routes/workspace/KnowledgePageView.tsx \
  src/web/src/routes/workspace/MarketplacePage.tsx \
  src/web/src/routes/workspace/SoloPageView.tsx
```

Phase 7 should run focused frontend and E2E checks before committing this group.

## Deployment/CI

Purpose: hand off CI, Docker, compose, env template, root lockfile, and deployment
validation changes to Phase 7.

Recommended paths:

```bash
git add -- \
  .github/workflows/ci.yml \
  Dockerfile.server \
  Dockerfile.web \
  config/.env.example \
  docker-compose.yml \
  package.json \
  pnpm-lock.yaml \
  scripts/deploy-validate.sh
```

Deployment validation must preserve this restricted-network baseline:

```bash
OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

## Contract docs

Purpose: hand off API, architecture, release, README, and decision docs to Phase
8 after backend/frontend/deployment surfaces are verified.

Recommended paths:

```bash
git add -- \
  README.md \
  docs/API.md \
  docs/architecture/current-system-contracts.md \
  docs/architecture/knowledge-evolution-decision.md \
  docs/architecture/solo-runtime-decision.md \
  docs/release/deployment-runtime-remediation.md \
  docs/release/rc-checklist.md
```

Phase 8 should reconcile these files against live routes, env names, and release
commands before committing this group.

## Historical/reference

Purpose: preserve historical context without mixing it into functional
implementation commits.

Default recommendation: do not stage with implementation.

If a future phase explicitly promotes these files, review and stage named paths:

```bash
git add -- \
  ARCHAEOLOGY_REPORT.md \
  CLAUDE.md \
  CURRENT_STATUS.md \
  ROADMAP.md \
  docs/reports/2026-04-04-codebase-analysis.md \
  docs/reports/2026-04-04-explicit-markers.md \
  docs/reports/2026-04-04-todo-tracker.md \
  docs/reports/2026-04-05-explicit-markers.md \
  docs/reports/2026-04-05-technical-audit.md \
  docs/reports/2026-04-05-todo-tracker.md \
  docs/superpowers/plans/2026-04-04-project-functional-completion.md \
  docs/superpowers/plans/2026-04-06-console-operations-overview-implementation.md \
  docs/superpowers/plans/2026-04-06-m0-baseline-freeze-closeout-implementation.md \
  docs/superpowers/plans/2026-04-06-workspace-main-flow-implementation.md \
  docs/superpowers/plans/2026-04-07-m1-mainline-runnable-closeout-implementation.md \
  docs/superpowers/plans/2026-04-07-m2a-knowledge-beta-closeout-implementation.md \
  docs/superpowers/plans/2026-04-08-mainline-engineering-governance-implementation.md \
  docs/superpowers/plans/2026-04-09-relay-plan-a-skeleton.md \
  docs/superpowers/plans/2026-04-09-relay-plan-b-handlers.md \
  docs/superpowers/plans/2026-04-09-relay-plan-c-router-engine.md \
  docs/superpowers/plans/2026-04-09-relay-plan-d-billing-hooks.md \
  docs/superpowers/plans/2026-04-10-relay-handler-router-integration.md \
  docs/superpowers/plans/2026-04-22-full-delivery-plan.md \
  docs/superpowers/specs/2026-04-06-m0-baseline-freeze-closeout-design.md
```

Historical/reference docs must not override `.planning/PROJECT.md`,
`.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, or `.planning/STATE.md`
unless a later phase records an explicit promotion decision.

## Do-not-stage-by-default

- `lobehub/`
- `new-api/`
- `.tmp/`
- `node_modules/`
- `dist/`
- `coverage/`
- `test-results/`
- `playwright-report/`
- Root historical/reference docs unless promoted
- Older `docs/superpowers/*` materials unless promoted

## Review command

Before every commit boundary, review the exact staged set:

```bash
git diff --cached --name-status
```

If the staged set includes files from multiple groups above, unstage and split
the commit before proceeding.
