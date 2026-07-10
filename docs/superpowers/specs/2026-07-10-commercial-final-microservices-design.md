# Oblivious 商业终版全微服务设计

> Status: Approved design
>
> Date: 2026-07-10
>
> Decision: All required services reach commercial parity before one production cutover.

## 1. Purpose

This specification defines the target required before Oblivious may be released as a functionally complete commercial product.

The release must simultaneously support:

- a global hosted SaaS deployment;
- a China hosted SaaS deployment;
- a fully self-hosted Kubernetes deployment, including an air-gapped profile;
- domestic and international model providers, payments, channels, and operating controls;
- independent microservices with no production dependency on the legacy aggregate server.

There are no production users or historical production data to migrate. The legacy aggregate runtime remains a behavior reference during development, but it is not part of the final production topology.

## 2. Binding Decisions

The following decisions are approved and binding:

1. The production release is a big-bang switch to the complete microservice system.
2. Development and integration remain continuous; big-bang does not mean first integration at the end.
3. Seventeen target services are organized into four domain units.
4. Every service owns its process, API, database, credentials, migrations, deployment, SLO, alerts, backup, and recovery evidence.
5. Direct cross-service database access, shared business tables, shared database credentials, and cross-database joins are prohibited.
6. Relay is the only authority allowed to hold shared AI provider credentials or perform billable AI provider calls.
7. Billing owns quota reservations and financial truth but may not re-estimate provider usage.
8. Global SaaS, China SaaS, and self-hosted use the same source, API semantics, image digests, migration set, and Helm interfaces.
9. Regional and deployment differences are implemented through certified adapters and capability profiles, not business-code forks.
10. Unsupported or unproven capabilities fail closed and are absent from commercial claims.

This specification supersedes the service-boundary portions of `docs/architecture/ADR-012-microservices-boundaries.md` and the dual-write/gray-migration strategy in the June 2026 fusion design. A replacement ADR must be created before implementation begins.

## 3. Scope Control

### 3.1 V1 scope rule

V1.0 includes launch-required and commercial-complete capabilities. Optional enhancements do not enter the V1 API, navigation, default configuration, pricing promises, or sales materials.

A capability enters V1 only when it closes at least one of:

- a primary golden journey;
- a P0 security, correctness, durability, or compliance risk;
- a signed customer requirement;
- a legal, tax, accounting, or regulatory requirement;
- a mandatory deployment or operations contract.

Reference projects identify expected depth and failure behavior. They do not define the backlog and do not count as Oblivious evidence.

### 3.2 Golden-journey limit

Each product module has at most three primary V1 golden journeys. Every module must deliver five surfaces together:

- user surface;
- operator surface;
- recovery surface;
- permission surface;
- audit and evidence surface.

Shell pages, health-only services, fixture success paths, demo fallbacks, and API-only scaffolds do not satisfy scope completion.

## 4. Target Service Architecture

### 4.1 Domain units

The domain units are organizational and operational groupings. They do not merge services or databases.

| Domain unit | Services |
|---|---|
| Foundation | `identity-access`, `api-gateway`, `event-contract-platform`, `platform-ops`, `observability-audit` |
| AI Runtime | `relay`, `knowledge-rag`, `tool-mcp`, `sandbox` |
| Product | `chat`, `agent`, `workflow`, `task-scheduler`, `channel` |
| Commerce | `billing-payment`, `marketplace`, `admin-console` |

### 4.2 Service ownership

