# Repository Rescan - 2026-06-16

## Current Truth

- Branch: `main`.
- Rescan evidence base before this strict target-verifier continuation: `31d4662` (`test(relay): prove tenant scoped file list`).
- Remote parity at this strict target-verifier continuation start: `HEAD == origin/main` (`31d4662`).
- Rescan evidence base before this Relay file-list continuation: `001a6ab` (`test(release): require all live payment rails`).
- Remote parity at continuation start: `HEAD == origin/main` (`001a6ab`).
- Working tree at continuation start: clean at the pushed target/live payment-rails verifier baseline.
- The earlier same-day scans at `1c52194`, `53aaca0`, `d4fdc37`, `75ff216`, `98a1683`, `c6d9b34`, `5969e4b`, `1e7c6ef`, `e222967`, `0694b44`, `984b6a7`, `cb18df4`, and `bd92f91` are now older baselines for this report.
- The project is still not complete against the four `docs/superpowers/specs/2026-06-04-*` specs.
- Current matrix remains `4 Proven / 10 Partial / 0 Gap / 0 Unverified`.
- Current progress estimate remains about `99/100`: repository-local evidence is broad, the Agent planning token-budget browser path and Task approval state/workspace guard are now closed locally, and final completion is still gated by target/live proof plus a strict no-skip release run.

## What Changed Since The Last Rescan

- Agent action IDs now fail closed when snake_case and camelCase aliases conflict for tool-run and plan-step actions.
- PlanningEngine tool steps now have direct ReAct handoff proof.
- MCP auth-token PostgreSQL evidence and strict verifier coverage were tightened.
- Agent gRPC generated-client coverage and Workflow/Task generated-client proof remain in the checked-in evidence set.
- The prior pushed memory slice proves LLM-assisted long-term memory extraction composes with `memory_key_consolidate`: generated keyed facts update the existing long-term memory in place instead of creating duplicates.
- Manual Agent plan-step draft insert/move/delete now preserves `dependsOn` references by logical step. Deleting a draft step that another step depends on fails closed instead of silently loosening the plan.
- Marketplace paid audit rows are now protected from publisher hard-delete, direct SQL cascade, and buyer uninstall regressions. `DeleteAgent` rejects hard deletion once marketplace order audit evidence exists for the agent in the publisher organization, and migration `0082_marketplace_audit_retention.sql` rebuilds paid audit foreign keys with non-cascading delete semantics.
- Scheduled-task due-worker failures now advance `next_run_at` through a dedicated failure path, so a failed claimed run cannot be immediately reclaimed while its old due time remains in the past.
- Scheduled Task HTTP routes now have active-organization isolation proof for run-history, status-update, and run-now paths. Cross-organization requests return 404, do not leak owner run evidence, do not mutate the owner task, and do not create extra owner runs.
- Admin top-up refund operator evidence now has no-skip PostgreSQL route proof: duplicate Admin Stripe refund submissions persist one refund row and one lifecycle transition with provider charge/payment-intent evidence, while missing Stripe charge/payment-intent evidence returns 400 without mutating refund, lifecycle, payment-intent, top-up, or quota state.
- Quota SQL isolation now aggregates quota lifecycle proof across quota store, HTTP route, Admin, and Stripe lifecycle tests in one no-skip PostgreSQL profile, including user-scoped balance mode, request-cap fallback, top-up no-credit-before-webhook, direct top-up rejection, Admin refund quota reversal, and Stripe top-up/refund accounting.
- Target/live release evidence now has a machine-readable manifest verifier. The strict commercial verifier requires `COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true` for final readiness, and the manifest validator checks exact commit, strict no-skip verifier result, deployment/backup/migration evidence, Kubernetes validation/rollout/failover evidence, live provider checkout/refund/payout/reconciliation evidence, Agent/Workflow/Task generated-client gRPC smoke evidence, target secret audit, workflow telemetry `successRate >= 0.99`, no skip-like fields, no embedded secret material, and no placeholder artifact refs. `--print-template` now emits a current-commit external manifest skeleton that is intentionally rejected until every `TODO` artifact reference is replaced with concrete target-run evidence.
- Chat realtime collaboration now has a repository-local implementation and proof slice. `/api/v1/ws` supports conversation-scoped Chat rooms, `chat_join`/`chat_leave`/`chat_typing` client frames, and server-side Chat sync/update/delete broadcasts; `ChatPage` opens the active-conversation socket, sends typing presence, renders collaborator typing state, and applies sync/update/delete events; OpenAPI documents and gates the Chat realtime WebSocket schemas.
- The current parallel read-only scan did not find a new broad product-area gap, but it did separate target/live blockers from local proof-depth candidates. The highest-priority local release-readiness gap from the prior baseline is now closed: Agent/Workflow/Task generated-client gRPC smoke has a first-class target helper, and Workflow/Task now expose real runtime gRPC listeners and deployment port contracts.
- Workflow-to-Agent direct client parity is now closed as a local proof-depth candidate. The standalone Workflow `AgentClient.StartAgentRun` now uses workspace-scoped sessions, normalizes execution mode, dispatches planning requests to `StartPlanningRun` and default/ReAct requests to `StartRun`, preserves workspace scope for tool approvals, and maps Agent run results with the same nil-safe/final-message fallback boundary as the HTTP adapter.
- Agent advanced runtime configuration is now closed locally for both the backend service run path and the formal product path. `RunWithTools`, approval resume, and the resumed tool loop apply persisted `Agent.Config.ModelRoutingRules`, `Skills`, and `MaxSkills` per iteration, while OpenAPI, web types, and Workspace Agents create/edit controls now expose the same config fields.
- Marketplace paid-install checkout-failure settlement proof is now included in the no-skip money-movement profile. The profile now runs `TestSettlementMarkPaidInstallCheckoutFailedMarksOrderAndIntent` alongside the existing HTTP `TestMarketplacePaidInstallCheckoutCreatorFailureMarksOrderFailed`, so checkout-creator failure is covered at both settlement-state and route-state boundaries under disposable/configured PostgreSQL with skipped tests rejected.
- Admin usage-limit settings now have no-skip PostgreSQL route proof. A real admin router test writes organization-scoped and user-scoped usage-limit settings through `/api/v1/admin/settings/usage-limits`, proves the session organization overrides a spoofed request body organization, verifies `concurrency_limits` and `token_rate_limits` rows, and reads both settings back through the GET route.
- Admin Billing webhook-event inspection now has no-skip PostgreSQL non-disclosure proof. The route test stores a sensitive raw provider payload in `stripe_webhook_events.payload`, proves the DB contains it, then verifies `/api/v1/admin/billing/webhook-events` returns only sanitized ledger metadata without a `payload` field or raw secret/customer/payment-method values.
- Workflow canvas context-menu test-node proof is now covered in the built Workspace browser journey. The Playwright fixture rejects `/api/v1/workflows/{workflowId}/test-node` payloads unless the selected canvas node ID and node input are preserved, and the browser test opens the real React Flow node context menu for `classify`, dispatches `Test this node`, and verifies the node-test result.
- Admin model-route CRUD payload proof is now covered in the built Admin browser journey. The Admin Commercial Config Playwright fixture fails closed unless route create/update payloads preserve model pattern, strategy, channel IDs, weights, priorities, and enabled flags, while the browser test creates a weighted multi-channel route, edits it to cost-aware routing, and deletes it back to the empty state.
- Chat realtime WebSocket proof is now covered in the built Chat browser journey. The Playwright fixture uses native `page.routeWebSocket` to intercept `/api/v1/ws` without connecting to a real backend, fails closed on unexpected socket paths, conversation IDs, client frame types, or malformed typing frames, and the browser test proves `chat_join`, `chat_typing`, server-pushed sync/update/delete events, and collaborator typing UI.
- Agent planning token-budget recovery proof is now covered in the built Workspace browser journey. The Agent planning Playwright fixture exposes a `token_budget_exceeded` plan-step route, fails closed unless `/continue-budget` receives `tokenBudget=45000`, and the browser test proves the page renders the stop reason and failed dependency-aware step, submits the increased budget, and refreshes to completed step evidence.
- Task approval bypass proof is now covered in the no-skip PostgreSQL app-stateful route profile. The route test drives `POST /api/v1/app/tasks/{taskId}/approve` through the real router with cookie and CSRF, proves only current-workspace `awaiting_confirmation` tasks advance to `running`, and proves completed, cancelled, running, draft, and cross-workspace tasks preserve task and step state.
- Marketplace paid-install WeChat Pay provider parity is now covered locally. HTTP paid-install success coverage runs both Alipay and WeChat Pay provider subcases, domestic hosted checkout creator tests assert Marketplace metadata query parity for both providers, settlement PostgreSQL tests preserve selected provider/currency and lifecycle keys for both providers under the money-movement profile, and the built Marketplace browser journey selects WeChat Pay and verifies the continuation link.
- Agent ReAct model-routing, skill runtime, `call_agent`, and websearch fallback proof are now part of the no-skip `agent-runtime-memory` commercial DB profile before the existing SQL persistence checks. The profile uses disposable/configured PostgreSQL, rejects skipped tests, and now covers second-iteration model switching after tool results, skill selection/tool filtering/instruction injection, recursive sub-agent depth guards, and websearch fallback/exhaustion.
- Target/live evidence validation now requires separate live provider evidence entries for Stripe, Alipay, and WeChat Pay. The target evidence template prints all three entries, and the verifier rejects a final manifest if any of the three first-party payment rails is missing checkout/refund/payout/reconciliation proof or concrete artifact refs.
- Relay file-list passthrough now has tenant-scoped ownership proof. `GET /v1/files` moved from production-disabled raw passthrough to a mapped, billed, trusted-identity route: it lists current-tenant SQL file mappings, skips upstream for tenants with no mappings, filters upstream file-list rows to mapped provider IDs, rewrites client-visible IDs back to local file IDs, and preserves `provider_file_id` evidence.
- Target/live strict verifier command validation now requires `strictVerifier.command` to include all four final gate flags: `COMMERCIAL_COMPLETION_RUN_DEPLOY=true`, `COMMERCIAL_COMPLETION_RUN_K8S=true`, `COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true`, and `COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true`. A manifest that records `scripts/verify-commercial-completion.sh` without those flags is rejected.
- Target/live gRPC evidence validation now requires a structured `grpcSmokeReport` copied from `scripts/target-grpc-smoke.sh`. The manifest verifier rejects missing smoke reports, missing/failed Agent/Workflow/Task smoke results, and smoke result addresses that do not match the manifest `grpc` entries.

