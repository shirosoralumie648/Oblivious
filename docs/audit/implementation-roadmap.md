# Implementation Roadmap

Date: 2026-07-02

Goal: turn the current scaffold/partial implementation into verifiable production runtime functionality. This roadmap intentionally avoids claiming completion from tests, OpenAPI, or matrix rows.

## Gates Before Any Future Completion Claim

1. Runtime evidence must come from production code paths, not only schema/API/tests.
2. Every billable flow must show preconsume, settle, refund/reconcile, usage ledger, and audit.
3. Every external integration claim must identify the real provider, configured runtime, webhook/callback path, and failure recovery.
4. Every long-running flow must be durable across process restart.
5. Every tenant-scoped flow must prove tenant isolation in both SQL and external stores.
6. Mocks/fakes/local-only/demo paths must be impossible to count as product evidence.
7. Completion matrix changes must be blocked unless paired with runtime evidence.

## First 5 Implementation Tickets

### 1. Finish fail-closed Relay/Chat runtime and true streaming evidence

Problem:

- Gateway proxy now forwards authenticated session requests into the Relay engine, but target-runtime provider evidence is still needed.
- Chat/Agent production gateway construction now fails closed when Relay is unavailable; target-runtime provider evidence is still missing.
- Relay `handler_new` Chat SSE now proxies upstream chunks before upstream completion and avoids duplicate captured-body writes (`src/server/internal/relay/handler_new/chat.go:140-151`, `src/server/internal/relay/handler_new/chat.go:178-218`); target abort/disconnect, usage capture, request-log linkage, and settlement proof remain.
- Relay Batch submit now persists durable polling jobs with request/user/org/token/billing context, the server and standalone Relay runtimes can start the polling worker when configured, completed Batch jobs write usage and settle durable quota/API-token reservations, and terminal failures now write error usage/audit records before refund/dead-letter; target provider/result reconciliation, request-log joins, and target evidence remain.

Scope:

- Keep demo reply generation non-production only and covered by production fail-closed tests.
- Keep `/api/v1/gateway/proxy/*` covered as an authenticated Relay-engine forwarding path with quota, route decision, request log, provider error mapping, and streaming behavior.
- Add true upstream SSE streaming, cancellation, client-abort handling, usage capture, and settlement.

Acceptance evidence:

- Live model request from UI and API against configured provider.
- Usage ledger records provider usage, cost, latency, route decision, and errors.
- No fallback can produce assistant text in production profile.

### 2. Build durable RAG ingestion and vector lifecycle

Problem:

- Upload now returns `202 Accepted`, persists parsed metadata plus raw upload bytes on `knowledge_ingestion_jobs`, and the ingestion worker can replay parser/chunk/embed/create outside the request path. Target Postgres/Qdrant recovery proof is still missing.
- Document delete/update now performs document-scoped Qdrant cleanup/replacement, vector index repair jobs have retry caps, owner-guarded leases, expired-lease reclaim, and terminal `dead_letter`, and SQL document create/update, chunk edit/split/merge, and delete writes can enqueue vector index jobs in the same transaction. Parser/embedding ingestion still needs target-runtime recovery evidence.
- Qdrant payloads now include document title/version/source/page metadata, but target Qdrant runtime proof and retrieval debug remain incomplete.

Scope:

- Prove upload ingestion enqueue/status and raw parser replay in the target runtime; evaluate object-store refs before large-file production rollout.
- Track document states: uploaded, parsing, chunking, embedding, indexed, failed, deleting, deleted.
- Keep transactional vector intent coverage as new document/chunk write paths are added; preserve vector repair retry/dead-letter semantics for worker execution.
- Keep full citation payload covered: title, version, source URL, page, chunk offsets, tenant.

Acceptance evidence:

- Upload returns job/document status; processing survives restart from raw payload or object-store refs.
- Failed embedding retries and dead-letters with diagnosable logs.
- Delete/update removes stale vector points; retrieval cannot return deleted chunks after worker recovery.

### 3. Complete pricing snapshots and settlement-grade ledger

Problem:

- Relay runtime prices now load from `relay_pricing_entries`, fail closed when a requested price is missing, billable Relay usage rows persist immutable price snapshots, Admin can reconcile ledger cost against snapshot cost, manual provider-source price imports can be diffed, rejected, approved, rollback-drafted, and audited, and the server can run a configured LiteLLM maintenance worker that records freshness/reconciliation runs in `relay_pricing_sync_runs`. Remaining work is target price freshness proof and cross-system reconciliation breadth.
- Non-Stripe providers are unconfigured by default (`src/server/internal/payment/provider.go:81-87`).
- Realtime/batch/file/fine-tuning surfaces lack commercial lifecycle.

Scope:

- Attach target provider price export artifacts and freshness SLO proof to the maintenance run ledger.
- Extend the append-only usage ledger snapshots with approved provider import lineage and broader reconciliation jobs across provider exports, request logs, checkout, webhook, quota, refund, and payout.
- Add reconciliation jobs for checkout/webhook/quota usage/refund.
- Disable billable routes that do not have settlement and audit.

Acceptance evidence:

- Request cost can be checked from immutable usage + price snapshot and surfaced to Admin as mismatch/missing-snapshot evidence.
- Provider-source price changes can be imported as pending diffs, rejected or approved by an admin, rollback-drafted from an approved import, and traced through per-entry event rows.
- Webhook duplicate/retry/reconciliation behavior is visible.
- Unsupported providers/surfaces are hidden or fail with explicit unsupported state.

### 4. Make agent/workflow execution durable, traceable, and sandboxed

Problem:

- Agent lightweight path is intentionally no-tool for non-tool agents; direct use with enabled tools now fails closed before plain gateway execution (`src/server/internal/agent/runner.go:378-381`).
- Tool-enabled Agent runs now fail closed when the gateway cannot produce structured replies (`src/server/internal/agent/runner.go:872-876`, `src/server/internal/agent/runner.go:998-1002`, `src/server/internal/agent/runner.go:2023-2026`).
- Custom Python runs locally without container isolation (`src/server/internal/agent/executor.go:171-278`).
- Workflow service-level status transitions now persist as `workflow_execution_events`; SQL node execution creation writes durable debug trace/variable snapshot rows used by debug snapshots; state replay has a tenant-scoped API; debug trace/variable retention has a CSRF-protected tenant-scoped prune API; signed webhook replay prevention and chat-driven conversation/semantic trigger replay prevention persist keys in `workflow_webhook_replay_keys`; scheduled trigger executions are idempotent on persisted `scheduledTaskRunId`; rebuilt services can resolve paused failures and continue due auto-retry nodes from persisted SQL state; and the old standalone debug tracer helper has been removed. Standalone state-machine history is still memory-local when used without a durable sink (`src/server/internal/workflow/executor/state_machine.go:16-23`).

Scope:

- Implement live structured streaming and cancellation.
- Move custom code execution to isolated worker/container with resource/network/FS policy.
- Prove Workflow trace retention/replay, paused-failure recovery, and due auto-retry continuation against target Postgres/deployed services, then attach target workflow telemetry and deployed gRPC smoke.

Acceptance evidence:

- Tool call -> approval -> execution -> trace -> budget survives restart.
- Workflow debugger can replay a previous run after restart.
- Custom Python cannot access host resources outside allowed policy.

### 5. Complete marketplace payout/review lifecycle and observability SLO loop

Problem:

- Marketplace payout now fails closed when no provider is configured and dispatch uses committed payout rows as a transactional outbox/retry ledger, but the full external payout/refund/reconciliation lifecycle still needs target proof.
- Review scanner is static/shallow.
- Request logs can be noop and alert sinks optional (`src/server/internal/observability/request_log.go:45-49`, `src/server/internal/observability/alert_delivery.go:171-181`).

Scope:

- Keep payable settlements fail-closed without a configured provider; preserve payout outbox retry semantics; add target payout webhook/reconciliation/refund/chargeback evidence.
- Add review history, appeals, delist/reinstate, policy versioning, and ranking abuse signals.
- Keep the production request-log sink fail-closed, retain `request_id` as a first-class ClickHouse column, and define SLO alerts from latency/error/cost metrics.