| Service | Authoritative responsibility and data |
|---|---|
| `identity-access` | Users, organizations, memberships, roles, invitations, sessions, API credentials, OIDC, SAML, SCIM, service identity policy |
| `api-gateway` | External HTTP/SSE/WebSocket ingress, trusted identity propagation, edge rate limits, routing, circuit state, request policy |
| `event-contract-platform` | Event envelope, schema registry, topic catalog, compatibility checks, DLQ and redrive requests |
| `platform-ops` | Release manifests, deployments, backup/restore jobs, DR exercises, upgrade evidence, compliance results |
| `observability-audit` | Request-log projections, ClickHouse analytics, traces, audit records, alerts, incidents, SLO evidence |
| `relay` | Provider registry, model capability catalog, AI routing, pricing snapshots, AI request IDs, authoritative measured usage, Realtime, Batch, Files |
| `knowledge-rag` | Knowledge bases, documents, versions, chunks, ingestion jobs, index jobs, retrieval tests, vector namespace, object prefix |
| `tool-mcp` | Tool registry, MCP servers, tool schemas, OAuth grants, credential references, network and approval policy |
| `sandbox` | Isolated executions, worker leases, resource policy, logs, artifacts, cancellation, retention |
| `chat` | Conversations, messages, branches, shares, attachment metadata, knowledge and tool bindings |
| `agent` | Agent definitions and versions, runs, plans, tool runs, memory references, approval state |
| `workflow` | Workflow definitions and versions, triggers, executions, node executions, snapshots, replay and compensation state |
| `task-scheduler` | Tasks, schedules, claims, target execution IDs, retry state, run history, misfire policy |
| `channel` | Channel accounts, inbound/outbound messages, webhook deduplication, delivery attempts, receipts, channel secret references |
| `billing-payment` | Metering intake, rating, quota, reservations, subscriptions, invoices, payments, refunds, disputes, immutable financial ledger |
| `marketplace` | Listings, versions, reviews, installs, entitlements, order intent, seller compliance references, settlement and payout obligations |
| `admin-console` | Customer administration, platform policy, approval requests, operating work items, break-glass controls, configuration overrides |

### 4.3 Database-per-service contract

Each service has:

- one logical database;
- one database owner role;
- one migration directory and checksum ledger;
- one backup and restore owner;
- one schema fingerprint in the release evidence bundle.

Self-hosted deployments may place multiple logical databases on one PostgreSQL cluster, but credentials and databases remain isolated. SaaS may use shared physical clusters within a domain unit, provided logical ownership, credentials, backup, and capacity are isolated.

Admin and Observability may build read models from events. They may not query or mutate another service's source database.

## 5. Communication and Consistency

### 5.1 External and internal protocols

- External customer and operator APIs use versioned HTTP, SSE, and WebSocket contracts through `api-gateway`.
- `/v1/*` AI-compatible routes always terminate at `relay`.
- Internal synchronous communication uses versioned gRPC.
- Long-running operations return an operation identifier and complete through durable state plus events.
- Kafka-compatible event delivery is at-least-once.
- Every producer uses a transactional outbox; every consumer uses an idempotent inbox.
- Distributed two-phase commit is prohibited.

### 5.2 Shared event envelope

Every HTTP request, gRPC request, and event propagates:

```text
tenant_id
principal_id
request_id
trace_id
correlation_id
causation_id
idempotency_key
region
deployment_id
occurred_at
schema_version
```

AI and execution flows additionally propagate:

```text
conversation_id
run_id
execution_id
task_run_id
ai_request_id
provider_request_id
usage_id
billing_reservation_id
ledger_transaction_id
```

Identity owns tenant and principal identities. Relay owns AI request and usage identities. Billing owns reservation, payment, and ledger identities. Domain services own their aggregate and execution identities.

### 5.3 Required sagas

- Relay usage: reserve quota, call provider, measure usage, settle billing, or release/refund.
- RAG ingestion: store object, parse, chunk, embed through Relay, index, publish ready, or remove orphan state.
- Marketplace purchase: create order intent, execute payment, grant entitlement, create settlement, or revoke/refund.
- Agent and Workflow: orchestration service owns the state machine; child services execute commands and emit results.
- Channel delivery: reserve quota when applicable, send, receive receipt, settle, or move to replayable failure.

Every saga has durable state, timeout policy, compensation commands, idempotency keys, manual takeover, and audit evidence.

## 6. Relay and Billing Authority

Relay authority is non-negotiable:

