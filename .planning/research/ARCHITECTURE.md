# Architecture Research

**Domain:** Brownfield multi-tenant AI SaaS commercial closure
**Researched:** 2026-07-14
**Overall confidence:** MEDIUM
**Decision:** Preserve the existing Go/React modular monolith as the behavioral authority, add shared identity, durability, evidence, and deployment-parity contracts, and extract services only after a measured boundary passes explicit ownership and parity gates.

## Research Boundary And Confidence

This document separates two classes of claims:

- **Observed [MEDIUM]:** facts cross-checked across the current project definition, codebase map, approved capability design, 2026-07-10 audits, and commercial gate contract. These are stronger than a single historical report but remain documentation-level observations until a phase rechecks production source and runtime behavior.
- **Recommended [MEDIUM]:** architecture decisions derived from those observed constraints and cross-checked against current official PostgreSQL, OpenTelemetry, Playwright, Testcontainers, AWS, and Kubernetes guidance.

The configured research seam selected `websearch`, but no WebSearch tool or `BRAVE_API_KEY` was available. The confidence-classification seam returned `MEDIUM` for findings verified against official sources. No LOW-confidence external claim is treated as authoritative.

## Observed Brownfield Baseline

| Observed fact | Architectural consequence | Confidence |
|---|---|---|
| The mainline is a React/Vite frontend plus a Go HTTP/API runtime with PostgreSQL, pgvector/Qdrant, Relay, domain packages, gRPC adapters, independent command entrypoints, and release-evidence automation. | Extend the current domain packages and runtime composition; do not begin with a greenfield platform rewrite. | MEDIUM |
| The monolith is the deepest implementation. Several independent service entrypoints and manifests are thinner or health-only, and dual-mode parity is not yet proven. | The monolith remains the behavioral oracle. A split mode is supported only for capabilities that pass the same contract and journey suite. | MEDIUM |
| Relay is the commercial invariant for billable AI calls, but request, usage, billing, request-log, and audit evidence can still diverge. | Close one authoritative Relay-to-ledger lifecycle before expanding provider or lifecycle API breadth. | MEDIUM |
| Durable models exist in several domains, but worker startup, retry, replay, and downstream execution are uneven; RAG worker wiring and Task target execution are named gaps. | Standardize job semantics and transactional outbox behavior, then migrate domain workers incrementally. | MEDIUM |
| Tenant handling exists at HTTP, service, and SQL layers, but cross-mode identity propagation and cross-tenant denial proof are incomplete. | Treat tenant identity as a required domain parameter and persisted execution attribute, not only middleware context. | MEDIUM |
| Browser coverage is broad but much of it intercepts `/api/v1/**`; target/live proof remains distinct from repository-local fixture evidence. | Add a separate real full-stack profile. Keep fixture suites for UI contracts but never promote them to live commercial proof. | MEDIUM |
| The target-evidence toolchain already distinguishes local fixtures, external artifact bodies, manifest validation, and final no-skip verification. | Preserve that hierarchy and make evidence collection an observer of runtime truth, not a second implementation of it. | MEDIUM |
| The approved design explicitly rejects cloning reference projects and implementing a full distributed target before launch closure. | Prefer layered closure and safe extraction criteria over a fixed 12-service or other topology target. | MEDIUM |

## Recommended Architecture

### System Overview

```text
Browser / API client / channel webhook
                 |
                 v
+-------------------------------------------------------------+
| Edge and admission                                           |
| Web + HTTP Gateway -> auth/session/token -> tenant/policy    |
| request_id + trusted actor/organization + trace context      |
+-------------------------------+-----------------------------+
                                |
                                v
+-------------------------------------------------------------+
| Modular domain runtime                                       |
| Chat | Knowledge | Agent | Workflow | Task | Marketplace     |
| Admin | Billing | Payment | Channel | MCP                    |
| Each domain owns commands, state machine, tables, and events |
+----------------------+--------------------+-----------------+
                       |                    |
          billable AI  |                    | durable command
                       v                    v
+-----------------------------+   +---------------------------+
| Relay authority             |   | Durable execution plane   |
| provider policy and secrets |   | job + lease + attempt     |
| route/retry/stream/cancel   |   | outbox + inbox/idempotency|
| usage + price snapshot      |   | retry/DLQ/replay/cancel   |
| quota reserve/settle/refund |   | domain worker adapters    |
+---------------+-------------+   +-------------+-------------+
                |                               |
                +---------------+---------------+
                                v
+-------------------------------------------------------------+
| Owned persistence and integrations                           |
| PostgreSQL | pgvector/Qdrant | object store | Redis/Kafka    |
| payment/payout rails | provider APIs | channel APIs          |
+-------------------------------+-----------------------------+
                                |
                                v
+-------------------------------------------------------------+
| Observability and release evidence                           |
| OTel traces/logs + metrics + ClickHouse request log          |
| read-only collectors -> external artifact bundle -> digest   |
| target manifest -> strict same-commit no-skip verifier       |
+-------------------------------------------------------------+
```

