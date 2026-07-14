# Project Research Summary

**Project:** Oblivious Commercial Complete & Target Release
**Domain:** Brownfield organization-focused multi-tenant AI SaaS commercial closure
**Researched:** 2026-07-14
**Confidence:** MEDIUM overall; HIGH for repository-specific scope and gaps

## Executive Summary

Oblivious is not a greenfield chatbot and should not be reduced to an MVP or rebuilt to match an old topology. It is an existing Go/React multi-tenant AI SaaS with substantial Chat, Knowledge RAG, Agent/SOLO, Workflow, Task, Admin, Billing, Marketplace, deployment, and verification foundations. The milestone is to turn that brownfield depth into a deployable, chargeable, operable, auditable, and recoverable commercial product. Relay remains the only billable AI path, organization identity remains mandatory across every synchronous and asynchronous boundary, and repository-local evidence remains distinct from target/live and final same-commit no-skip evidence.

The recommended approach is layered closure. Preserve the modular monolith as the current behavioral authority, make ownership and evidence identities explicit, close shared tenant and outbound-security boundaries, then complete durable workers and real Task targets. After those prerequisites, prove one authoritative Relay-to-usage-to-ledger lifecycle and run focused browser journeys against the real Web, Go backend, PostgreSQL, workers, and applicable external rails. Financial reconciliation, persistent observability, recovery, optional service parity, immutable artifacts, and target evidence follow in dependency order. Service extraction is conditional on measured value and parity; it is not a completion goal by itself.

The largest risks are false readiness claims, split Provider/usage/billing authority, tenant identity loss, durable records without a running owner, unreconciled money movement, fixture-only E2E, and deployment claims backed only by health endpoints or manifests. Mitigation is structural: fail closed, persist shared business identities, use owned stores plus transactional outbox/idempotent consumers, keep telemetry as a projection rather than a ledger, require real full-stack and target profiles, and bind final proof to immutable release digests. Exact external dependency versions in [STACK.md](./STACK.md) are low-confidence release-cut candidates and must be refreshed before they become implementation requirements.

## Key Findings

### Recommended Stack

Preserve the current technology architecture and upgrade in isolated, reversible slices. The product remains Go services and domain packages, React/Vite, PostgreSQL with pgvector as the baseline vector path, Redis/Asynq for existing ephemeral coordination and jobs, OpenTelemetry-compatible instrumentation, and Docker/Kubernetes only for deployment profiles that are actually advertised. A maintained S3-compatible object-storage contract is required for shared production blobs. Qdrant, ClickHouse, Kafka, PgBouncer, and independent services are conditional profile dependencies rather than universal product requirements.

Toolchain and stateful-service changes must not compete with runtime closure. Canonical builders, frozen dependency graphs, immutable image digests, full-SHA CI actions, migration/restore proof, SBOM, scanning, signatures, and provenance matter more than adopting framework majors. Go, Node, database, queue, observability, and supply-chain patch versions listed in STACK.md are **LOW external confidence** and must be rechecked against official sources and target compatibility at phase entry and release cut.

**Core technologies and policies:**

- **Go + existing HTTP/gRPC/protobuf boundaries:** retain the implemented backend; no language or framework rewrite.
- **React 18 + Vite + pnpm:** retain the current UI architecture; upgrade runtime/tooling separately from product closure.
- **PostgreSQL + pgvector:** remain the baseline system of record, ledger, durable execution store, outbox, and vector path.
- **Redis/Asynq:** retain for current cache, rate-limit, session, and job contracts; prove failure and recovery semantics.
- **Relay adapters:** remain the sole Provider and authoritative usage boundary; do not add a universal second SDK/runtime.
- **S3-compatible object storage:** use a maintained target-owned implementation for multi-replica blobs and recovery.
- **OTel + metrics + ClickHouse where declared:** complete deployed collection and target proof rather than introducing another telemetry API.
- **Immutable release toolchain:** canonical images, digests, SBOM, scans, signatures, provenance, and same-release verification.

### Expected Features

Commercial launch is defined by complete customer and operator journeys, not by provider, tool, channel, node, page, route, schema, proto, or test counts.

**Must have for the commercial launch baseline:**