1. Only Relay holds shared model-provider credentials.
2. Chat, Agent, Workflow, Knowledge, MCP, and supported OpenAI-compatible routes may not call providers directly.
3. Relay produces `ai_request_id`, records provider request identity, captures authoritative measured usage, and emits `usage_id`.
4. Billing reserves quota before the call and settles from Relay usage after the call.
5. Billing may not estimate token usage when provider or Relay usage exists.
6. Observability correlates Relay usage and Billing ledger entries but does not become a financial source of truth.

Production network policy and static checks must make provider bypass impossible, not merely discouraged.

## 7. Product Completion Boundary

The detailed module capability baseline remains defined by `2026-07-10-module-capability-reference-design.md`. The following V1 rules refine that baseline.

### 7.1 Customer product modules

- Chat must provide durable conversations, real provider streaming, stop/retry/edit/branch, attachments, multimodal file/image support, Knowledge/Tool binding, citations, sharing, accurate settlement, and failure recovery.
- Knowledge must provide object storage, durable ingestion and index workers, parser isolation, OCR, chunking, Relay embeddings, hybrid retrieval, reranking, citations, document versioning, evaluation, delete propagation, retry, and DLQ operations.
- Agent must provide versioned definitions, durable runs, plan and tool state, memory evidence, approvals, budgets, structured provider streaming, checkpoint resume, sandboxed tools, artifacts, cancellation, and human takeover.
- Workflow must provide versioned typed graphs, triggers, durable execution, retry and failure branches, pause/resume/cancel, snapshots, debugger, checkpoint replay, compensation, worker leases, and retention.
- Task must invoke real Agent or Workflow targets and provide Cron/timezone, idempotent claim, misfire policy, history, retry, cancellation, distributed scheduling, and backlog SLOs.
- MCP and Tools must provide versioned schemas, tenant credentials, OAuth, risk policy, approvals, timeout, cancellation, network policy, health, cost, audit, and fail-closed behavior.
- Sandbox must be the only authority for custom code execution and provide non-root isolation, default-deny networking, resource limits, cancellation, logs, artifacts, malware controls, retention, and capacity management.
- Channel must certify a fixed V1 matrix: Feishu, DingTalk, Slack, and Telegram. Additional channels remain unsupported unless substituted before scope freeze.
- Customer Console and Platform Admin must be separate permission surfaces with organization, billing, usage, evidence, jobs, providers, tools, channels, alerts, policies, reconciliation, and break-glass operations.

### 7.2 Deferred enhancements

The following are not V1 requirements unless a signed requirement is approved before F0:

- desktop or mobile native clients;
- real-time collaborative editing;
- public unreviewed plugin or tool marketplace;
- user-defined sandbox images or persistent notebooks;
- knowledge graphs and arbitrary vector-store plugins;
- multi-agent teams, supervisor patterns, and automated skill learning;
- unlimited provider, channel, parser, or connector coverage;
- custom BI or no-code admin builders.

Deferred APIs and UI entries must not appear as partially working commercial surfaces.

## 8. Regional and Commercial Architecture

### 8.1 Sovereign planes

The release has four planes:

- Global Release Plane: signed software, schemas, public capability catalogs, and non-personal aggregate release metadata.
- International Commerce Plane: international tenant data, Stripe-class payment rails, international providers, tax and invoice integrations.
- China Commerce Plane: China tenant data, Alipay and WeChat Pay rails, domestic providers, domestic invoice integrations.
- Self-hosted Plane: customer-owned deployment with no required Oblivious SaaS callback or cross-border telemetry.

Tenant content, credentials, payment identity, KYC, invoices, request logs, and backups do not cross regional planes. Only signed product metadata, software versions, schema versions, and non-personal aggregate operational metadata may be synchronized.

### 8.2 Commercial services

`billing-payment` internally separates:

- commercial catalog and price books;
- metering and rating;
- quota reservations;
- order and payment lifecycle;
- invoice and tax documents;
- immutable double-entry ledger;
- reconciliation.

Money uses integer minor units plus currency. High-precision model costs use fixed-point decimal. Floating-point money is prohibited.

Posted journal entries are immutable. Corrections use reversal and replacement entries. Balances are projections from postings and are never directly edited.

