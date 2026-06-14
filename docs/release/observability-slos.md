# Observability SLOs

Phase 23 closes `OPS-04` and `OPS-05` by adding structured logs, Prometheus metrics, OpenTelemetry span hooks, repository-local alert rules, a Grafana dashboard artifact, and SLO definitions for Relay outage, quota settlement failure, webhook failure, migration failure, provider failure, tenant isolation, workflow, RAG, and Agent signals. The Grafana dashboard artifact also covers usage analytics dashboard views for model usage, feature usage, user cost, time trend, cross-dimension usage/cost analytics, workflow execution health, RAG health, and Agent run/tool health. Phase 24 closes the v07 runbook/evidence layer through release, rollback, incident, disaster recovery, and v07 closeout verification. This is not final commercial readiness; v08 Product Completeness remains required.

## Evidence Commands

- `cd src/server && go test ./internal/observability -count=1`
- `cd src/server && go test ./internal/http ./internal/metrics -run 'Observability|Logging|Metrics|Request' -count=1`
- `cd src/server && go test ./internal/relay ./internal/relay/handler -run 'Observability|RouteDecision|ProviderFailure' -count=1`
- `cd src/server && go test ./internal/stripe ./internal/quota ./internal/marketplace ./internal/task ./cmd/migrate -run 'Observability|Metrics|Failure|Webhook|Settlement|Migration' -count=1`
- `GOCACHE=/tmp/oblivious-go-cache GOMODCACHE=/tmp/oblivious-go-mod-cache bash scripts/verify-commercial-db-evidence.sh observability-alert-recovery-persistence`
- `bash scripts/check.sh docs`

## Signal Map

| Area | OPS | Metrics | Structured event or span | Dashboard panel | Owner |
| --- | --- | --- | --- | --- | --- |
| HTTP requests | OPS-04 | `http_requests_total`, `http_request_duration_seconds` | `http.request`, span `http.request` | HTTP error rate, HTTP latency p95 | platform-ops |
| Relay decisions | OPS-04 | `relay_route_decisions_total` | `relay.route_decision`, span `relay.route_policy` | Relay route decisions | relay-ops |
| Provider failures | OPS-04 | `provider_failures_total`, `provider_request_duration_seconds` | `relay.provider_failure`, span `relay.provider_call` | Provider failures, Provider latency p95 | relay-ops |
| Billing lifecycle | OPS-04 | `billing_lifecycle_events_total` | span `billing.lifecycle` | Billing lifecycle | billing-ops |
| Quota settlement | OPS-04 | `quota_settlement_failures_total` | spans `quota.preconsume`, `quota.settlement`, `quota.refund` | Quota settlement failures | billing-ops |
| Stripe webhooks | OPS-04 | `stripe_webhook_events_total`, `stripe_webhook_failures_total` | span `stripe.webhook` | Stripe webhook failures | billing-ops |
| Marketplace settlement | OPS-04 | `marketplace_settlement_events_total` | spans `marketplace.paid_install_completed`, `marketplace.refund`, `marketplace.payout_pending` | Marketplace settlement | marketplace-ops |
| Jobs/tasks | OPS-04 | `job_events_total` | spans `job.task_start`, `job.task_approve`, `job.task_pause`, `job.task_resume`, `job.task_cancel` | Task and job events | platform-ops |
| Workflow executions | OPS-04 | `workflow_execution_total`, `workflow_execution_duration_seconds`, `workflow_execution_active`, `workflow_execution_active_age_seconds`, `workflow_node_error_rate` | workflow execution and node lifecycle records | Workflow active executions, Workflow active execution age | workflow-ops |
| RAG retrieval | OPS-04 | `rag_document_processing_duration_seconds`, `rag_retrieval_latency_seconds`, `rag_chunk_count` | RAG document processing and retrieval records | RAG retrieval latency, RAG document processing duration, RAG chunk count | rag-ops |
| Agent runs | OPS-04 | `agent_run_total`, `agent_tool_call_total`, `agent_iteration_count` | Agent run and tool lifecycle records | Agent run total, Agent tool call total, Agent iteration count | agent-ops |
| Migrations | OPS-04 | `migration_runs_total` | span `migration.apply` | Migration runs | platform-ops |
| Tenant isolation | OPS-05 | `http_requests_total{status_class="4xx"}` plus tenant-isolation test evidence | `http.request` with `organization_id` and `user_id` when known | Tenant isolation incident signal | security-ops |