## Repository Inventory

- First-party tracked files after this current rescan:
  - `src`: 1002
  - `docs`: 93
  - `scripts`: 39
  - `deploy`: 42
  - `.planning`: 210
  - other tracked files: 27
  - total tracked files: 1413
- Server internal shape:
  - `src/server/internal`: 591 tracked files.
  - largest active domains: `relay` 110, `http` 104, `mcp` 43, `admin` 39, `agent` 34, `workflow` 32, `knowledge` 30, `observability` 28, `channel` 27, `migration` 24, `marketplace` 16.
- Test inventory:
  - Go test files: 236
  - Web component/API test files: 67
  - Web Playwright specs: 16
  - Web E2E fixture files: 16
- Migration inventory:
  - Top-level PostgreSQL SQL migration files: 94.
  - ClickHouse SQL migration files: 1.
  - Microservice split SQL migration files: 12.
  - Microservice split migrations exist for `admin`, `agent`, `billing`, `channel`, `chat`, `gateway`, `marketplace`, `observability`, `rag`, `relay`, `task`, and `workflow`.
- First-party `AGENTS.md`: none found after excluding `.git`, `node_modules`, `.tmp`, `src/web/node_modules`, and `reference`.

## Spec Surface

The active completion target is still the four June 4 specs:

- `2026-06-04-complete-fusion-design.md`: Gateway/Relay, Workflow, Knowledge/RAG.
- `2026-06-04-complete-fusion-design-part2.md`: Agent, Billing, Marketplace, Publishing channels, frontend, roadmap.
- `2026-06-04-complete-fusion-design-part3.md`: schema, API, Kubernetes deployment, monitoring, security, migration, success criteria.
- `2026-06-04-functional-logic-details.md`: detailed logic for Relay, Workflow, RAG, Agent, Publishing, Billing, Frontend UI, Marketplace, Observability.