This is one logical architecture. In monolith mode the boxes can run in one Go process and one PostgreSQL cluster. In dual or split mode the same contracts cross HTTP/gRPC/event boundaries. Process count must not change business semantics.

## Component Responsibilities

| Component | Owns | Must not own | Primary dependencies | Confidence |
|---|---|---|---|---|
| Web and API edge | Browser delivery, external routing, request normalization, CSRF/CORS, public auth challenge, request size and abuse policy | Provider routing, billing decisions, tenant authorization derived from caller-supplied internal headers | Auth, domain APIs, route manifest | MEDIUM |
| Identity and tenant authority | Sessions/tokens, organization membership, role/permission evaluation, trusted internal actor envelope | Domain object existence or domain-specific authorization decisions | PostgreSQL, audit | MEDIUM |
| Domain services | Aggregate state machines, user-visible invariants, authorization for owned resources, domain events | Direct provider credentials, generic cross-domain table mutation | Owned stores, Relay client, outbox | MEDIUM |
| Relay | Provider credentials and capabilities, routing, retries, streaming/cancellation, upstream request identity, authoritative usage, immutable price snapshot, quota reservation finalization, request log | Subscription lifecycle, Marketplace settlement, customer UI state | Provider APIs, quota/billing contract, request-log sink | MEDIUM |
| Billing and quota ledger | Plans, balances, reservations, ledger entries, entitlement, settlement/refund accounting, reconciliation results | Provider routing or inferred usage when Relay has authoritative usage | PostgreSQL, Relay usage events, payment events | MEDIUM |
| Payment and payout adapters | External intent/dispatch, signed webhook ingestion, provider event ledger, reconciliation | Marking money movement complete from an outbound HTTP 2xx alone | Provider rails, billing/Marketplace outbox | MEDIUM |
| Durable execution plane | Claim/lease/heartbeat, attempts, retry policy, cancellation, dead letter, replay, outbox dispatch, inbox deduplication | Domain success semantics or one universal payload schema | PostgreSQL first; Kafka only when justified | MEDIUM |
| Vector and object storage adapters | Tenant-scoped binary objects, checksums, retention, vector payload/filter lifecycle | Independent tenant or document authority | Knowledge domain, object store, pgvector/Qdrant | MEDIUM |
| Observability | Trace/log/metric export, ClickHouse request-log projection, SLO and alert state | Authorization, money truth, workflow truth, readiness claims based on no-op sinks | OTel, metrics backend, ClickHouse, alert sink | MEDIUM |
| Evidence plane | Read-only collection, canonical manifest, artifact digests, environment/release identity, strict verification | Generating synthetic business outcomes or storing secrets in git | Runtime APIs/stores, CI, cluster, external workdir | MEDIUM |
| Runtime composer | Monolith/dual-mode dependency wiring, worker lifecycle, health/readiness, graceful shutdown | Silent fallback from a failed production dependency to demo behavior | All enabled components | MEDIUM |

## Source-Of-Truth Ownership

Ownership must be explicit before roadmap work adds new flows or extracts a service.

| Record or decision | Authoritative owner | Replicas/projections | Required join keys |
|---|---|---|---|
| Organization, membership, actor permissions | Identity/Tenant | Admin views, audit projection | `organization_id`, `user_id`, `actor_id` |
| Conversation/message/knowledge binding | Chat | Search/UI projections | `organization_id`, `conversation_id`, `request_id` |
| Agent/workflow/task logical execution | Owning domain | Admin and observability projections | `organization_id`, `execution_id`, `run_id`, `attempt_id` |
| Provider route and upstream attempt | Relay | ClickHouse request log, operator views | `request_id`, `relay_attempt_id`, `trace_id` |
| Provider usage and request price snapshot | Relay | Billing ledger input, analytics projection | `request_id`, `usage_id`, `reservation_id` |
| Balance, quota, charge, refund | Billing/Quota ledger | Console/Admin projections | `organization_id`, `ledger_entry_id`, `usage_id` |
| External checkout/webhook/refund event | Payment adapter event ledger | Billing and Marketplace consumers | `payment_id`, `provider_event_id`, `idempotency_key` |
| Marketplace order, settlement, payout | Marketplace | Billing/Admin projections | `order_id`, `settlement_id`, `payout_id`, `payment_id` |
| Trace/log/metric data | Observability | Dashboards and evidence exports | `trace_id` plus validated business IDs |
| Commercial release claim | External evidence bundle for one immutable release | Repository verification record containing digests only | `release_id`, commit SHA, image digest, environment ID |