- Secure organization onboarding, sessions, roles, tenant-scoped state, and cross-tenant denial across HTTP/gRPC, service, SQL, vector, job, retry, Admin, and analytics paths.
- Relay-authoritative Provider routing, streaming, cancellation, pricing, quota, usage, settlement/refund, request logging, and audit with joinable evidence identities.
- Durable Chat, RAG, Agent, Workflow, and Task execution, including production worker ownership, retries, dead letter, replay, cancellation, approvals, budgets, sandbox and visible recovery.
- Real browser journeys without Oblivious API interception, reaching real Go services, migrations, PostgreSQL and applicable workers/external rails.
- Idempotent subscription, top-up, quota, invoice, payment, refund, Marketplace order, settlement, payout, dispute and reconciliation behavior.
- Admin operations backed by real tenant-safe mutations and audit, plus at least one real channel lifecycle if multi-channel publishing remains a launch claim.
- Persistent request/usage/billing joins, SLOs, alert delivery, recovery audit, backup/restore, rollback, and honest readiness.
- Reproducible immutable artifacts and final target evidence from the same release commit with no critical environment skips.

**Should have as competitive differentiators after baseline closure:**

- One Relay governance plane across every AI surface.
- A request-to-revenue evidence spine joining execution, usage, billing, payment, settlement, audit, and trace.
- Durable AI execution with human takeover and restart continuation.
- Explainable and recoverable RAG with citations, lifecycle diagnostics, and measured quality.
- Marketplace financial governance with creator operations, refund impact, external payout, and appeals.
- Evidence-first releases and evidence-triggered deployment parity rather than topology-first decomposition.
- Quality, cost, margin, Provider-health, retrieval, Agent and Workflow control views.

**Defer until the launch baseline is proven:**

- Advanced Relay lifecycle APIs that lack full identity, billing, compensation, audit, and target proof.
- Multi-agent teams, knowledge graphs, advanced retrieval evaluation, and additional publishing channels.
- Desktop/PWA expansion, multi-region/residency, and broad compliance programs.
- Marketplace recommendations and creator analytics before paid lifecycle and reconciliation are trustworthy.
- MFA, OIDC/SAML, SCIM, access review, retention/legal-hold and policy packs unless an enterprise launch tier explicitly requires them; if claimed, they become table stakes for that tier.

### Architecture Approach

Use one logical architecture that can run in the existing modular monolith and, only where proven, across independent processes. The process count must not change business semantics. Domain services own commands, state machines, tables and events; Relay owns Provider routing and authoritative usage; Billing owns monetary ledger truth; payment/payout adapters own external event ingestion and reconciliation; the durable execution plane owns leases, attempts, retries, dead letters and replay semantics; observability owns projections; the evidence plane collects read-only proof outside git. PostgreSQL transactional outbox plus idempotent inbox/consumers is the default commit boundary. Kafka is introduced only for measured distribution needs and never replaces the owning database transaction.

**Major components:**

1. **Edge and identity admission** - authenticates callers, resolves trusted organization/actor context, strips untrusted internal headers, and propagates request/trace context.
2. **Domain runtime** - owns Chat, Knowledge, Agent, Workflow, Task, Marketplace, Admin, Billing, Payment, Channel and MCP business state.
3. **Relay authority** - owns Provider capability, route/retry/stream/cancel, authoritative usage and price snapshots, and the quota settlement handshake.
4. **Durable execution plane** - provides domain-owned jobs with common lease, attempt, idempotency, outbox, retry, dead-letter, replay and shutdown semantics.
5. **Owned persistence and integrations** - PostgreSQL, vectors, shared objects, optional Redis/Kafka, Provider and financial/channel rails under explicit ownership.
6. **Observability and evidence plane** - projects logs/traces/metrics/request logs and collects redacted target proof bound to release identity and immutable digests.
7. **Runtime composer and deployment profiles** - wires only supported capabilities, makes readiness reflect real workers/dependencies, and proves parity for every advertised topology.

### Critical Pitfalls

