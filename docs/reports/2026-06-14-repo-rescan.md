# Repository Rescan - 2026-06-14

## Current Truth

- Branch: `main`; this report refreshes the June 14 scan after the Chat router checkpoint, Scheduled Task DB evidence slice, Workflow SQL active-organization isolation DB evidence slice, Publishing channel active-organization isolation DB evidence slice, Admin Relay channel active-organization isolation DB evidence slice, Admin Relay read-surface active-organization isolation DB evidence slice, Quota SQL tenant isolation DB evidence slice, Tenant membership DB evidence slice, Tenant cross-surface DB evidence slice, Auth security persistence plus reset-token replay/expiry/non-enumeration DB evidence slice, Relay file-mapping tenant ownership DB evidence slice, Relay runtime channel active-organization isolation DB evidence slice, Admin Observability provider secret-response DB evidence slice, Publishing channel secret-response DB evidence slice, Admin Relay channel secret-response DB evidence slice, Agent planning Playwright browser proof, Chat-to-SOLO Playwright browser proof, Marketplace paid-install provider browser proof, Workflows mobile responsive browser proof, Agent gRPC runtime-gateway proof, Agent gRPC authenticated service-adapter proof, HTTP panic recovery proof, Console API token usage sanitization proof, and the post-Admin-Relay-read-isolation rescan.
- Worktree status at the start of this Admin Relay read-surface closeout: dirty with the current slice changes relative to `origin/main`; latest committed checkpoint before these changes was `1009190 docs: refresh repository rescan checkpoint`.
- The project is still **not complete** against the four 2026-06-04 fusion specs.
- The current completion matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate after this rescan: **84/100**. The repository owns most core product surfaces and has strong focused evidence. Recent Agent planning, Chat-to-SOLO, Marketplace paid-provider, Workflows mobile responsive browser proof, Agent gRPC runtime/service-adapter proof, HTTP panic recovery proof, Console API token usage sanitization proof, Tenant membership, Tenant cross-surface isolation, Auth security persistence and reset-token replay/expiry/non-enumeration depth, Relay file-mapping tenant ownership, Relay runtime channel active-organization isolation, Workflow SQL active-organization isolation, Publishing channel active-organization isolation, Admin Relay channel active-organization isolation, Admin Relay read-surface active-organization isolation, Quota SQL tenant isolation, Admin Observability provider secret-response safety, Publishing channel secret-response safety, Admin Relay channel secret-response safety, Scheduled Task runtime, and all-profile DB evidence narrows frontend, marketplace-provider wiring, Agent service-boundary, repository-owned recovery behavior, Console user-visible security posture, DB-backed tenant/security/quota and publishing-channel/Admin Relay route/read isolation, runtime Relay routing isolation, provider-secret response safety, DB-backed workflow, and release-readiness risk. The remaining progress is still dominated by target-environment proof, broader security/tenant-isolation depth, production deployment validation, and final no-skip release readiness.

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
- `scripts/verify-commercial-db-evidence.sh workflow-sql-isolation` now provides no-skip PostgreSQL evidence for Workflow SQL store tenant isolation and real HTTP router active-organization isolation. Cross-organization Workflow list responses omit the other organization's workflow, detail/update/execute/list-executions requests fail closed with 404, execution detail and debug snapshot reads fail closed with 404, denied updates preserve the stored workflow name, and denied execution creates no extra execution row.
- `scripts/verify-commercial-db-evidence.sh publishing-channel-isolation` now provides no-skip PostgreSQL evidence for real Publishing channel HTTP active-organization isolation. It proves active-organization channel lists omit other-organization channels; get/update/delete/status/test/send/message-log/failed-message/retry routes fail closed for another organization's channel; fallback retry cannot select another organization's fallback channel; and denied mutations preserve the other organization's channel/message rows.
- `scripts/verify-commercial-db-evidence.sh admin-relay-channel-isolation` now provides no-skip PostgreSQL evidence for real Admin Relay channel HTTP active-organization isolation. It proves active-organization channel lists omit other-organization channels; cross-organization read/update/delete/test/health/model-sync/model-detect/model-apply/balance-refresh requests fail closed; mixed-organization batch updates are all-or-nothing rejected; and denied mutations preserve active and other-organization channel rows.
- `scripts/verify-commercial-db-evidence.sh admin-relay-read-isolation` now provides no-skip PostgreSQL evidence for real Admin Relay read-surface active-organization isolation. It proves `/api/v1/admin/channels/stats` returns runtime stats only for active-organization channels and `/api/v1/admin/models` returns model inventory and usage aggregates only for the active organization; the model inventory route is now mounted, documented in OpenAPI, and present in the route-surface manifest.
- `scripts/verify-commercial-db-evidence.sh quota-sql-isolation` now provides no-skip PostgreSQL evidence for Quota SQL store tenant isolation plus real quota HTTP active-organization isolation. It proves billing-session idempotency lookup is organization-scoped, wrong-organization settle/refund fail closed without changing quota balances or billing-session state, top-up checkout-session/failure/status mutations require the matching organization, and the real quota route uses the active organization.
- `scripts/verify-commercial-db-evidence.sh tenant-membership-lifecycle` now provides no-skip PostgreSQL evidence for Tenant SQL organization/member/invitation/ownership lifecycle plus HTTP member list, ownership transfer, remove-member, and session-revocation behavior.
- `scripts/verify-commercial-db-evidence.sh tenant-cross-surface` now provides no-skip PostgreSQL evidence for active-organization isolation across Chat, Knowledge, Console, Agent, Memory, MCP, Quota, Marketplace publisher, Marketplace settlement preferences, Agent run detail, and Agent tool-run approve/reject/retry HTTP surfaces.
- `scripts/verify-commercial-db-evidence.sh auth-security-persistence` now provides no-skip PostgreSQL evidence for Auth password policy, password-reset token confirmation, reset-token replay rejection, expired-token rejection without password/session mutation, unknown-email no-token behavior, non-test HTTP reset-request responses that do not enumerate emails or expose raw tokens, session revocation, persisted rate-limit blocking, password hash storage, login hash verification, stable auth response contracts, unauthenticated `/me` rejection, and sensitive organization action rate limiting.
- `scripts/verify-commercial-db-evidence.sh relay-file-mapping-tenant-ownership` now provides no-skip PostgreSQL evidence for the real Relay files upload route writing SQL file ownership mappings, mapped GET passthrough using the provider file ID, and wrong-user/wrong-organization lookup failing closed before any upstream call.
- `scripts/verify-commercial-db-evidence.sh relay-runtime-channel-isolation` now provides no-skip PostgreSQL evidence for runtime Relay active-organization channel selection. It proves trusted organization requests select only same-organization channels across model-route candidates, default/fallback selection, explicit conversation affinity, retry selection, `/v1/models` discovery, and SQL pool loading; cross-organization configured route candidates fail closed rather than falling back to global channels.
- The HTTP test database bootstrap now includes `concurrency_limits` and `token_rate_limits`, so quota active-organization isolation can run inside the same disposable PostgreSQL cross-surface profile.
- Agent tool-run approval/reject/retry tenant-scope coverage now prepares retry evidence from a separate failed workflow fixture, avoiding a false failure caused by approving a pending tool run and completing the original run before the retry assertion.
- `scripts/verify-commercial-db-evidence.sh secret-response-safety` now provides no-skip PostgreSQL evidence for SQL-backed Admin Observability alert-provider secret redaction. The real admin HTTP router writes raw SMTP `password`, Slack `webhook_url`, PagerDuty `routing_key`, Opsgenie `api_key`, and `private_key` values into PostgreSQL, while create/list/update responses return only redacted markers and omit encrypted-secret column names.
- Redacted-marker updates for Admin Observability provider configs now have DB-backed proof that the stored raw secret is preserved while non-secret fields update normally.
- The same `secret-response-safety` profile now proves SQL-backed Publishing channel response redaction. The real `/api/v1/channels` routes write raw `secret`, `webhookSecret`, `api_key`, and `password` config values into PostgreSQL while create/list/detail/update responses return only redacted markers; marker updates preserve the stored raw secret while non-secret config fields update normally.
- The same `secret-response-safety` profile now proves SQL-backed Admin Relay channel response redaction. The real `/api/v1/admin/channels` routes write raw create/rotated API keys into PostgreSQL while create/list/detail/update responses omit raw keys, `apiKey`, and `api_key_encrypted`; audit-log responses retain only the redacted `apiKey` marker.
- `scripts/verify-commercial-db-evidence.sh all` now runs the full DB-backed commercial evidence profile set, including Relay runtime channel active-organization isolation, Admin Relay channel active-organization isolation, and Admin Relay read-surface active-organization isolation, and `scripts/verify-commercial-completion.sh` delegates its DB step to that aggregate instead of only `backend-journey`.
- Console API token usage and Console recent usage now use a Console-only `ConsoleAPITokenUsageItem` shape. User-facing Console responses preserve token/request/model/status/accounting evidence but omit internal provider/channel routing fields.
- The OpenAPI contract gate now requires Console usage responses to reference `ConsoleAPITokenUsageItem` and fails if provider/channel fields reappear.
- The Console Access and Usage pages no longer render provider/channel labels for ordinary user usage history.
- `scripts/verify-commercial-db-evidence.sh app-stateful-routes` now includes `TestConsoleUsageListsCurrentUserRecentRelayRequests`, so PostgreSQL no-skip app-stateful evidence covers the sanitized recent-usage response as well as token create/list/revoke.