## Completion Matrix Snapshot

Proven rows:

- API gateway and relay.
- Knowledge base and RAG.
- Multi-channel publishing.
- Database schema and migrations.

Partial rows:

- Workflow engine.
- Agent system.
- Billing and monetization.
- Marketplace ecosystem.
- Frontend shell and core pages.
- Observability metrics, logs, alerts, recovery.
- API contract.
- Deployment and operations.
- Security and tenant isolation.
- Migration strategy and release readiness.

No rows are currently marked `Gap` or `Unverified`.

## Evidence Boundary

- `docs/release/fusion-spec-evidence-pack.md` still states that the pack is not a final completion claim; any `Partial` row remains open until row-specific proof is recorded and rerun on the target environment where required.
- `scripts/verify-target-release-evidence.sh` now provides the external target/live evidence manifest gate for proof that cannot be collected from repository-local tests.
- The target/live manifest gate now requires Stripe, Alipay, and WeChat Pay live checkout/refund/payout/reconciliation entries individually; a single live provider entry is not sufficient for final readiness.
- The target/live manifest gate also requires `strictVerifier.command` to carry the deploy, Kubernetes, backup/restore, and target-evidence flags. This keeps final manifests aligned with the strict `scripts/verify-commercial-completion.sh` path instead of accepting partial local verifier invocations.
- The target/live manifest gate requires `grpcSmokeReport.results` to include generated-client pass rows for Agent, Workflow, and Task, and each smoke address must match the manifest `grpc` address for that service.
- Chat realtime repository proof is local and focused: `go test ./internal/ws`, Chat handler realtime publish tests, focused Chat Vitest, built-app Playwright WebSocket proof, OpenAPI contract verification, web TypeScript, and docs gate cover this slice.
- Task approval state/workspace proof is local and focused: `scripts/verify-commercial-db-evidence.sh app-stateful-routes` covers the real app router, cookie/CSRF, SQL store, allowed transition, denied terminal/draft/current-state cases, and cross-workspace no-mutation boundary.
- Agent runtime/tool proof is local and focused: `scripts/verify-commercial-db-evidence.sh agent-runtime-memory` now reruns ReAct model routing, skill selection/tool filtering/instruction injection, `call_agent` recursion guard, websearch fallback, and Agent SQL runtime/memory persistence with skipped tests rejected.
- Relay file-list ownership proof is local and focused: `scripts/verify-commercial-db-evidence.sh relay-file-mapping-tenant-ownership` now covers upload, mapped get, tenant-scoped list, wrong-tenant empty list, and list store ownership filtering against disposable/configured PostgreSQL with skipped tests rejected.
- `scripts/verify-commercial-db-evidence.sh` currently exposes 24 focused profiles plus the `all` aggregator:
  - `backend-journey`
  - `marketplace-money-movement`
  - `marketplace-governance-review`
  - `marketplace-recommendation-search`
  - `marketplace-template-routes`
  - `billing-checkout-topup-http`
  - `billing-provider-lifecycle`
  - `admin-usage-analytics-db`
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
  - `observability-alert-recovery-persistence`
  - `quota-sql-isolation`
  - `core-sql-persistence`
- `scripts/verify-commercial-completion.sh` is strict by default. It requires `TEST_DATABASE_URL`, deploy validation, Kubernetes validation, backup/restore smoke, target live evidence manifest validation, docs/security gates, web TypeScript, full Go suites, DB-backed evidence profiles, and diff hygiene.
- A run with `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true` remains partial local evidence only and cannot count as final readiness.

## Current Blockers

Pure target/live blockers that still keep final status below `100/100`:

- Target Kubernetes validation with reachable cluster access and a filled untracked `OBLIVIOUS_K8S_SECRET_FILE`.
- Target deployment failover/recovery evidence, including real platform restart, scale, and failover behavior.
- Live billing and Marketplace provider rails for checkout, refund, payout, and reconciliation.
  - The manifest verifier now requires separate Stripe, Alipay, and WeChat Pay live evidence entries for these rails; local fake/provider tests still do not satisfy this blocker.
- Target Agent/Workflow/Task gRPC reachability using generated clients against running service endpoints.
- Target secret audit for deployed provider/channel/workflow/runtime secrets.
- Target or CI `TEST_DATABASE_URL` runs broad enough to count as final release evidence, not only disposable local profile proof.
- A final `scripts/verify-commercial-completion.sh` run with no environment skips and `COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true` pointing at an external, secret-free target evidence manifest for the exact release commit.
  - The external target manifest must also record the strict verifier command with `COMMERCIAL_COMPLETION_RUN_DEPLOY=true`, `COMMERCIAL_COMPLETION_RUN_K8S=true`, `COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true`, and `COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true`.
  - The external target manifest must embed the `grpcSmokeReport` JSON copied from the target gRPC smoke artifact, not only free-form gRPC artifact references.

Repository-local proof-depth candidates surfaced by this rescan after closing the gRPC release-readiness local blocker, the Task approval state guard, and Marketplace WeChat Pay paid-install parity:

Closed this slice:

- Target gRPC release-readiness local blocker.
  - Evidence: Workflow and Task now have runtime gRPC listeners and deployment port contracts, `cmd/grpc-smoke` plus `scripts/target-grpc-smoke.sh` call checked-in generated clients for Agent/Workflow/Task with per-service dial/RPC timeouts, and deployment/target evidence verifiers assert ports `50063`, `50064`, and `50065`.
  - Boundary: this is repository-local generated-client reachability proof using validation-only RPCs; DB/Kafka/business-success readiness and target endpoint reachability remain external evidence.