1. **Evidence laundering** - never promote docs, fixtures, local smoke, historical evidence, or counts to target commercial readiness; attach evidence class, environment, commit, migration state, skips, and risk to every claim.
2. **Split Relay, usage, or billing authority** - prohibit direct Provider paths outside Relay and duplicate billable writers; reconcile one request through route, usage, quota, ledger, request log, audit and refund.
3. **Tenant identity lost between boundaries** - make organization and actor identity explicit and persisted in services, stores, vectors, jobs, retries, gRPC, Admin and analytics; test positive and deny cases at every layer.
4. **Durable records with no runtime owner** - require a production entrypoint, lease/heartbeat, retry budget, dead letter, readiness/lag metric, drain and restart proof for every job; Task must invoke a real Agent or Workflow target.
5. **Money movement without reconciliation** - persist signed Provider events and durable dispatch identities before transitions; use transactional outbox, idempotency, replay and explicit reconciliation for payment, refund, settlement and payout.
6. **Unsafe outbound and code execution** - enforce one DNS/IP/redirect/credential/body/timeout policy across tools, MCP, Workflow HTTP, channels, Provider URLs and payout; fail closed when the sandbox is unavailable.
7. **Fixture-only E2E and health-only services** - retain fast fixture suites as lower evidence, but require real full-stack journeys and capability parity before advertising a deployment mode.
8. **Local/mutable production state and observability islands** - use shared object storage and immutable images; keep telemetry as an access-controlled projection of authoritative business records.

## Implications for Roadmap

Roadmap phases should follow dependency and evidence flow, not the historical module count. The suggested structure is intentionally fine-grained so each phase can produce a verifiable contract or customer journey without requiring a big-bang rewrite.

### Phase 1: Release Contract, Ownership, and Build Baseline

**Rationale:** Every later slice needs an agreed release scope, authoritative owner, evidence class, deployment profile and reproducible build identity.
**Delivers:** Machine-readable capability/deployment manifest; owner/datastore/writer map; current route/API/migration baseline; canonical server/web build path; immutable digest capture; phase-entry compatibility checks for proposed toolchain versions.
**Addresses:** Honest deployment claims, reproducible artifacts, current-contract priority.
**Avoids:** Fixed topology as completion, mutable artifact mismatch, low-confidence version advice becoming an unreviewed hard requirement.

### Phase 2: Tenant Identity and Shared Outbound Security

**Rationale:** Tenant and network trust must precede new workers, external integrations, gRPC expansion, and real E2E.
**Delivers:** Trusted actor/organization envelope; explicit scoped service/store contracts; cross-tenant denial matrix; shared fail-closed DNS/IP/redirect/credential policy across Agent, Workflow, MCP, webhooks, channels, Provider URLs and payout.
**Addresses:** Secure organization journey, tenant-safe tools and integrations.
**Avoids:** Middleware-only isolation, forged internal headers, SSRF, credential leakage, semantic-cache cross-context leakage.

### Phase 3: Durable Execution and Shared Object Lifecycle

**Rationale:** Real customer journeys cannot be stable while persisted jobs have no owner, Task simulates work, or multi-replica files are local.
**Delivers:** Common lease/attempt/idempotency/outbox/inbox/replay semantics; production RAG ingestion/index workers; real Task-to-Agent/Workflow target execution; sandbox authority; shared S3-compatible object lifecycle with checksum, retention, tombstone, malware/orphan and restore proof.
**Addresses:** Durable RAG, Agent/Workflow/Task execution, production sandbox and recoverable blobs.
**Avoids:** Orphaned queues, duplicate effects, host execution fallback, local-file state loss.

### Phase 4: Relay, Chat, and the Evidence Spine

**Rationale:** Relay must become the proven Provider/usage authority before billing, analytics, or broad customer journeys can rely on it.
**Delivers:** One request identity across route attempts, streaming/cancellation, Provider usage, immutable price snapshot, quota reservation, settlement/refund, request log and audit; real Chat SSE lifecycle and compensation; explicit fail-closed status for unsupported lifecycle APIs.
**Addresses:** Relay and Chat launch table stakes, request-to-revenue foundation.
**Avoids:** Direct Provider bypass, duplicate usage rows, billing from estimated or missing terminal stream data.

### Phase 5: Real Full-Stack Customer and Admin Journeys

**Rationale:** Once trust, durability, and Relay semantics are stable, browser proof can validate actual product wiring instead of fixtures.
**Delivers:** Built Web -> Go -> migrations -> PostgreSQL/workers journeys for identity, Chat, RAG, Agent, Workflow/Task, Admin and applicable channel behavior; persisted-ID assertions; failure artifacts and contract-drift rejection. Add one real channel only if publishing remains a launch claim.
**Addresses:** Real customer journeys, admin-backed operations and lower-to-higher evidence separation.
**Avoids:** Fixture-only E2E, route/page/schema completeness claims, unsupported capability appearing enabled.

