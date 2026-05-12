# Codebase Concerns

**Analysis Date:** 2026-05-12

## Current Blocker

DEPLOY-01 remains blocked on real runtime validation.

Evidence:
- `.planning/STATE.md` states that TEST-01, TEST-02, and DOC-01 are complete, while DEPLOY-01 lacks real Docker/Kubernetes startup evidence.
- `scripts/deploy-validate.sh` requires Docker, Docker Compose, successful `docker info`, image builds, compose startup, and a `/healthz` smoke.
- `docs/release/deployment-runtime-remediation.md` records the host-level blocker: Docker daemon access is unavailable and Kubernetes tooling is missing.

Impact:
- Docker, compose, Kubernetes, smoke, and release checklist assets exist.
- Configuration being present is not the same as the actual stack starting and passing health checks.
- v03.2 should not be archived or called shipped until a real Docker or Kubernetes path passes.

Mitigation:
- Restore Docker daemon access or provide Kubernetes tooling.
- Run `bash scripts/deploy-validate.sh` or an equivalent real Kubernetes deployment smoke.
- Record the runtime evidence in the Phase 4 release artifacts before closing DEPLOY-01.

## Dirty Worktree Risk

The repository is very dirty. Current changes span planning files, docs, scripts, backend packages, frontend routes, tests, CI, and untracked local agent/runtime directories.

Impact:
- This map describes the working tree, not a clean commit.
- Broad cleanup, reset, or generated-file churn could destroy unrelated user work.
- Line-level claims can drift quickly while many source files remain modified.

Mitigation:
- Keep codebase map edits scoped to `.planning/codebase/*.md`.
- Before any implementation phase, re-check `git status --short` and the specific files touched by that phase.
- Avoid interpreting existing dirty changes as authored by the current agent.

## Router Composition Debt

`src/server/internal/http/router.go` is still a very large composition root and route registry.

Evidence:
- `router.go` directly registers auth, preferences, chat, knowledge, task, agent, memory, MCP, console, quota, notifications, WebSocket, admin, and marketplace routes.
- `src/server/internal/http/routes_auth.go`, `routes_chat.go`, `routes_knowledge.go`, `routes_preferences.go`, `routes_task.go`, and `routes_console.go` define modular route registration functions.
- `rg` shows these `register*Routes` functions are defined but not called by `NewRouter`.

Impact:
- Route ownership is split between an active monolithic file and inactive modular registration files.
- Future changes can update the wrong file and silently leave runtime behavior unchanged.
- Merge conflict and regression risk is high around `router.go`.

Mitigation:
- Either wire the modular `register*Routes` functions into `NewRouter` with tests, or remove the unused route modules.
- Keep route contract checks in `docs/API.md`, `docs/architecture/current-system-contracts.md`, and handler tests synchronized.

## Relay Handler Gaps

Several Relay handlers still contain TODO-level behavior.

Evidence:
- `src/server/internal/relay/handler/responses.go` notes Responses SSE streaming still needs implementation.
- `src/server/internal/relay/handler/batch.go` notes pre-billing and async polling tasks are still pending.
- `src/server/internal/relay/handler/files.go` notes file mapping persistence is still pending.
- `src/server/internal/relay/handler/realtime.go` notes auth, pre-billing, and settlement are still pending.

Impact:
- The `/v1` route surface is broader than the fully implemented behavior.
- Billing, quota, file lifecycle, realtime auth, and settlement can be incomplete for non-chat endpoints.
- Documentation that says a route exists should not imply full production parity.

Mitigation:
- Classify Relay endpoints by native complete, passthrough, and stub/partial behavior.
- Add endpoint-specific tests before advertising production support.
- Keep quota and billing assertions close to Relay handler tests.

## Deployment Secret Placeholder Risk

Committed deployment examples intentionally use placeholder values.

Evidence:
- `docker-compose.yml` includes placeholder `SESSION_SECRET`, empty provider-key variables, and local service credentials.
- `config/.env.example` contains local development placeholders.
- `deploy/kubernetes/secret.example.yaml` is an example manifest, not a real secret.

Impact:
- Copying examples directly to production would create weak sessions or missing provider integrations.
- Provider calls may fall back to demo behavior or fail depending on env values.