## Repository Inventory

- Tracked file distribution:
  - `src`: 966 files
  - `.planning`: 210 files
  - `docs`: 91 files
  - `scripts`: 37 files
  - `deploy`: 42 files
- Server shape:
  - `src/server/internal`: 587 tracked files
  - `src/server/migrations`: 106 SQL migration files: 93 top-level versioned migrations plus 13 SQL files in `clickhouse/` and `microservices/`
  - largest active server domains are `relay`, `http`, `mcp`, `admin`, `agent`, `workflow`, `knowledge`, `observability`, `channel`, `migration`, and `marketplace`
- Web shape:
  - `src/web/src/routes`: 80 tracked files
  - `src/web/src/features`: 51 tracked files
  - route families are mostly `workspace`, `admin`, `console`, `marketing`, and `marketplace`
- Test inventory:
  - Go test files: 226
  - Web component/API test files: 67
  - Web Playwright specs: 5 specs, plus 5 E2E fixture files
- Latest checked-in top-level migration: `src/server/migrations/0081_admin_relay_channel_organization_scope.sql`.
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

This scan does not reclassify any Partial row to Proven. The Agent, Workflow, Frontend, Observability, API contract, Billing, and Security rows gained real app-router, Playwright browser, service-adapter, repository-owned recovery, DB-backed stateful-route, cross-surface tenant HTTP isolation, SQL-backed Relay runtime channel active-organization isolation, SQL-backed Publishing channel active-organization isolation, SQL-backed Admin Relay channel and read-surface active-organization isolation, SQL-backed Admin Observability, Publishing channel, and Admin Relay channel response redaction, and Console usage sanitization evidence, but still need broader browser/runtime/target-environment proof before any open row can be called complete.

