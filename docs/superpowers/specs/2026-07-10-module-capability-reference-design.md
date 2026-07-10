# Oblivious Module Capability and Reference Design

> Status: Approved module boundary design
>
> Date: 2026-07-10
>
> Scope: Defines what every Oblivious module must provide at the launch baseline and at the complete commercial target. Reference projects provide capability depth and interaction patterns only; their source code or architecture is not Oblivious implementation evidence.

## 1. Goal

Oblivious is a multi-tenant AI SaaS platform combining:

- LobeHub-style customer-facing AI interaction;
- New-API-style provider, routing, quota, billing, and operator control;
- durable Agent, Workflow, Task, Knowledge RAG, and Marketplace execution;
- production deployment, security, observability, recovery, and release evidence.

The platform must support an incremental path:

1. A commercially usable launch baseline on the existing monolith or dual-mode runtime.
2. A complete commercial target with independent services, target-environment evidence, mature external integrations, and operational closure.

## 2. Design Decision

### 2.1 Selected approach: layered reference fusion

Each module has:

- one or two primary reference projects for its core product model;
- secondary references for specialized capabilities;
- Oblivious-specific contracts that references cannot override.

The implementation must not clone a single project wholesale. Reference projects are used to identify expected capability depth, user journeys, failure behavior, and operational controls.

### 2.2 Rejected approaches

#### Clone one reference project per module

This is initially fast but creates incompatible identities, state models, billing semantics, UI conventions, and deployment assumptions.

#### Implement the full target architecture before launch

This creates unnecessary migration and distributed-systems risk before the core customer journeys are proven.

## 3. Platform-Wide Invariants

### 3.1 Relay authority

All billable AI operations from Chat, Agent, Workflow, Knowledge, MCP, and supported OpenAI-compatible endpoints must pass through Relay.

Direct provider calls outside Relay and provider adapter packages are prohibited.

### 3.2 Tenant isolation

All customer-owned state must carry an organization identity. Authorization must be enforced in:

- HTTP and gRPC boundaries;
- service methods;
- SQL queries and mutations;
- vector-store payloads and filters;
- background jobs and retry queues;
- audit, usage, and observability queries.

### 3.3 Shared evidence identity

The following identifiers must remain joinable across the system:

- `organization_id`
- `user_id`
- `request_id`
- `conversation_id`
- `run_id`
- `execution_id`
- `usage_id`
- `trace_id`
- `payment_id`
- `settlement_id`

The request, execution, usage, billing, request-log, and audit records must not become disconnected evidence islands.

### 3.4 Fail-closed behavior

Unsupported providers, incomplete lifecycle APIs, missing prices, missing sandboxes, missing payment rails, and missing production observability backends must fail explicitly.

Demo generators, mock providers, fixture payments, local payout fallbacks, and simulated telemetry must never be enabled as commercial behavior.

### 3.5 Evidence classes

Every completion claim must identify its evidence class:

1. fixture or unit evidence;
2. repository-local runtime evidence;
3. target-environment runtime evidence;
4. final no-skip commercial release evidence.

Lower evidence classes cannot substitute for higher ones.

## 4. Module Capability Matrix

## 4.1 Identity, Tenant, and Access

### Launch baseline

- registration, login, logout, and session refresh;
- password policy, login throttling, and secure password reset;
- organization, membership, workspace, and invitation lifecycle;
- owner, admin, member, and viewer authorization boundaries;
- active-organization selection and tenant-scoped queries;
- CSRF, secure cookies, session rotation, and session revocation;
- tenant-scoped audit records for security-sensitive operations.

### Complete commercial target

- MFA and recovery codes;
- OIDC and SAML enterprise login;
- SCIM provisioning and deprovisioning;
- configurable role and permission matrix;
- enterprise domain verification;
- access review and dormant-account policies;
- organization-level security and retention controls.

### References

- Primary: `open-webui`, `LibreChat`
- Secondary: `dify`, `sub2api`, `new-api`

### Oblivious boundary

Authentication breadth is less important than complete tenant isolation. No enterprise identity feature may bypass organization-scoped authorization.

## 4.2 API Gateway

### Launch baseline

- unified external HTTP entry;
- authentication and tenant context injection;
- CORS, CSRF, panic recovery, request size, and rate-limit middleware;
- request ID and trace context propagation;
- route registration and route-surface verification;
- health, readiness, and metrics endpoints;
- trusted internal identity propagation to Relay and service handlers.

