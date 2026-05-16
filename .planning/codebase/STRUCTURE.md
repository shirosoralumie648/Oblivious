# Codebase Structure

**Analysis Date:** 2026-05-16

## Directory Layout

```
Oblivious/
├── src/                          # All mainline source code (server + web)
│   ├── server/                   # Go backend (mainline)
│   │   ├── cmd/                  # Process entrypoints (one main.go per binary)
│   │   │   ├── server/main.go    # Application HTTP server binary
│   │   │   └── migrate/main.go   # Migration runner binary
│   │   ├── internal/             # Private application code, one dir per bounded context
│   │   │   ├── admin/            # Admin domain (channels, plans, routes, users, audit)
│   │   │   ├── agent/            # Agent domain (runner, executor, tool loop)
│   │   │   ├── auth/             # Session + user auth
│   │   │   ├── chat/             # Conversations, messages, RelayGateway
│   │   │   ├── config/           # Env-loaded `Config` struct
│   │   │   ├── console/          # Read-only operator console summaries
│   │   │   ├── db/               # PostgreSQL pool open helper
│   │   │   ├── http/             # HTTP transport (router, handlers, middleware)
│   │   │   ├── knowledge/        # Knowledge bases + documents + retrieval
│   │   │   ├── marketplace/      # Marketplace agents, installs, reviews, search
│   │   │   ├── mcp/              # MCP client + built-in tools
│   │   │   ├── memory/           # Memory documents + chunker + embedder (Relay client)
│   │   │   ├── metrics/          # Prometheus metric definitions
│   │   │   ├── notification/     # Persistent notifications
│   │   │   ├── quota/            # Quota service injected into Relay billing
│   │   │   ├── relay/            # Relay engine (gin) — the load-bearing boundary
│   │   │   │   ├── channel/      # OpenAI adapter + channel-typed plumbing
│   │   │   │   ├── handler/      # Per-API-type handlers + 35-route table
│   │   │   │   ├── migrations/   # Relay-owned SQL (in addition to root migrations/)
│   │   │   │   └── types/        # Channel, RouteChannel, APIType, headers, ctx keys
│   │   │   ├── stripe/           # Checkout + webhook for quota top-up
│   │   │   ├── task/             # SOLO bounded runtime (state machine MVP)
│   │   │   ├── usage/            # Per-request usage recorder (Console source)
│   │   │   ├── userprefs/        # User preferences (defaultMode, modelStrategy…)
│   │   │   └── ws/               # WebSocket hub + handler
│   │   ├── migrations/           # Numbered PostgreSQL SQL migrations (0001…0024+)
│   │   ├── go.mod / go.sum       # Module: oblivious/server, Go 1.25
│   │   └── src/                  # (legacy nested mirror; do not add code here)
│   └── web/                      # React 18 + Vite SPA (mainline)
│       ├── src/
│       │   ├── main.tsx          # `ReactDOM.createRoot` mount
│       │   ├── app/              # Composition root (App, router, providers, context)
│       │   ├── routes/           # Page components grouped by area
│       │   │   ├── marketing/    # Public marketing + login + register
│       │   │   ├── workspace/    # Authenticated workspace (chat, knowledge, solo, marketplace, settings)
│       │   │   ├── console/      # Console operator pages
│       │   │   ├── marketplace/  # Marketplace browse / detail / publish
│       │   │   └── admin/        # Admin pages (gated by AdminRoute)
│       │   ├── features/         # Domain logic colocated with feature
│       │   │   ├── auth/         # Store, ProtectedRoute, AdminRoute, bootstrap
│       │   │   ├── chat/         # `api.ts`
│       │   │   ├── knowledge/    # `api.ts`
│       │   │   ├── tasks/        # `api.ts`
│       │   │   ├── admin/        # `api.ts` + tests
│       │   │   ├── marketplace/  # `api.ts` + tests
│       │   │   ├── console/      # `api.ts` + `components/`
│       │   │   └── layouts/      # Marketing / Workspace / Console / Admin layout shells
│       │   ├── services/http/    # `createHttpClient`, envelope unwrap, errors, stream, upload
│       │   ├── components/
│       │   │   ├── ui/           # shadcn-style headless primitives (Radix-backed)
│       │   │   └── shared/       # Cross-domain composites (DataTable, EmptyState, ...)
│       │   ├── lib/utils.ts      # `cn` className helper
│       │   ├── store/app.ts      # Cross-cutting client store
│       │   ├── test/setup.ts     # Vitest + jest-dom bootstrap
│       │   ├── theme/            # Design tokens + global CSS
│       │   └── types/            # Backend contract types (api.ts, admin.ts)
│       ├── e2e/                  # Playwright suites + fixtures
│       │   ├── admin-marketplace.spec.ts
│       │   └── fixtures/
│       ├── index.html            # Vite entry HTML
│       ├── vite.config.ts        # Vite + Vitest config
│       ├── playwright.config.ts  # Playwright config
│       ├── tailwind.config.ts    # Tailwind v3 config
│       ├── postcss.config.mjs    # PostCSS
│       ├── tsconfig.json         # TS config
│       ├── components.json       # shadcn config
│       └── package.json          # `oblivious-web` workspace
├── config/                       # Mainline configuration (env contract lives here)
│   └── .env.example              # Authoritative env contract (sync with config/config.go)
├── deploy/                       # Deployment manifests
│   └── kubernetes/               # configmap, namespace, postgres, redis, server, web, secret.example
├── scripts/                      # Repo-wide automation entrypoints
│   ├── check.sh                  # Docs + web build + server unit checks (CI gate)
│   ├── test.sh                   # Web + server tests
│   ├── dev.sh                    # Local dev helper
│   ├── deploy-smoke.sh           # Post-deploy smoke
│   ├── deploy-validate.sh        # Pre-deploy validation
│   └── verify-quality-gates.sh   # Cross-cutting release gate
├── docs/                         # Authoritative documentation
│   ├── API.md                    # Canonical route index
│   ├── architecture/             # Design contracts (current-system-contracts, solo-runtime-decision, knowledge-evolution-decision)
│   ├── governance/               # Process docs
│   ├── release/                  # Release runbooks (rc-checklist, deployment-runtime-remediation)
│   ├── reports/                  # Time-stamped audits / progress reports
│   └── superpowers/              # Plans and specs (specs/, plans/)
├── .planning/                    # GSD planning artifacts
│   ├── codebase/                 # This directory — STACK / ARCHITECTURE / STRUCTURE / ...
│   ├── milestones/               # Milestone plans
│   └── phases/                   # Phase plans + closeouts
├── .github/workflows/            # CI definitions (single root pipeline)
├── .claude/, .claude-flow/       # Local Claude agent + skills + helpers
├── .worktrees/                   # Branch worktrees (non-mainline)
├── lobehub/, new-api/            # Non-mainline reference checkouts (do not import)
├── docker-compose.yml            # Local + CI integration stack (postgres, redis, server, web)
├── Dockerfile.server             # Builds oblivious-server and oblivious-migrate
├── Dockerfile.web                # Builds Vite SPA into nginx with /api + /v1 proxy
├── package.json                  # Root pnpm workspace + `dev/test/check` script orchestration
├── pnpm-workspace.yaml           # Lists `src/web` as the workspace package
├── pnpm-lock.yaml                # Single lockfile for the root workspace
├── README.md
├── ROADMAP.md
├── CURRENT_STATUS.md
├── ARCHAEOLOGY_REPORT.md
├── CLAUDE.md                     # Project behavioral + file-organization rules
└── .gitignore / .dockerignore / .mcp.json / .codex
```