- Workflow-to-Agent direct client parity.
  - Evidence: `AgentClient.StartAgentRun` now matches the HTTP Workflow adapter's planning/ReAct split and workspace-scoped session behavior, `ApproveAgentToolRun` preserves workspace scope for reloads, and `toWorkflowAgentRunResult` keeps messages/tool-runs/plan-steps when `Run` is nil while falling back to the latest assistant message. `TestAgentClientStartAgentRunUsesPlanningModeAndWorkspaceSession`, `TestAgentClientStartAgentRunDefaultsToReactPath`, `TestAgentClientApproveAgentToolRunUsesWorkspaceSession`, and `TestAgentClientRunResultMappingIsNilSafeAndFallsBackToAssistantMessage` cover the regression boundary.
  - Boundary: this is repository-local Workflow/Agent adapter proof; deployed Agent/Workflow compatibility, target workflow telemetry, and final no-skip release evidence remain external.
- Agent advanced runtime configuration backend service path.
  - Evidence: `RunWithTools`, `ResumeAfterApprovedToolWithTokenBudget`, and `resumeToolLoop` now call the same runtime-configuration helper for each structured ReAct iteration. `TestServiceStartRunUsesAgentConfigModelRoutingRules` proves persisted model routing switches the second iteration after a completed tool result, and `TestServiceStartRunUsesAgentConfigSkillsAndMaxSkills` proves persisted skills are selected from the Agent config, injected into the system prompt, and used to filter tools under `maxSkills`.
  - Boundary: this closes backend service execution for tool-enabled ReAct runs and approval resume. Target release proof remains external.
- Agent advanced runtime configuration product path.
  - Evidence: `docs/api/openapi.yaml` now documents `AgentModelRoutingRule`, `AgentSkill`, and the `AgentConfig.modelRoutingRules` / `skills` / `maxSkills` fields, `scripts/verify-openapi-contract.sh` gates those schemas, `src/web/src/types/api.ts` exposes typed web config fields, and `src/web/src/routes/workspace/AgentsPage.tsx` can create/edit the fields through Workspace controls. `AgentsPage.test.tsx` and `agentsApi.test.ts` prove create/update payloads preserve the advanced runtime config.
  - Boundary: this is repository-local product-path proof; deployed runtime compatibility, target/live evidence, and final no-skip release proof remain external.

Remaining local proof-depth candidates:

- No additional broad product-area gap was found in this refresh. The matrix stays partial because of the target/live gates, not because a new repository-owned domain is unimplemented.

Closed in this continuation:

- Agent planning token-budget browser recovery.
  - Evidence: `src/web/e2e/agent-planning.spec.ts` now covers `/agent-runs/run_browser_agent_budget/plan-steps`, renders `token_budget_exceeded`, stop reason, counters, failed step, and dependency evidence, submits an increased token budget, and verifies completed refreshed step evidence.
  - Boundary: this is repository-local built-app browser proof with a fail-closed fixture for the `/continue-budget` payload. It does not replace deployed Agent runtime, target gRPC, target database, or final no-skip release evidence.
- Task approval bypass PostgreSQL proof.
  - Evidence: `src/server/internal/http/task_handler_test.go` now drives `POST /api/v1/app/tasks/{taskId}/approve` through the real router with cookie and CSRF, proves only current-workspace `awaiting_confirmation` tasks advance to `running`, and proves completed, cancelled, running, draft, and cross-workspace tasks do not mutate task or step rows. `scripts/verify-commercial-db-evidence.sh app-stateful-routes` now includes that test and rejects skips.
  - Boundary: this is repository-local app-stateful route proof. It does not replace deployed Task runtime, target database, or final no-skip release evidence.
- Marketplace WeChat Pay paid-install provider parity.
  - Evidence: `src/server/internal/http/marketplace_payment_provider_test.go` now runs Alipay and WeChat Pay paid-install success subcases through the selected provider, settlement request, checkout creator metadata, recorded checkout session, and checkout-session response. `src/server/internal/http/payment_provider_config_test.go` asserts both domestic hosted checkout creators preserve Marketplace order/agent/version/publisher query metadata. `src/server/internal/marketplace/settlement_test.go` asserts selected provider/currency and marketplace lifecycle transition keys for Alipay and WeChat Pay. `src/web/src/features/marketplace/api.test.ts`, `src/web/src/routes/marketplace/MarketplacePage.test.tsx`, `src/web/e2e/fixtures/adminMarketplace.ts`, and `src/web/e2e/admin-marketplace.spec.ts` prove the Marketplace UI/API/browser path selects WeChat Pay and renders its checkout continuation link.
  - Verification: `go test ./internal/http -run 'TestMarketplacePaidInstallCheckoutUsesSelectedProviderAndReturnsCheckoutSession|TestBuildPaymentCheckoutProvidersEnablesDomesticHostedProviders' -count=1 -v`, focused Marketplace Vitest, `scripts/verify-commercial-db-evidence.sh marketplace-money-movement` with disposable pgvector PostgreSQL and skipped tests: none, `playwright test e2e/admin-marketplace.spec.ts --project=chromium`, web TypeScript, docs gate, and `git diff --check` passed.
  - Boundary: this is repository-local provider-selection and checkout-metadata proof. It does not replace live WeChat Pay checkout, refund, payout, reconciliation, target database, or final no-skip release evidence.
- Agent ReAct/runtime-tool no-skip profile coverage.
  - Evidence: `scripts/verify-commercial-db-evidence.sh agent-runtime-memory` now runs `TestExecuteReActWithModelRouting`, `TestExecuteReActModelSwitching`, skill selection/tool-filtering/instruction-injection tests, `call_agent` registration and recursion-depth tests, and websearch primary/fallback/exhaustion/config tests before the existing Agent SQL run/plan-step/config/memory persistence tests.
  - Verification: `GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh agent-runtime-memory` passed with disposable pgvector PostgreSQL and skipped tests: none.
  - Boundary: this is repository-local runtime/tool evidence. It does not replace deployed Agent runtime compatibility, target gRPC reachability, target database, or final no-skip release evidence.