## Verification Run During This Rescan

```bash
git status --short --branch
git log --oneline -n 5
find . \( -path '*/node_modules/*' -o -path './.tmp/*' -o -path './reference/*' \) -prune -o -name AGENTS.md -print
bash scripts/verify-commercial-db-evidence.sh secret-response-safety
bash scripts/verify-commercial-db-evidence.sh auth-security-persistence
bash scripts/verify-commercial-db-evidence.sh relay-file-mapping-tenant-ownership
bash scripts/verify-commercial-db-evidence.sh relay-runtime-channel-isolation
bash scripts/verify-commercial-db-evidence.sh workflow-sql-isolation
bash scripts/verify-commercial-db-evidence.sh publishing-channel-isolation
bash scripts/verify-commercial-db-evidence.sh admin-relay-channel-isolation
bash scripts/verify-commercial-db-evidence.sh admin-relay-read-isolation
bash scripts/verify-commercial-db-evidence.sh quota-sql-isolation
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/relay/... -count=1
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/admin ./internal/http -run 'Test(ListChannelRuntimeStats|ServiceListModelInventory|ModelInventory|AdminHandlerListsChannelRuntimeStats|AdminHandlerListsModelInventory|AdminRelayReadSurfaces|RouteSurfaceAdminSubRoutesRequireAdminWithoutDatabase|RouteSurfaceAdminSubRoutesRejectNonAdminWithoutDatabase)' -count=1 -v
bash scripts/verify-openapi-contract.sh
bash scripts/verify-commercial-db-evidence.sh all
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
git diff --check
```

Result:

- `scripts/verify-commercial-db-evidence.sh secret-response-safety` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, covering Admin Observability provider, Publishing channel, and Admin Relay channel SQL-backed response redaction.
- `scripts/verify-commercial-db-evidence.sh auth-security-persistence` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, covering Auth SQL password reset/session revocation/rate-limit persistence, reset-token replay/expiry/unknown-email fail-closed behavior, non-test reset-request non-enumeration, plus HTTP auth route persistence and security behavior.
- `scripts/verify-commercial-db-evidence.sh relay-file-mapping-tenant-ownership` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, covering real Relay files upload/passthrough behavior with SQL-backed tenant file ownership.
- `scripts/verify-commercial-db-evidence.sh relay-runtime-channel-isolation` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, covering organization-scoped runtime Relay model-route/default/fallback selection, explicit channel affinity, retry selection, model discovery, and SQL pool loading.
- `scripts/verify-commercial-db-evidence.sh workflow-sql-isolation` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, covering Workflow SQL store organization filtering plus real-router active-organization denial for cross-organization workflow read/update/execute/list-executions/execution-detail/debug-snapshot paths.
- `scripts/verify-commercial-db-evidence.sh publishing-channel-isolation` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, covering real-router active-organization denial for cross-organization Publishing channel read/update/delete/status/test/send/message-log/failed-message/retry paths plus fallback-channel isolation.
- `scripts/verify-commercial-db-evidence.sh admin-relay-channel-isolation` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, covering real-router active-organization denial for cross-organization Admin Relay channel read/update/delete/test/health/model-sync/model-detect/model-apply/balance-refresh paths plus mixed-organization batch rejection.
- `scripts/verify-commercial-db-evidence.sh admin-relay-read-isolation` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, covering real-router active-organization isolation for Admin Relay runtime stats and model inventory reads.
- `scripts/verify-commercial-db-evidence.sh quota-sql-isolation` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, covering Quota SQL billing-session and top-up mutation tenant isolation plus real-router quota active-organization isolation.
- `go test ./internal/relay/... -count=1` passed, covering the full Relay package after the runtime channel isolation changes.
- The focused Admin Relay read-surface Go test command passed; its DB-backed HTTP test skips under plain `go test` without `TEST_DATABASE_URL`, while service/store/handler/route-surface tests run locally.
- `bash scripts/verify-openapi-contract.sh` passed after adding `/api/v1/admin/models` and its schema-fidelity checks.
- `scripts/verify-commercial-db-evidence.sh all` passed with disposable pgvector PostgreSQL and reported `skipped tests: none`, including the expanded secret-response-safety, auth-security-persistence, relay-file-mapping-tenant-ownership, relay-runtime-channel-isolation, workflow-sql-isolation, publishing-channel-isolation, admin-relay-channel-isolation, admin-relay-read-isolation, and quota-sql-isolation profiles.
- `bash scripts/check.sh docs` passed.
- `git diff --check` passed.

Continuation recheck:

```bash
git status --short --branch
git log --oneline -n 12
git ls-files | awk '...inventory counters...'
rg -n "TODO|FIXME|XXX|stub|placeholder|Unimplemented|DisabledInProduction" src scripts deploy docs/release docs/reports
rg -n "OrganizationID|ErrNotFound" src/server/internal/http/workflow_handler.go src/server/internal/workflow
```

Result:

- The live worktree remained clean on `main...origin/main`.
- Inventory counters now show `src=966`, `src/server/internal=587`, and Web component/API test files `67`; the top-level matrix count did not change.
- Workflow HTTP adapter calls still thread `session.OrganizationID` into list/get/update/delete/execute/execution-control paths, while `workflow.ErrNotFound` maps to HTTP `404`.
- Workflow store tests and `workflow-sql-isolation` now prove cross-organization workflow and execution reads fail closed through both SQL store and real HTTP router paths.

## Notable Scan Findings

- `scripts/check.sh` still exposes the main gates: `all`, `docs`, `relay-security`, `security`, `web`, and `server`.
- `scripts/verify-commercial-db-evidence.sh` has sixteen no-skip DB evidence profiles:
  - `backend-journey`
  - `marketplace-money-movement`
  - `app-stateful-routes`
  - `tenant-membership-lifecycle`
  - `tenant-cross-surface`
  - `secret-response-safety`
  - `agent-runtime-memory`
  - `scheduled-task-runtime`
  - `auth-security-persistence`
  - `relay-file-mapping-tenant-ownership`
  - `relay-runtime-channel-isolation`
  - `workflow-sql-isolation`
  - `publishing-channel-isolation`
  - `admin-relay-channel-isolation`
  - `admin-relay-read-isolation`
  - `quota-sql-isolation`
