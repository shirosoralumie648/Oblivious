# Pitfalls Research: Commercial Target Closure

**Domain:** Brownfield multi-tenant AI SaaS commercial completion
**Project:** Oblivious
**Researched:** 2026-07-14
**Confidence:** HIGH for repository-specific failure modes; MEDIUM for general ecosystem ordering

## Research Boundary And Confidence

This document focuses on failure modes that can invalidate Oblivious's commercial-release claim or force expensive rework. Repository source, the approved capability design, current codebase map, audit matrices, release gates, and the three completed research documents are treated as primary evidence. The configured external research providers were unavailable in this runtime, so no unverified external claim is promoted into a hard requirement.

The governing distinction is the four-level evidence model:

1. fixture or unit evidence;
2. repository-local runtime evidence;
3. target-environment runtime evidence;
4. final same-commit no-skip commercial evidence.

Most critical pitfalls below are caused by silently promoting a lower evidence class to a higher one.

## Critical Pitfalls

### Pitfall 1: Evidence Laundering Into A Final Readiness Claim

**What goes wrong:**
Passing docs checks, fixture-backed browser tests, local Docker smoke, or historical Phase 30 evidence is reported as current target commercial readiness.

**Why it happens:**
The repository has strong verification tooling and extensive completion language, so green local output feels conclusive even when real Provider, payment, payout, ClickHouse, Kubernetes, secret, backup/restore, and artifact-body evidence is absent.

**How to avoid:**
Tag every requirement and verification result with its evidence class, environment, commit, command, migrations, skips, and residual risk. Require one final run with the filled external manifest, downloaded artifact bodies, real target dependencies, and all final flags enabled.

**Warning signs:**
- A completion statement cites only `.planning`, docs, fixtures, mocks, or counts.
- `COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true` appears in a final command.
- The target manifest, artifact directory, or Kubernetes secret file is missing.
- Evidence was generated from a commit different from the release candidate.

**Phase to address:**
Evidence identity and release-contract foundation, then final target release closure.

---

### Pitfall 2: Split Provider, Usage, Or Billing Authority

**What goes wrong:**
Chat, Agent, Workflow, Knowledge, MCP, or an OpenAI-compatible handler calls a Provider outside Relay, or multiple layers write usage independently without a canonical ownership rule. Quota, cost, refund, audit, and Console totals diverge.

**Why it happens:**
Feature teams optimize for a local success path, direct Provider access looks simpler, and existing Chat plus Relay usage writers can be mistaken for complementary rather than duplicate authorities.

**How to avoid:**
Keep Relay as the only billable AI path. Define one immutable usage and price-snapshot contract, one settlement idempotency key, and one request identity that joins route decision, Provider response, usage, quota, billing, refund, request log, and audit. Add bypass scans and ledger reconciliation tests.

**Warning signs:**
- Provider base URLs or SDK calls appear outside adapter/Relay packages.
- Chat and Relay both create billable rows for one request.
- Console aggregates raw rows without ownership or deduplication semantics.
- Cancellation or Provider failure lacks a terminal usage/refund record.

**Phase to address:**
Relay authority and evidence identity before real customer journey expansion.

---

### Pitfall 3: Tenant Identity Is Lost Between Boundaries

**What goes wrong:**
HTTP authorization succeeds, but organization scope disappears in a service method, SQL mutation, vector payload, cache key, background job, retry queue, gRPC call, request log, or admin query. Cross-tenant data or cost leakage becomes possible.

**Why it happens:**
Tenant checks are treated as middleware rather than an end-to-end data invariant. Async work and observability paths are added later and inherit incomplete identity envelopes.

**How to avoid:**
Make `organization_id` mandatory in domain commands, persisted work, vector filters, audit envelopes, usage records, and internal service metadata. Test representative cross-tenant reads and writes at route, service, SQL, vector, worker, and admin levels. Default caches to tenant isolation.

**Warning signs:**
- Store methods accept only a resource ID.
- Queue payloads or retry records omit organization identity.
- Qdrant/pgvector filters are optional.
- A semantic cache key contains only query and model.
- Admin analytics use global aggregation without authorization scope.

**Phase to address:**
Tenant and outbound-security closure before durable workers or service extraction.

---

### Pitfall 4: Durable Records Exist But No Runtime Owns Them

