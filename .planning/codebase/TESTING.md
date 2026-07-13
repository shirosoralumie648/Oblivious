# Testing Patterns

**Analysis Date:** 2026-07-13

## Test Entry Points

Use the repository scripts before ad hoc commands:

- `bash scripts/test.sh all` — web Vitest, server Go tests, and browser E2E.
- `bash scripts/test.sh web` — `pnpm --dir src/web test`.
- `bash scripts/test.sh server` — Go tests under `src/server/`; DB integration is skipped when `TEST_DATABASE_URL` is unset unless `OBLIVIOUS_REQUIRE_TEST_DATABASE=true`.
- `bash scripts/test.sh e2e` — Playwright browser E2E under `src/web/e2e/`.
- `bash scripts/check.sh all` — docs/release gates, web checks, server checks, Relay security, and dependency/security checks.
- `bash scripts/verify-quality-gates.sh` — release/docs/evidence quality gate.

The scripts create or use `.tmp/corepack`, `.tmp/go-build`, `.tmp/go-mod`, and `.tmp/ms-playwright` by default.

## Backend Go Tests

### Layout

- Backend tests live beside packages as `*_test.go` under `src/server/`.
- Command package tests live under `src/server/cmd/*/main_test.go`.
- Domain tests live under paths such as `src/server/internal/agent/`, `src/server/internal/workflow/`, `src/server/internal/knowledge/`, `src/server/internal/marketplace/`, `src/server/internal/quota/`, and `src/server/internal/relay/`.
- HTTP route and contract tests live under `src/server/internal/http/`.
- gRPC adapter tests live under `src/server/pkg/agent/`, `src/server/pkg/workflow/`, and `src/server/internal/grpc/*/`.

### Patterns

- Use table-driven tests with `t.Run` for route, service, and store permutations.
- Use `httptest` for HTTP routes and middleware behavior.
- Keep persistence behavior in store tests and orchestration behavior in service tests.
- When changing tenant scope, add tests in both the domain package and the HTTP layer if the behavior is user/API-visible.
- When changing Relay/provider paths, include Relay security, quota/billing, and request-log expectations where relevant.

### Database-backed tests

- `TEST_DATABASE_URL` controls DB-backed coverage.
- `scripts/test.sh server` runs `go test ./... -count=1` when `TEST_DATABASE_URL` is absent, then prints an explicit integration-skip message.
- Set `OBLIVIOUS_REQUIRE_TEST_DATABASE=true` when a command must fail closed without `TEST_DATABASE_URL`.
- `scripts/verify-commercial-db-evidence.sh` provides focused DB evidence profiles such as `tenant-membership-lifecycle`, `tenant-cross-surface`, `quota-sql-isolation`, `workflow-sql-isolation`, `agent-runtime-memory`, and `marketplace-money-movement`.
- `scripts/verify-commercial-db-evidence-profiles.sh` and `scripts/verify-commercial-db-evidence-profiles-fixtures.sh` guard that required tests remain pinned in profiles.

## Frontend Unit And Component Tests

### Framework

- Frontend tests use Vitest with jsdom from `src/web/vite.config.ts`.
- Test setup is `src/web/src/test/setup.ts`.
- Package scripts in `src/web/package.json` define `test`, `build`, and `test:e2e`.
- Common libraries include `@testing-library/react`, `@testing-library/jest-dom`, `vitest`, and `jsdom`.

### Layout

- Page tests live beside route components, for example `src/web/src/routes/workspace/WorkflowsPage.test.tsx`, `src/web/src/routes/workspace/ChatPage.behavior.test.tsx`, `src/web/src/routes/workspace/KnowledgePage.test.tsx`, and `src/web/src/routes/admin/AdminAlertsPage.tsx` coverage.
- Feature API tests live beside feature API clients, for example `src/web/src/features/admin/api.test.ts`.
- Router-level behavior is covered in `src/web/src/app/router.test.tsx`, which is intentionally broad and heavily mocked.

### Mocking conventions

- Use `vi.mock` for feature API clients and heavy UI dependencies such as `@xyflow/react`.
- Keep API serialization assertions in feature API tests rather than only in page tests.
- Reset mocks in `beforeEach` for route-level suites.
- Do not let router/page mocks become proof of backend persistence or provider behavior.