Rules:

1. A projection can be rebuilt and never becomes the command authority merely because it is convenient to query.
2. Cross-domain updates use an owned command API or event; they do not mutate another domain's tables directly.
3. The business transaction and its outbox event commit atomically in the owning PostgreSQL transaction.
4. Observability loss cannot change business state. Business state must remain inspectable and replayable without ClickHouse or a tracing backend.
5. Evidence collectors read authoritative state and operation outputs; they do not infer success from route existence, fixtures, or documentation.

## Shared Identity And Correlation Flow

### Identity Model

| Identifier | Lifetime and authority | Propagation rule |
|---|---|---|
| `organization_id` | Trusted tenant identity derived from authenticated membership | Required explicit service/store argument and persisted on every tenant-owned job/event; never accepted from public internal headers or untrusted baggage |
| `user_id` / `actor_id` | Authenticated human, API token, service principal, or worker actor | Persist on commands and audit events; distinguish initiator from current worker |
| `request_id` | One admitted edge request | Edge generates or normalizes it; preserve through Relay, logs, usage, and immediate side effects |
| `command_id` | One idempotent business intent | Required for retried POST/webhook/schedule commands; scoped by organization and command type |
| `execution_id` | Stable logical Agent/Workflow/Task lifecycle | Survives process restart and retry; parent for attempts, usage, artifacts, and replay |
| `attempt_id` | One worker claim or external dispatch attempt | New on retry; records lease owner, timing, error class, and result |
| `event_id` | Immutable outbox event | Consumer inbox deduplicates by `(consumer, event_id)`; payload carries schema version |
| `usage_id` / `reservation_id` | One metered operation and its quota reservation | Relay and ledger finalize idempotently; retries must not double charge |
| `payment_id` / `settlement_id` | External money and Marketplace lifecycle | Persist provider references and reconciliation evidence; never derive from trace IDs |
| `trace_id` / `span_id` | Telemetry causality | Use W3C TraceContext across HTTP/gRPC and explicit inject/extract for jobs; never use as authorization or idempotency authority |
| `release_id` | One deployable commit plus immutable artifact set | Stamp images, manifests, migration evidence, runtime smoke, and external artifact bundle |

### Request-To-Evidence Flow

```text
1. Edge authenticates caller and strips untrusted internal identity headers.
2. Tenant authority resolves organization membership and emits a trusted actor envelope.
3. Domain command persists aggregate change + command_id + correlation IDs + outbox event.
4. Worker claims an eligible row, creates attempt_id, restores trace parent, and rechecks tenant/resource invariants.
5. Billable AI work reserves quota and calls Relay with execution_id/request_id/reservation_id.
6. Relay records route decision, provider attempt, final usage, price snapshot, and terminal outcome.
7. Ledger idempotently settles or refunds the reservation; domain execution reaches a terminal state.
8. Outbox projects request log, audit, Admin, and reconciliation views without changing command truth.
9. E2E asserts user-visible result plus persisted joins across execution, usage, ledger, request log, and audit.
10. Target collectors export redacted proof keyed by release_id into an external artifact directory.
```

OpenTelemetry baggage may carry sanitized correlation hints inside trusted boundaries, but it has no built-in integrity. Authorization-bearing tenant identity and sensitive data belong in trusted internal envelopes and persisted records, not baggage. Do not use `organization_id` as an unbounded metric label; keep it in secured logs/traces and evidence queries.

## Durable Workers, Outbox, And Replay

### Recommended Common Contract

Use common semantics with domain-owned tables or partitions. Do not force every domain into a single payload schema.

```text
job identity:      job_id, organization_id, job_type, aggregate_id
causality:         command_id, execution_id, parent_event_id, traceparent
state:             pending | claimed | running | retry_wait | succeeded |
                   failed_terminal | dead_letter | canceled
claim:             lease_owner, lease_expires_at, heartbeat_at
retry:             attempt_count, max_attempts, next_attempt_at, error_class
idempotency:       idempotency_key, input_digest, consumer inbox record
payload:           schema_version, payload_ref, result_ref
audit:             created_at, started_at, completed_at, updated_at
```