Marketplace money movement must split gross amount, tax, platform commission, seller payable, payment fee, reserve, refund, and chargeback effects. Actual funds remain with licensed payment and payout providers; Oblivious records the business and accounting mirror.

### 8.3 Self-hosted commercial behavior

Self-hosted and air-gapped profiles default to:

- offline signed licenses or a customer-local license server;
- customer-owned model providers, KMS, IAM, SIEM, email, object storage, and ERP integrations;
- local metering, quota, entitlement, and audit;
- no Oblivious-hosted payment checkout;
- no platform seller KYC, settlement, or payout;
- no required cross-tenant provider pool;
- no required external telemetry.

Customer-supplied payment adapters may be supported, but financial responsibility and merchant accounts belong to the customer.

## 9. Deployment Profiles

### 9.1 Shared release contract

The same Git SHA is built once. All profiles use:

- identical application image digests;
- identical Helm chart interfaces;
- identical API, Proto, and event versions;
- identical migration set;
- identical observability attributes;
- profile-specific service bindings and certified adapters.

### 9.2 Hosted SaaS profile

The reference hosted stack uses:

- multi-AZ Kubernetes;
- managed PostgreSQL, Redis/Valkey, Kafka, ClickHouse, and object storage;
- cloud KMS and Secret Manager through External Secrets;
- Argo CD and Helm;
- strict mTLS, default-deny network policy, controlled egress;
- private endpoints for state services;
- cross-account immutable backups;
- regional data planes and tenant home-region routing.

### 9.3 Self-hosted profile

The certified self-hosted stack uses:

- RKE2 as the default Kubernetes profile;
- OpenShift as an additional certified profile;
- CloudNativePG, PgBouncer, and pgBackRest;
- Strimzi Kafka;
- an approved Redis/Valkey HA topology;
- Altinity-managed ClickHouse;
- S3-compatible object storage;
- Vault, enterprise PKI, or customer KMS;
- Harbor or an approved private registry;
- offline image, signature, SBOM, vulnerability database, and license bundles.

Arbitrary infrastructure combinations are unsupported until they pass the conformance suite.

### 9.4 Common infrastructure contracts

Every binding exposes endpoint, TLS CA, credential reference, region, durability class, owner, health, RPO, RTO, and last recovery-verification time.

Business services emit OTLP, Prometheus metrics, and structured logs without direct cloud-vendor telemetry dependencies.

## 10. Security and Compliance

Commercial release requires:

- tenant isolation across HTTP, gRPC, SQL, vector payloads, jobs, events, audit, and admin queries;
- fine-grained RBAC or ABAC and time-limited audited break-glass access;
- OIDC, SAML, SCIM, MFA, recovery controls, session and device management;
- shared outbound-request policy for SSRF, DNS rebinding, redirects, user headers, and egress allowlists;
- sandbox escape and host-execution prevention;
- signed and replay-protected webhooks;
- secret encryption, rotation, access audit, and non-disclosure in responses;
- digest-only deployment, Cosign signatures, SBOM, provenance, dependency, container, IaC, SAST, DAST, and secret-history scans;
- regional data classification, retention, deletion, support-access, and residency policy;
- legal, tax, finance, and security approval for payment, KYC, payout, and invoice controls.

No unresolved Critical or High security finding may ship without an approved, time-limited exception, compensating control, and named owner.

## 11. Reliability and Operations

Initial service-level objectives are:

| Capability | Availability | RPO | RTO |
|---|---:|---:|---:|
| Public API and Identity | 99.90% | 5 minutes | 60 minutes |
| Relay request acceptance | 99.95% | 5 minutes | 30 minutes |
| Core PostgreSQL business data | 99.95% | 5 minutes | 60 minutes |
| Event delivery | 99.90% | 5 minutes | 2 hours |
| ClickHouse request logs | 99.90% | 15 minutes | 4 hours |
| Object and RAG source files | 99.90% | 15 minutes | 4 hours |

Every service has dashboards, alerts, incident runbooks, capacity limits, maintenance procedures, backup, restore, and owner escalation.

Restore proof includes record counts, checksums, tenant isolation, replay results, and application-level validation. A successful backup command is not restore evidence.

