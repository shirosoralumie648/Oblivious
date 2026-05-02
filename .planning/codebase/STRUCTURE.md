---
last_mapped_commit: c0e55fdbb3aaed7da80a0f7f2399237aed13bca3
mapped_dirty_worktree: true
---

# Codebase Structure

**Analysis Date:** 2026-05-02

## Top-Level Layout

```text
Oblivious/
├── .github/workflows/        # CI release, web, and server jobs
├── .planning/                # GSD state, roadmap, phases, and codebase map
├── config/                   # Example runtime environment
├── docs/                     # API, architecture, governance, release, reports, specs
├── lobehub/                  # Imported/reference LobeHub source tree
├── new-api/                  # Imported/reference new-api source tree
├── scripts/                  # check/test/dev automation
├── src/server/               # Active Go backend
├── src/web/                  # Active React frontend
├── package.json              # Root scripts and pnpm manager pin
├── pnpm-workspace.yaml       # Active pnpm workspace membership: src/web only
└── pnpm-lock.yaml
```

## Active Backend Structure

```text
src/server/
├── cmd/
│   ├── migrate/              # migration runner
│   └── server/               # production/dev server entrypoint
├── internal/
│   ├── admin/                # admin stats/users/channels/routes/plans/audit/reviews domain
│   ├── agent/                # agent CRUD, conversations, runner, tool execution
│   ├── auth/                 # users, sessions, password hashing, ID generation
│   ├── chat/                 # conversations, message gateway, Relay gateway
│   ├── config/               # env loader and config validation
│   ├── console/              # usage/access/models/billing summaries
│   ├── db/                   # database open helper
│   ├── http/                 # router, middleware, handlers, response envelope
│   ├── knowledge/            # knowledge bases/documents/retrieval
│   ├── marketplace/          # published agents, installs, reviews, categories, search
│   ├── mcp/                  # MCP client and builtin tools
│   ├── memory/               # vector memory documents, chunking, embeddings
│   ├── metrics/              # Prometheus metrics helpers
│   ├── notification/         # notification service
│   ├── quota/                # quotas, packages, topups, billing sessions
│   ├── relay/                # provider relay engine and handlers
│   ├── stripe/               # Stripe webhook package
│   ├── task/                 # task CRUD/runtime state machine
│   ├── usage/                # usage recorder
│   ├── userprefs/            # user preferences
│   └── ws/                   # WebSocket hub/handler
└── migrations/               # app DB migrations 0001-0024
```

## Active Frontend Structure

```text
src/web/
├── components.json           # shadcn configuration
├── package.json              # frontend dependencies and scripts
├── postcss.config.mjs
├── tailwind.config.ts
├── tsconfig.json
├── vite.config.ts
└── src/
    ├── app/                  # router, providers, root context
    ├── features/
    │   ├── auth/             # auth API/store/bootstrap/protected routes
    │   ├── chat/             # chat API wrapper
    │   ├── console/          # console API and dashboard components
    │   ├── knowledge/        # knowledge API wrapper
    │   ├── layouts/          # marketing/workspace/console/admin layouts
    │   └── tasks/            # task API wrapper
    ├── lib/                  # frontend utilities
    ├── routes/
    │   ├── admin/            # AdminHomePage, AdminUsersPage
    │   ├── console/          # console pages
    │   ├── marketing/        # public marketing/auth pages
    │   └── workspace/        # chat, knowledge, solo, settings, marketplace
    ├── services/http/        # HTTP client, envelope, errors, stream/upload helpers
    ├── store/                # app-level store helpers
    ├── test/                 # Vitest setup
    ├── theme/                # global CSS and design tokens
    └── types/                # shared frontend API types
```

## Where To Add Code

**Backend HTTP endpoint:**
- Add request/response handling to `src/server/internal/http/<domain>_handler.go`.
- Register the route in `src/server/internal/http/router.go`.
- Keep service logic in `src/server/internal/<domain>/service.go`.
- Keep SQL in `src/server/internal/<domain>/store.go` or a focused `*_store.go`.
- Add or update migrations under `src/server/migrations/` for schema changes.
- Add tests next to the touched package as `*_test.go`.

**Backend domain behavior:**
- Prefer adding methods to the existing domain `Store` interface and `Service` struct.
- For admin subdomains, follow the existing split in `src/server/internal/admin/channel_*`, `route_*`, `plan_*`, `user_*`, and `audit_*`.
- For marketplace, use `src/server/internal/marketplace/service.go`, `store.go`, `search.go`, and `types.go`.

**Frontend page:**
- Add route components under `src/web/src/routes/<area>/`.
- Add reusable API calls under `src/web/src/features/<domain>/api.ts`.
- Add shared domain types to `src/web/src/types/api.ts`.
- Wire navigation in the relevant layout under `src/web/src/features/layouts/`.
- Add route declarations to `src/web/src/app/router.tsx`.

**Frontend test:**
- Co-locate tests next to the component/module: `Name.test.tsx` for components, `name.test.ts` for stores/helpers.
- Use `createMemoryRouter` via `createAppRouter(initialEntries)` when testing routes.

## Naming Patterns

**Go:**
- Package names are short lowercase domain names: `auth`, `chat`, `relay`, `marketplace`.
- Public service constructors use `NewService`, `NewSQLStore`, or focused names like `NewRelayGateway`.
- Request DTOs use `CreateXRequest`, `UpdateXRequest`, or domain-specific names.
- JSON fields are camelCase even when Go fields are PascalCase.

**TypeScript/React:**
- Components/pages/layouts use `PascalCase.tsx`.
- APIs, stores, hooks, and utilities use `camelCase.ts`.
- API factories use `create<Name>Api`.
- Test files use `.test.ts` or `.test.tsx`; behavior-level interaction tests may use `.behavior.test.tsx`.

## Planning And Docs

**GSD artifacts:**
- Current state: `.planning/STATE.md`.
- Project overview: `.planning/PROJECT.md`.
- Roadmap: `.planning/ROADMAP.md`.
- Codebase map: `.planning/codebase/`.
- Phase artifacts: `.planning/phases/`.

**Product/design docs:**
- API reference: `docs/API.md`.
- Current contracts: `docs/architecture/current-system-contracts.md`.
- Specs and plans: `docs/superpowers/specs/` and `docs/superpowers/plans/`.

## Reference Trees

- `lobehub/` and `new-api/` are large external references. Do not run normal repo-wide edits or package commands inside them unless the task explicitly targets those imports.
- `.worktrees/` contains local worktree snapshots and should be ignored for mapping and default search.
- Generated/dependency/cache directories such as `node_modules/`, `.git/`, `.tmp/`, and build outputs should stay out of codebase maps.

## Files That Often Matter First

- Backend wiring: `src/server/internal/http/router.go`, `src/server/internal/http/server.go`.
- Backend config: `src/server/internal/config/config.go`, `config/.env.example`.
- Backend migrations: `src/server/migrations/`.
- Frontend routing: `src/web/src/app/router.tsx`.
- Frontend HTTP: `src/web/src/services/http/client.ts`, `src/web/src/services/http/envelope.ts`.
- Verification: `scripts/check.sh`, `scripts/test.sh`, `.github/workflows/ci.yml`.