### Processing Rules

1. Write aggregate state and outbox row in the same transaction. Never perform a database commit followed by an unprotected event publish.
2. Claim bounded batches with deterministic ordering and `FOR UPDATE SKIP LOCKED`; PostgreSQL documents this specifically as suitable for multiple consumers of queue-like tables.
3. A lease expiry makes work reclaimable. Reclaim creates a new `attempt_id` while retaining `execution_id` and `job_id`.
4. Every consumer persists inbox/deduplication state in the same transaction as its effect. Assume at-least-once delivery.
5. External side effects follow intent -> dispatch attempt -> provider acknowledgment/webhook -> reconciliation. An HTTP 2xx is not final money or payout success.
6. Retry only classified transient failures. Validation, tenant denial, unsupported capability, and policy failures are terminal and fail closed.
7. Dead-letter records retain redacted input reference, error class, attempt history, owning domain, and an audited replay command.
8. Replay creates a new command/attempt linked to the original; it never erases prior terminal evidence or double applies ledger effects.
9. Readiness must fail when a runtime declares a critical worker enabled but cannot claim, heartbeat, or reach its owned dependencies.
10. Shutdown stops new claims, drains bounded in-flight work, extends or releases leases safely, and records incomplete attempts.

### Event Bus Decision

Start with PostgreSQL outbox dispatch because the current monolith already depends on PostgreSQL and needs atomic ownership more than broker scale. Kafka remains an integration target, not an initial source of truth. Introduce or expand Kafka only after measured outbox lag, retention, fan-out, or isolation needs justify it, and retain the transactional outbox as the commit boundary.

## Relay Authority And Billing Handshake

All billable Chat, Knowledge embedding, Agent, Workflow, MCP, and supported `/v1/*` operations use one Relay client contract in both monolith and split modes.

```text
Domain -> Billing: reserve(org, command_id, estimated ceiling)
Billing -> Domain: reservation_id
Domain -> Relay: execute(request_id, execution_id, reservation_id, capability)
Relay -> Provider: selected route + sanitized trace context
Provider -> Relay: chunks/outcome/authoritative usage
Relay -> stores: route decision + attempts + usage + price snapshot
Relay -> Billing: settle_or_refund(reservation_id, usage_id, outcome)
Billing -> Domain: terminal ledger result
Domain -> User: final result or explicit compensated failure
```

Architectural constraints:

- Relay is the provider and usage authority; Billing is the monetary ledger authority.
- A domain stores references to Relay usage and ledger entries, not a competing token-price calculation.
- Streaming cancellation and disconnect are terminal lifecycle events that must settle or refund idempotently.
- Fallback and retry create provider attempts under one request; billing is based on the final authoritative lifecycle, not the number of handler invocations.
- Realtime, Batch, Files, or other lifecycle APIs stay disabled until auth, tenant, quota, settlement/refund, request log, audit, compensation, and target evidence all exist.
- User-controlled outbound URLs go through the shared fail-closed outbound policy. That policy complements Relay and must not become a provider-routing bypass.

## Tenant Enforcement Architecture

Tenant isolation is defense in depth, with each layer independently testable:

| Layer | Required control | Required proof |
|---|---|---|
| Public HTTP/WebSocket | Authenticate, resolve active organization, strip internal headers, reject missing membership | Positive and cross-tenant route tests |
| Internal HTTP/gRPC | mTLS or trusted network plus signed/derived actor envelope; deadlines and trace context | Forged/missing identity rejection and metadata propagation tests |
| Service method | Explicit `organization_id` and actor; resource ownership/permission check | Service tests cannot call tenant-owned operations without scope |
| SQL store | Organization predicate on every tenant-owned read/write; scoped uniqueness/idempotency | Store and DB-backed cross-tenant denial tests |
| PostgreSQL RLS | Optional backstop for high-risk tables; `FORCE ROW LEVEL SECURITY` where owner bypass is unsafe | Policy tests under production-like roles, including owner/bypass review |
| Vector store | Organization and knowledge/document version in payload and mandatory filter | Cross-tenant vector retrieval/delete denial |
| Object store | Tenant-scoped object key plus authorization metadata; no public bucket trust | Signed access, wrong-tenant denial, retention/delete proof |
| Jobs/outbox | Persist tenant and actor; consumer validates aggregate ownership before effect | Poisoned/mismatched event denial and no cross-tenant retry leak |
| Admin/support | Explicit target tenant, privileged permission, reason, before/after audit | High-risk operation authorization and audit tests |
| Logs/analytics | Access-controlled tenant field; no tenant secret in metric labels | Query isolation and redaction tests |