## Directory Purposes

**`src/server/`:**
- Purpose: Mainline Go backend.
- Contains: One binary per directory in `cmd/`, one bounded context per directory in `internal/`, SQL migrations under `migrations/`.
- Key files: `cmd/server/main.go`, `cmd/migrate/main.go`, `internal/http/router.go`, `internal/http/server.go`, `internal/relay/relay.go`.

**`src/server/internal/`:**
- Purpose: Private application packages (Go `internal/` convention).
- Contains: Bounded contexts, each typically with `service.go` (use cases), `store.go` (SQL), `*_test.go`, and optional `types.go`.
- Key files: `chat/service.go`, `chat/relay_gateway.go`, `agent/runner.go`, `task/runtime.go`, `relay/router.go`, `auth/service.go`.

**`src/server/internal/http/`:**
- Purpose: HTTP transport layer — bridge between `net/http` and bounded-context services.
- Contains: `server.go` (factory that mounts Relay), `router.go` (the full `/api/v1/*` route table), `middleware.go`, `auth_middleware.go`, `response.go`, one `<context>_handler.go` per domain, `routes_<area>.go` files for grouped route registrations, `route_surface_test.go` (route surface contract test).

**`src/server/internal/relay/`:**
- Purpose: The relay boundary — every LLM call passes through here.
- Contains: `relay.go` (gin engine assembly), `router.go` (routing + billing), `pool.go`, `loadbalancer.go`, `circuitbreaker.go`, `tokenbucket.go`, `healthchecker.go`, `billing.go`, `billing_worker.go`, `pricing.go`, `retry.go`, `tokenizer.go`, `store.go`, `errors.go`. Subdirectories: `handler/` (per-API-type), `channel/` (OpenAI adapter), `types/`, `migrations/`.