## Browser E2E Tests

### Framework and layout

- Browser E2E uses Playwright via `pnpm --dir src/web test:e2e`.
- Specs live under `src/web/e2e/*.spec.ts`.
- Fixtures live under `src/web/e2e/fixtures/`.
- Many fixtures route `**/api/v1/**` with `page.route` and return deterministic API responses.

### Fixture conventions

- Use `fixture_contract_mismatch` errors for unexpected query parameters, request methods, or payload shapes.
- Keep fixture data realistic enough to exercise user-visible quota, billing, tenant, review, settlement, and error states.
- Add a backend route/API test when a browser fixture adds a new backend contract.
- Treat Playwright fixture passes as UI proof only unless they hit a real backend.

## Release And Evidence Tests

### Core verifier families

- `scripts/verify-commercial-completion.sh` is the strict commercial completion orchestrator.
- `scripts/run-target-release-evidence.sh` is the target evidence runner for final release proof.
- `scripts/verify-target-release-evidence.sh` and `scripts/verify_target_release_evidence.py` validate target evidence manifests.
- `scripts/assemble-target-release-evidence.sh` and `scripts/assemble_target_release_evidence.py` assemble external proof files into a manifest.
- `scripts/verify-commercial-preflight.mjs` blocks unsafe final-readiness inputs before expensive checks.
- `scripts/verify-openapi-contract.sh` and `scripts/verify_openapi_contract.py` protect OpenAPI/API route behavior.

### Fixture verifier pattern

- Every verifier that accepts complex evidence should have a paired fixture script, such as `scripts/verify-target-release-evidence-fixtures.sh`, `scripts/assemble-target-release-evidence-fixtures.sh`, and `scripts/verify-commercial-preflight-fixtures.sh`.
- Negative fixtures should mutate one field at a time and assert the exact failure class.
- When adding a new required release evidence field, update validator code, fixture mutation code, fixture shell checks, release docs, and quality gates in the same change.

## What To Run By Change Type

- **Go domain service/store change:** `cd src/server && go test ./internal/<domain> -count=1`; add `TEST_DATABASE_URL` for SQL-backed behavior.
- **HTTP route/API change:** package-level `go test ./internal/http -count=1`, then `bash scripts/verify-openapi-contract.sh` if public contract changes.
- **Relay/outbound/tool change:** focused Relay/agent/tool tests plus `bash scripts/verify-relay-security.sh`.
- **Frontend page change:** `COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec vitest run <page-or-feature-test>`; run `pnpm --dir src/web exec tsc --noEmit` for type-sensitive changes.
- **Browser journey change:** relevant `pnpm --dir src/web exec playwright test e2e/<spec>.spec.ts --project=chromium`, with `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH` when using system Chrome.
- **Release verifier change:** narrow fixture script, `bash scripts/verify-quality-gates.sh`, and `git diff --check`.
- **Dependency change:** relevant package-manager install/update, `bash scripts/verify-dependency-security.sh`, and lockfile review across pnpm/npm lockfiles.
- **Docs/release claim change:** `bash scripts/check.sh docs` and any verifier mentioned in the changed docs.

## CI / Local Boundary

- Local absence of `TEST_DATABASE_URL` is an explicit integration skip in `scripts/test.sh`, not proof of DB behavior.
- Strict final commercial verification requires DB, deploy, Kubernetes, backup/restore, and target evidence inputs; local fixture runs do not satisfy target/live proof.
- Use `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true` only for non-final local evidence and never in final target readiness commands.
- Use `OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=true` only for local investigation; strict final readiness rejects it.

## Test Hygiene

- Prefer narrow tests near the changed package over expanding the large router or workflow page suites.
- Keep mocks reset between tests and avoid shared mutable fixture state.
- Use explicit skip messages when environment requirements are unavailable.
- Preserve line-oriented shell syntax checks with `bash -n` for changed scripts.
- Run `git diff --check` before committing docs or script-heavy changes.

---

*Testing analysis: 2026-07-13*