PostgreSQL RLS is a backstop, not a substitute for scoped stores. Owners and `BYPASSRLS` roles can bypass policies, and referential-integrity behavior can reveal information; production roles and backup/recovery paths need explicit review.

## Real End-To-End Harness

Maintain three separate profiles with explicit evidence classes.

### Profile A: Fixture And Contract

- Existing handler, frontend route, and Playwright interception tests remain fast regression coverage.
- They verify serialization, UX states, and verifier behavior only.
- They cannot close persistence, tenant, provider, billing, or target-release claims.

### Profile B: Repository-Local Real Full Stack

1. Create an isolated run ID and disposable network.
2. Start PostgreSQL/pgvector, Qdrant, ClickHouse, Redis, object storage, and any dependency required by the selected journeys. Use existing Compose assets or Testcontainers where lifecycle control is needed.
3. Run the exact production migrations and record checksums.
4. Start the real Go runtime in the declared mode and wait on readiness, not process existence.
5. Start the production-built web application. Playwright can manage multiple web servers and wait for their URLs.
6. Run browser journeys without `page.route` or equivalent interception for Oblivious product APIs.
7. Use a protocol-faithful local provider only for deterministic repository-local coverage and label the result accordingly.
8. Assert UI result and authoritative database joins: tenant, execution, usage, ledger, request log, audit, and terminal worker state.
9. On failure retain redacted service logs, trace IDs, screenshots/video, database diagnostics, worker backlog, and migration state.
10. Tear down by run ID; never reuse developer services in CI.

Core journey order: identity/tenant -> Chat/Relay -> RAG ingestion/retrieval -> Agent/Workflow/Task -> Billing/Marketplace. Later journeys depend on the evidence spine proved by earlier ones.

### Profile C: Target Live Release

- Run against the exact immutable release candidate and declared production topology.
- Use live provider/payment/payout rails, external secrets, target databases/stores, deployed observability, Kubernetes rollout, backup/restore, and rollback evidence.
- Collect artifact bodies outside git, refresh canonical digests, validate the filled manifest, then run the strict commercial verifier with no environment skip or commit-mismatch escape hatch.
- A Profile B pass is necessary but cannot substitute for Profile C.

## Declared Deployment Parity

Publish a machine-readable capability manifest per release with:

- supported topology: `monolith`, `dual`, or named extracted services;
- capability and lifecycle status: supported, disabled/fail-closed, or experimental;
- owning process and datastore;
- migration set and schema compatibility window;
- required dependency, readiness condition, and SLO;
- contract suite and target-smoke evidence reference;
- image digest and release ID.

The same capability contract and critical journey suite run against every declared topology. A service with only health endpoints is not a supported extracted capability. If split mode lacks parity, remove it from default manifests and release claims rather than adding stubs to satisfy an old topology count.

### Safe Service Extraction Criteria

Extract a domain only when all criteria pass:

1. A cohesive ownership boundary and measured scaling, failure-isolation, release-cadence, or security reason exists.
2. Owned tables, vectors, objects, jobs, migrations, and secrets are enumerated; no other service writes them directly.
3. Versioned command/query and event contracts cover current monolith behavior.
4. Tenant, actor, request, execution, trace, deadline, and idempotency metadata cross the boundary safely.
5. Transactional outbox, consumer deduplication, retry, replay, and compensation are working.
6. Monolith and extracted modes pass the same contract, tenant-denial, and user-journey tests.
7. Independent readiness, migration, rollout, rollback, backup/restore, dashboards, alerts, and runbooks exist.
8. Target smoke proves functional behavior, not only health.
9. The release can disable or roll back the extracted mode without data loss or double processing.

RAG is a reasonable first parity pilot only after ingestion/index workers, object ownership, vector lifecycle, and restart replay are closed. This is a conditional pilot, not a mandate to extract RAG or recreate the old fixed topology.

## Observability Architecture

Use OpenTelemetry/W3C TraceContext for causal telemetry across HTTP, gRPC, and explicit job/outbox carriers.