**`src/server/migrations/`:**
- Purpose: Numbered PostgreSQL migrations applied in order by `cmd/migrate`.
- Contains: `0001_phase1_foundation.sql` through `0024_categories_tags.sql` (and growing). Each file is a single migration; never edit a committed file — write a new numbered one.

**`src/web/src/app/`:**
- Purpose: Composition root for the SPA.
- Contains: `main.tsx` is mounted from outside; `App.tsx` is the React entry, `router.tsx` builds the data router, `providers.tsx` wraps the auth + preferences context, `appContext.tsx` defines the context and a test-safe fallback, `routerFuture.ts` enables future v6 router flags.

**`src/web/src/routes/<area>/`:**
- Purpose: Page-level components consumed directly by the router.
- Contains: One `.tsx` per page + matching `*.test.tsx`. Workspace pages also have `*View.tsx` splits for complex pages (e.g., `SoloPage.tsx` + `SoloPageView.tsx`).
- Key files: `workspace/ChatPage.tsx`, `workspace/SoloPage.tsx`, `workspace/KnowledgePage.tsx`, `console/UsagePage.tsx`, `admin/AdminChannelsPage.tsx`.

**`src/web/src/features/<domain>/`:**
- Purpose: Domain logic and reusable widgets, colocated with the feature.
- Contains: Always `api.ts` (typed HTTP client). Some have `store.ts`, hooks (e.g., `useAuthBootstrap.ts`), guards (`ProtectedRoute.tsx`, `AdminRoute.tsx`), layouts (`features/layouts/*Layout.tsx`), and tests (`*.test.ts(x)`).
- Key files: `auth/store.ts`, `auth/ProtectedRoute.tsx`, `auth/AdminRoute.tsx`, `layouts/WorkspaceLayout.tsx`, `chat/api.ts`, `marketplace/api.ts`.

**`src/web/src/services/http/`:**
- Purpose: One envelope-aware HTTP client shared by every feature `api.ts`.
- Contains: `client.ts` (createHttpClient), `envelope.ts` (unwrap), `errors.ts` (HttpError), `stream.ts`, `upload.ts`, plus tests.

**`src/web/src/components/ui/`:**
- Purpose: Headless primitives (shadcn-style on Radix). Reserved for low-level building blocks.

**`src/web/src/components/shared/`:**
- Purpose: Cross-domain composites built from `ui/` primitives.
- Contains: `DataTable.tsx`, `EmptyState.tsx`, `MetricCard.tsx`, `StatChart.tsx`, `Pagination.tsx`, `RatingStars.tsx`, `SearchBar.tsx`, `ConfirmDialog.tsx`, `DrawerForm.tsx`, `FilterPanel.tsx`, `StatusBadge.tsx`.

**`src/web/src/types/`:**
- Purpose: Backend contract types mirroring server JSON.
- Contains: `api.ts` (auth, chat, knowledge, task, console, preferences), `admin.ts` (admin/marketplace).

**`src/web/e2e/`:**
- Purpose: Playwright suites against a running server + web stack.
- Contains: `admin-marketplace.spec.ts`, `fixtures/`. Playwright config sits at `src/web/playwright.config.ts`.

**`config/`:**
- Purpose: Mainline configuration source-of-truth.
- Contains: `.env.example` only. Per `docs/architecture/current-system-contracts.md` §9, any env-var change must update `config/.env.example`, `src/server/internal/config/config.go`, and `docs/architecture/current-system-contracts.md` together.

**`scripts/`:**
- Purpose: Repo-wide automation, treated as the single root verification surface.
- Contains: `check.sh` (docs + web build + server checks), `test.sh` (vitest + go test), `dev.sh`, `deploy-validate.sh`, `deploy-smoke.sh`, `verify-quality-gates.sh`.

