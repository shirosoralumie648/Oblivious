# Repository Rescan - 2026-06-14

## Current Truth

- Branch: `main`; current pushed `HEAD` is `634b674 feat(agent): persist structured plan step dependencies`.
- Worktree status at scan time: clean against `origin/main`.
- The project is still **not complete** against the four 2026-06-04 fusion specs.
- The current completion matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate after this rescan: **73/100**. The repository owns most core product surfaces and has strong focused evidence, but the remaining progress is dominated by target-environment proof, broader DB-backed commercial reruns, security/tenant-isolation depth, production deployment validation, and final no-skip release readiness.

## What Changed Since The Previous Rescan

- Agent durable planning now persists structured plan-step metadata:
  - `description`
  - `dependsOn`
- Migration `0080_agent_plan_step_structure.sql` adds durable SQL columns for that metadata.
- Backend store/service/API, OpenAPI, DB evidence scripts, and the Workspace Agent plan-step UI now carry the fields end to end.
- Legacy ordering semantics are preserved:
  - no explicit dependencies means all lower-index steps must be completed or skipped;
  - explicit dependencies require only the listed dependency step indexes to be completed or skipped.
- The slice was independently verified, committed, and pushed as `634b674`.

## Repository Inventory

- Tracked file distribution:
  - `src`: 960 files
  - `.planning`: 210 files
  - `docs`: 90 files
  - `scripts`: 37 files
  - `deploy`: 42 files
- Server shape:
  - `src/server/internal`: 586 tracked files
  - `src/server/migrations`: 106 migration files
  - largest active server domains are `relay`, `http`, `mcp`, `admin`, `agent`, `workflow`, `knowledge`, `observability`, `channel`, `migration`, and `marketplace`
- Web shape:
  - `src/web/src/routes`: 80 tracked files
  - `src/web/src/features`: 51 tracked files
  - route families are mostly `workspace`, `admin`, `console`, `marketing`, and `marketplace`
- Test inventory:
  - Go test files: 225
  - Web component/API test files: 67
  - Web E2E specs: 6
- Latest checked-in migration: `src/server/migrations/0080_agent_plan_step_structure.sql`.

## Completion Matrix Snapshot

Proven rows:

- API gateway and relay
- Knowledge base and RAG
- Multi-channel publishing
- Database schema and migrations

Partial rows:

- Workflow engine
- Agent system
- Billing and monetization
- Marketplace ecosystem
- Frontend shell and core pages
- Observability metrics, logs, alerts, recovery
- API contract
- Deployment and operations
- Security and tenant isolation
- Migration strategy and release readiness

This scan does not reclassify any Partial row to Proven. The Agent row gained structured plan-step persistence and DB-backed evidence, but still needs broader dual-engine runtime, resume, memory consolidation, and planning UX proof before it can be called complete.

## Verification Run During This Rescan

```bash
git diff --check
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/agent ./internal/http -count=1
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web test src/routes/workspace/AgentPlanStepsPage.test.tsx -- --runInBand
bash scripts/verify-openapi-contract.sh
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit
COREPACK_HOME=/tmp/codex-corepack GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh agent-runtime-memory
```

Result:

- All commands passed.
- `agent-runtime-memory` used a disposable pgvector PostgreSQL container.
- The DB-backed profile reported no skipped tests.

## Notable Scan Findings

- `scripts/check.sh` still exposes the main gates: `all`, `docs`, `relay-security`, `security`, `web`, and `server`.
- `scripts/verify-commercial-db-evidence.sh` now has four no-skip DB evidence profiles:
  - `backend-journey`
  - `marketplace-money-movement`
  - `app-stateful-routes`
  - `agent-runtime-memory`
- Active source TODO/stub scan did not reveal a new broad implementation gap. Most matches are test stubs, generated gRPC `Unimplemented*` boilerplate, one intentional duplicate-builtin panic guard, and one service-template TODO.
- `docs/release/relay-route-table.md` still records production-disabled future OpenAI-compatible endpoints such as fine-tuning and Assistants/Threads/Runs. Those are documented future support boundaries rather than current fusion-spec completion blockers.
- `.tmp/rescan-stale-artifacts/` remains a quarantine area from the previous cleanup and should not be treated as release source.

## Recommended Next Slices

1. Agent browser journey: prove `/agents -> start planning run -> plan steps -> approve/execute/continue/tool decision` through the real app shell.
2. Frontend Chat app-router regression: cover `/chat/:conversationId` through the actual router and shell, not just page-level APIs.
3. Scheduled-task or tenant DB coverage: add a no-skip PostgreSQL profile for a currently Partial row that still depends on DB-backed proof.
4. Broader commercial verifier rerun: run the remaining `verify-commercial-db-evidence.sh` profiles and record any environment-specific residual risk.
5. Deployment validation: only after repo-owned rows are narrowed further, run deploy/Kubernetes/backup-restore proof on the target installation.

## Boundary

This report is a repository rescan and evidence checkpoint, not a final completion claim. Rows marked `Partial` in `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` remain open until the row-specific proof is recorded and rerun in the required environment.