**What goes wrong:**
RAG ingestion/index jobs, Workflow transitions, Agent tool runs, or Tasks are persisted but never consumed, cannot resume after restart, or report success after only changing state fields.

**Why it happens:**
Schema, repository, API, and UI work are counted as a complete feature before production worker wiring, lease ownership, heartbeat, drain, retry, dead-letter, cancellation, and replay are proven.

**How to avoid:**
For every durable job type, define the production entrypoint, owner token, claim lease, idempotency key, retry budget, dead-letter rule, readiness/lag metric, graceful drain, and restart-replay test. Task must point to a real Agent or Workflow execution and derive status and cost from that target.

**Warning signs:**
- Enqueue code exists but no `cmd/**` or server startup path creates the worker.
- Health is green while queue lag grows.
- A Task completes without a downstream `run_id` or `execution_id`.
- Retry creates a second side effect rather than resuming idempotently.
- Debug state is memory-only after restart.

**Phase to address:**
Durable RAG and automation runtime closure before real E2E and service parity.

---

### Pitfall 5: Money Movement Without Reconciliation Authority

**What goes wrong:**
Checkout, webhook, quota credit, refund, Marketplace install, settlement, payout, or chargeback succeeds in one subsystem but not another. Duplicate delivery or retry causes double credit, install, settlement, or payout.

**Why it happens:**
Happy-path state machines are implemented before immutable event ledgers, transactional outbox, idempotency, provider references, replay, and cross-system reconciliation.

**How to avoid:**
Persist verified provider events before applying transitions. Bind payment, order, settlement, payout, refund, and audit identities. Use transactional state changes and outbox dispatch. Make reconciliation a first-class operation that compares provider exports, webhooks, internal ledgers, quota, refunds, and payout state.

**Warning signs:**
- Quota or entitlement is granted before verified payment evidence.
- Payout dispatch occurs before binding the provider request to a durable row.
- Duplicate webhooks change totals twice.
- Refund state does not update Marketplace settlement and publisher balance.
- Domestic providers are shown before live checkout and signed webhook configuration is complete.

**Phase to address:**
Financial lifecycle and Marketplace governance before paid launch proof.

---

### Pitfall 6: Outbound Features Bypass A Shared Security Policy

**What goes wrong:**
Agent API tools, Workflow HTTP nodes, MCP, channel webhooks, Provider base URLs, imports, or payout callbacks reach private networks, follow unsafe redirects, leak credentials, or behave differently across runtimes.

**Why it happens:**
Each feature implements URL checks independently. DNS resolution and redirect validation are treated as unit-level string parsing rather than a runtime policy with connection-time enforcement.

**How to avoid:**
Use one fail-closed outbound policy for scheme, credentials, DNS/IP classification, redirect hops, proxy behavior, body limits, timeout, audit, and secret attachment. Make DNS tests deterministic and revalidate the resolved destination at connection time. Separate public Provider allowlists from user-controlled URLs.

**Warning signs:**
- URL validation is duplicated across domains.
- Tests depend on the live DNS result of `example.com`.
- Redirect targets are not revalidated.
- Secrets are attached before the final destination is authorized.
- Production falls back to host-process Python or unrestricted HTTP.

**Phase to address:**
First local runtime and security closure phase.

---

### Pitfall 7: Fixture-Only E2E Hides Broken Product Wiring

**What goes wrong:**
The React workflow passes while actual Go routes, migrations, Provider streaming, workers, webhooks, and persistence fail. API fixtures drift from OpenAPI and backend serialization.

**Why it happens:**
Fixture tests are fast and deterministic, so they expand until they are mistaken for integration proof.

**How to avoid:**
Keep fixture suites for UI behavior, but add a focused real full-stack profile using the built web app, real Go server, disposable PostgreSQL/pgvector, applicable Redis/Qdrant/ClickHouse dependencies, and controlled external test rails. Fail fixtures on unknown paths and validate serializers against backend contracts.

**Warning signs:**
- Every Playwright request is intercepted with `page.route`.
- Browser evidence cannot name a persisted row or real request ID.
- Backend and frontend tests pass independently but no built-stack journey exists.
- Provider cancellation, webhook retry, worker recovery, and migration replay are absent from E2E.

**Phase to address:**
Real customer journeys after local runtime owners are proven.

---

### Pitfall 8: Deployment Topology Is Claimed Before Capability Parity