## Alert Definitions

### Relay Outage

- Alert: `RelayOutage`
- Metric: `relay_route_decisions_total`
- Threshold: allowed route decisions remain at zero for 10 minutes.
- Severity: critical
- Owner: platform-ops
- Runbook: `docs/release/incident-response-runbook.md` for Relay outage triage and `docs/release/release-rollback-runbook.md` for rollback decision.

### Quota Settlement Failure

- Alert: `QuotaSettlementFailure`
- Metric: `quota_settlement_failures_total`
- Threshold: any preauthorization, settlement, or refund failure for 5 minutes.
- Severity: critical
- Owner: billing-ops
- Runbook: `docs/release/incident-response-runbook.md` for quota ledger reconciliation, idempotency check, failed-session remediation, and rollback trigger.

### Webhook Failure

- Alert: `StripeWebhookFailure`
- Metrics: `stripe_webhook_failures_total`, `stripe_webhook_events_total`
- Threshold: any invalid signature, ledger write, or lifecycle apply failure for 5 minutes.
- Severity: critical
- Owner: billing-ops
- Runbook: `docs/release/incident-response-runbook.md` for Stripe signing secret check, raw body handling, and idempotency ledger replay.

### Migration Failure

- Alert: `MigrationFailure`
- Metric: `migration_runs_total`
- Threshold: any migration failure within 30 minutes.
- Severity: critical
- Owner: platform-ops
- Runbook: `docs/release/release-rollback-runbook.md` for release stop/rollback and `docs/release/disaster-recovery-runbook.md` for restore when data recovery is required.

### High Provider Error Rate

- Alert: `HighProviderErrorRate`
- Metrics: `provider_failures_total`, `provider_request_duration_seconds_count`
- Threshold: provider failure rate above 5 percent for 10 minutes.
- Severity: warning
- Owner: relay-ops
- Runbook: `docs/release/incident-response-runbook.md` for provider/channel mitigation and Relay-only provider access checks.

### Tenant Isolation Incident

- Alert: `TenantIsolationIncident`
- Metric: `http_requests_total{status_class="4xx"}`
- Threshold: elevated 4xx denial volume for 10 minutes.
- Severity: critical
- Owner: security-ops
- Runbook: `docs/release/incident-response-runbook.md` for tenant/user request inspection, cross-tenant denial tests, and access freeze handling.

### Workflow Execution Failure Rate

- Alert: `WorkflowExecutionFailureRate`
- Metrics: `workflow_execution_total{status=~"failed|timeout|max_iterations"}`, `workflow_execution_total`
- Threshold: failed, timed out, or max-iteration terminal executions exceed 10 percent for 5 minutes with at least 10 executions in the window.
- Severity: warning
- Owner: workflow-ops
- Runbook: `docs/release/incident-response-runbook.md` for workflow failure triage, node execution inspection, and release rollback decision.

### Workflow Execution Stuck

- Alert: `WorkflowExecutionStuck`
- Metrics: `workflow_execution_active{status="running"}`, `workflow_execution_active_age_seconds{status="running"}`
- Threshold: at least one running execution exists and the oldest running execution age exceeds 3,600 seconds for 10 minutes.
- Severity: warning
- Owner: workflow-ops
- Runbook: `docs/release/incident-response-runbook.md` for active execution triage, concurrency/resource-limit checks, and pause/cancel/retry decisions.

### Workflow Execution Duration High

- Alert: `WorkflowExecutionDurationHigh`
- Metric: `workflow_execution_duration_seconds`
- Threshold: completed workflow execution p95 duration exceeds 3,600 seconds for 10 minutes.
- Severity: warning
- Owner: workflow-ops
- Runbook: `docs/release/incident-response-runbook.md` for slow execution triage, node timing inspection, and retry/cancel decisions.

### Workflow Queue Backlog

- Alert: `WorkflowQueueBacklog`
- Metrics: `workflow_execution_active{status="queued"}`, `workflow_execution_active_age_seconds{status="queued"}`
- Threshold: at least one queued execution exists and the oldest queued execution age exceeds 300 seconds for 10 minutes.
- Severity: warning
- Owner: workflow-ops
- Runbook: `docs/release/incident-response-runbook.md` for queued execution backlog triage and concurrency policy inspection.