**`docs/`:**
- Purpose: Authoritative documentation. Mainline docs live here, NOT in the repo root.
- Contains: `API.md` (route index), `architecture/` (contracts + decisions), `release/` (RC checklist + runbooks), `governance/`, `reports/` (timestamped audits), `superpowers/` (specs + plans).

**`.planning/`:**
- Purpose: GSD planning artifacts.
- Contains: `codebase/` (these maps), `milestones/`, `phases/`. Generated by `/gsd:*` commands.

**`deploy/kubernetes/`:**
- Purpose: K8s manifests for staging/prod parity.
- Contains: `namespace.yaml`, `configmap.yaml`, `secret.example.yaml`, `postgres.yaml`, `redis.yaml`, `server.yaml`, `web.yaml`.

**`lobehub/`, `new-api/`:**
- Purpose: Non-mainline reference checkouts only. They are NOT part of root workspace, CI, or release. Do not import from them; do not modify them in mainline commits.

## Key File Locations

**Entry Points:**
- `src/server/cmd/server/main.go` — Backend process entry.
- `src/server/cmd/migrate/main.go` — Migration runner entry.
- `src/web/src/main.tsx` — SPA mount (ReactDOM root).
- `src/web/src/app/App.tsx` — App component, builds router via `useMemo`.
- `src/web/src/app/router.tsx` — Route tree (Marketing / Workspace / Console / Admin).

**Configuration:**
- `config/.env.example` — Authoritative env contract.
- `src/server/internal/config/config.go` — Env parsing into `Config` struct.
- `src/web/vite.config.ts` — Vite + Vitest config.
- `src/web/tailwind.config.ts` — Tailwind v3 setup.
- `src/web/playwright.config.ts` — E2E config.
- `package.json` (root) — pnpm workspace + script orchestration.
- `pnpm-workspace.yaml` — Lists `src/web` only (server is not a workspace package).
- `docker-compose.yml`, `Dockerfile.server`, `Dockerfile.web` — Containerization.
- `.github/workflows/ci.yml` — CI definition.

**Core Logic:**
- `src/server/internal/http/router.go` — All `/api/v1/*` route registrations.
- `src/server/internal/http/server.go` — Mounts Relay under `/v1/*`, ensures default channel.
- `src/server/internal/relay/relay.go` — Relay gin engine assembly.
- `src/server/internal/relay/router.go` — `Router.Route` and `RouteWithBilling`.
- `src/server/internal/relay/handler/router.go` — 35-route OpenAI-compatible table.
- `src/server/internal/chat/relay_gateway.go` — Self-HTTP client into the Relay.
- `src/server/internal/agent/runner.go` — Tool-calling loop.
- `src/server/internal/task/runtime.go` — SOLO state machine.
- `src/web/src/services/http/client.ts` — Envelope-aware HTTP client.
- `src/web/src/features/auth/store.ts` — Auth state machine store.

**Testing:**
- Server unit tests: `*_test.go` colocated with each `*.go` (e.g., `chat/service_test.go`, `relay/router_test.go`).
- Server integration: `internal/http/route_surface_test.go`, `internal/http/server_test.go` — gated by `TEST_DATABASE_URL`.
- Web unit tests: `*.test.ts(x)` colocated with source (e.g., `routes/workspace/ChatPage.test.tsx`, `services/http/client.test.ts`).
- Web E2E: `src/web/e2e/admin-marketplace.spec.ts` + `src/web/e2e/fixtures/`.

**Documentation:**
- `docs/architecture/current-system-contracts.md` — Authoritative contract baseline (HTTP envelope, routes, env, change rules).
- `docs/architecture/solo-runtime-decision.md` — SOLO runtime scope boundary.
- `docs/architecture/knowledge-evolution-decision.md` — Knowledge retrieval scope.
- `docs/API.md` — Canonical route index.
- `docs/release/rc-checklist.md` — Release gate.

## Naming Conventions

**Files (Go):**
- `service.go` — Use-case orchestration for a bounded context.
- `store.go` — SQL persistence; matching `*_test.go` runs unit + integration tests (integration is skipped without `TEST_DATABASE_URL`).
- `<feature>_handler.go` — HTTP handler in `internal/http/` (e.g., `chat_handler.go`, `admin_handler.go`).
- `routes_<feature>.go` — Optional supplementary route registration helpers in `internal/http/`.
- `types.go` — Shared structs within a package.
- `<concern>_test.go` — Tests live next to source.
- Migrations: `NNNN_descriptive_snake_case.sql` (4-digit zero-padded, monotonically increasing).
- Binaries: `cmd/<binary-name>/main.go`, one directory per binary.