Mitigation:
- Use committed files only as templates.
- Store real secrets outside the repository or in the deployment platform secret store.
- Check `docs/release/rc-checklist.md` before recording release evidence.

## Integration Test Coverage Gap

DB-backed HTTP integration tests are optional and can be skipped.

Evidence:
- `scripts/test.sh` runs server unit tests, then skips `go test ./internal/http` when `TEST_DATABASE_URL` is unset.
- `docs/release/rc-checklist.md` requires explicit release evidence describing whether `TEST_DATABASE_URL` was set or skipped.

Impact:
- `bash scripts/test.sh server` can pass without proving the full HTTP persistence path.
- Release evidence must distinguish unit coverage from DB-backed integration coverage.

Mitigation:
- Keep the explicit skip message intact.
- For release candidates, run with a disposable Postgres `TEST_DATABASE_URL` when possible.
- If skipped, record the skip reason in the release artifact rather than implying integration proof.

## Reference Tree Confusion

`lobehub/` and `new-api/` are large enough to distort searches, dependency analysis, and TODO counts.

Evidence:
- Root `pnpm-workspace.yaml` includes only `src/web`.
- `scripts/check.sh` fails if `lobehub` or `new-api` appears as a root workspace member.
- Repo searches find many TODOs and dependencies inside reference trees that do not belong to the active root product.

Impact:
- Agents can accidentally map or change reference code while intending to work on the active product.
- Dependency and security analysis can over-report because it includes inactive reference projects.

Mitigation:
- Treat reference-tree findings as reference-only unless a phase explicitly scopes into those directories.
- Use mainline filters for release work: `src/server`, `src/web`, `config`, `scripts`, `.github/workflows`, `docs`, and deployment files.

## MCP Built-In Tool Placeholders

The MCP built-in tool adapters include placeholder behavior.

Evidence:
- `src/server/internal/mcp/builtin.go` reports placeholder search results for `web_search`.
- The calculator path reports placeholder expression handling instead of a robust expression parser.

Impact:
- Agent/tool-loop demos can appear functional while not providing real search or calculation semantics.
- Product claims about built-in tools should stay narrow until these adapters are implemented or renamed as demos.

Mitigation:
- Label placeholder tools clearly in UI/API responses.
- Replace placeholder adapters with real providers before making them default production tools.
- Add tests that prove behavior beyond successful handler plumbing.

## Stripe Wiring Ambiguity

Stripe helper code exists, but active HTTP route registration was not found in the current router scan.

Evidence:
- `src/server/internal/stripe/checkout.go` and `src/server/internal/stripe/webhook.go` implement checkout/webhook helpers.
- `src/server/internal/http/router.go` route scans did not show mounted Stripe webhook or checkout routes.

Impact:
- Billing UI or plan management may depend on code that is not reachable from the active app router.
- Webhook idempotency and subscription updates cannot be proven from helper presence alone.

Mitigation:
- Add explicit route registration and route-level tests before relying on Stripe flows.
- Document whether billing is placeholder, admin-managed, or live Stripe-backed in release notes.

## Frontend Legacy Surface Debt

There is a legacy workspace marketplace page alongside the active marketplace route set.

Evidence:
- Active routes in `src/web/src/app/router.tsx` use `src/web/src/routes/marketplace/*` for `/marketplace`.
- `src/web/src/routes/workspace/MarketplacePage.tsx` still exists but is not the active route target.
- `.planning/RETROSPECTIVE.md` records the legacy workspace marketplace page as obsolete debt.

Impact:
- Future edits can land in the obsolete page and have no user-visible effect.
- Searches for marketplace logic return both active and legacy surfaces.

Mitigation:
- Remove the legacy page or rename it as archived/reference if no route depends on it.
- Keep marketplace API and UI tests pointed at the active `src/web/src/routes/marketplace` tree.

## Migration Execution Risk

Migrations are applied by executing every SQL file in sorted order.

Evidence:
- `src/server/cmd/migrate/main.go` reads `src/server/migrations`, sorts `.sql` filenames, and executes each statement.
- No explicit migration ledger or applied-version table is visible in the migration command.

Impact:
- Running migrations against a non-empty database depends on SQL idempotency.
- Editing old migrations could break existing environments.

Mitigation:
- Keep old migrations immutable.
- Apply fixes through new append-only migrations.
- Test migration behavior against disposable databases before release validation.