**What goes wrong:**
Kubernetes exposes Gateway, RAG, Billing, Marketplace, Observability, or other independent services that provide only health or partial behavior while documentation claims a production microservice architecture.

**Why it happens:**
Manifests, proto files, service commands, and health endpoints create the appearance of decomposition before data ownership, identity propagation, retries, migrations, observability, and target smoke are complete.

**How to avoid:**
Declare only deployment modes that pass a parity matrix. Extract one bounded service only when it owns its data and lifecycle, has complete HTTP/gRPC contracts, receives trusted tenant/request metadata, supports migrations and rollback, and passes the same user journey as the monolith. Remove incomplete services from default release topology.

**Warning signs:**
- A service registers only health routes.
- `HealthCheckTargets` or upstream routes are empty.
- Proto responses are synthetic or not backed by the monolith domain store.
- The monolith and split mode produce different billing or authorization results.
- Raw stateful manifests are presented as HA without failover proof.

**Phase to address:**
Declared deployment parity after the monolith/dual-mode commercial baseline is stable.

---

### Pitfall 9: Local Files Or Mutable Images Become Production State

**What goes wrong:**
Knowledge uploads, Relay Files, logs, or evidence disappear across replicas or restart; mutable container tags produce a different artifact from the one tested.

**Why it happens:**
Local directories and broad image tags work in developer Compose and are carried into a multi-replica target without an ownership, retention, digest, backup, or restore contract.

**How to avoid:**
Use a target-owned S3-compatible contract for shared blobs, with checksum, malware scan, retention, tombstone, orphan cleanup, tenant prefix, backup, and restore proof. Pin every release image by digest, generate SBOM/provenance, sign the digest, and deploy exactly that digest.

**Warning signs:**
- Pods mount ephemeral or node-local upload directories.
- Database rows point to files that are not included in recovery tests.
- `latest`, broad major tags, or local image names appear in final manifests.
- Evidence contains a commit SHA but not the deployed image digest.

**Phase to address:**
Storage and supply-chain closure before target deployment proof.

---

### Pitfall 10: Observability Becomes A Second, Inconsistent Ledger

**What goes wrong:**
Metrics, ClickHouse request logs, traces, billing rows, and audit events use different IDs or totals. Alerts fire without delivery proof, recovery is audit-only, and operators cannot reconstruct one incident or customer charge.

**Why it happens:**
Telemetry is treated as optional instrumentation rather than a required projection of authoritative domain events.

**How to avoid:**
Propagate the shared evidence identity to logs, traces, metrics exemplars, alerts, and recovery records. Keep billing authority in the ledger and use observability as a queryable projection. Prove ingest, query, alert delivery, acknowledgement, recovery action, and post-recovery audit in the target environment.

**Warning signs:**
- Request logs omit `request_id` or organization identity.
- Usage totals differ between Console, billing and ClickHouse.
- Alert tests validate YAML only.
- A recovery controller records intent but no platform action or outcome.
- Production silently uses an in-memory/no-op reporter.