### Complete commercial target

- service discovery and independent service routing;
- health-aware traffic shifting;
- canary and staged rollout policies;
- circuit breaking and upstream timeout policy;
- WAF and abuse controls;
- multi-region routing;
- Envoy or equivalent extension integration.

### References

- Primary: `ai-gateway`, `gateway`
- Secondary: `llmgateway`, `Bifrost`, `one-api`

### Oblivious boundary

The Gateway owns external traffic policy. It must not duplicate Relay provider routing or billing logic.

## 4.3 Relay and Provider Runtime

### Launch baseline

- provider and channel registry;
- encrypted provider credentials;
- model inventory and capability metadata;
- weighted, priority, and health-aware route selection;
- retry and fallback with route-decision audit;
- API-token and session authorization;
- quota preauthorization, settlement, and refund;
- authoritative provider usage capture;
- streaming proxy with client cancellation;
- SQL-backed price catalog and immutable request price snapshots;
- usage, latency, provider error, and request-log persistence;
- explicit production disablement for unsupported lifecycle APIs.

### Complete commercial target

- production Realtime lifecycle;
- Batch submission, polling, settlement, refund, and reconciliation;
- Files mapping, tombstone, retention, and audit lifecycle;
- provider price synchronization and approval workflow;
- semantic cache governance;
- regional routing and provider failover;
- full provider-export, usage, billing, and request-log reconciliation;
- controlled support for additional OpenAI-compatible lifecycle APIs.

### References

- Primary: `new-api`
- Secondary: `LiteLLM`, `Bifrost`, `one-api`, `Helicone`, `sub2api`

### Oblivious boundary

An endpoint is commercial only when identity, quota, settlement, request logging, audit, failure compensation, and target evidence are all present.

## 4.4 Chat

### Launch baseline

- persistent conversations and messages;
- model and generation configuration;
- system prompt and persona support;
- true SSE response streaming;
- retry, regenerate, edit, fork, export, and share;
- Knowledge binding and citations;
- draft preservation and recoverable errors;
- quota, Relay, and provider failure presentation;
- authoritative usage recording where the provider supplies usage;
- conversion from a conversation into a SOLO or Task draft.

### Complete commercial target

- attachments and multimodal messages;
- multi-model comparison;
- conversation search and topic management;
- message branching and rollback;
- Artifacts and structured outputs;
- voice input and output;
- cross-device synchronization;
- desktop and PWA behavior;
- richer trace and provider-debug views.

### References

- Primary: `lobe-chat`, `lobehub`
- Secondary: `NextChat`, `open-webui`, `LibreChat`

### Oblivious boundary

Chat is a customer surface over Relay. It must not own a second provider runtime or independent billing model.

## 4.5 Knowledge and RAG

### Launch baseline

- knowledge-base and document lifecycle;
- durable upload and ingestion jobs;
- parser, chunking, embedding, and index stages;
- retry, lease reclaim, dead-letter, and status inspection;
- raw upload replay or external object reference;
- pgvector or Qdrant indexing under tenant scope;
- vector, keyword, hybrid, and rerank retrieval;
- source citations with document, version, page, and chunk metadata;
- document update, delete, reindex, and stale-vector cleanup;
- retrieval test cases and operator diagnostics.

### Complete commercial target

- OCR and DeepDoc document understanding;
- configurable parser and chunk strategies;
- large-file and batch ingestion;
- external object storage;
- knowledge graph and entity extraction;
- version-aware retrieval;
- retrieval quality evaluation;
- advanced debug and relevance explanations;
- index migration and multi-vector-store support.

### References

- Primary: `ragflow`
- Secondary: `MaxKB`, `FastGPT`, `dify`, `anything-llm`, `open-webui`

### Oblivious boundary

RAG claims require actual embedding-backed retrieval and citations. Text search or fixture vectors cannot be marketed as complete RAG.

## 4.6 Agent and SOLO

### Launch baseline

- Agent definition, version, model, prompt, and tool configuration;
- persistent conversations, messages, runs, and tool runs;
- ReAct and plan-based execution;
- tool schema validation;
- approval, rejection, retry, and cancellation;
- budget and iteration limits;
- memory search and injection evidence;
- Knowledge and MCP access boundaries;
- durable plan steps and operator-visible status;
- structured provider usage and Relay billing;
- fail-closed behavior when structured tool execution is unavailable.