## 12. Engineering and Agent Governance

### 12.1 Human team structure

The recommended 12-15 person structure is:

- Architecture and Contracts: 2;
- Edge and Relay: 3;
- Data and Automation: 3;
- Commerce and Governance: 2-3;
- Platform and Quality: 3-4.

Every service has a primary and secondary human owner. Architecture, database ownership, security exceptions, financial controls, and release approval remain human responsibilities.

### 12.2 Agent rules

- At most six to eight write agents operate concurrently.
- One agent owns one issue, one branch or worktree, and one explicit write set.
- A service has at most one active write agent.
- Shared contracts, generated clients, dependency locks, migration registries, deployment assets, and release gates require an exclusive human-approved lease.
- Implementer agents may not approve their own changes.
- Test and review agents validate through public behavior and evidence.
- Two identical failures stop automatic retries and require a minimal reproduction and human decision.
- Agents do not receive production secrets, execute production migrations, or approve releases.
- Every delivery includes changed paths, contract impact, migration impact, verification commands, risk, and recovery method.

### 12.3 Branch and integration policy

Use trunk-based development:

- `main` remains deployable and protected;
- feature branches normally live one or two days;
- merge queue validation runs against current `main`;
- contract changes are separate from service implementation changes;
- an integration branch may exist for no more than 48 hours;
- F2 creates a release branch that accepts blocker fixes only.

## 13. Program Phases and Gates

### F0: Release charter and boundary freeze

Deliverables:

- replacement service-boundary ADR;
- service owners and CODEOWNERS;
- V1 golden journeys and capability matrix;
- regional and self-hosted responsibility matrix;
- SLO, evidence, and legal-control owners.

No implementation starts while service ownership or authority remains disputed.

### F1: Contract freeze

Deliverables:

- external OpenAPI;
- complete internal Proto packages;
- common event envelope and topic catalog;
- error, pagination, streaming, deadline, retry, and idempotency semantics;
- database ownership and migration registry;
- Relay and Billing authority contracts.

Breaking changes after F1 require an RFC and approval from all affected owners.

### F2: Platform baseline

Deliverables:

- service template and generated SDKs;
- identity and authorization middleware;
- outbox/inbox libraries;
- shared outbound security policy;
- telemetry and evidence SDK;
- common Helm charts and deployment profiles;
- reproducible local, integration, global SaaS, China SaaS, and self-hosted environments.

### F3: Service vertical completion

Each service independently reaches Service Definition of Done while integration environments continuously deploy the full system.

### F4: Integration convergence

All primary golden journeys pass through the public Gateway with real state services. Production-like failures, retries, cancellations, restarts, and duplicate events are exercised.

### F5: Non-functional hardening

Required evidence includes:

- race and duplicate-delivery tests;
- 1.5x expected peak load for 60 minutes;
- 2x expected peak for 30 minutes without data or ledger inconsistency;
- eight-hour soak at expected peak;
- pod, database, queue, object store, vector store, and provider-failure chaos;
- complete security and supply-chain gates;
- backup, restore, upgrade, rollback, and forward-fix exercises.

### F6: Release candidate

The release manifest locks commit SHA, image digests, Chart versions, schema versions, migrations, adapter versions, SBOM, provenance, and evidence references.

Only blocker fixes are accepted.

### F7: Commercial cutover

The production environment starts from clean databases and object storage. It performs read-only smoke before write traffic is enabled.

After writes are enabled, the legacy aggregate runtime is not a supported production fallback. Recovery uses the approved restore or forward-fix plan.

## 14. Testing and Evidence

### 14.1 Required layers

- static format, lint, type, license, secret, SAST, dependency, container, and IaC checks;
- unit tests for rules, state machines, permissions, rating, and retry behavior;
- component tests with real PostgreSQL, Redis, Kafka, object storage, Qdrant, and ClickHouse;
- provider and consumer contract tests for OpenAPI, gRPC, and events;
- service integration tests with migrations and workers;
- real full-stack browser and API tests from the public Gateway;
- race, load, soak, chaos, security, backup/restore, upgrade, and rollback tests.

