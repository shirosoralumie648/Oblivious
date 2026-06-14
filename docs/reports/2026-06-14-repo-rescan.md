# Repository Rescan - 2026-06-14

## Current Truth

- Branch: `main`; this report refreshes the June 14 scan after the Chat router checkpoint, Scheduled Task DB evidence slice, Tenant membership DB evidence slice, Tenant cross-surface DB evidence slice, Admin Observability provider secret-response DB evidence slice, Publishing channel secret-response DB evidence slice, Agent planning Playwright browser proof, Chat-to-SOLO Playwright browser proof, Marketplace paid-install provider browser proof, Workflows mobile responsive browser proof, Agent gRPC runtime-gateway proof, Agent gRPC authenticated service-adapter proof, HTTP panic recovery proof, and Console API token usage sanitization proof.
- Worktree status at report close: clean against `origin/main` after the independently verified slice is committed and pushed.
- The project is still **not complete** against the four 2026-06-04 fusion specs.
- The current completion matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate after this rescan: **81/100**. The repository owns most core product surfaces and has strong focused evidence. Recent Agent planning, Chat-to-SOLO, Marketplace paid-provider, Workflows mobile responsive browser proof, Agent gRPC runtime/service-adapter proof, HTTP panic recovery proof, Console API token usage sanitization proof, Tenant membership, Tenant cross-surface isolation, Admin Observability provider secret-response safety, Publishing channel secret-response safety, Scheduled Task runtime, and all-profile DB evidence narrows frontend, marketplace-provider wiring, Agent service-boundary, repository-owned recovery behavior, Console user-visible security posture, DB-backed tenant/security, provider-secret response safety, DB-backed workflow, and release-readiness risk, but the remaining progress is still dominated by target-environment proof, broader security/tenant-isolation depth, production deployment validation, and final no-skip release readiness.

## What Changed Since The Previous Rescan

- Agent durable planning now persists structured plan-step metadata:
  - `description`
  - `dependsOn`
- Migration `0080_agent_plan_step_structure.sql` adds durable SQL columns for that metadata.
- Backend store/service/API, OpenAPI, DB evidence scripts, and the Workspace Agent plan-step UI now carry the fields end to end.
- Legacy ordering semantics are preserved:
  - no explicit dependencies means all lower-index steps must be completed or skipped;
  - explicit dependencies require only the listed dependency step indexes to be completed or skipped.
