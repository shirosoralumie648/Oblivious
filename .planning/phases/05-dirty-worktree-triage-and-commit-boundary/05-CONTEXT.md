# Phase 5: Dirty Worktree Triage and Commit Boundary - Context

**Gathered:** 2026-05-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 5 does not implement the backend, frontend, deployment, or documentation changes already present in the worktree. It classifies those existing changes into coherent work slices, defines safe commit boundaries, and creates the inventory that Phase 6 through Phase 8 can consume.

The phase is complete when a maintainer can tell which files belong to each later work slice, which files are reference/historical material, and which paths must stay out of commits or be handled separately.

</domain>

<decisions>
## Implementation Decisions

### Commit Boundary Policy
- **D-01:** Keep planning artifacts separate from source, deployment, and documentation implementation commits. Phase 5 may create planning/inventory artifacts, but must not stage broad source changes with them.
- **D-02:** Use explicit path staging only. Do not use `git add .` or `git add -A`.
- **D-03:** Treat the current dirty worktree as user-owned input. Do not revert, delete, or "clean" source files just because they are untracked.
- **D-04:** Group current work into at least these slices: backend integration, frontend/E2E, deployment/CI, contract docs, and historical/reference docs.

### Source of Truth
- **D-05:** `.planning/PROJECT.md`, `.planning/REQUIREMENTS.md`, `.planning/ROADMAP.md`, and `.planning/STATE.md` are the active planning source of truth.
- **D-06:** Active product code is bounded to `src/server`, `src/web`, `config`, `scripts`, `.github/workflows`, `Dockerfile.server`, `Dockerfile.web`, `docker-compose.yml`, and root workspace metadata.
- **D-07:** `lobehub/` and `new-api/` are reference trees only and must stay out of Phase 5 classification unless a future phase explicitly targets them.
- **D-08:** Root `CURRENT_STATUS.md`, root `ROADMAP.md`, `ARCHAEOLOGY_REPORT.md`, and older `docs/superpowers/*` materials are historical/reference inputs. They must not override `.planning` state unless Phase 5 explicitly records a reason to incorporate them.

### Work Slice Expectations
- **D-09:** Backend integration slice should include route registration, route split files, Agent/Memory/MCP/Notification/Quota/WebSocket handlers and services, Relay store changes, auth/session/userprefs changes, and SQL migrations.
- **D-10:** Frontend/E2E slice should include Playwright config/specs/fixtures, frontend package updates, theme/type/config changes, and new workspace route pages.
- **D-11:** Deployment/CI slice should include GitHub Actions, Dockerfiles, compose, env templates, root package/lock changes, and deployment validation scripts.
- **D-12:** Contract docs slice should include `docs/API.md`, `docs/architecture/current-system-contracts.md`, release docs, README links, and any route/command assertions used by docs checks.
- **D-13:** Historical/reference docs should be inventoried separately before inclusion. They should not be mixed into functional implementation commits by default.

### Verification Boundary
- **D-14:** Phase 5 should prefer non-destructive checks: `git status --short`, `git diff --name-status`, `git diff --check`, targeted `rg` inventory checks, and explicit path lists.
- **D-15:** Full backend, frontend, E2E, and Docker runtime verification belongs to Phase 6 through Phase 8 after the inventory has been split into coherent work.
- **D-16:** The v03.2 restricted-network Docker command remains the deployment proof baseline that later phases must preserve.

### the agent's Discretion
- The exact inventory filename and table format may be chosen during planning, as long as downstream phases can consume it directly.
- The planner may decide whether Phase 5 produces one inventory document or a small set of inventory/checklist artifacts.
- The planner may include lightweight consistency scripts if they only read state and do not mutate source files.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Active planning
- `.planning/PROJECT.md` — Current v03.3 goal, active requirements, constraints, and decisions.
- `.planning/REQUIREMENTS.md` — CONS-01 and traceability for Phase 5.
- `.planning/ROADMAP.md` — Phase 5 boundary, success criteria, and downstream phase split.
- `.planning/STATE.md` — Current worktree context, previous v03.2 baseline, and deferred items.
- `.planning/MILESTONES.md` — v03.2 archive summary and known deferred items.

### Codebase maps
- `.planning/codebase/STACK.md` — Active workspace boundary, tools, verification commands, and reference-tree exclusions.
- `.planning/codebase/ARCHITECTURE.md` — Active backend/frontend/deployment surfaces and current route/service shape.
- `.planning/codebase/CONVENTIONS.md` — Naming, formatting, test, staging, and code-style conventions.

### Runtime and release baseline
- `docs/release/deployment-runtime-remediation.md` — Deployment limitations and restricted-network remediation context.
- `docs/release/rc-checklist.md` — Existing release evidence expectations.
- `docs/architecture/current-system-contracts.md` — Contract docs that must eventually match live routes.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `git diff --name-status` and `git ls-files --others --exclude-standard` already provide a stable base for the Phase 5 inventory.
- `.planning/codebase/*.md` maps provide bounded summaries, so Phase 5 does not need to inspect every source file to classify the worktree.
- Existing scripts `scripts/check.sh`, `scripts/test.sh`, and `scripts/deploy-validate.sh` define later verification gates and should be referenced rather than duplicated.

### Established Patterns
- Active backend uses a handler -> service -> store pattern under `src/server/internal/<domain>`.
- `src/server/internal/http/router.go` is the high-churn composition root, while route split files exist as a partial refactor.
- Active frontend code lives under `src/web`, with route components in `src/web/src/routes`, feature APIs under `src/web/src/features`, and tests beside route/feature code.
- Root workspace intentionally excludes `lobehub/` and `new-api/`; those trees are reference material, not active release code.

### Integration Points
- Backend slice connects through `src/server/internal/http/router.go`, `src/server/internal/http/routes_*.go`, domain services, and migrations `src/server/migrations/0013` through `0019`.
- Frontend/E2E slice connects through `src/web/package.json`, `src/web/playwright.config.ts`, `src/web/e2e/`, route pages, and API type definitions.
- Deployment/CI slice connects through `.github/workflows/ci.yml`, Dockerfiles, `docker-compose.yml`, `config/.env.example`, root `package.json`, `pnpm-lock.yaml`, and `scripts/deploy-validate.sh`.
- Contract docs slice connects through `docs/API.md`, `docs/architecture/current-system-contracts.md`, `docs/release/*`, and `README.md`.

</code_context>

<specifics>
## Specific Ideas

- The inventory should explicitly mark root `CURRENT_STATUS.md` and root `ROADMAP.md` as historical/reference unless deliberately promoted.
- The inventory should preserve the current untracked source additions as first-class work, not cleanup noise.
- Phase 5 should make later commits easier by producing file groups and recommended commit boundaries before any implementation hardening starts.

</specifics>

<deferred>
## Deferred Ideas

- Backend route/service hardening is Phase 6.
- Frontend, E2E, CI, and deployment runtime verification is Phase 7.
- API, architecture, release docs, and final verification evidence is Phase 8.
- Phase 01 summary reconstruction remains backlog 999.1.
- Legacy workspace Marketplace route cleanup remains backlog 999.2 unless Phase 5 inventory reveals it belongs to an active slice.

</deferred>

---

*Phase: 05-dirty-worktree-triage-and-commit-boundary*
*Context gathered: 2026-05-14*