- Create spans at edge admission, domain command, worker attempt, Relay route/provider attempt, ledger finalization, webhook/reconciliation, and evidence collection.
- Inject/extract trace context into job and event metadata. Preserve links when replay starts a new trace.
- Put stable business IDs in controlled log/trace attributes after validation; keep high-cardinality tenant and execution IDs out of metric labels.
- Sanitize incoming external trace context and avoid propagating internal baggage to public providers.
- ClickHouse request logs are a query projection joined by request/usage/trace IDs, not the business source of truth.
- Readiness reports whether the declared capability can accept work: migrations current, required stores reachable, critical workers active, and no fail-closed dependency missing. Liveness reports process progress and must not restart a healthy process merely because an external dependency is temporarily unavailable.
- Alerts must carry a real delivery ID and recovery audit ID when used as release evidence.

## Evidence Collection Architecture

```text
runtime truth + deployment status + migration ledger + external rail results
                              |
                       read-only collectors
                              |
        redacted artifact bodies in external release workdir
                              |
               canonical manifest + content digests
                              |
       strict preflight -> target verifier -> no-skip verifier
                              |
           repository record of command, digest, result, risk
```

Requirements:

1. Every artifact includes `release_id`, commit SHA, image digest, environment class, collector version, collection time, target reference, and redaction status.
2. Raw secrets, kubeconfig, provider keys, database URLs, and sensitive payloads never enter the repository or evidence bundle.
3. The manifest references immutable artifact bodies; pass-only JSON without underlying proof is invalid.
4. Collectors are read-only except for purpose-built smoke transactions identified by run ID and safely cleaned or retained as audit evidence.
5. Fixture mutations test verifier rejection. They never produce target evidence.
6. Evidence from another commit, migration state, topology, or environment cannot be silently combined.
7. Release evidence must cover deployment, Kubernetes, provider rails, request-log joins, RAG, gRPC, service databases, payment/payout reconciliation, secret audit, backup/restore, rollback, and residual risk for the capabilities actually declared.

## Internal And External Integration Points

| Integration | Pattern | Failure/ownership rule | Confidence |
|---|---|---|---|
| Web -> Go API | Versioned HTTP/OpenAPI plus CSRF/session/token rules | Edge resolves tenant; frontend never calls providers directly | MEDIUM |
| Domain -> Relay | In-process interface in monolith, versioned HTTP/gRPC client in split mode | Same lifecycle semantics in both modes; fail closed on missing Relay | MEDIUM |
| Domain -> durable workers | Transactional job/outbox row plus lease worker | At-least-once, idempotent consumer, explicit terminal state | MEDIUM |
| Service -> service | Versioned gRPC/HTTP with trusted identity envelope, deadline, trace context | No direct cross-owner table writes | MEDIUM |
| PostgreSQL | Domain-owned tables/migrations and transactional outbox | Scoped queries, append-only migration ledger, restore proof | MEDIUM |
| pgvector/Qdrant | Tenant-filtered vector lifecycle | Document/version deletion and stale-vector cleanup are owned by Knowledge | MEDIUM |
| Object storage | Checksum-addressed tenant objects | Shared, multi-replica safe, retention and malware policy | MEDIUM |
| Redis | Cache/rate limit/ephemeral coordination only | Loss cannot erase business truth | MEDIUM |
| Kafka | Outbox-fed distribution when measured need exists | Broker acknowledgment is not aggregate commit; consumers deduplicate | MEDIUM |
| Provider APIs | Relay adapters and outbound policy | Sanitized context, timeout/cancel, route audit, usage reconciliation | MEDIUM |
| Payment/payout rails | Intent + signed webhook + reconciliation | Provider event ledger and idempotency before final state | MEDIUM |
| ClickHouse/OTel backend | Asynchronous telemetry projection | No-op/in-memory sink is non-production and fails readiness if declared required | MEDIUM |
| Kubernetes | Immutable images, probes, controlled rollout, status and rollback | Readiness gates traffic; rollout proof is tied to release ID | MEDIUM |

## Explicit Dependencies And Build Order

