# Codebase Structure

**Analysis Date:** 2026-05-12

## Scope

This structure map describes the current working tree. The active product boundary is `src/server`, `src/web`, `config`, `scripts`, `.github/workflows`, `docs`, `deploy/kubernetes`, and the root Docker files. `lobehub/` and `new-api/` are reference trees and are not root workspace members.

## Top-Level Layout

- `.planning/` - GSD project state, requirements, roadmap, milestone artifacts, phase artifacts, and codebase maps.
- `.github/workflows/ci.yml` - CI jobs for release docs, web build/tests, Playwright E2E, and server checks/tests.
- `src/server/` - active Go backend module.
- `src/web/` - active React/Vite frontend package and the only pnpm workspace member.
- `config/.env.example` - local env contract and placeholder runtime values.
- `scripts/` - local dev, test, check, deployment validation, and release assertions.
- `docs/API.md` - current app and Relay API index.
- `docs/architecture/` - current architecture contracts and decisions.
- `docs/release/` - RC checklist and deployment runtime remediation.
- `deploy/kubernetes/` - Kubernetes manifests for namespace, config, server, web, Postgres, Redis, and example secrets.
- `Dockerfile.server`, `Dockerfile.web`, `docker-compose.yml` - container release path.
- `lobehub/` - reference upstream-style app tree, excluded from root workspace.
- `new-api/` - reference upstream-style Go/API tree, excluded from root workspace.

## Planning Layout

- `.planning/STATE.md` - current GSD status. As of this map, v03.2 is blocked on DEPLOY-01 runtime validation.
- `.planning/PROJECT.md` - project scope, current milestone status, and release truth.
- `.planning/REQUIREMENTS.md` - current milestone and backlog requirements.
- `.planning/ROADMAP.md` - milestone/phase roadmap and current blocked state.
- `.planning/MILESTONES.md` - milestone archive/status.
- `.planning/phases/01-relay-integration/` - relay integration phase context and verification.
- `.planning/phases/02-agent-memory-enhancement/` - agent/memory phase context, discussion, plan, and summary.
- `.planning/phases/03-admin-marketplace/` - admin/marketplace backend and design phase artifacts.
- `.planning/phases/03.1-admin-marketplace-ui/` - admin/marketplace UI plans, summaries, UAT, security, and validation.
- `.planning/phases/04-quality-release/` - quality/release plans, summaries, completion audit, context, and discussion log.
- `.planning/codebase/` - seven generated codebase map documents.

## Backend Structure

- `src/server/go.mod`, `src/server/go.sum` - active Go module metadata.
- `src/server/cmd/server/main.go` - HTTP server entry point.
- `src/server/cmd/migrate/main.go` - SQL migration entry point.
- `src/server/migrations/` - app migrations `0001` through `0024`.
- `src/server/internal/config/` - env loading and validation.
- `src/server/internal/db/` - PostgreSQL connection helper.
- `src/server/internal/http/` - router, server wrapper, middleware, handlers, response envelopes, and HTTP tests.
- `src/server/internal/auth/` - users, sessions, password hashing, workspace bootstrap, and session store.
- `src/server/internal/userprefs/` - user preference service/store.
- `src/server/internal/chat/` - conversations, messages, model gateway, Relay gateway, composite gateway, and usage recording hooks.
- `src/server/internal/knowledge/` - knowledge base/document CRUD and retrieval.
- `src/server/internal/task/` - SOLO task service, runtime, task state, budgets, approvals, tool allow/deny lists, and store.
- `src/server/internal/agent/` - agent CRUD, conversations, runner, tool executor, and SQL store.
- `src/server/internal/memory/` - memory documents, chunker, embedder, vector search service, and SQL store.
- `src/server/internal/mcp/` - MCP server store, JSON-RPC client, and built-in tools.
- `src/server/internal/relay/` - Relay engine, route manager, channels, billing, retry, token buckets, pricing, tokenizer, circuit breaker, health checker, load balancer, provider types, and handlers.
- `src/server/internal/admin/` - admin dashboard, channels, routes, plans, users, audit logs, review queues, and stores.
- `src/server/internal/marketplace/` - marketplace agents, installs, search, categories, tags, publisher stats, and review flow.
- `src/server/internal/quota/` - package/quota/top-up/preconsume/settlement service.
- `src/server/internal/notification/` - notification service and store.
- `src/server/internal/console/` - console summaries for usage, access, models, and billing.
- `src/server/internal/usage/` - usage recorder and store.
- `src/server/internal/metrics/` - Prometheus relay metrics.
- `src/server/internal/ws/` - WebSocket hub and handler.
- `src/server/internal/stripe/` - Stripe checkout/webhook helper code; routes are not visibly mounted in the active router.

## Backend Route Files

The active app router is `src/server/internal/http/router.go`.

Additional modular route files exist:
- `src/server/internal/http/routes_auth.go`
- `src/server/internal/http/routes_chat.go`
- `src/server/internal/http/routes_console.go`
- `src/server/internal/http/routes_knowledge.go`
- `src/server/internal/http/routes_preferences.go`
- `src/server/internal/http/routes_task.go`

These files define `register*Routes` functions, but `router.go` currently registers corresponding routes inline instead of calling them.

