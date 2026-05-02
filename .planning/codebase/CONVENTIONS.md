---
last_mapped_commit: c0e55fdbb3aaed7da80a0f7f2399237aed13bca3
mapped_dirty_worktree: true
---

# Coding Conventions

**Analysis Date:** 2026-05-02

## General Rules

- Treat `src/server/` and `src/web/` as the active product code.
- Treat `lobehub/` and `new-api/` as reference imports unless a task explicitly scopes work there.
- Keep changes narrowly scoped. This checkout often has many unrelated local changes.
- Use structured APIs and existing domain services before adding ad hoc parsing or cross-package SQL.

## Go Backend

### Package Structure

- Add backend code under `src/server/internal/<domain>/`.
- Keep the HTTP layer in `src/server/internal/http/`.
- Keep migrations append-only under `src/server/migrations/`.
- Keep SQL implementation in a domain store, not in handlers.

### Service/Store Pattern

Use the existing split:

```go
type Store interface {
    // persistence methods
}

type Service struct {
    store Store
}

func NewService(store Store) *Service {
    return &Service{store: store}
}
```

Examples:
- `src/server/internal/chat/service.go` and `src/server/internal/chat/store.go`.
- `src/server/internal/agent/service.go` and `src/server/internal/agent/store.go`.
- `src/server/internal/memory/service.go` and `src/server/internal/memory/store.go`.
- `src/server/internal/admin/*_service.go` and `src/server/internal/admin/*_store.go`.

### Context And Auth

- Put `context.Context` first in service/store methods.
- Pass `auth.Session` to service methods that must enforce user ownership.
- Validate resource ownership in services before store mutations, as in `src/server/internal/agent/service.go` and `src/server/internal/memory/service.go`.
- Use `auth.NewID("<prefix>")` for app IDs when a package does not require UUIDs.

### Errors

- Wrap lower-level errors with operation context using `fmt.Errorf("operation: %w", err)`.
- Return domain strings such as `"access denied"` and `"not found"` only where handlers already map those strings to status codes.
- Prefer exported sentinel errors for new reusable conditions rather than matching error strings in new code.

### HTTP Handlers

- Decode JSON in handlers, trim/validate request fields, and delegate to services.
- Use `writeSuccess` and `writeError` from `src/server/internal/http/response.go`.
- Keep route-level method switching in `src/server/internal/http/router.go` or a focused route helper.
- Use `sessionFromContext(r)` after `authMiddleware.requireSession`.
- Use `authMiddleware.requireAdmin` for admin-only routes.

### Config

- Add new env vars to all relevant places:
  - `src/server/internal/config/config.go`
  - `config/.env.example`
  - `docs/architecture/current-system-contracts.md`
  - `scripts/check.sh` docs consistency checks when required

### SQL And Migrations

- Add schema changes as new numbered files in `src/server/migrations/`.
- Do not edit old migrations to change already-recorded behavior; add a corrective migration instead.
- Use PostgreSQL-native features already present in the repo: arrays with `pq.Array`, JSONB, filtered aggregates, and pgvector indexes.
- Keep user isolation in SQL predicates where possible, especially for memory, knowledge, agents, MCP servers, and tasks.

## Relay Conventions

- Keep provider-agnostic routing in `src/server/internal/relay/router.go`.
- Keep provider request translation in `src/server/internal/relay/channel/`.
- Keep OpenAI-compatible endpoint handlers under `src/server/internal/relay/handler/`.
- Use `RouteWithBilling` when provider traffic should affect quota.
- Preserve trusted internal identity propagation through `src/server/internal/relay/types/` when app-originated calls enter Relay.

## TypeScript Frontend

### File Naming

- Route/page components: `PascalCase.tsx`, for example `ChatPage.tsx` and `AdminUsersPage.tsx`.
- Layout components: `PascalCase.tsx` under `src/web/src/features/layouts/`.
- API modules and stores: `camelCase.ts`, for example `client.ts`, `store.ts`, `api.ts`.
- Tests: co-located `.test.ts` or `.test.tsx`.

### API Calls

- Prefer typed API factories over direct `fetch`:
  - `src/web/src/features/chat/api.ts`
  - `src/web/src/features/knowledge/api.ts`
  - `src/web/src/features/console/api.ts`
  - `src/web/src/features/tasks/api.ts`
- Use `createHttpClient` from `src/web/src/services/http/client.ts`.
- Let `unwrapEnvelope` handle `{ ok, data, error }` responses.
- New Admin and Marketplace frontend work should add typed API wrappers before adding more direct `fetch` calls.

### React Patterns

- Keep global app state small and centralized in `src/web/src/app/appContext.tsx`.
- Use route layouts for shell/navigation concerns.
- Keep page-local loading/error/UI state inside the page component unless multiple pages share it.
- Prefer Testing Library tests for user-visible behavior.

### Styling

- Theme tokens and shadcn/Tailwind imports live in `src/web/src/theme/tokens.css`.
- Global reset and legacy base styling live in `src/web/src/theme/global.css`.
- New UI should use the existing shadcn/Tailwind setup, not a separate CSS framework.
- Keep page-specific classes stable enough for tests and avoid coupling behavior tests to visual-only class names.

## Documentation Conventions

- Planning docs should cite real paths in backticks.
- Keep status docs grounded in code/routes/tests when `.planning/STATE.md` drifts.
- Record known gaps in `.planning/codebase/CONCERNS.md` rather than hiding them in optimistic roadmap language.

## Verification Conventions

- Use repo scripts before ad hoc commands when they cover the task:
  - `bash scripts/check.sh docs`
  - `bash scripts/check.sh web`
  - `bash scripts/check.sh server`
  - `bash scripts/test.sh web`
  - `bash scripts/test.sh server`
- For broad backend changes, run `cd src/server && GOCACHE=../../.tmp/go-build GOMODCACHE=../../.tmp/go-mod go test ./...`.
- For frontend changes, run `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test` and `COREPACK_HOME=.tmp/corepack pnpm --dir src/web build`.