| Order | Architecture increment | Depends on | Unlocks | Completion signal |
|---:|---|---|---|---|
| 1 | Ownership and declared-mode contract | Current source/audit inventory | All later roadmap slices | Every capability has owner, datastore, mode, lifecycle status, and evidence class |
| 2 | Trusted identity/correlation envelope and tenant defense | Identity/Tenant foundation | Safe workers, gRPC, E2E, evidence joins | HTTP/gRPC/store/vector/job cross-tenant denial plus identity propagation tests |
| 3 | Shared outbound policy and fail-closed production composition | Step 2 | Agent/Workflow/MCP/payout safety | All user-controlled URL paths share DNS/IP/redirect enforcement |
| 4 | Durable job/outbox/inbox/replay substrate | Steps 1-2 | RAG workers, Task execution, reconciliation | Lease reclaim, duplicate delivery, cancellation, DLQ, replay, shutdown tests |
| 5 | Close RAG worker and Task real-target vertical slices | Step 4 | Real RAG/Agent/Workflow journeys | Enqueue reaches real terminal state across restart and preserves execution identity |
| 6 | Relay usage/price/quota/ledger evidence spine | Steps 2 and 4 | Chat live proof, cost control, observability joins | One request joins route, attempt, usage, reservation, ledger, request log, and audit |
| 7 | Repository-local real full-stack harness | Steps 2-6 | Regression-safe commercial journeys | No product API interception; real DB and worker side effects asserted |
| 8 | Payment/Marketplace reconciliation and persistent observability | Steps 4 and 6 | Money and SLO closure | Signed webhook, retry, refund/payout reconciliation, ClickHouse join, alert recovery |
| 9 | Conditional service-parity pilot | Steps 1-8 | Evidence-based extraction template | Same journeys pass monolith and pilot mode; owned DB/migrations and rollback proven |
| 10 | Supply chain and target-release closure | Stable declared topology and Profile B | Commercial release claim | Immutable signed artifacts/SBOM/provenance plus Profile C no-skip verifier |

This ordering intentionally closes identity, safety, durability, and evidence semantics before distribution. Work can run in parallel inside an order only when it does not invent conflicting ownership or contracts.

## Scaling And Target-Release Implications

| Scale / pressure | Recommended response | Do not do |
|---|---|---|
| Early commercial load | Keep modular monolith; run domain workers as separate processes only where operationally useful; use PostgreSQL outbox and bounded claims | Split services to match an old diagram |
| Rising worker backlog | Add queue partitions, worker pools, lease metrics, backpressure, and per-tenant concurrency before introducing a broker | Increase retries without idempotency or capacity limits |
| High Relay volume | Isolate Relay process/pool, stream without buffering, partition request logs, and tune route/provider concurrency | Duplicate provider logic in Chat/Agent services |
| Large RAG corpus | Move raw objects to shared object storage, scale ingestion/index workers, partition vector collections, preserve tenant/version filters | Store multi-replica files on local disk |
| High event fan-out/retention | Feed Kafka from transactional outbox, version events, maintain consumer inboxes | Replace the database commit boundary with best-effort publish |
| Independent release or fault domain needed | Extract only the measured domain that passes the criteria above | Extract all named services at once |
| Multi-region target | Establish global identity and idempotency, region ownership, replicated object/event strategy, reconciliation, and failover evidence first | Claim active-active from duplicated manifests alone |

The first bottlenecks are likely worker backlog, provider concurrency/streaming, request-log volume, and vector/object ingestion rather than HTTP process count. Measure these before selecting extraction or partitioning work.

## Anti-Patterns To Avoid

### Fixed Topology As Completion

**Problem:** Creating every historical service entrypoint or Kubernetes Deployment while behavior remains in the monolith.

**Instead:** Declare only proven modes and require contract, journey, datastore ownership, and target-smoke parity.

### Middleware-Only Tenant Isolation

**Problem:** A handler carries organization context, but stores, vectors, jobs, retries, or Admin queries can omit it.

**Instead:** Make organization scope explicit in every owned interface and persist it with work; add DB-backed denial tests.

### Trace Or Baggage As Business Identity

**Problem:** Authorization, idempotency, or settlement depends on forgeable telemetry context.

**Instead:** Persist trusted business IDs and use trace context only for causal observability.

### Dual Write Without Outbox

**Problem:** Commit domain state and then publish an event or enqueue a job; a crash leaves one side missing.

**Instead:** Commit state plus outbox atomically and make consumers idempotent.

### Exactly-Once Assumption

**Problem:** Retries, lease expiry, webhook redelivery, or broker behavior duplicate effects and charges.

**Instead:** Design for at-least-once delivery, scoped idempotency, immutable attempts, and reconciliation.

### Health-Only Readiness

**Problem:** A service returns 200 while migrations are stale, workers are absent, or its required backend is a no-op.

**Instead:** Readiness reflects the capabilities declared for that process; liveness remains a narrow progress check.

### Fixture Evidence Promotion

**Problem:** Mock browser routes, fake providers, or verifier fixtures are called real commercial E2E.

**Instead:** Keep explicit evidence classes and require Profile B and Profile C for their respective claims.

### Observability As A Second Ledger