### Complete commercial target

- live structured tool-call streaming;
- sub-agent execution;
- team and supervisor patterns;
- dynamic model routing;
- skill selection;
- pause and resume across restarts;
- human takeover;
- richer trace, artifact, and evaluation views;
- distributed Agent workers and capacity policies.

### References

- Primary: `coze-studio`, `LibreChat`
- Secondary: `dify`, `FastGPT`, `open-webui`, `lobehub`

### Oblivious boundary

Agent state, tool state, memory evidence, usage, and billing must be part of one coherent execution record.

## 4.7 Workflow

### Launch baseline

- workflow CRUD and versioning;
- typed node registry and node validation;
- manual, webhook, schedule, conversation, and semantic triggers;
- graph validation and deterministic execution ordering;
- persistent workflow and node execution state;
- retry, failure branch, pause, resume, skip, and cancel;
- durable execution events, trace entries, and variable snapshots;
- signed webhook replay prevention;
- debug snapshot and state replay;
- tenant-scoped retention and pruning;
- resource and concurrency controls.

### Complete commercial target

- versioned adapter migration;
- branch experiments and traffic allocation;
- live visual debugger;
- execution replay from checkpoints;
- distributed workers;
- long-running compensation and Saga behavior;
- workflow templates and Marketplace distribution;
- production success-rate telemetry and SLO enforcement.

### References

- Primary: `dify`, `coze-studio`
- Secondary: `Flowise`, `FastGPT`, `MaxKB`, `ragflow`

### Oblivious boundary

Workflow definitions are not complete unless execution, failure, replay, and observability survive process restart.

## 4.8 Task and Scheduler

### Launch baseline

- task definition with target, budget, authorization, and status;
- scheduled task definition and Cron validation;
- due-task claim with idempotency;
- run history and error state;
- manual run and enable or disable controls;
- actual invocation of Agent or Workflow targets;
- retry and next-run calculation;
- tenant-scoped scheduler ownership.

### Complete commercial target

- distributed scheduling;
- priority and dependency graphs;
- resource reservations;
- compensation and dead-letter workflows;
- long-running orchestration;
- leader election and partition recovery;
- scheduling SLOs and backlog telemetry.

### References

- Primary: `coze-studio`, `dify`
- Secondary: `FastGPT`

### Oblivious boundary

A task is not complete if it only simulates progress. Starting a task must invoke a real Agent or Workflow execution and preserve the resulting identity.

## 4.9 MCP and Tool Platform

### Launch baseline

- built-in and custom tool registry;
- JSON Schema input and output contracts;
- commercial enablement policy;
- approval and risk classification;
- timeout, cancellation, output limits, and audit;
- MCP server registration and health;
- tenant-scoped credentials;
- network tools disabled unless explicitly configured;
- versioned tool metadata.

### Complete commercial target

- managed MCP server lifecycle;
- tool and skill Marketplace;
- OAuth-backed tool authorization;
- remote Tool Gateway;
- policy packs and tenant-level allowlists;
- tool version migration;
- tool health, latency, and cost telemetry.

### References

- Primary: `open-webui`, `LibreChat`
- Secondary: `dify`, `FastGPT`, `Bifrost`, `Flowise`

### Oblivious boundary

Tool availability must be policy-driven. Registering a tool in code must not automatically expose it to commercial Agents.

## 4.10 Sandbox and Custom Execution

### Launch baseline

- isolated container execution;
- network disabled by default;
- CPU, memory, PID, and timeout limits;
- read-only root filesystem and temporary work directory;
- non-root execution and dropped capabilities;
- source, argument, output, and artifact limits;
- cancellation classification;
- execution context and audit metadata;
- retained stdout, stderr, exit status, and truncation evidence.

### Complete commercial target

- multi-language execution pools;
- persistent artifacts;
- dependency and image caching;
- approved outbound network policy;
- queueing, autoscaling, and capacity governance;
- malware scanning;
- notebook-style execution sessions.

### References

- Primary: `FastGPT`, `open-webui`
- Secondary: `Flowise`, `MaxKB`

### Oblivious boundary

