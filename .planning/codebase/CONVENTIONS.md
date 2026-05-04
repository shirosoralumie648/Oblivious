---
last_mapped_commit: 98576468acf0d72bbca7e61317dc83cd5c6ad7a9
mapped_dirty_worktree: true
analysis_date: 2026-05-04
mapper: sequential-fallback
---

# Coding Conventions

## Repository Boundaries

- Active implementation lives in `src/server`, `src/web`, `scripts`, `config`, `docs`, `.github`, `deploy`, and root Docker/compose files.
- `lobehub/` and `new-api/` are reference/imported trees. Do not add them back to `pnpm-workspace.yaml`; `scripts/check.sh docs` explicitly rejects workspace membership for those names.
- Keep `.planning` updates scoped to the active GSD workflow artifacts.

## Backend Conventions

- Packages are organized by feature under `src/server/internal/<domain>/`.
- HTTP composition happens centrally in `src/server/internal/http/router.go`; feature handlers stay in `src/server/internal/http/*_handler.go`.
- Services and stores are separated:
  - service files: `service.go`, domain-specific `*_service.go`
  - store files: `store.go`, domain-specific `*_store.go`
  - shared types: `types.go`
- Tests are colocated as `*_test.go`.
- Route handlers should use the shared response helpers in `src/server/internal/http/response.go`.
- Auth and admin protection should go through middleware in `src/server/internal/http/auth_middleware.go`.
- Avoid bypassing Relay for LLM behavior. Chat/Agent/Memory integrations are wired through Relay gateways/embedders when `RELAY_ENABLED` is true.
- Migrations are append-only under `src/server/migrations/`; do not mutate old migrations to repair already-mapped schema decisions.
- Config should be read through `src/server/internal/config/config.go`, not directly scattered through business logic.

## Frontend Conventions

- Routes are declared in `src/web/src/app/router.tsx`.
- Route components live under `src/web/src/routes/<area>/`.
- Feature API clients live under `src/web/src/features/<feature>/api.ts`.
- Auth and admin access wrappers live under `src/web/src/features/auth/`.
- Layout components live under `src/web/src/features/layouts/`.
- Shared product UI belongs in `src/web/src/components/shared/`; primitive UI belongs in `src/web/src/components/ui/`.
- Tests are colocated with routes/features using `*.test.ts` or `*.test.tsx`.
- E2E specs live in `src/web/e2e/` and use deterministic fixtures rather than live providers.
- Use the app HTTP envelope helpers in `src/web/src/services/http/` for API calls.

## Release Script Conventions

- Prefer root scripts:
  - `bash scripts/check.sh docs|web|server|all`
  - `bash scripts/test.sh web|server|all`
  - `COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e`
  - `bash scripts/deploy-validate.sh`
- Use repo-local caches for repeatability:
  - `COREPACK_HOME=.tmp/corepack`
  - `GOCACHE=.tmp/go-build`
  - `GOMODCACHE=.tmp/go-mod`
- `scripts/test.sh server` must explicitly print the `TEST_DATABASE_URL` skip when DB-backed HTTP integration tests are not run.
- `scripts/verify-quality-gates.sh` is intentionally fixed-string based; when docs or release commands change, update the script and run `bash scripts/check.sh docs`.

## Documentation Conventions

- `docs/API.md` is the canonical API index.
- `docs/architecture/current-system-contracts.md` is the behavior contract for current routes/env/test/release commands.
- `docs/release/rc-checklist.md` is the release-candidate evidence checklist.
- `docs/release/deployment-runtime-remediation.md` documents host-level remediation for Docker/kubectl blockers.
- `.planning/phases/*/*-SUMMARY.md` files are evidence only when they contain concrete verification commands and observed results.

## Deployment Conventions

- Committed config must use placeholders only. Do not commit provider keys, Stripe secrets, database passwords, kubeconfig material, or real session secrets.
- Kubernetes real secrets should be copied from `deploy/kubernetes/secret.example.yaml` to an untracked location and filled outside git.
- `Dockerfile.server` and `Dockerfile.web` must target `src/server` and `src/web`; do not build `lobehub/` or `new-api/`.
- `scripts/deploy-validate.sh` is the local Docker proof. It should fail early if Docker daemon access is missing.

## GSD Workflow Conventions

- Treat `.planning/STATE.md`, `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, and phase artifacts as routing inputs, but verify against code/tests when docs drift.
- When `gsd-sdk` is unavailable, route from local files and record exact blockers in planning artifacts.
- Backlog items `999.1` and `999.2` are accepted debt; do not silently fold them into unrelated work.

## Worktree Safety

- The checkout is dirty and has large pre-existing changes.
- Do not revert unrelated changes.
- When updating generated maps, only edit `.planning/codebase/*.md` unless the current command explicitly requires other files.