Acceptance evidence:

- Paid install can move through checkout, webhook, settlement hold, external payout, refund/chargeback, and reconciliation.
- Review/appeal actions are auditable.
- Request log -> cost/latency/error -> alert -> delivery -> recovery-audit record works in target runtime.

## P0 Gap Backlog

1. Gateway proxy target-runtime evidence, request-log joins, and true streaming coverage.
2. Target-runtime live Chat provider/request-log/usage evidence.
3. Realtime production prebill/abort settlement/request-log/target-runtime proof after repository-local auth, origin, and usage-capture hardening.
4. Batch target provider/result reconciliation plus request-log/usage/billing join proof.
5. Relay target provider-source price freshness proof and cross-system reconciliation depth beyond the maintenance run ledger plus ledger-vs-snapshot checks.
6. Fine-tuning/Assistants/Threads/Runs commercial lifecycle.
7. RAG upload raw parser replay target-runtime proof.
8. Qdrant target-runtime retrieval debug evidence.
9. Marketplace target external payout provider evidence.
11. Agent sandbox for custom Python.
12. Agent live structured streaming/cancellation.
13. Workflow target Postgres replay proof plus full state-machine/debug trace replay.
14. Workflow durable trace/variable retention prune proof in target runtime.
15. Observability target ClickHouse request-log proof, including request-log to usage/billing joins by `request_id`.
16. Observability alert sink/SLO default.
17. Secret plaintext migration/production deny policy.
18. WebSocket origin enforcement.

## Phase Plan

### Phase 0: Evidence Hygiene

- Add a "runtime evidence required" checklist to PR review.
- Add production-profile guards that fail when demo/fake/local-only paths are reachable.
- Update completion evidence process to reject OpenAPI/tests/matrix-only claims.

### Phase 1: Relay, Chat, Billing Foundation

- Implement real proxy and true streaming.
- Keep chat demo fallback impossible in production.
- Collect target provider-source freshness reports and broaden scheduled reconciliation reports on top of catalog-backed prices, manual/import-sync approval/rollback, the maintenance run ledger, persisted usage snapshots, and the Admin ledger-vs-snapshot check.
- Require request logging and route-decision audit for every Relay request, with `request_id` join evidence across ClickHouse request logs, Relay usage, and billing settlement records.

### Phase 2: RAG Durability

- Prove durable upload enqueue/status and raw parser replay storage under target Postgres/Qdrant worker recovery.
- Keep transactional vector intent outbox coverage pinned by tests while adding ingestion workers.
- Add retrieval debug and citation completeness checks.

### Phase 3: Agent and Workflow Runtime

- Persist all run/step/trace/state-machine events.
- Enforce tool gateway capability.
- Sandbox custom tools.
- Add restart/replay verification.

### Phase 4: Marketplace and Observability

- Prove payout provider lifecycle in target runtime.
- Add review/appeal/delist/recommendation governance.
- Wire request log -> SLO alert -> delivery -> recovery-audit record, with no implied infrastructure mutation.

### Phase 5: Target Runtime Verification

- Run a target environment with real providers configured.
- Execute all 10 vertical slices.
- Record evidence for UI/API/state/restart/tenant/audit/billing/failure diagnosis.

## Forbidden In Future PRs

- Counting mocks, fake providers, fake checkout, fake payout, fake telemetry, or demo generators as product functionality.
- Adding OpenAPI/schema/tests/matrix rows without matching production runtime code.
- Hardcoding model/provider/user/tenant/org IDs, prices, settlement states, provider responses, or success rates in production code.
- Adding endpoints that only return accepted/success without performing the operation.
- Long-running ingestion/execution in request path without durable jobs.
- Request logs, audit logs, or billing ledgers that silently no-op in production.
- Local temp storage for durable product state.
- Bypassing auth, tenant checks, billing, or CSRF for convenience.
- In-memory trace/state history used as the only diagnostic record.
- Exposing planned providers/features as runtime-ready.