**Files (TypeScript / React):**
- Page components: `PascalCasePage.tsx` (e.g., `ChatPage.tsx`, `AdminUsersPage.tsx`).
- View splits for complex pages: `<Page>View.tsx` (e.g., `SoloPageView.tsx`, `KnowledgePageView.tsx`).
- Layouts: `<Area>Layout.tsx` under `features/layouts/`.
- UI primitives: `kebab-case.tsx` under `components/ui/` (shadcn convention: `button.tsx`, `dropdown-menu.tsx`).
- Shared composites: `PascalCase.tsx` under `components/shared/` (`DataTable.tsx`, `EmptyState.tsx`).
- Feature modules: lowercase domain dirs (`features/auth/`, `features/marketplace/`); files inside use `camelCase.ts` for utilities (`useAuthBootstrap.ts`, `store.ts`, `api.ts`) and `PascalCase.tsx` for components (`ProtectedRoute.tsx`, `AdminRoute.tsx`).
- Tests: `<source>.test.ts(x)` colocated. Behavioral tests use `<source>.behavior.test.tsx` (e.g., `ChatPage.behavior.test.tsx`).
- E2E specs: `*.spec.ts` under `src/web/e2e/`.

**Directories:**
- Go: lowercase, one word per bounded context (`auth`, `chat`, `knowledge`, `marketplace`, `relay/handler`).
- TS: lowercase area names under `routes/` and `features/` (`workspace`, `console`, `admin`, `marketplace`); component baskets use `ui/` and `shared/`.

**Go module / package:**
- Module path: `oblivious/server` (set in `src/server/go.mod`).
- Package name follows directory name; tests use `package <name>` (same package), not `_test` package, except where black-box testing is needed.

## Where to Add New Code

**New backend feature (new bounded context):**
- Implementation: `src/server/internal/<feature>/service.go`, `store.go`, `types.go`, plus tests.
- HTTP handler: `src/server/internal/http/<feature>_handler.go`. Define a constructor `new<Feature>Handler(...)` mirroring `newChatHandler`.
- Route registration: edit `src/server/internal/http/router.go` and add `mux.Handle(...)` blocks under `authMiddleware.requireSession` (or `requireAdmin` for admin routes). Group related sub-paths into a single `mux.Handle("/api/v1/.../", ...)` with `strings.Split` dispatch, matching the chat/task/agent patterns.
- DB schema: add the next-numbered file in `src/server/migrations/`. Do NOT edit existing migrations.
- Tests: `service_test.go`, `store_test.go`, `<feature>_handler_test.go`. Update `route_surface_test.go` to cover the new surface.

**New LLM-callable capability:**
- Always go through `chat.ChatGateway` / `chat.RelayGateway` (HTTP self-call to `/v1`). Do not import `internal/relay` from outside `internal/http/server.go` and the Relay package itself.
- If you need a new upstream API type, add it to `internal/relay/handler/router.go` route table, write the corresponding handler in `internal/relay/handler/`, and extend `internal/relay/channel/openai_adapter.go` if the request shape differs.

**New `/api/v1/*` endpoint on an existing context:**
- Method on the existing `<Feature>Service` in `internal/<feature>/service.go`.
- Method on the existing `<feature>Handler` in `internal/http/<feature>_handler.go`.
- Route line in `internal/http/router.go`. Always use `writeJSON` / `writeError` for responses.

**New frontend page:**
- File: `src/web/src/routes/<area>/<Name>Page.tsx` (+ optional `<Name>PageView.tsx` split for complex pages).
- Wire-up: add the route in `src/web/src/app/router.tsx` under the correct layout. Auth-required pages must sit under `<ProtectedRoute>`; admin pages must additionally sit under `<AdminRoute>`.
- API access: add or extend `src/web/src/features/<domain>/api.ts`; never call `fetch` directly from a page component.
- Types: extend `src/web/src/types/api.ts` (or `types/admin.ts`) to match the backend response shape.
- Tests: `<Name>Page.test.tsx` next to the page.