Host process execution is never a production fallback. Missing sandbox capacity must fail closed.

## 4.11 Billing, Quota, and Payments

### Launch baseline

- plan, package, balance, and quota lifecycle;
- token and request accounting;
- preauthorization, settlement, and refund;
- idempotency and transactional ledger changes;
- Stripe checkout and signed webhook ledger;
- subscriptions, invoices, top-ups, refunds, and failed-payment states;
- immutable price snapshots;
- admin inspection and reconciliation views.

### Complete commercial target

- native Alipay and WeChat Pay lifecycle;
- provider refund and reconciliation jobs;
- chargeback and dispute handling;
- provider price synchronization;
- cost, revenue, margin, and anomaly analytics;
- tax and invoice integrations;
- cross-system financial reconciliation.

### References

- Primary: `new-api`, `sub2api`
- Secondary: `LiteLLM`, `CPA-Manager`, `one-api`

### Oblivious boundary

Payment intent state, provider webhook state, quota state, usage state, and Marketplace settlement state must reconcile without manual database edits.

## 4.12 Marketplace

### Launch baseline

- publish, version, review, approve, reject, and request-changes flow;
- search, category, tag, detail, install, and uninstall;
- free and paid install;
- order, settlement, platform fee, and publisher revenue state;
- reviews, abuse reports, takedown, appeal, and reinstatement;
- payout state and transactional dispatch outbox;
- audit history and operator inspection.

### Complete commercial target

- external payout provider integration;
- payout webhook and reconciliation;
- refund and chargeback impact;
- ranking and recommendation;
- risk and policy scanning;
- reviewer evidence workflow;
- creator analytics;
- configurable settlement cycles and fee tiers.

### References

- Primary: `coze-studio`, `dify`
- Secondary: `lobe-chat`, `Flowise`, `FastGPT`, `lobehub`

### Oblivious boundary

Local payout simulation is prohibited. Paid Marketplace operation requires external rail, reconciliation, refund, and operator evidence.

## 4.13 Multi-Channel Publishing

### Launch baseline

- channel configuration and credential storage;
- message format conversion;
- signed outbound and inbound webhooks;
- delivery log and retry;
- idempotency and duplicate suppression;
- channel health and failure visibility;
- tenant-scoped publishing permissions.

### Complete commercial target

- WeChat, Feishu, DingTalk, Slack, Discord, Telegram, and additional adapters;
- synchronized conversation state;
- channel failover;
- inbound command and event routing;
- media conversion;
- channel-specific analytics and limits.

### References

- Primary: `coze-studio`
- Secondary: `dify` and official channel SDKs

### Oblivious boundary

Publishing must use the shared Chat, Agent, Workflow, identity, audit, and usage models rather than creating independent channel silos.

## 4.14 Admin and Customer Console

### Launch baseline

- users and organizations;
- channels, models, routes, and prices;
- plans, packages, billing, and refunds;
- API tokens and usage logs;
- audit logs;
- Marketplace review queue;
- alerts and operational settings;
- customer models, usage, billing, access, and notification pages;
- explicit supported, planned, configurable, and runtime-ready provider states.

### Complete commercial target

- bulk operations;
- cost and margin analysis;
- provider health and quality scoring;
- support impersonation with audit;
- price import approval and rollback;
- reconciliation control center;
- tenant risk and security dashboards;
- configurable operational policy.

### References

- Primary: `new-api`, `CPA-Manager`
- Secondary: `llmgateway`, `open-webui`, `Cli-Proxy-API-Management-Center`, `sub2api`

### Oblivious boundary

An Admin page is not evidence that the backing operation or integration exists. UI actions must map to real, tenant-safe service behavior.

## 4.15 Observability and Audit

### Launch baseline

- structured logs;
- request, run, execution, and trace identifiers;
- Prometheus metrics;
- ClickHouse request logs;
- latency, status, provider, model, token, and cost dimensions;
- alert state, routing, delivery attempts, and failure history;
- request-log to usage and billing joins;
- recovery audit records;
- operator-visible SLO proof.

### Complete commercial target

- distributed tracing backend;
- alert grouping, deduplication, inhibition, and escalation;
- provider and route quality scoring;
- anomaly detection;
- automated but reviewable remediation;
- retention and access policies;
- multi-region dashboards and SLOs.

### References