- `app-stateful-routes` now covers Console API token create/list/revoke and sanitized Console recent usage in the same DB-backed profile.
- `tenant-cross-surface` now covers active-organization isolation across Chat, Knowledge, Console, Agent, Memory, MCP, Quota, Marketplace publisher, Marketplace settlement preferences, Agent run detail, and Agent tool-run decision/retry routes.
- Admin Observability provider, Publishing channel, and Admin Relay channel secret-response safety now have SQL-backed HTTP proof; Auth security persistence, Auth reset-token replay/expiry depth, Relay file-mapping tenant ownership, Relay runtime channel active-organization isolation, Workflow SQL active-organization isolation, Publishing channel active-organization isolation, Admin Relay channel and read-surface active-organization isolation, and Quota SQL tenant isolation now have no-skip DB evidence.
- The post-Admin-Relay runtime isolation boundary is now covered at repository level: `RelayStore` loads `organization_id`, `ChannelPool` and `LoadBalancer` expose organization-scoped selection paths, `Router.RouteWithBilling` passes trusted organization scope into affinity/retry selection, and `/v1/models` scopes model discovery to trusted tenant channels. This does not reclassify the Relay routing semantics row or the broader Security row because target-environment tenant-isolation proof and broader security depth remain open.
- The Admin Relay read-surface boundary is now covered at repository level: runtime stats are filtered through the active organization's channel IDs, model inventory fails closed without organization scope, SQL model inventory queries filter channel rows by `organization_id`, and usage aggregates join by both organization and model ID. This narrows Admin Relay inspection leakage risk but does not prove every future Admin read surface.
- Active source TODO/stub scan still does not reveal a new broad implementation gap. Most matches are test stubs, generated gRPC `Unimplemented*` boilerplate, placeholder-secret/runbook language, UI input placeholders, benign nil-return no-row paths, and explicit tests that assert placeholder output is not used.
- First-party active TODO boundaries are narrow and already documented as future or non-release proof:
  - `src/server/internal/relay/handler/realtime.go` has auth/prebill/settlement TODOs, while `docs/release/relay-route-table.md` marks Realtime `DisabledInProduction`.
  - `src/server/internal/relay/handler/policy.go` explicitly disables fine-tuning and Assistants/Threads/Runs as future commercial support.
  - `scripts/migrate-service-template.sh` contains a service-template TODO.
  - `src/server/internal/admin/channel_service.go` fails closed for unimplemented channel providers.
- `src/server/internal/relay/handler_new/` contains stale/alternate handler code with TODOs, but current runtime registration imports `src/server/internal/relay/handler/*` through `src/server/internal/relay/relay.go`; do not use `handler_new` as completion evidence.
- `.tmp/rescan-stale-artifacts/` remains a quarantine area from the previous cleanup and should not be treated as release source.

## Recommended Next Slices

1. DB-backed security depth: continue remaining tenant-isolation audits outside the covered Relay runtime, Admin Relay channel/read, Publishing channel, Workflow, Quota, Tenant, Auth, and cross-surface profiles; add equivalent response-safety proof for any newly added provider/channel secret surfaces.
2. Broader Browser/E2E route proof: continue extending high-value commercial workflows beyond the current Agent planning, Chat-to-SOLO, Marketplace paid-provider, and Workflows mobile responsive browser journeys.
3. Strict commercial verifier rerun on target infrastructure with deploy and backup/restore enabled.
4. Observability and recovery proof: continue from the panic recovery proof into target-environment OOM/crash restart execution, scale-down, and failover evidence.
5. Deployment validation: only after repo-owned rows are narrowed further, run deploy/Kubernetes/backup-restore proof on the target installation.

## Boundary

This report is a repository rescan and evidence checkpoint, not a final completion claim. Rows marked `Partial` in `docs/reports/2026-06-07-fusion-spec-completion-matrix.md` remain open until the row-specific proof is recorded and rerun in the required environment.