- Target provider evidence strictness.
  - Evidence: `scripts/verify-target-release-evidence.sh` now prints separate `stripe`, `alipay`, and `wechatpay` provider slots in `--print-template`, rejects duplicate provider names, and rejects manifests missing any of the three required first-party payment rails.
  - Verification: `bash scripts/verify-target-release-evidence.sh --print-template`, a temporary fully populated current-commit manifest, a temporary manifest missing `wechatpay`, `bash -n scripts/verify-target-release-evidence.sh scripts/verify-quality-gates.sh`, docs gate, and `git diff --check` passed.
  - Boundary: this proves verifier strictness only. It does not provide live checkout, refund, payout, reconciliation, or target environment artifacts.
- Relay file-list tenant ownership.
  - Evidence: `src/server/internal/relay/handler/files.go` now requires a list-capable file mapping store and trusted tenant identity before `GET /v1/files`, `src/server/internal/relay/store.go` exposes `ListFileMappings` scoped by user and organization, `src/server/internal/relay/handler/policy.go` marks the list route production-enabled, and the route table records the mapped-list boundary.
  - Verification: focused handler policy/list tests, focused Relay SQL tests, `scripts/verify-commercial-db-evidence.sh relay-file-mapping-tenant-ownership`, docs gate, and `git diff --check` passed.
  - Boundary: this is repository-local file-list ownership proof. It does not replace target-environment tenant-isolation or final no-skip release evidence.
- Target verifier final-gate flag strictness.
  - Evidence: `scripts/verify-target-release-evidence.sh` now rejects `strictVerifier.command` unless it runs `scripts/verify-commercial-completion.sh`, avoids `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true`, and includes `COMMERCIAL_COMPLETION_RUN_DEPLOY=true`, `COMMERCIAL_COMPLETION_RUN_K8S=true`, `COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true`, and `COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true`. `scripts/verify-quality-gates.sh` locks those required strings into the release quality gate.
  - Verification: temporary valid/invalid target manifests, docs gate, and diff hygiene cover this strictness slice.
  - Boundary: this proves manifest verifier strictness only. It does not provide target Kubernetes, live provider rails, deployed Agent/Workflow/Task gRPC reachability, target secret audit, workflow telemetry, or a final no-skip commercial completion run.
- Target gRPC smoke report attachment strictness.
  - Evidence: `scripts/verify-target-release-evidence.sh` now requires `grpcSmokeReport.evidenceRef`, ISO-8601 `recordedAt`, `timeout`, and `results` rows for Agent, Workflow, and Task. Each result must have `generatedClient=pass`, a validation status, the required service port, and an address matching the manifest `grpc` service address.
  - Verification: temporary missing-report, failed-result, and mismatched-address manifests are rejected; a filled current-commit manifest with matching smoke report passes.
  - Boundary: this proves the manifest can no longer claim deployed gRPC compatibility from `grpc` rows alone. It still does not replace actually running `scripts/target-grpc-smoke.sh` against deployed endpoints.

## TODO And Placeholder Scan

The active TODO/stub scan did not expose a new broad implementation gap. The remaining matches are known categories:

- Future Relay surfaces intentionally marked `DisabledInProduction` in `docs/release/relay-route-table.md`.
- Stale or alternate `src/server/internal/relay/handler_new` TODOs that are not the active runtime handler path.
- Test stubs and generated gRPC `Unimplemented*` boilerplate.
- Placeholder-only release docs, config examples, and UI input placeholders.
- `scripts/migrate-service-template.sh` template TODO.

## Local Follow-Up Candidates

Closed after this rescan:

- Marketplace publisher delete/uninstall settlement audit.
  - `src/server/internal/marketplace/store.go` now rejects publisher hard delete once order audit evidence exists, `src/server/migrations/0082_marketplace_audit_retention.sql` rebuilds paid audit foreign keys with non-cascading delete semantics, and `scripts/verify-commercial-db-evidence.sh marketplace-money-movement` now includes PostgreSQL proof that direct SQL delete attempts are blocked and buyer uninstall preserves paid order/settlement rows while clearing `marketplace_orders.install_id`.
- Scheduled task failure re-claim behavior.
  - `src/server/internal/schedule/worker.go` now sends due-worker failures through `FailScheduledTaskRun`, `src/server/internal/schedule/store.go` atomically marks the claimed run `failed` and advances `scheduled_tasks.next_run_at`, and `scripts/verify-commercial-db-evidence.sh scheduled-task-runtime` now includes PostgreSQL proof that the failed due task is not immediately claimed again.
- Scheduled Task HTTP cross-tenant isolation.
  - `src/server/internal/schedule/service.go` now verifies task ownership before listing run history, `src/server/internal/http/schedule_handler.go` returns 404 for missing/cross-organization run-history requests, and `scripts/verify-commercial-db-evidence.sh tenant-cross-surface` now includes PostgreSQL proof for cross-organization list-runs, status-update, and run-now denial without leakage or mutation.
- Admin top-up refund operator-evidence DB proof.
  - `src/server/internal/http/admin_billing_handler_test.go` now asserts persisted `billing_refunds` provider charge/payment-intent/top-up evidence, the exact operator lifecycle transition, idempotent duplicate submission behavior, and a real-router rejection path that preserves refund/lifecycle ledgers, payment intent, top-up order, and quota state. `scripts/verify-commercial-db-evidence.sh marketplace-money-movement` now includes the rejection-preservation test.
- Quota lifecycle DB evidence aggregation.
  - `scripts/verify-commercial-db-evidence.sh quota-sql-isolation` now runs quota SQL store lifecycle/isolation tests, quota/Admin/Billing HTTP route tests, and Stripe top-up/refund lifecycle balance-accounting tests in one no-skip PostgreSQL profile.
- Target/live release evidence manifest gate.
  - `scripts/verify-target-release-evidence.sh` now validates the external JSON evidence package required for live provider rails, deployed gRPC reachability, target secret audit, Kubernetes failover, workflow telemetry, and strict no-skip verifier proof, while rejecting embedded secret material and placeholder artifact refs.
- Target/live release evidence template generation.
  - `scripts/verify-target-release-evidence.sh --print-template` now emits a current-commit manifest skeleton for an external file outside git. The generated template contains `TODO` artifact refs and is intentionally rejected by the verifier until real target-run evidence replaces those values.