### Workflow Node Error Rate High

- Alert: `WorkflowNodeErrorRateHigh`
- Metric: `workflow_node_error_rate`
- Threshold: any low-cardinality workflow node type reports an error rate above 10 percent for 10 minutes.
- Severity: warning
- Owner: workflow-ops
- Runbook: `docs/release/incident-response-runbook.md` for node-specific failure triage, input replay, and rollback decisions.

### RAG Retrieval Slowness

- Alert: `RAGRetrievalSlowness`
- Metric: `rag_retrieval_latency_seconds`
- Threshold: RAG retrieval p95 latency exceeds 2 seconds for 5 minutes.
- Severity: warning
- Owner: rag-ops
- Runbook: `docs/release/incident-response-runbook.md` for vector store, hybrid retrieval, and embedding dependency triage.

### Qdrant Down

- Alert: `QdrantDown`
- Metric: `up{job="qdrant"}`
- Threshold: the Qdrant target is down or absent for 1 minute.
- Severity: critical
- Owner: rag-ops
- Runbook: `docs/release/incident-response-runbook.md` for vector store availability checks and restore/escalation decisions.

### Relay Semantic Cache Low Hit Rate

- Alert: `RelaySemanticCacheLowHitRate`
- Metric: `relay_semantic_cache_events_total`
- Threshold: semantic cache hit rate remains below 20 percent for an active API type and model over 15 minutes.
- Severity: warning
- Owner: relay-ops
- Runbook: `docs/release/incident-response-runbook.md` for cache-key, embedding, and provider-routing inspection.

### Agent Run Failure Rate

- Alert: `AgentRunFailureRate`
- Metric: `agent_run_total`
- Threshold: failed, max-iteration, or token-budget-exceeded Agent runs exceed 10 percent for 5 minutes with at least 10 runs in the window.
- Severity: warning
- Owner: agent-ops
- Runbook: `docs/release/incident-response-runbook.md` for Agent execution, model, tool, and policy triage.

### Agent Tool Call Failure Rate

- Alert: `AgentToolCallFailureRate`
- Metric: `agent_tool_call_total`
- Threshold: failed or rejected Agent tool calls exceed 10 percent for 5 minutes with at least 10 tool calls in the window.
- Severity: warning
- Owner: agent-ops
- Runbook: `docs/release/incident-response-runbook.md` for tool reliability, approval policy, and MCP/provider triage.

### Agent Iteration Count High

- Alert: `AgentIterationCountHigh`
- Metric: `agent_iteration_count`
- Threshold: Agent run p95 iteration count exceeds 13 for 10 minutes.
- Severity: warning
- Owner: agent-ops
- Runbook: `docs/release/incident-response-runbook.md` for runaway loop, prompt, tool-choice, and budget policy triage.

## Artifact Map

- Alert rules: `deploy/observability/prometheus-alerts.yaml`
- Grafana dashboard: `deploy/observability/grafana-dashboard.json`, including usage/cost analytics panels for model usage, feature usage, user cost, time trend, cross-dimension usage/cost analytics, workflow execution health, RAG health, and Agent run/tool health.
- Release and rollback runbook: `docs/release/release-rollback-runbook.md`
- Incident response runbook: `docs/release/incident-response-runbook.md`
- Disaster recovery runbook: `docs/release/disaster-recovery-runbook.md`
- Quality gate: `scripts/verify-quality-gates.sh`
- Commercial gate boundary: `docs/release/commercial-gates.md`
- Release checklist reference: `docs/release/rc-checklist.md`

## Boundary

`OPS-04` and `OPS-05` are repository-local evidence gates. They do not prove that a Prometheus server, Grafana instance, OpenTelemetry collector, or external error-tracking vendor is deployed. Usage/cost analytics and workflow active execution dashboard coverage in `deploy/observability/grafana-dashboard.json` proves repository-local panel intent only; importing it into an external Grafana workspace and connecting live data sources remains deployment-specific evidence. `workflow_execution_active_age_seconds` is the current oldest active execution age per low-cardinality status (`running`, `queued`, `paused`); it must not add execution IDs or other high-cardinality labels. Phase 24 runbook and release/rollback evidence closes the v07 Operations Gate while still recording those external runtime integrations as deployment-specific checks when unavailable.
