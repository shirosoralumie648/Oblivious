# Repository Rescan - 2026-06-14

## Current Truth

- Branch: `main`; current pushed `HEAD` is `5288222 test(frontend): cover chat router journey`.
- Worktree status at scan time: clean against `origin/main`.
- The project is still **not complete** against the four 2026-06-04 fusion specs.
- The current completion matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate after this rescan: **74/100**. The repository owns most core product surfaces and has strong focused evidence. Recent Agent and Chat app-router proof narrows frontend risk, but the remaining progress is still dominated by target-environment proof, broader DB-backed commercial reruns, security/tenant-isolation depth, production deployment validation, and final no-skip release readiness.

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
- Real Workspace app-router coverage now proves the Agent planning route journey from `/agents` into `/agent-runs/:runId/plan-steps`, including tool approval, plan-step approval/execution, and continue-plan refresh behavior. This was committed and pushed as `c82abda`.
- Real Workspace app-router coverage now proves `/chat/:conversationId` beyond route parameter loading: conversation settings save, streamed message send, final message refresh, conversion to SOLO draft, SOLO task creation/start, and navigation to `/solo?taskId=task_router_new&returnTo=%2Fchat%2Fconversation_router`. This was committed and pushed as `5288222`.

## Repository Inventory

- Tracked file distribution:
  - `src`: 960 files
  - `.planning`: 210 files
  - `docs`: 91 files
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
  - Web Playwright specs: 3 specs, plus 3 E2E fixture files
- Latest checked-in migration: `src/server/migrations/0080_agent_plan_step_structure.sql`.
- Project-local `AGENTS.md`: none at the main repo root or under first-party source; discovered `AGENTS.md` files are in dependency caches or nested `reference/*` repositories.

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

This scan does not reclassify any Partial row to Proven. The Agent and Frontend rows gained real app-router evidence, but still need broader browser/runtime/target-environment proof before either row can be called complete.

## Verification Run During This Rescan

```bash
git status --short --branch
git log --oneline -n 6
git diff --check
bash scripts/check.sh docs
```

Result:

- All commands passed during this report refresh.
- The previous Agent structured-plan slice remains covered by `agent-runtime-memory` evidence recorded in the matrix.
- The previous Agent and Chat router slices remain covered by their focused Vitest/TypeScript/diff checks recorded in the matrix.

## Notable Scan Findings

- `scripts/check.sh` still exposes the main gates: `all`, `docs`, `relay-security`, `security`, `web`, and `server`.
- `scripts/verify-commercial-db-evidence.sh` now has four no-skip DB evidence profiles:
  - `backend-journey`
  - `marketplace-money-movement`
  - `app-stateful-routes`
  - `agent-runtime-memory`
- Active source TODO/stub scan still does not reveal a new broad implementation gap. Most matches are test stubs, generated gRPC `Unimplemented*` boilerplate, placeholder-secret/runbook language, and explicit tests that assert placeholder output is not used.
- First-party active TODO boundaries are narrow and already documented as future or non-release proof:
  - `src/server/internal/relay/handler/realtime.go` has auth/prebill/settlement TODOs, while `docs/release/relay-route-table.md` marks Realtime `DisabledInProduction`.
  - `src/server/internal/relay/handler/policy.go` explicitly disables fine-tuning and Assistants/Threads/Runs as future commercial support.
  - `scripts/migrate-service-template.sh` contains a service-template TODO.
  - `src/server/internal/admin/channel_service.go` fails closed for unimplemented channel providers.
- `src/server/internal/relay/handler_new/` contains stale/alternate handler code with TODOs, but current runtime registration imports `src/server/internal/relay/handler/*` through `src/server/internal/relay/relay.go`; do not use `handler_new` as completion evidence.
- `.tmp/rescan-stale-artifacts/` remains a quarantine area from the previous cleanup and should not be treated as release source.

## Recommended Next Slices

1. Scheduled-task or tenant DB coverage: add a no-skip PostgreSQL profile for a currently Partial row that still depends on DB-backed proof.
2. Broader commercial verifier rerun: run the remaining `verify-commercial-db-evidence.sh` profiles and record any environment-specific residual risk.
3. Browser/E2E route proof: extend from focused app-router tests into Playwright coverage for the most important commercial workflows.
4. Observability and recovery proof: strengthen the gap between static dashboard/policy checks and target-environment recovery behavior.
5. Deployment validation: only after repo-owned rows are narrowed further, run deploy/Kubernetes/backup-restore proof on the target installation.

## Boundary

This report is a repository rescan and evidence checkpoint, not a final completion claim. Rows marked `Partial` in `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` remain open until the row-specific proof is recorded and rerun in the required environment.