- Target gRPC release-readiness local blocker.
  - `src/server/cmd/workflow/main.go` now starts a real Workflow gRPC listener on `WORKFLOW_GRPC_PORT`/`GRPC_PORT` defaulting to `50064`, while `src/server/cmd/task/main.go` starts a real Task gRPC listener on `TASK_GRPC_PORT`/`GRPC_PORT` defaulting to `50065`.
  - `deploy/docker`, `docker-compose.yml`, `deploy/kubernetes`, `config/.env.example`, and `deploy/docker/.env.example` now publish/configure Workflow `8082/50064` and Task `8084/50065`, add Workflow/Task probes, expose client-side smoke address examples, and avoid `localhost:9092` Kafka fallbacks in Compose/Kubernetes deployments.
  - `src/server/cmd/grpc-smoke` and `scripts/target-grpc-smoke.sh` provide validation-only generated-client smoke calls for Agent `50063`, Workflow `50064`, and Task `50065` with per-service timeout coverage; the output is intended to be stored outside git and referenced from the target evidence manifest.
- Workflow-to-Agent direct client parity.
  - `src/server/internal/workflow/agent_client.go` now shares the HTTP adapter's planning/ReAct dispatch contract for direct Workflow Agent calls, including workspace-scoped start and tool-approval sessions.
  - `src/server/internal/workflow/agent_client_test.go` locks planning-mode dispatch, default/ReAct fallback, workspace preservation for tool approval and reload, nil-safe result mapping, and latest-assistant final-message fallback.
- Agent advanced runtime configuration backend service path.
  - `src/server/internal/agent/runner.go` now applies persisted `modelRoutingRules`, `skills`, and `maxSkills` in the mature `RunWithTools` service path and in approval resume loops rather than relying only on the newer standalone `ExecuteReAct` path.
  - `src/server/internal/agent/service_test.go` locks the service-level behavior for model routing after a tool result and skill-selected tool/prompt narrowing.
- Agent advanced runtime configuration product path.
  - `docs/api/openapi.yaml`, `scripts/verify-openapi-contract.sh`, and `src/web/src/types/api.ts` now expose and guard Agent runtime model-routing and skill config fields.
  - `src/web/src/routes/workspace/AgentsPage.tsx`, `src/web/src/routes/workspace/AgentsPage.test.tsx`, and `src/web/src/features/agents/agentsApi.test.ts` now prove create/edit payloads carry `modelRoutingRules`, `skills`, and `maxSkills`.
- Chat realtime collaboration repository slice.
  - `src/server/internal/ws/hub.go` now tracks conversation rooms and broadcasts Chat events only to room subscribers, excluding the sender for client-originated typing.
  - `src/server/internal/http/chat_handler.go` and `src/server/internal/http/chat_actions_handler.go` publish message sync/update/delete events after successful Chat mutations while keeping `listMessages` read-only for realtime to avoid sync loops.
  - `src/web/src/features/chat/api.ts` and `src/web/src/routes/workspace/ChatPage.tsx` connect the active conversation to `/api/v1/ws`, send typing presence, and apply realtime transcript events.
  - `docs/api/openapi.yaml` and `scripts/verify-openapi-contract.sh` now document and guard Chat realtime WebSocket frame schemas.
- Marketplace paid-install checkout-failure no-skip profile inclusion.
  - `scripts/verify-commercial-db-evidence.sh marketplace-money-movement` now includes `TestSettlementMarkPaidInstallCheckoutFailedMarksOrderAndIntent`, complementing the existing HTTP route coverage for `TestMarketplacePaidInstallCheckoutCreatorFailureMarksOrderFailed`.
  - Verification: `GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-money-movement` passed with disposable pgvector PostgreSQL and skipped tests: none.
- Admin usage-limit settings route PostgreSQL proof.
  - `src/server/internal/http/admin_marketplace_handler_test.go` now includes `TestAdminUsageLimitSettingsRoutePersistsWithPostgres`, a real-router PostgreSQL test for Admin usage-limit settings persistence and listing.
  - `scripts/verify-commercial-db-evidence.sh quota-sql-isolation` now includes that route test alongside quota store, Admin user quota, Billing top-up, and Stripe top-up/refund balance-accounting proof.
  - Verification: `GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh quota-sql-isolation` passed with disposable pgvector PostgreSQL and skipped tests: none.
- Admin Billing webhook-event raw-payload non-disclosure proof.
  - `src/server/internal/http/admin_billing_handler_test.go` now includes `TestAdminBillingWebhookEventsDoNotExposeRawPayload`, a real-router PostgreSQL test proving raw provider webhook payloads remain in DB but are not returned through Admin Billing inspection.
  - `scripts/verify-commercial-db-evidence.sh marketplace-money-movement` now includes that route test in the no-skip Admin Billing money-movement profile.
  - Verification: `GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-money-movement` passed with disposable pgvector PostgreSQL and skipped tests: none.
- Workflow canvas context-menu test-node browser proof.
  - `src/web/e2e/workflows.spec.ts` now covers opening the real Workflow canvas context menu for the `classify` node in the built Workspace shell, preserving the selected node ID and node input in the debug form, dispatching `Test this node`, and rendering the returned node-test result.
  - `src/web/e2e/fixtures/workflows.ts` now fails closed unless `/api/v1/workflows/workflow_release/test-node` receives `nodeId=classify` and either the debug form input or the selected canvas node input.
  - Verification: `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/workflows.spec.ts --project=chromium` passed with 4 browser tests.
- Admin model-route CRUD payload browser proof.
  - `src/web/e2e/admin-commercial-config.spec.ts` now covers `/admin/routes` in the built Admin shell: empty state, create weighted route with two target channels, edit to `cost_aware` routing while changing weights and disabling the second channel, and delete back to empty state.
  - `src/web/e2e/fixtures/adminCommercialConfig.ts` now fails closed unless Admin route create/update payloads preserve `model`, `strategy`, channel IDs, `weight`, `priority`, and `enabled`; it also provides the channel option list required by the route drawer.
  - Verification: `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/admin-commercial-config.spec.ts --project=chromium` passed with 4 browser tests.
