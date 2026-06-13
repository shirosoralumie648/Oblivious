# Repository Rescan - 2026-06-14

## Current Truth

- Branch: `main`; this report refreshes the June 14 scan after the Chat router checkpoint, Scheduled Task DB evidence slice, Tenant membership DB evidence slice, Agent planning Playwright browser proof, Chat-to-SOLO Playwright browser proof, Marketplace paid-install provider browser proof, Workflows mobile responsive browser proof, Agent gRPC runtime-gateway proof, and Agent gRPC authenticated service-adapter proof.
- Worktree status at scan time: clean against `origin/main`.
- The project is still **not complete** against the four 2026-06-04 fusion specs.
- The current completion matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate after this rescan: **80/100**. The repository owns most core product surfaces and has strong focused evidence. Recent Agent planning, Chat-to-SOLO, Marketplace paid-provider, Workflows mobile responsive browser proof, and Agent gRPC runtime/service-adapter proof, plus Tenant membership, Scheduled Task runtime, and all-profile DB evidence, narrows frontend, marketplace-provider wiring, Agent service-boundary, DB-backed tenant/security, DB-backed workflow, and release-readiness risk, but the remaining progress is still dominated by target-environment proof, broader security/tenant-isolation depth, production deployment validation, and final no-skip release readiness.

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
- Real Playwright browser coverage now proves the Agent planning journey from `/agents` into `/agent-runs/run_browser_agent/plan-steps`, including default planning-mode run settings, tool approval with operator reason, plan-step approval/execution, dependency evidence, and continue-plan completion. This was committed and pushed as `8079f8c`.
- Real Workspace app-router coverage now proves `/chat/:conversationId` beyond route parameter loading: conversation settings save, streamed message send, final message refresh, conversion to SOLO draft, SOLO task creation/start, and navigation to `/solo?taskId=task_router_new&returnTo=%2Fchat%2Fconversation_router`. This was committed and pushed as `5288222`.
- Real Playwright browser coverage now proves the Chat-to-SOLO journey from `/chat/conversation_browser_solo` into `/solo?taskId=task_browser_solo&returnTo=%2Fchat%2Fconversation_browser_solo`, including saved conversation settings carried into stream overrides, SOLO draft conversion, authorization scope, knowledge-base, allow-list, deny-list, task start, and Back-to-chat return behavior.
- Real Playwright browser coverage now proves the Marketplace paid-install provider journey from `/marketplace/agents/agent_paid_release_helper`, including Workspace Marketplace active navigation, configured Stripe/Alipay provider discovery, Alipay selection, selected version and provider propagation to the install route, checkout-session response handling, hosted checkout link rendering, and no direct installed-success message for paid checkout.
- Real Playwright browser coverage now proves the `/workflows` mobile responsive/accessibility boundary at `390x844`, including active Workspace navigation, exactly one `main` landmark, no document-level horizontal overflow, contained React Flow canvas scrolling, node-sequence evidence, and signed-webhook signature header evidence.
- `src/server/pkg/agent` now fails closed without a configured runtime gateway, forwards create-run / execute / approval fields into an injected runtime boundary, and returns runtime-derived run/tool-call status instead of synthesizing fixed success strings. The Agent gRPC proto also carries `user_id` for ExecuteReAct plus `user_id` and `reason` for tool approval decisions, and the package has a concrete adapter that maps those authenticated requests into the internal Agent service `StartRun`, `ListRuns`, `GetRunWithMessages`, `ApproveToolRun`, and `RejectToolRun` methods.
- `scripts/verify-commercial-db-evidence.sh scheduled-task-runtime` now provides no-skip PostgreSQL evidence for Scheduled Task SQL runtime persistence, route dispatch, and Workflow schedule-trigger sync.
- `scripts/verify-commercial-db-evidence.sh tenant-membership-lifecycle` now provides no-skip PostgreSQL evidence for Tenant SQL organization/member/invitation/ownership lifecycle plus HTTP member list, ownership transfer, remove-member, and session-revocation behavior.
- `scripts/verify-commercial-db-evidence.sh all` now runs the full DB-backed commercial evidence profile set, and `scripts/verify-commercial-completion.sh` delegates its DB step to that aggregate instead of only `backend-journey`.

## Repository Inventory

- Tracked file distribution:
  - `src`: 962 files
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
  - Web Playwright specs: 5 specs, plus 5 E2E fixture files
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

This scan does not reclassify any Partial row to Proven. The Agent, Workflow, and Frontend rows gained real app-router and Playwright browser evidence, plus gRPC package boundary and authenticated service-adapter proof, but still need broader browser/runtime/target-environment proof before any open row can be called complete.

## Verification Run During This Rescan