- Real Workspace app-router coverage and real Playwright browser coverage prove the Agent planning route journey from `/agents` into `/agent-runs/:runId/plan-steps`, including tool approval, plan-step approval/execution, structured dependency evidence, and continue-plan completion.
- Real Workspace app-router and Playwright browser coverage now prove Chat-to-SOLO continuity from `/chat/:conversationId` into `/solo`, including saved conversation settings carried into stream overrides, SOLO draft conversion, task start, and Back-to-chat return behavior.
- Real Playwright browser coverage proves the Marketplace paid-install provider journey, including provider discovery, Alipay selection, provider/version propagation, hosted checkout link rendering, and no direct installed-success message for paid checkout.
- Real Playwright browser coverage proves the `/workflows` mobile responsive/accessibility boundary at `390x844`, including active Workspace navigation, exactly one `main` landmark, no document-level horizontal overflow, contained React Flow canvas scrolling, node-sequence evidence, and signed-webhook signature header evidence.
- `src/server/pkg/agent` now fails closed without a configured runtime gateway, forwards create-run / execute / approval fields into an injected runtime boundary, and has a concrete adapter into the internal Agent service for create/run/detail/tool-approval operations.
- HTTP recovered panics now create critical alert state plus `record-http-panic` restart recovery actions, and recovery policies can match panic/OOM signal fields before generic critical HTTP policies.
- `scripts/verify-commercial-db-evidence.sh scheduled-task-runtime` now provides no-skip PostgreSQL evidence for Scheduled Task SQL runtime persistence, route dispatch, and Workflow schedule-trigger sync.
- `scripts/verify-commercial-db-evidence.sh tenant-membership-lifecycle` now provides no-skip PostgreSQL evidence for Tenant SQL organization/member/invitation/ownership lifecycle plus HTTP member list, ownership transfer, remove-member, and session-revocation behavior.
- `scripts/verify-commercial-db-evidence.sh tenant-cross-surface` now provides no-skip PostgreSQL evidence for active-organization isolation across Chat, Knowledge, Console, Agent, Memory, MCP, Quota, Marketplace publisher, Marketplace settlement preferences, Agent run detail, and Agent tool-run approve/reject/retry HTTP surfaces.
- The HTTP test database bootstrap now includes `concurrency_limits` and `token_rate_limits`, so quota active-organization isolation can run inside the same disposable PostgreSQL cross-surface profile.
- Agent tool-run approval/reject/retry tenant-scope coverage now prepares retry evidence from a separate failed workflow fixture, avoiding a false failure caused by approving a pending tool run and completing the original run before the retry assertion.
- `scripts/verify-commercial-db-evidence.sh secret-response-safety` now provides no-skip PostgreSQL evidence for SQL-backed Admin Observability alert-provider secret redaction. The real admin HTTP router writes raw SMTP `password`, Slack `webhook_url`, PagerDuty `routing_key`, Opsgenie `api_key`, and `private_key` values into PostgreSQL, while create/list/update responses return only redacted markers and omit encrypted-secret column names.
- Redacted-marker updates for Admin Observability provider configs now have DB-backed proof that the stored raw secret is preserved while non-secret fields update normally.
- The same `secret-response-safety` profile now proves SQL-backed Publishing channel response redaction. The real `/api/v1/channels` routes write raw `secret`, `webhookSecret`, `api_key`, and `password` config values into PostgreSQL while create/list/detail/update responses return only redacted markers; marker updates preserve the stored raw secret while non-secret config fields update normally.
- `scripts/verify-commercial-db-evidence.sh all` now runs the full DB-backed commercial evidence profile set, and `scripts/verify-commercial-completion.sh` delegates its DB step to that aggregate instead of only `backend-journey`.
- Console API token usage and Console recent usage now use a Console-only `ConsoleAPITokenUsageItem` shape. User-facing Console responses preserve token/request/model/status/accounting evidence but omit internal provider/channel routing fields.
- The OpenAPI contract gate now requires Console usage responses to reference `ConsoleAPITokenUsageItem` and fails if provider/channel fields reappear.
- The Console Access and Usage pages no longer render provider/channel labels for ordinary user usage history.
- `scripts/verify-commercial-db-evidence.sh app-stateful-routes` now includes `TestConsoleUsageListsCurrentUserRecentRelayRequests`, so PostgreSQL no-skip app-stateful evidence covers the sanitized recent-usage response as well as token create/list/revoke.

## Repository Inventory

- Tracked file distribution:
  - `src`: 964 files
  - `.planning`: 210 files
  - `docs`: 91 files
  - `scripts`: 37 files
  - `deploy`: 42 files
- Server shape:
  - `src/server/internal`: 586 tracked files
  - `src/server/migrations`: 105 tracked SQL migration files: 92 top-level versioned migrations plus 13 SQL files in `clickhouse/` and `microservices/`
  - largest active server domains are `relay`, `http`, `mcp`, `admin`, `agent`, `workflow`, `knowledge`, `observability`, `channel`, `migration`, and `marketplace`
- Web shape:
  - `src/web/src/routes`: 80 tracked files
  - `src/web/src/features`: 51 tracked files
  - route families are mostly `workspace`, `admin`, `console`, `marketing`, and `marketplace`
- Test inventory:
  - Go test files: 225
  - Web component/API test files: 67
  - Web Playwright specs: 5 specs, plus 5 E2E fixture files
- Latest checked-in top-level migration: `src/server/migrations/0080_agent_plan_step_structure.sql`.
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

This scan does not reclassify any Partial row to Proven. The Agent, Workflow, Frontend, Observability, API contract, Billing, and Security rows gained real app-router, Playwright browser, service-adapter, repository-owned recovery, DB-backed stateful-route, cross-surface tenant HTTP isolation, SQL-backed Admin Observability and Publishing channel response redaction, and Console usage sanitization evidence, but still need broader browser/runtime/target-environment proof before any open row can be called complete.