**Phase to address:**
Observability and recovery before final target evidence collection.

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Keep broad container tags | Faster updates | Non-reproducible evidence and unreviewed drift | Local development only |
| Add another direct Provider client | Fast feature demo | Split pricing, usage, quota and audit | Never for billable production AI |
| Store tenant scope only in context | Less schema work | Async and replay paths lose authorization | Never for durable customer state |
| Use dual writes without outbox | Simple integration | Partial commits and unreconcilable retries | Never for money or durable execution |
| Mark unsupported APIs experimental but enabled | Broader compatibility claim | Unbilled or unaudited lifecycle behavior | Never in commercial defaults |
| Keep raw bytes or files on local disk | Easy implementation | Replica, restore and retention failure | Disposable local profile only |
| Depend on fixture-only E2E | Fast stable CI | Integration drift remains invisible | As a lower evidence class only |
| Split services before parity | Architectural progress | Duplicate contracts and operating burden | Only after a measured extraction gate |
| Ignore duplicate package lock families | Less maintenance | Security scan and dependency drift | Never while both are tracked |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| LLM Providers | Assume final streaming usage always arrives | Record partial/cancel state and reconcile according to Provider capability |
| Stripe/Alipay/WeChat Pay | Trust redirect success or unsigned payloads | Apply only signature-verified, idempotent Provider events |
| Marketplace payout | Dispatch before durable binding | Persist payout/outbox identity, then dispatch and reconcile |
| Qdrant/pgvector | Filter after retrieval | Enforce organization and document/version filters in the vector query |
| ClickHouse | Count request logs as billing authority | Join to immutable usage/price/settlement records by shared request identity |
| Redis/Asynq/Kafka | Treat delivery as exactly once | Make consumers idempotent and prove replay, lease expiry and dead-letter recovery |
| S3-compatible storage | Rely on bucket existence only | Prove tenant prefixing, checksum, scanning, retention, tombstone and restore |
| Kubernetes | Validate YAML without runtime smoke | Prove rollout, migration, probes, secret class, failover and rollback on the target cluster |
| OpenTelemetry | Export secrets or high-cardinality customer content | Apply redaction, bounded attributes and stable identity fields in the Collector |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Per-request synchronous cross-system reconciliation | High tail latency and cascading failures | Commit authoritative local state, reconcile asynchronously with bounded retry | Provider or analytics latency spikes |
| Unbounded streaming buffers | Memory growth and slow cancellation | Backpressure, chunk limits, prompt cancellation and terminal usage state | Long responses or slow clients |
| Tenant filters added after vector search | Low recall or cross-tenant risk | Tenant-aware index/query strategy and recall benchmarks | Large shared collections |
| Unbounded workflow/agent events | Slow debug queries and storage growth | Retention, pagination, archive and trace sampling contracts | Long-running or high-frequency workflows |
| One global queue | Head-of-line blocking across domains/tenants | Priority, concurrency and tenant-aware quotas with observable lag | Provider outage or ingestion burst |
| High-cardinality metrics labels | Prometheus instability | Keep customer/request IDs in logs/traces; use bounded metric dimensions | Production traffic scale |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Trust internal identity headers from public traffic | Tenant impersonation | Strip at the edge and mint trusted metadata only after authentication |
| Reuse Provider secrets across organizations | Cross-tenant cost and data exposure | Encrypt tenant-owned credentials and bind access to organization policy |
| Execute code when sandbox is unavailable | Remote code execution and host compromise | Production fail closed with isolated resource-limited runners only |
| Persist raw prompts/responses without policy | Sensitive-data leakage | Configurable retention, redaction, access control and deletion evidence |
| Embed secrets in target evidence | Credential leakage into artifacts or git | External controlled evidence workdir, secret audit and safe metadata only |
| Share semantic cache entries by query/model alone | Cross-context and cross-tenant response leakage | Full context fingerprint, privacy classification and tenant isolation by default |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Disabled capability looks available | Users hit late runtime failures | Hide it or show explicit configuration/availability state before action |
| Async work has no durable progress | Uploads and runs appear frozen | Show queued/running/retrying/dead-letter/cancelled state and remediation |
| Usage and price are only visible after failure | Customers cannot manage cost | Show budget, quota, route and estimated/authoritative usage context |
| Approval does not show exact side effect | Users approve unsafe tools blindly | Present tool, arguments, destination, credential scope, cost and timeout |
| Checkout success implies entitlement immediately | Users see inconsistent purchase state | Show pending verification and update from authoritative webhook lifecycle |
| Operator errors expose internal detail | Security leakage and poor recovery | Return stable public codes with request ID and operator-side diagnostics |

## Looks Done But Isn't Checklist