```bash
git status --short --branch
git log --oneline -n 6
git diff --check
COREPACK_HOME=/tmp/codex-corepack GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh scheduled-task-runtime
COREPACK_HOME=/tmp/codex-corepack GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh tenant-membership-lifecycle
COREPACK_HOME=/tmp/codex-corepack GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh all
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/agent-planning.spec.ts --project=chromium
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/chat-solo.spec.ts --project=chromium
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/admin-marketplace.spec.ts --project=chromium
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/workflows.spec.ts --project=chromium
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit
bash scripts/check.sh docs
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./pkg/agent -count=1 -v
```

Result:

- All commands passed during this report refresh.
- `scheduled-task-runtime` used a disposable pgvector PostgreSQL container and reported `skipped tests: none`.
- `tenant-membership-lifecycle` used a disposable pgvector PostgreSQL container and reported `skipped tests: none`.
- `all` used a disposable pgvector PostgreSQL container and reported `skipped tests: none`.
- The Agent planning Playwright browser journey passed against the managed Vite build/preview server with Chrome at `/usr/bin/google-chrome`.
- The Chat-to-SOLO Playwright browser journey passed against the managed Vite build/preview server with Chrome at `/usr/bin/google-chrome`.
- The Admin/Marketplace Playwright browser journey passed all four cases, including the new paid-install Alipay checkout path, against the managed Vite build/preview server with Chrome at `/usr/bin/google-chrome`.
- The Workflows Playwright browser journey passed both cases, including the new mobile responsive/accessibility proof, against the managed Vite build/preview server with Chrome at `/usr/bin/google-chrome`.
- `go test ./pkg/agent` passed after replacing the fixed gRPC stub with an injected runtime gateway boundary, fail-closed behavior when no runtime is configured, authenticated proto fields for execute/approval calls, and a concrete internal Agent service adapter for create/run/detail/tool-approval operations.
- `pnpm --dir src/web exec tsc --noEmit` passed after adding the new E2E fixture and spec.
- The previous Agent structured-plan slice remains covered by `agent-runtime-memory` evidence recorded in the matrix.
- The previous Agent and Chat router slices remain covered by their focused Vitest/TypeScript/diff checks recorded in the matrix.

## Notable Scan Findings

- `scripts/check.sh` still exposes the main gates: `all`, `docs`, `relay-security`, `security`, `web`, and `server`.
- `scripts/verify-commercial-db-evidence.sh` now has six no-skip DB evidence profiles:
  - `backend-journey`
  - `marketplace-money-movement`
  - `app-stateful-routes`
  - `tenant-membership-lifecycle`
  - `agent-runtime-memory`
  - `scheduled-task-runtime`
- Active source TODO/stub scan still does not reveal a new broad implementation gap. Most matches are test stubs, generated gRPC `Unimplemented*` boilerplate, placeholder-secret/runbook language, and explicit tests that assert placeholder output is not used.
- First-party active TODO boundaries are narrow and already documented as future or non-release proof:
  - `src/server/internal/relay/handler/realtime.go` has auth/prebill/settlement TODOs, while `docs/release/relay-route-table.md` marks Realtime `DisabledInProduction`.
  - `src/server/internal/relay/handler/policy.go` explicitly disables fine-tuning and Assistants/Threads/Runs as future commercial support.
  - `scripts/migrate-service-template.sh` contains a service-template TODO.
  - `src/server/internal/admin/channel_service.go` fails closed for unimplemented channel providers.
- `src/server/internal/relay/handler_new/` contains stale/alternate handler code with TODOs, but current runtime registration imports `src/server/internal/relay/handler/*` through `src/server/internal/relay/relay.go`; do not use `handler_new` as completion evidence.
- `.tmp/rescan-stale-artifacts/` remains a quarantine area from the previous cleanup and should not be treated as release source.

## Recommended Next Slices

1. Broader Browser/E2E route proof: continue extending high-value commercial workflows beyond the new Agent planning, Chat-to-SOLO, Marketplace paid-provider, and Workflows mobile responsive browser journeys.
2. Strict commercial verifier rerun on target infrastructure with deploy and backup/restore enabled.
3. Observability and recovery proof: strengthen the gap between static dashboard/policy checks and target-environment recovery behavior.
4. Broader tenant/security proof: continue beyond the new tenant membership profile into remaining data-isolation and provider-secret response paths.
5. Deployment validation: only after repo-owned rows are narrowed further, run deploy/Kubernetes/backup-restore proof on the target installation.

## Boundary

This report is a repository rescan and evidence checkpoint, not a final completion claim. Rows marked `Partial` in `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` remain open until the row-specific proof is recorded and rerun in the required environment.