### 14.2 Mandatory golden journeys

- provider and channel configuration through authenticated administration;
- Chat streaming through Relay with usage, quota, billing, request-log, and trace linkage;
- Realtime disconnect and Batch/File lifecycle settlement;
- RAG upload through durable ingestion to indexed retrieval and citations;
- Agent approval, sandboxed tool execution, checkpoint resume, artifacts, and billing;
- Workflow trigger, failure, compensation, replay, and restart recovery;
- scheduled Task dispatch, misfire, retry, and duplicate suppression;
- Marketplace publish, review, appeal, purchase, entitlement, refund, settlement, and payout where enabled;
- upgrade and backup restore followed by continued business execution;
- clean self-hosted and air-gapped installation with no SaaS dependency.

### 14.3 Evidence classes

Evidence remains separated into:

1. unit or fixture evidence;
2. repository-local runtime evidence;
3. target-environment runtime evidence;
4. final no-skip commercial release evidence.

Lower classes never substitute for higher classes.

## 15. Final No-Skip Release Gate

Release is prohibited unless all conditions hold:

1. Required tests contain no temporary skip, only, dynamic ignore, or expired quarantine.
2. Every service satisfies Service Definition of Done.
3. Contract providers and consumers pass against the RC contract set.
4. All mandatory golden journeys pass repeatedly in Global SaaS, China SaaS, and self-hosted target environments.
5. Migration replay, upgrade validation, backup restore, rollback, and forward-fix exercises pass.
6. Race, load, soak, chaos, and security thresholds pass.
7. Target dashboards, traces, audit records, request logs, and alert delivery are queryable.
8. Payment, refund, dispute, invoice, settlement, payout, and reconciliation evidence exists for every enabled commercial rail.
9. Test commit, image digest, Helm release, SBOM, provenance, schema versions, and evidence manifest match.
10. Repository-local and target-environment evidence are both current.
11. The worktree and release commit are clean and reproducible.
12. Human owners for Product, Engineering, Security, Operations, Finance, Tax, and Release approve the final go/no-go record.

## 16. Definitions of Done

### 16.1 Service Definition of Done

A service is complete only when it has:

- approved API, event, and data contracts;
- production implementation and dependency wiring;
- migrations and schema ownership;
- unit, component, contract, integration, and failure-path tests;
- tenant isolation, permissions, idempotency, timeout, and recovery behavior;
- SLO, dashboards, alerts, traces, request IDs, and audit records;
- backup, restore, upgrade, rollback, and forward-fix procedures;
- threat model and security verification;
- operator runbook and owner escalation;
- target-environment evidence.

### 16.2 Product Definition of Done

The product is complete only when all services satisfy Service Definition of Done and the same RC digest passes the complete target matrix and final no-skip gate.

Code, scripts, fixtures, documentation, or repository-local green tests alone never justify the statement that the commercial final release is complete.

## 17. Primary Risks

- Big-bang blast radius if contracts or integration are deferred.
- Service-count pressure on an 8-15 person team.
- Divergence between hosted and self-hosted behavior.
- China and international code forks.
- incomplete independent services hidden behind deployment manifests;
- usage and financial double-writing;
- cross-tenant or cross-region evidence leakage;
- payment, payout, tax, and marketplace regulatory exposure;
- backups that have not been restored;
- agent-generated changes to shared contracts without ownership control;
- commercial claims based on fixtures or stale evidence.

Mitigation is contract-first integration, fixed adapter certification, controlled agent concurrency, continuous full-system deployment, target-environment evidence, and strict release refusal when evidence is incomplete.

## 18. Next Design Artifacts

After this specification is reviewed, the next artifacts are:

1. replacement microservice boundary ADR;
2. V1 golden-journey and capability matrix;
3. service ownership and CODEOWNERS plan;
4. API, Proto, event, and database contract plan;
5. dual-profile infrastructure and conformance plan;
6. commerce and regional compliance plan;
7. phased implementation plan with disjoint agent write sets;
8. target-evidence and final release plan.