## Frontend Structure

- `src/web/package.json` - frontend package scripts and dependencies.
- `src/web/vite.config.ts` - Vite/Vitest config and `@/*` path alias.
- `src/web/tsconfig.json` - strict TypeScript configuration.
- `src/web/tailwind.config.ts`, `src/web/postcss.config.mjs` - Tailwind and PostCSS config.
- `src/web/components.json` - shadcn/Radix component config.
- `src/web/index.html` - Vite HTML entry.
- `src/web/src/main.tsx` - React mount point.
- `src/web/src/app/` - app root, providers, router, route future flags, and app context.
- `src/web/src/features/auth/` - auth API, store, bootstrap controller, `ProtectedRoute`, and `AdminRoute`.
- `src/web/src/features/layouts/` - marketing, workspace, console, and admin layouts.
- `src/web/src/features/chat/`, `knowledge/`, `tasks/`, `console/`, `admin/`, `marketplace/` - feature API clients and reusable feature components.
- `src/web/src/routes/marketing/` - public home, login, register, pricing, and download surfaces.
- `src/web/src/routes/workspace/` - chat, knowledge, SOLO, onboarding, settings, and legacy workspace marketplace file.
- `src/web/src/routes/marketplace/` - active marketplace home, agent detail, publish, and my-agents pages.
- `src/web/src/routes/console/` - console overview, usage, models, billing, and access pages.
- `src/web/src/routes/admin/` - admin dashboard, channels, routes, plans, users, audit log, and reviews pages.
- `src/web/src/components/ui/` - low-level UI primitives.
- `src/web/src/components/shared/` - product-specific reusable widgets such as tables, search/filter, status badges, and empty states.
- `src/web/src/services/http/` - fetch client, envelope handling, errors, streaming, and upload helpers.
- `src/web/src/theme/` - global CSS and tokens.
- `src/web/src/types/` - shared TypeScript API/admin contracts.
- `src/web/e2e/` - Playwright E2E specs and fixtures.

## Frontend Route Tree

`src/web/src/app/router.tsx` organizes routes as:

- Marketing layout: `/`, `/login`, `/register`
- Protected workspace layout: `/onboarding`, `/chat`, `/chat/:conversationId`, `/knowledge`, `/knowledge/:knowledgeBaseId`, `/solo`, `/solo/new`, `/marketplace`, `/marketplace/agents/:agentId`, `/marketplace/publish`, `/marketplace/my-agents`, `/settings`
- Console layout: `/console`, `/console/models`, `/console/usage`, `/console/billing`, `/console/access`
- Admin layout under `AdminRoute`: `/admin`, `/admin/channels`, `/admin/routes`, `/admin/plans`, `/admin/users`, `/admin/audit-log`, `/admin/reviews`

## Script And CI Structure

- `scripts/dev.sh` - starts local server and web processes using repo-local Go and Corepack caches.
- `scripts/check.sh` - verifies release assets, docs/env consistency, workspace boundaries, web build, and server checks.
- `scripts/test.sh` - runs web Vitest, server Go tests, and optional DB-backed HTTP integration tests.
- `scripts/verify-quality-gates.sh` - fixed-string release asset and checklist assertions.
- `scripts/deploy-smoke.sh` - polls `/healthz`.
- `scripts/deploy-validate.sh` - checks Docker availability, renders compose config, builds images, starts the stack, and runs smoke.
- `.github/workflows/ci.yml` - separates release-gates, web, e2e, and server jobs.

## Deployment Structure

- `docker-compose.yml` - Postgres, Redis, server, and web services for local/container validation.
- `Dockerfile.server` - Go server build/runtime image.
- `Dockerfile.web` - Vite build plus nginx runtime image.
- `deploy/kubernetes/namespace.yaml` - namespace.
- `deploy/kubernetes/configmap.yaml` - non-secret app config.
- `deploy/kubernetes/secret.example.yaml` - example secret names and placeholders.
- `deploy/kubernetes/server.yaml`, `web.yaml`, `postgres.yaml`, `redis.yaml` - workload and service manifests.

## Reference Tree Structure

- `lobehub/` contains its own workspace, package graph, Next.js/desktop/server code, Docker files, tests, and docs. It is useful as a reference, not as an active root package.
- `new-api/` contains a separate Go API and web frontend with its own module, Docker setup, router, relay code, i18n, and tests. It is useful as a reference, not as an active root package.

## Generated And Local Output

- `.tmp/`, `.tmp/corepack`, `.tmp/go-build`, and `.tmp/go-mod` are repo-local caches used by scripts.
- `src/web/test-results/` can exist after Playwright runs.
- `src/web/dist/` can exist after frontend builds.
- `.claude-flow/`, `.claude/`, and `.codex` are local agent/runtime artifacts.

## Structural Notes

- The worktree is dirty and contains many modified/untracked source, docs, and planning files. Treat this map as a current working-tree map, not a clean commit baseline.
- Empty nested directories under `src/server/src/` were visible in the directory tree but did not contain files in the checked depth. They should not be treated as active code unless files are added there.
- Current mainline route and release structure is larger than the older README summary: Relay, Agent, Memory, MCP, Quota, Admin, Marketplace, WebSocket, and notifications are now present in the active tree.
