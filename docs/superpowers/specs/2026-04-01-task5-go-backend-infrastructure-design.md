# Task 5 Design — Go Backend Infrastructure (Auth Placeholder Included)

## Historical Note

This document reflects the original Task 5 scaffold target only.

It is no longer the current execution baseline because the codebase has already moved beyond scaffold scope:

- PostgreSQL runtime and migrations exist
- real auth and session flows exist
- user preferences exist
- chat, knowledge, task, and console modules exist

For current execution, use:

- `docs/architecture/current-system-contracts.md`
- `docs/reports/2026-04-06-execution-progress-review.md`

Keep the remaining sections in this file as historical context for how Task 5 started.

## Context
This design defines the backend foundation slice for Task 5 in the current phase: build a runnable and extensible server shell aligned with the approved monorepo framework. It includes auth route placeholders (no real auth logic yet).

## Goals
- Establish `src/server` as the backend module root.
- Provide a stable HTTP bootstrap path (`cmd/server` → config → router → middleware).
- Expose baseline platform routes for health and auth placeholders.
- Define consistent JSON response/error shape for the scaffold stage.
- Add minimal tests that lock routing/config behavior for future tasks.

## Out of Scope (Task 5)
- No PostgreSQL connection or migrations.
- No real session/token validation.
- No user persistence, workspace logic, or chat logic.
- No provider/gateway/billing implementation.

## Directory and Boundaries
```text
src/server/
  go.mod
  cmd/server/main.go
  internal/config/
    config.go
    config_test.go
  internal/http/
    server.go
    router.go
    middleware.go
    response.go
    server_test.go
```

### Boundary Rules
- `cmd/server` only wires startup and shutdown lifecycle.
- `internal/config` owns env loading and normalized runtime config.
- `internal/http` owns transport concerns (routing, middleware, response formatting).
- Business modules are not introduced in Task 5.

## Runtime Configuration
Environment-driven config with safe defaults for scaffold stage:
- `SERVER_PORT` (default `8080`)
- `APP_ENV` (default `development`)
- `CORS_ALLOWED_ORIGINS` (optional, comma-separated)

Validation rules for this phase:
- Port must be numeric and in valid range.
- Missing optional values fall back to defaults.
- Invalid required format returns explicit startup error.

## HTTP Surface (Task 5)
### Health
- `GET /healthz`
  - `200 OK`
  - Body: `{ "status": "ok" }`

### Auth Placeholders
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/register`
- `GET /api/v1/auth/me`
- `POST /api/v1/auth/logout`

All placeholder routes return deterministic scaffold payloads with `200` and a marker showing this is a placeholder contract for Task 5.

Example shape:
```json
{
  "ok": true,
  "data": {
    "placeholder": true,
    "route": "login"
  },
  "error": null
}
```

## Middleware Stack
Initial chain order:
1. Recover middleware (panic to structured 500)
2. Request ID middleware (attach request id to context/response header)
3. Basic access log middleware (method/path/status/duration)

Constraints:
- Middleware must stay transport-level only.
- No auth enforcement yet in Task 5.

## Response/Error Contract
Uniform JSON envelope for scaffold consistency:
- Success: `{ ok: true, data: <payload>, error: null }`
- Failure: `{ ok: false, data: null, error: { code, message } }`

Task 5 only needs generic failure codes for malformed requests and internal errors.

## Testing Strategy
Minimum tests in Task 5:
1. `internal/config/config_test.go`
   - loads defaults correctly
   - rejects invalid port values
2. `internal/http/server_test.go`
   - `/healthz` returns 200 and expected body
   - each auth placeholder route is registered and returns expected envelope
3. (Optional in same file) middleware smoke assertion
   - request id header exists on response

No DB or integration test coupling in this phase.

## Acceptance Criteria
Task 5 is complete when:
- `src/server` module builds.
- Server starts with env-based config.
- Defined routes respond with stable JSON contracts.
- Config and HTTP route tests pass.
- Structure leaves clear extension points for Task 6/7 modules.

## Task 6/7 Handoff Notes
- Task 6 replaces auth placeholder handlers with real auth/user/workspace behavior.
- Task 7 introduces conversation/chat/gateway/billing/console/system modules behind the same HTTP shell.
- Existing response contract can be reused or evolved without breaking routing bootstrap.