- Chat realtime WebSocket browser proof.
  - `src/web/e2e/chat-solo.spec.ts` now covers the built Chat route creating `/api/v1/ws`, sending `chat_join`, sending `chat_typing` with a boolean flag after draft input, rendering collaborator typing state, applying `chat_messages_synced`, applying `chat_message_updated`, and removing the message on `chat_message_deleted`.
  - `src/web/e2e/fixtures/chatSolo.ts` now uses Playwright `page.routeWebSocket` as the mocked WebSocket server and fails closed on unexpected socket paths, conversation IDs, client frame types, non-JSON frames, non-string frames, and malformed typing payloads.
  - Verification: `PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/chat-solo.spec.ts --project=chromium` passed with 2 browser tests.

The remaining pure external follow-up still does not have a repository-only substitute:

1. Target/live release evidence collection.
   - Risk: repository-local proof is broad but still cannot replace real Kubernetes, provider rails, deployed gRPC reachability, target secret audit, and a strict no-skip release run.
   - Likely files: release evidence attachments under `docs/release/` after target environment runs complete.

## Commands Run For This Rescan

```bash
git status --short --branch
git rev-parse HEAD origin/main
git log --oneline --decorate -12
rg -o '\| (Proven|Partial|Gap|Unverified) \|' docs/reports/2026-06-07-fusion-spec-completion-matrix.md | sort | uniq -c
find . -path './.git' -prune -o -path './node_modules' -prune -o -path './src/web/node_modules' -prune -o -path './.tmp' -prune -o -path './reference' -prune -o -name AGENTS.md -print
git ls-files | awk '...tracked file distribution...'
git ls-files src/server/internal | awk '...server domain counts...'
git ls-files 'src/server/**/*_test.go' | wc -l
git ls-files 'src/web/src/**/*.test.ts' 'src/web/src/**/*.test.tsx' 'src/web/src/**/*.spec.ts' 'src/web/src/**/*.spec.tsx' | sort -u | wc -l
find src/server/migrations -maxdepth 1 -type f -name '*.sql' | wc -l
find src/server/migrations/clickhouse -maxdepth 1 -type f -name '*.sql' | wc -l
find src/server/migrations -mindepth 2 -type f -name '*.sql' ! -path 'src/server/migrations/clickhouse/*' | wc -l
rg -n 'Remaining local proof-depth candidates|Current Blockers|Marketplace WeChat Pay|strict verifier|COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE|Current progress estimate' docs/reports/2026-06-16-repo-rescan.md docs/release/fusion-spec-evidence-pack.md docs/reports/2026-06-07-fusion-spec-completion-matrix.md scripts/verify-commercial-completion.sh scripts/verify-target-release-evidence.sh
rg -n 'TODO|FIXME|XXX|panic\(|skip\(|t\.Skip|describe\.skip|test\.skip|it\.skip|\.only\(' src scripts docs/reports docs/release --glob '!**/node_modules/**' --glob '!**/.tmp/**' --glob '!**/generated/**' | head -n 200
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
git diff --check
git status --short --branch
git rev-parse HEAD origin/main
git ls-files | awk '...tracked file distribution...'
git ls-files src/server/internal | awk '...server domain counts...'
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/agent -run 'TestServiceStartRunUsesAgentConfig(ModelRoutingRules|SkillsAndMaxSkills)' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/agent -count=1
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec vitest run src/routes/workspace/AgentsPage.test.tsx src/features/agents/agentsApi.test.ts
bash scripts/verify-openapi-contract.sh
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
git diff --check
git diff -- src/server/internal/workflow/agent_client.go
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/workflow -run 'TestAgentClient' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/workflow -count=1
rg -n 'real-time|realtime|collaboration|typing|WebSocket|chat' docs/superpowers/specs docs/reports docs/release -g '*.md'
rg -n 'COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE|target live|live evidence|verify-target-release-evidence|ChatRealtime|chat_' docs scripts src -g '*.md' -g '*.sh' -g '*.go' -g '*.tsx' -g '*.ts'
git log --oneline --decorate -12
git log --oneline --decorate -8
find . \( -path './.git' -o -path './node_modules' -o -path './src/web/node_modules' -o -path './.tmp' -o -path './reference' \) -prune -o -name AGENTS.md -print
find . -maxdepth 3 -type f \( -name 'package.json' -o -name 'go.mod' -o -name 'pnpm-lock.yaml' -o -name 'Makefile' -o -name 'Dockerfile*' -o -name 'docker-compose*.yml' -o -name 'AGENTS.md' \) | sort
git ls-files | awk '...tracked file distribution...'
git ls-files src/server/internal | awk '...server domain counts...'
git ls-files 'src/server/**/*_test.go' | wc -l
git ls-files 'src/web/src/**/*.test.ts' 'src/web/src/**/*.test.tsx' | wc -l
git ls-files 'src/web/e2e/*.spec.ts' | wc -l
git ls-files 'src/web/e2e/fixtures/*' | wc -l
git ls-files src/server/migrations | awk '...top-level/clickhouse/microservices migration counts...'
git ls-files src/server/internal/relay/migrations/*.sql | wc -l
git ls-files 'src/server/**/*.sql' | sed -n '1,220p'
rg -o '\| (Proven|Partial|Gap|Unverified) \|' docs/reports/2026-06-07-fusion-spec-completion-matrix.md | sort | uniq -c
rg -n '\| Proven \|' docs/reports/2026-06-07-fusion-spec-completion-matrix.md
rg -n '\| Partial \|' docs/reports/2026-06-07-fusion-spec-completion-matrix.md
rg -n '^#|^##|^###' docs/superpowers/specs/2026-06-04-*.md
rg -n '^run_[a-z0-9_]+_profile\(\)|run_all_profiles|case "\$profile"' scripts/verify-commercial-db-evidence.sh
rg -n 'target|Kubernetes|secret|payment|provider|failover|gRPC|grpc|no-skip|live|target-environment|cluster|COMMERCIAL_COMPLETION_RUN_K8S' docs/release/commercial-completion-audit.md docs/release/fusion-spec-evidence-pack.md docs/reports/2026-06-07-fusion-spec-completion-matrix.md scripts/verify-commercial-completion.sh scripts/k8s-validate.sh
rg -n 'COMMERCIAL_COMPLETION_RUN_DEPLOY|COMMERCIAL_COMPLETION_RUN_K8S|COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE|COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE|verify-target-release-evidence|target live evidence' docs/release docs/reports scripts -S
rg -n 'TODO|FIXME|XXX|stub|placeholder|Unimplemented|DisabledInProduction' src scripts deploy docs/release docs/reports -g '!src/web/test-results/**' -g '!src/web/playwright-report/**' -g '!**/*.pb.go' -g '!docs/reports/archive/**'
sed -n '1,340p' scripts/verify-target-release-evidence.sh
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/ws -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run 'TestChatHandler(SendMessagePublishesRealtimeSync|ListMessagesDoesNotPublishRealtimeSync|MessageActionsPublishRealtimeEvents|StreamMessagePublishesRealtimeSyncAfterCompletion)' -count=1 -v
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec vitest run src/features/chat/api.test.ts src/routes/workspace/ChatPage.behavior.test.tsx
bash scripts/verify-openapi-contract.sh
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit
sed -n '1,180p' docs/release/commercial-completion-audit.md
git diff --stat
git diff -- scripts/verify-commercial-completion.sh scripts/verify-fusion-evidence-pack.sh scripts/verify-quality-gates.sh scripts/verify-target-release-evidence.sh docs/release/commercial-gates.md docs/release/fusion-spec-evidence-pack.md docs/release/rc-checklist.md docs/release/commercial-completion-audit.md docs/reports/2026-06-07-fusion-spec-completion-matrix.md docs/reports/2026-06-16-repo-rescan.md
nl -ba src/server/internal/marketplace/store.go | sed -n '330,390p;740,790p'
nl -ba src/server/migrations/0030_marketplace_settlement_governance.sql | sed -n '1,180p'
rg -n 'marketplace_audit_retention|AuditRetention|DeleteWithPaidOrder|BuyerUninstallPreserves|scheduled_task_store_pattern' scripts/verify-commercial-db-evidence.sh src/server/internal/marketplace/settlement_test.go src/server/internal/marketplace/store.go src/server/migrations/0082_marketplace_audit_retention.sql src/server/internal/schedule/worker_test.go src/server/internal/schedule/store_test.go
nl -ba src/server/internal/schedule/worker.go | sed -n '1,260p'
nl -ba src/server/internal/schedule/store.go | sed -n '250,520p'
nl -ba src/server/internal/schedule/worker_test.go | sed -n '229,420p;520,760p'
nl -ba src/server/internal/schedule/store_test.go | sed -n '360,620p'
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/schedule -count=1
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh scheduled-task-runtime
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh tenant-cross-surface
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/schedule -run 'TestListScheduledTaskRunsUsesOrganizationScope' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run '^TestAdminBilling(RecordsTopupRefundAndAdjustsQuota|RejectsTopupRefundWithoutOperatorEvidenceAndPreservesLedger)$' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh marketplace-money-movement
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh quota-sql-isolation
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
bash -n scripts/verify-commercial-db-evidence.sh
bash -n scripts/verify-target-release-evidence.sh scripts/verify-commercial-completion.sh scripts/verify-fusion-evidence-pack.sh scripts/verify-quality-gates.sh
bash scripts/verify-target-release-evidence.sh --help
bash scripts/verify-target-release-evidence.sh --print-template
tmpdir=$(mktemp -d)
# temp valid/invalid target evidence manifests outside git:
#   generated template failed until TODO artifact refs were replaced
#   valid manifest passed for current HEAD
#   missing path failed
#   commit mismatch failed
#   non-live provider failed
#   non-empty skippedChecks failed
#   embedded secret material failed
git diff --check
sed -n '1,180p' scripts/verify-commercial-completion.sh
sed -n '1,140p' docs/release/fusion-spec-evidence-pack.md
```

