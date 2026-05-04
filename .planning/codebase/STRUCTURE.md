---
last_mapped_commit: 98576468acf0d72bbca7e61317dc83cd5c6ad7a9
mapped_dirty_worktree: true
analysis_date: 2026-05-04
mapper: sequential-fallback
---

# Codebase Structure

## Top-Level Layout

- `.planning/` - GSD project state, requirements, roadmap, phase artifacts, and codebase maps.
- `.github/workflows/ci.yml` - CI release, web, E2E, and server jobs.
- `src/server/` - active Go backend.
- `src/web/` - active React/Vite frontend.
- `scripts/` - development, test, release, and deployment validation entry points.
- `config/.env.example` - env var contract.
- `docs/` - API, architecture, governance, reports, release docs.
- `deploy/kubernetes/` - Kubernetes manifests for release stack.
- `Dockerfile.server`, `Dockerfile.web`, `docker-compose.yml` - container release path.
- `lobehub/`, `new-api/` - imported/reference trees, not active workspace members.

## Planning Structure

- `.planning/STATE.md` - current status. As of this map, v03.2 is blocked only on DEPLOY-01 runtime validation.
- `.planning/REQUIREMENTS.md` - current and historical requirement checklist.
- `.planning/ROADMAP.md` - phase roadmap and backlog.
- `.planning/MILESTONES.md` - milestone archive/status.
- `.planning/phases/04-quality-release/` - current quality/release plans, summaries, and completion audit.
- `.planning/codebase/` - this map. Expected files:
  - `STACK.md`
  - `INTEGRATIONS.md`
  - `ARCHITECTURE.md`
  - `STRUCTURE.md`
  - `CONVENTIONS.md`
  - `TESTING.md`
  - `CONCERNS.md`

## Backend Directory Structure

- `src/server/go.mod`, `src/server/go.sum` - Go module metadata.
- `src/server/cmd/server/main.go` - HTTP server entry point.
- `src/server/cmd/migrate/main.go` - migration entry point.
- `src/server/migrations/` - app migrations `0001` through `0024`.
- `src/server/internal/http/` - composition root, handlers, middleware, response envelopes, route tests.
- `src/server/internal/config/` - env config loading and validation.
- `src/server/internal/db/` - database open/helper code.
- `src/server/internal/auth/` - session/auth service and store.
- `src/server/internal/chat/` - chat service, SQL store, local generator, Relay gateway, composite gateway.
- `src/server/internal/agent/` - agent service, runner, executor, SQL store.
- `src/server/internal/memory/` - document chunking, embedding via Relay, memory service.
- `src/server/internal/mcp/` - MCP client, built-in tools.
- `src/server/internal/relay/` - relay engine, routing, channel adapters, billing, token/rate/health logic.
- `src/server/internal/admin/` - channel/route/plan/user/audit/review services and stores.
- `src/server/internal/marketplace/` - marketplace service, search, publisher analytics, store, types.
- `src/server/internal/knowledge/` - knowledge base/document service and store.
- `src/server/internal/task/` - task service/runtime/store.
- `src/server/internal/console/` - console dashboards and access/model/usage data.
- `src/server/internal/quota/` - quota package/top-up/preconsume/settlement service.
- `src/server/internal/notification/` - notifications service/store.
- `src/server/internal/usage/` - usage recorder/store.
- `src/server/internal/userprefs/` - preferences service/store.
- `src/server/internal/ws/` - WebSocket handler/hub.
- `src/server/internal/stripe/` - checkout/webhook integration stubs.

## Frontend Directory Structure

- `src/web/package.json` - web scripts and dependencies.
- `src/web/vite.config.ts`, `tailwind.config.ts`, `postcss.config.mjs`, `tsconfig.json` - web toolchain.
- `src/web/src/main.tsx` - application bootstrap.
- `src/web/src/app/` - app root, providers, router, app context.
- `src/web/src/features/auth/` - auth API, store, bootstrap, route guards.
- `src/web/src/features/layouts/` - marketing, workspace, console, admin layouts.
- `src/web/src/features/admin/api.ts` - typed Admin API client.
- `src/web/src/features/marketplace/api.ts` - typed Marketplace API client.
- `src/web/src/features/chat/api.ts`, `knowledge/api.ts`, `tasks/api.ts`, `console/api.ts` - feature API clients.
- `src/web/src/routes/marketing/` - public marketing/auth pages.
- `src/web/src/routes/workspace/` - chat, knowledge, solo, settings; includes legacy `MarketplacePage.tsx` debt.
- `src/web/src/routes/marketplace/` - active marketplace home, detail, publish, my-agents routes.
- `src/web/src/routes/admin/` - admin dashboard/pages.
- `src/web/src/routes/console/` - console pages.
- `src/web/src/components/ui/` - low-level UI primitives.
- `src/web/src/components/shared/` - product-specific reusable widgets.
- `src/web/src/services/http/` - HTTP client, envelope, errors, stream/upload helpers.
- `src/web/e2e/` - Playwright Admin/Marketplace E2E specs and fixtures.

## Route Structure

Frontend routes in `src/web/src/app/router.tsx`:

- Marketing: `/`, `/login`, `/register`
- Workspace: `/onboarding`, `/chat`, `/chat/:conversationId`, `/knowledge`, `/knowledge/:knowledgeBaseId`, `/solo`, `/solo/new`, `/settings`
- Marketplace: `/marketplace`, `/marketplace/agents/:agentId`, `/marketplace/publish`, `/marketplace/my-agents`
- Console: `/console`, `/console/models`, `/console/usage`, `/console/billing`, `/console/access`
- Admin: `/admin`, `/admin/channels`, `/admin/routes`, `/admin/plans`, `/admin/users`, `/admin/audit-log`, `/admin/reviews`

Backend route groups are defined in `src/server/internal/http/router.go`; Relay-compatible `/v1/*` route definitions are in `src/server/internal/relay/handler/router.go`.

## Release/Deployment Structure

- `scripts/check.sh` - docs/web/server release checks.
- `scripts/test.sh` - web/server tests and optional DB-backed HTTP integration tests.
- `scripts/verify-quality-gates.sh` - fixed-string release asset assertions.
- `scripts/deploy-smoke.sh` - `/healthz` polling smoke command.
- `scripts/deploy-validate.sh` - real Docker compose build/up/smoke gate.
- `docs/release/rc-checklist.md` - canonical RC command/evidence checklist.
- `docs/release/deployment-runtime-remediation.md` - host remediation when Docker/kubectl are unavailable.

## Generated/Build Output

- `src/web/dist/` can exist after build and is ignored by `.dockerignore`.
- `src/web/test-results/` can exist after Playwright runs.
- `.tmp/`, `.tmp/corepack`, `.tmp/go-build`, `.tmp/go-mod` are repo-local caches used by scripts.

## Dirty Worktree Notes

This map was produced with a dirty worktree. Known current changes include release/deployment assets, planning artifacts, scripts, and many pre-existing project modifications. Do not interpret this map as a clean-commit baseline.