**New shared UI primitive:**
- shadcn-style headless primitive → `src/web/src/components/ui/<kebab-case>.tsx`.
- Cross-domain composite built from primitives → `src/web/src/components/shared/<PascalCase>.tsx`.

**New utility / hook:**
- Cross-domain util → `src/web/src/lib/utils.ts` (extend) or a new file in `lib/`.
- Domain-specific hook → `src/web/src/features/<domain>/use<Name>.ts`.

**New environment variable:**
- Add to `config/.env.example` with a description.
- Parse in `src/server/internal/config/config.go` with sensible default.
- Document in `docs/architecture/current-system-contracts.md` §8 (Environment Variable Matrix). All three must be updated in the same change.

**New SQL migration:**
- File: `src/server/migrations/NNNN_<snake_case>.sql` where NNNN is `(max existing + 1)`, zero-padded.
- Never edit a previously committed migration; always add a new one.

**New script / automation:**
- `scripts/<name>.sh` — keep idempotent, prefer bash + `set -euo pipefail`.
- Wire into `scripts/check.sh` or `scripts/test.sh` if it should be a default gate.

**New documentation:**
- Architecture decisions / contracts → `docs/architecture/<name>.md`.
- Release runbooks → `docs/release/<name>.md`.
- Time-stamped audits → `docs/reports/YYYY-MM-DD-<topic>.md`.
- Roadmap-level plans → `docs/superpowers/plans/` or `docs/superpowers/specs/`.
- Never create markdown in the repo root (per `CLAUDE.md`). The few root-level `*.md` files (`README.md`, `ROADMAP.md`, `CLAUDE.md`, `CURRENT_STATUS.md`, `ARCHAEOLOGY_REPORT.md`) are grandfathered; do not add more.

## Special Directories

**`lobehub/` and `new-api/`:**
- Purpose: Non-mainline reference checkouts kept for design parity / migration research.
- Generated: No (vendored).
- Committed: Yes (but excluded from CI, `pnpm-workspace.yaml`, and release artifacts).
- Rule: Do not import from these directories in `src/server` or `src/web`. Do not bundle them. Do not run their tests as part of mainline gates.

**`.worktrees/`:**
- Purpose: Git worktrees for parallel branches (`codex-integration-foundation`, `functional-completion`).
- Generated: Yes (created by `git worktree add`).
- Committed: No.

**`.claude/`, `.claude-flow/`:**
- Purpose: Local Claude agent configuration, skills, helpers, sessions, learning.
- Generated: Partially (sessions/metrics/logs auto-populated).
- Committed: Selectively (skills + commands committed; logs and sessions ignored).

**`.planning/`:**
- Purpose: GSD command artifacts (codebase maps, milestone plans, phase plans).
- Generated: Yes (by `/gsd:*` commands).
- Committed: Yes.

**`src/server/src/server/`:**
- Purpose: Legacy nested mirror created by an earlier scaffold; effectively dead but present on disk.
- Generated: No (committed historical artifact).
- Committed: Yes.
- Rule: Do not add new code here. All new server code goes under `src/server/internal/<context>/` or `src/server/cmd/<binary>/`.

**`.tmp/`:**
- Purpose: Local-only Go build cache and corepack cache (referenced by `package.json` scripts via `GOCACHE=.tmp/go-build`, `GOMODCACHE=.tmp/go-mod`, `COREPACK_HOME=.tmp/corepack`).
- Generated: Yes.
- Committed: No (gitignored).

## File Organization Rules (from `CLAUDE.md`)

- **NEVER save working files, tests, or markdown to the repo root.** Use the subdirectories below.
- Source code → `src/` (split into `src/server/` and `src/web/`).
- Tests → colocated with source (`*_test.go` next to `*.go`; `*.test.ts(x)` next to source). E2E → `src/web/e2e/`.
- Documentation → `docs/` (architecture / release / governance / reports / superpowers).
- Configuration → `config/` (`.env.example` lives here as the single env contract).
- Utility scripts → `scripts/`.
- Examples → `/examples` (currently unused; create only if examples are needed).
- **Files must stay under 500 lines.** `src/server/internal/http/router.go` is over budget and is a known split target.
- Public APIs must use typed interfaces (Go: define an interface in the consumer package; TS: declare types in `src/web/src/types/` or alongside `api.ts`).
- Never commit secrets, credentials, or `.env` files. `.env.example` only.

---

*Structure analysis: 2026-05-16*