**Problem:** ClickHouse or traces become the only record of usage, payment, or execution state.

**Instead:** Keep transactional truth in owned stores and project it asynchronously to observability.

### In-Repository Target Proof

**Problem:** Secrets, downloaded artifact bodies, filled manifests, or stale proof are committed or mixed across releases.

**Instead:** Use an external workdir, immutable digests, redaction, release identity, and same-commit strict verification.

## Roadmap Implications

- Roadmap phases should follow the build-order dependency chain, not the historical module count.
- The first architectural milestone should produce enforceable ownership, identity, durability, and declared-mode contracts rather than more UI inventory.
- The first user-visible proof milestone should be a real full-stack vertical slice whose IDs join from browser request through Relay or worker execution to ledger/request-log/audit state.
- External integration phases need explicit environment and credential dependencies; they cannot be closed by repository-local mocks.
- A service extraction phase is optional and evidence-triggered. If parity adds no measured operational value, keep the domain in the modular monolith.
- Target release is a distinct final phase because it depends on a stable topology, immutable artifacts, external infrastructure, and same-release evidence.

## Open Validation Questions

These are phase-entry checks, not placeholders in the architecture:

1. Which independent service modes are actually intended for the next commercial release, as opposed to retained experimental manifests?
2. Which current tables have multiple writers, especially usage, billing, request-log, Marketplace settlement, and Agent/Workflow execution records?
3. Which internal identity headers are currently accepted at the public edge, and which gRPC adapters validate their provenance?
4. Can current job tables adopt one lease/attempt contract without destructive migration, or should adapters normalize multiple schemas?
5. Which Profile B dependencies are mandatory for every journey versus enabled only for a targeted evidence profile?
6. What measured load or failure-isolation requirement, if any, justifies the first service extraction?

## Sources

### Repository Sources

- `.planning/PROJECT.md` - current project goal, invariants, completion definition, and layered delivery strategy.
- `.planning/codebase/ARCHITECTURE.md` - current runtime and package boundaries.
- `.planning/codebase/CONCERNS.md` - release-evidence, fixture, complexity, integration, and security risks.
- `docs/superpowers/specs/2026-07-10-module-capability-reference-design.md` - approved capability boundaries and rejected topology-first approaches.
- `docs/audit/2026-07-10-full-repository-scan.md` - observed production assembly and target-proof gaps.
- `docs/audit/2026-07-10-module-capability-gap-matrix.md` - module-specific first slices and dependency ordering.
- `docs/release/commercial-gates.md` - evidence classes, strict target manifest/artifact requirements, and claim rules.

### External Official Sources

- AWS Prescriptive Guidance, Transactional outbox pattern: https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html - atomic state/event write, ordering, duplicate delivery, and idempotent consumers. Accessed 2026-07-14.
- PostgreSQL current documentation, `SELECT` locking clause: https://www.postgresql.org/docs/current/sql-select.html - `SKIP LOCKED` is suitable for multiple consumers of a queue-like table and not for general consistent reads. Accessed 2026-07-14.
- PostgreSQL current documentation, Row Security Policies: https://www.postgresql.org/docs/current/ddl-rowsecurity.html - default-deny policies, owner/BYPASSRLS behavior, and referential-integrity cautions. Accessed 2026-07-14.
- OpenTelemetry, Context propagation: https://opentelemetry.io/docs/concepts/context-propagation/ - W3C TraceContext, manual carrier propagation, correlation, and external-boundary security. Last modified 2026-01-14; accessed 2026-07-14.
- OpenTelemetry, Baggage: https://opentelemetry.io/docs/concepts/signals/baggage/ - cross-service metadata and lack of built-in integrity. Accessed 2026-07-14.
- Playwright, Web server: https://playwright.dev/docs/test-webserver - starting and waiting for one or multiple real application servers. Current `main` documentation accessed 2026-07-14.
- Testcontainers for Go: https://golang.testcontainers.org/ - disposable container dependencies for integration and smoke tests. Current `main` documentation accessed 2026-07-14.
- Kubernetes, Liveness, Readiness, and Startup Probes: https://kubernetes.io/docs/concepts/configuration/liveness-readiness-startup-probes/ - traffic gating, restart behavior, startup protection, and probe cautions. Last modified 2026-04-17; accessed 2026-07-14.
- Kubernetes, Deployments: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/ - declarative rollout, status, controlled replacement, and rollback. Current documentation accessed 2026-07-14.

---
*Architecture research for Oblivious Commercial Complete & Target Release*