### Phase 6: Financial and Marketplace Reconciliation

**Rationale:** Paid launch depends on the evidence spine and durable dispatch semantics from earlier phases.
**Delivers:** Signed/idempotent checkout and webhook ledger; quota/entitlement timing; refund and dispute handling; Marketplace order, settlement, external payout, retry, replay and reconciliation; operator exception/remediation view.
**Addresses:** Metering, payment, paid Marketplace, publisher and finance journeys.
**Avoids:** Double credit/payout, premature entitlement, local payout simulation, disconnected refund and settlement state.

### Phase 7: Persistent Observability, SLO, and Recovery

**Rationale:** Operations can be proven only after authoritative execution and ledger identities exist.
**Delivers:** Deployed OTel/metrics/request-log path for the declared profile; request/usage/billing joins; alert delivery/acknowledgement/escalation; bounded recovery actions and audit; backup/restore for every claimed state store; release/rollback and DR smoke against the stable local profile.
**Addresses:** Operator journeys, SLOs, observability, backup, rollback and disaster recovery.
**Avoids:** No-op/in-memory production sinks, telemetry as a second ledger, alert-YAML-only evidence, PostgreSQL-only recovery claims.

### Phase 8: Declared Deployment Parity

**Rationale:** Decomposition is valuable only after the commercial behavior is stable and a measured boundary justifies extraction.
**Delivers:** A decision to narrow the release to the proven monolith/dual profile or a conditional RAG parity pilot with owned datastore/migrations, complete HTTP/gRPC contracts, tenant/evidence propagation, identical journeys, target smoke and rollback. Other services follow only if the pilot and measured need justify them.
**Addresses:** Honest deployment profiles and service-level scaling/failure isolation.
**Avoids:** Health-only microservices, direct cross-service table access, big-bang Kafka/database-per-service migration.

### Phase 9: Supply Chain and Target Commercial Release

**Rationale:** Final target evidence is meaningful only after topology and all promised journeys are stable.
**Delivers:** Refreshed/pinned release toolchain; immutable images; SBOM, scan, signature and provenance; target migrations, secrets, Provider/payment/payout, storage, observability, Kubernetes, restore and rollback proof; external artifact bodies and canonical digests; same-commit strict no-skip commercial verifier.
**Addresses:** E3 target runtime and E4 commercial release completion.
**Avoids:** In-repository secrets/evidence, evidence mixed across commits or environments, local green tests reported as final readiness.

### Phase Ordering Rationale

- Ownership and release scope come first so later work does not create competing authorities or support accidental deployment modes.
- Tenant identity and outbound security precede asynchronous and external behavior because those paths amplify cross-tenant and SSRF failures.
- Durable execution precedes real E2E; otherwise browser tests can only observe simulated or orphaned state.
- Relay and the evidence spine precede financial and observability joins because they define authoritative usage and request identity.
- Real full-stack journeys precede service extraction; the same suite becomes the parity oracle.
- Financial closure and observability build on stable identities, then produce the external evidence needed for target release.
- Target proof is last and distinct: the evidence runner observes completed behavior but cannot manufacture missing runtime capability.

### Research Flags

Phases likely needing deeper research during planning:

- **Phase 1:** Refresh all LOW-confidence external version and image recommendations; choose exact supported versions/digests only after compatibility and rollback checks.
- **Phase 3:** Select the target S3-compatible service and sandbox deployment/capacity model; confirm migration and recovery constraints.
- **Phase 4:** Research each live Provider's streaming, cancellation, usage and lifecycle semantics; do not generalize one Provider's behavior.
- **Phase 6:** Confirm actual payment and payout rails, webhook contracts, refund/dispute exports, reconciliation windows, test environments and regulatory constraints.
- **Phase 7:** Confirm target observability backend, retention/access policy, recovery platform APIs and non-PostgreSQL backup/restore mechanisms.
- **Phase 8:** Research only after a measured extraction candidate and advertised deployment mode are selected.
- **Phase 9:** Refresh supply-chain tool versions, CI identity/signing method, target cluster constraints and evidence-handling policy at release cut.

Phases with established repository-grounded patterns that normally do not need a separate research phase:

- **Phase 2:** Tenant-denial and shared outbound-policy work already has approved boundaries and named repository gaps.
- **Phase 5:** The three evidence profiles and real full-stack harness pattern are already defined; planning should focus on journeys and deterministic fixtures/external rails.
- **Core of Phase 3:** PostgreSQL lease, transactional outbox, idempotency, retry, dead-letter and replay semantics are documented; deeper research is needed only for selected external storage/sandbox targets.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | MEDIUM | Repository-observed architecture is HIGH confidence. Exact external patches, EOL dates, image versions and migration paths are LOW confidence and must be refreshed before implementation. |
| Features | MEDIUM | Current capability/gap classification is HIGH confidence from the approved design and live audits; broader ecosystem ordering is MEDIUM because external research providers were unavailable. |
| Architecture | MEDIUM | The recommended ownership, outbox, identity, E2E and parity patterns are coherent and cross-sourced, but phase entry must recheck current writers, headers, stores and runtime composition. |
| Pitfalls | HIGH for repository-specific risks; MEDIUM for ordering | Multiple current repository sources agree on the critical failure modes. External ecosystem ranking was not independently refreshed. |

**Overall confidence:** MEDIUM

### Gaps to Address

- **Advertised topology is unresolved:** decide which monolith, dual, or independent-service modes the next release actually supports before parity requirements are finalized.
- **Authority inventory needs live confirmation:** enumerate all current writers of usage, billing, request logs, Marketplace settlement and Agent/Workflow/Task execution state.
- **Internal identity provenance needs live confirmation:** audit public-edge header stripping and every HTTP/gRPC adapter before trusting cross-process tenant context.
- **Job schema convergence is unresolved:** determine whether current job tables can adopt a common lease/attempt contract or need domain adapters and phased migrations.
- **Profile B dependency set is unresolved:** define which databases/services are mandatory for every local journey and which are enabled only in targeted profiles.
- **External vendors and environments are not selected or available:** Provider, payment, payout, object storage, observability, Kubernetes and signing details remain phase-entry decisions with target credentials/evidence dependencies.
- **Exact stack versions are not hard requirements:** refresh official support, package compatibility, image digests and migration paths at implementation and release cut.
- **Service extraction has no measured trigger yet:** collect load, backlog, failure-isolation and release-cadence data before committing to RAG or another pilot.
- **Enterprise-tier launch scope is unresolved:** MFA, SSO/SCIM, retention, policy packs, residency and compliance are deferred unless the commercial launch explicitly promises them.

## Sources

### Primary (HIGH confidence)

- [PROJECT.md](../PROJECT.md) - current product objective, users, invariants, active gaps, evidence hierarchy, completion definition and layered delivery strategy.
- [FEATURES.md](./FEATURES.md) - repository-grounded launch table stakes, current gaps, differentiators, anti-features and dependencies.
- [PITFALLS.md](./PITFALLS.md) - current repository failure modes, warning signs, recovery and roadmap mapping.
- `docs/superpowers/specs/2026-07-10-module-capability-reference-design.md` - approved module launch/target boundary and rejected topology-first approaches.
- `docs/audit/2026-07-10-full-repository-scan.md` - live repository maturity, P0 blockers and commercial completion criteria.
- `docs/audit/2026-07-10-module-capability-gap-matrix.md` - module-specific runtime gaps, first slices and dependency ordering.
- `docs/audit/oblivious-gap-matrix.md` - conservative Observed/Partial/Gap boundary.
- `.planning/codebase/{ARCHITECTURE,CONCERNS,STACK,TESTING}.md` and current source/config manifests - current brownfield structure and verification surfaces.

### Secondary (MEDIUM confidence)

- [ARCHITECTURE.md](./ARCHITECTURE.md) - synthesized ownership, identity, durable execution, E2E, observability, evidence and extraction patterns.
- Approved reference-project synthesis in the capability design - product-family expectations only; reference code is not Oblivious implementation evidence.
- Official PostgreSQL, OpenTelemetry, AWS outbox, Playwright, Testcontainers and Kubernetes guidance cited in ARCHITECTURE.md - pattern validation, not proof of current Oblivious runtime behavior.

### Tertiary (LOW confidence)

- [STACK.md](./STACK.md) exact external tool, runtime, database, queue, observability and supply-chain version recommendations - release-cut candidates that require fresh official verification, compatibility testing and immutable digest selection.

---
*Research completed: 2026-07-14*
*Ready for roadmap: yes*