- Primary: `Helicone`
- Secondary: `Bifrost`, `LiteLLM`, `CPA-Manager`

### Oblivious boundary

No-op sinks, in-memory reporters, and fixture dashboards do not count as production observability.

## 4.16 Deployment, Security, and Recovery

### Launch baseline

- reproducible Docker images;
- Docker Compose runtime validation;
- Kubernetes manifests and rollout validation;
- external secret injection;
- network policies;
- migration and rollback controls;
- health, readiness, and smoke checks;
- PostgreSQL backup and restore verification;
- release, rollback, incident, and disaster-recovery runbooks;
- dependency and secret scanning;
- WebSocket origin and webhook signature enforcement.

### Complete commercial target

- high availability and multi-region deployment;
- autoscaling based on service metrics;
- regular restore and disaster-recovery drills;
- automated secret rotation;
- signed artifacts and supply-chain attestations;
- penetration testing;
- compliance controls and evidence retention;
- capacity, load, and long-duration testing.

### References

- Primary: `ai-gateway`, `Helicone`
- Secondary: `ragflow`, `dify`, `Bifrost`, `litellm`

### Oblivious boundary

Repository-local manifests and local-cluster smoke are necessary but do not prove target production readiness.

## 4.17 Microservices, gRPC, and Events

### Launch baseline

- monolith remains fully runnable;
- twelve service ownership boundaries are documented;
- database table ownership is explicit;
- gRPC contracts are versioned;
- service entrypoints and deployment manifests exist;
- migration compatibility is verified;
- dual-mode operation does not change customer behavior.

### Complete commercial target

- functional parity between monolith and independent services;
- database-per-service deployment;
- Kafka or equivalent transactional outbox events;
- cross-service idempotency;
- Saga and compensation behavior;
- service-level scaling and failure isolation;
- distributed trace continuity;
- removal of direct cross-service database access.

### References

- Combined reference: `coze-studio`, `dify`, `ragflow`, `ai-gateway`

### Oblivious boundary

No single reference project defines the target. Oblivious must preserve Relay authority, unified usage and billing, tenant isolation, and shared execution evidence across service boundaries.

## 5. Delivery Priority

## 5.1 P0: launch closure

1. Shared request, run, usage, billing, and trace evidence spine.
2. Real Relay and Chat provider lifecycle proof.
3. Durable RAG worker startup and target recovery.
4. Real Task-to-Agent or Task-to-Workflow execution.
5. SSRF protection for custom API tools, Workflow HTTP nodes, webhooks, and payout endpoints.
6. Real payment, refund, reconciliation, and Marketplace payout proof.
7. Persistent target observability and request-log joins.
8. Real browser-to-backend-to-database end-to-end tests.
9. Final target evidence workflow and no-skip verifier.

## 5.2 P1: commercial maturity

1. Workflow replay and distributed worker maturity.
2. Agent structured streaming and restart continuation.
3. Provider price synchronization and reconciliation.
4. Marketplace policy, ranking, and creator operations.
5. Enterprise identity and fine-grained authorization.
6. Full service parity for independent deployments.
7. Coverage, race, static analysis, and broader browser gates.

## 5.3 P2: expansion

1. Additional lifecycle-compatible Relay APIs.
2. Multi-agent teams.
3. Knowledge graph and advanced RAG.
4. More publishing channels.
5. Desktop, PWA, and additional clients.
6. Multi-region and compliance expansion.

## 6. Acceptance Criteria

The design is implemented only when:

- each launch-baseline capability has production runtime code;
- each customer-visible action reaches real state or a real external integration;
- unsupported functions are explicitly disabled;
- tenant isolation is proven across API, service, database, vector, and job layers;
- request, usage, billing, trace, and audit evidence can be joined;
- local full-stack and target-environment evidence both pass;
- the final commercial verifier runs without environment skips;
- tracked documentation describes current behavior rather than intended behavior.

## 7. Reference Guardrails

- Do not count `reference/` source as Oblivious implementation.
- Do not copy architecture without reconciling tenant, Relay, billing, and evidence contracts.
- Do not use README claims as implementation proof.
- Prefer concrete source paths and runtime flows over feature lists.
- Record capabilities that should remain intentionally unsupported.
- Promote a capability to complete only after target evidence, not after adding an API schema or fixture test.