- [ ] **Relay endpoint:** A route and handler exist; verify identity, pricing, quota, settlement, refund, request log, cancellation and target Provider proof.
- [ ] **RAG upload:** The API accepts a file; verify a production worker consumes it, vectors become queryable, citations are correct, retries recover and deletes remove stale vectors.
- [ ] **Agent or Workflow:** The UI shows completed; verify durable tool/node state, restart replay, cancellation, budget, sandbox and target traces.
- [ ] **Task:** Steps change status; verify a real Agent/Workflow target ran and its actual usage and terminal state were linked.
- [ ] **Payment:** Checkout returns a URL; verify signed webhook, idempotent transition, refund, reconciliation and no premature quota or entitlement.
- [ ] **Marketplace payout:** A payout row exists; verify external dispatch, webhook lifecycle, retry/replay, reconciliation and refund impact.
- [ ] **Microservice:** A command, proto and health endpoint exist; verify full behavior parity, data ownership, trusted identity and target smoke.
- [ ] **Observability:** Dashboards and alert YAML exist; verify live ingest/query, alert delivery, acknowledgement, recovery action and audit IDs.
- [ ] **Kubernetes:** Manifests validate; verify external secrets, migrations, rollout, probes, failover, backup/restore and rollback in the target cluster.
- [ ] **Commercial verifier:** The script exits zero locally; verify same-commit target manifest, artifact bodies and all no-skip flags.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| False readiness claim | MEDIUM | Retract claim, classify existing evidence, rerun preflight, collect missing target artifacts and publish residual risk |
| Duplicate usage or settlement | HIGH | Freeze affected flows, identify canonical IDs, reconcile ledger, compensate balances and add idempotency regression tests |
| Tenant leakage | HIGH | Disable affected path, preserve audit evidence, rotate secrets if needed, repair scope, notify through incident policy and add cross-tenant tests |
| Orphaned durable jobs | MEDIUM | Stop new enqueue, deploy an owner worker, reclaim expired leases, replay idempotently and inspect dead-letter rows |
| Unsafe outbound behavior | HIGH | Disable external action, rotate exposed credentials, fix shared policy, scan audit logs and prove DNS/redirect cases deterministically |
| Service parity failure | MEDIUM | Remove split mode from advertised/default topology, route to proven monolith behavior, then re-enter extraction gate |
| Missing blob or recovery state | HIGH | Restore from authoritative backup/object store, reconcile references, add checksum/orphan scan and expand DR scope |
| Mutable artifact mismatch | HIGH | Reject release, rebuild from the candidate commit, pin digest, regenerate SBOM/provenance/signature and recollect evidence |

## Pitfall-To-Roadmap Mapping

| Pitfall | Prevention Phase Topic | Verification |
|---------|------------------------|--------------|
| Tenant loss and unsafe outbound | Tenant and outbound-security closure | Cross-tenant route/service/SQL/vector/job denial plus deterministic DNS/redirect tests |
| Split Relay/usage authority | Evidence identity and Relay authority | One request joins route, usage, quota, price, billing, request log, audit and refund |
| Orphaned durable work | RAG and automation runtime closure | Enqueue, worker ownership, retry, dead-letter, drain, restart replay and real Task target |
| Money lifecycle divergence | Billing and Marketplace financial closure | Duplicate webhook/retry/refund/payout reconciliation against Provider evidence |
| Fixture-only journeys | Real full-stack customer journeys | Built Web -> Go -> DB -> controlled external rail with persisted IDs and no API interception |
| Local/mutable production state | Shared storage and supply-chain closure | Blob restore/orphan scan plus digest/SBOM/provenance/signature verification |
| Observability islands | Observability, SLO and recovery closure | Live request-to-ledger query, alert delivery, recovery action and post-recovery audit |
| Premature service split | Declared deployment parity | Same journey and contract suite in monolith and each advertised split mode |
| Evidence laundering | Target release closure | External manifest/artifact bodies, target dependencies and same-commit no-skip verifier |

## Sources

### Primary Repository Sources

- `.planning/PROJECT.md` - product objective, invariants, evidence hierarchy and completion definition.
- `.planning/codebase/CONCERNS.md` - release boundaries, complexity hotspots, integration risks and false-positive filters.
- `.planning/research/STACK.md` - retained stack, conditional topology, version confidence and supply-chain recommendations.
- `.planning/research/FEATURES.md` - launch baseline, dependencies, anti-features and evidence expectations.
- `.planning/research/ARCHITECTURE.md` - authority ownership, identity flow, durable worker pattern, E2E profiles and extraction gates.
- `docs/audit/2026-07-10-full-repository-scan.md` - current module depth, P0 blockers and complete-release criteria.
- `docs/audit/2026-07-10-module-capability-gap-matrix.md` - module-specific runtime and target gaps.
- `docs/audit/oblivious-gap-matrix.md` - conservative Observed/Partial/Gap boundary.
- `docs/release/commercial-gates.md` - commercial gate and no-final-readiness contract.
- `docs/release/rc-checklist.md` - target evidence inputs and operator release procedure.

### Confidence Note

Repository-specific findings are HIGH confidence because multiple current sources agree. General ordering recommendations are MEDIUM confidence and should be refreshed during phase planning if external research providers become available. No LOW-confidence external version claim is used as a pitfall requirement.

---
*Pitfalls research for: Oblivious Commercial Complete & Target Release*
*Researched: 2026-07-14*