## Verification Run During This Rescan

```bash
git status --short --branch
git log --oneline -n 5
find . \( -path '*/node_modules/*' -o -path './.tmp/*' -o -path './reference/*' \) -prune -o -name AGENTS.md -print
bash scripts/verify-commercial-db-evidence.sh secret-response-safety
bash scripts/verify-commercial-db-evidence.sh all
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
git diff --check
```

Result:

- `scripts/verify-commercial-db-evidence.sh secret-response-safety` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, covering Admin Observability provider and Publishing channel SQL-backed response redaction.
- `scripts/verify-commercial-db-evidence.sh all` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, including the expanded secret-response-safety profile.
- `bash scripts/check.sh docs` passed.
- `git diff --check` passed.

## Notable Scan Findings

- `scripts/check.sh` still exposes the main gates: `all`, `docs`, `relay-security`, `security`, `web`, and `server`.
- `scripts/verify-commercial-db-evidence.sh` has eight no-skip DB evidence profiles:
  - `backend-journey`
  - `marketplace-money-movement`
  - `app-stateful-routes`
  - `tenant-membership-lifecycle`
  - `tenant-cross-surface`
  - `secret-response-safety`
  - `agent-runtime-memory`
  - `scheduled-task-runtime`
- `app-stateful-routes` now covers Console API token create/list/revoke and sanitized Console recent usage in the same DB-backed profile.
- `tenant-cross-surface` now covers active-organization isolation across Chat, Knowledge, Console, Agent, Memory, MCP, Quota, Marketplace publisher, Marketplace settlement preferences, Agent run detail, and Agent tool-run decision/retry routes.
- Admin Observability provider and Publishing channel secret-response safety now have SQL-backed HTTP proof; the next sharper Security slice is equivalent DB-backed response redaction for Admin Relay channel API keys.
- Active source TODO/stub scan still does not reveal a new broad implementation gap. Most matches are test stubs, generated gRPC `Unimplemented*` boilerplate, placeholder-secret/runbook language, UI input placeholders, benign nil-return no-row paths, and explicit tests that assert placeholder output is not used.
- First-party active TODO boundaries are narrow and already documented as future or non-release proof:
  - `src/server/internal/relay/handler/realtime.go` has auth/prebill/settlement TODOs, while `docs/release/relay-route-table.md` marks Realtime `DisabledInProduction`.
  - `src/server/internal/relay/handler/policy.go` explicitly disables fine-tuning and Assistants/Threads/Runs as future commercial support.
  - `scripts/migrate-service-template.sh` contains a service-template TODO.
  - `src/server/internal/admin/channel_service.go` fails closed for unimplemented channel providers.
- `src/server/internal/relay/handler_new/` contains stale/alternate handler code with TODOs, but current runtime registration imports `src/server/internal/relay/handler/*` through `src/server/internal/relay/relay.go`; do not use `handler_new` as completion evidence.
- `.tmp/rescan-stale-artifacts/` remains a quarantine area from the previous cleanup and should not be treated as release source.

## Recommended Next Slices

1. Broader Browser/E2E route proof: continue extending high-value commercial workflows beyond the current Agent planning, Chat-to-SOLO, Marketplace paid-provider, and Workflows mobile responsive browser journeys.
2. Strict commercial verifier rerun on target infrastructure with deploy and backup/restore enabled.
3. Observability and recovery proof: continue from the panic recovery proof into target-environment OOM/crash restart execution, scale-down, and failover evidence.
4. DB-backed secret response safety: continue with Admin Relay channel API keys, proving real SQL-backed routes persist raw secrets but never return raw secret fields or encrypted-secret columns in HTTP responses; then continue into auth-security persistence, Relay file-mapping tenant ownership, and deeper SQL-store isolation for Workflow/Channel/Quota.
5. Deployment validation: only after repo-owned rows are narrowed further, run deploy/Kubernetes/backup-restore proof on the target installation.

## Boundary

This report is a repository rescan and evidence checkpoint, not a final completion claim. Rows marked `Partial` in `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` remain open until the row-specific proof is recorded and rerun in the required environment.