## Verification For This Report

This report includes the Agent runtime-configuration backend service slice, the Agent runtime-configuration product path, the Chat realtime repository slice, the Workflow canvas context-menu browser proof slice, the Admin model-route CRUD browser proof slice, the Chat realtime WebSocket browser proof slice, the Agent planning token-budget browser proof slice, and the Task approval PostgreSQL proof slice. The 2026-06-16 `c0df1fa` refresh itself is documentation-only and should be verified with the docs gate plus diff hygiene; the underlying slices should be verified with their focused commands:

```bash
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/agent -run 'TestServiceStartRunUsesAgentConfig(ModelRoutingRules|SkillsAndMaxSkills)' -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/agent -count=1
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec vitest run src/routes/workspace/AgentsPage.test.tsx src/features/agents/agentsApi.test.ts
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/ws -count=1 -v
GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache go test ./internal/http -run 'TestChatHandler(SendMessagePublishesRealtimeSync|ListMessagesDoesNotPublishRealtimeSync|MessageActionsPublishRealtimeEvents|StreamMessagePublishesRealtimeSyncAfterCompletion)' -count=1 -v
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec vitest run src/features/chat/api.test.ts src/routes/workspace/ChatPage.behavior.test.tsx
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/chat-solo.spec.ts --project=chromium
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/workflows.spec.ts --project=chromium
PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/usr/bin/google-chrome COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec playwright test e2e/admin-commercial-config.spec.ts --project=chromium
bash scripts/verify-openapi-contract.sh
COREPACK_HOME=/tmp/codex-corepack pnpm --dir src/web exec tsc --noEmit
bash -n scripts/verify-target-release-evidence.sh scripts/verify-commercial-completion.sh scripts/verify-fusion-evidence-pack.sh scripts/verify-quality-gates.sh
bash scripts/verify-target-release-evidence.sh --help
bash scripts/verify-target-release-evidence.sh --print-template
target evidence manifest generated-template/valid/invalid checks
COREPACK_HOME=/tmp/codex-corepack bash scripts/check.sh docs
git diff --check
```
