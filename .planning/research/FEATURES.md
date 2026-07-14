# Feature Research

**Domain:** Organization-focused commercial multi-tenant AI SaaS
**Researched:** 2026-07-14
**Overall confidence:** MEDIUM
**Project-state confidence:** HIGH - approved capability contract and two live repository audits agree on the current module boundaries and gaps.
**Ecosystem-classification confidence:** MEDIUM - the approved reference synthesis covers the relevant product families, but the research-plan-selected external providers were unavailable in this runtime, so no unverified external claims are promoted as authoritative.

## Research Scope and Evidence Vocabulary

This is a subsequent commercial-completion milestone for an existing brownfield product. It is not a greenfield feature wishlist and does not reduce Oblivious to an MVP or RC. Features are classified by what an organization must be able to do in a real customer journey, not by route, page, schema, fixture, or reference-code existence.

**Current-state labels:**

- **Existing:** substantive production code and repository-local evidence exist; target proof may still be absent.
- **Partial:** meaningful implementation exists, but a runtime, security, integration, recovery, or evidence link is incomplete.
- **Gap:** the promised journey is simulated, unassembled, health-only, fixture-only, or absent.

**Evidence classes:**

- **E1 - fixture/unit:** isolated unit, contract, fixture, or mocked behavior.
- **E2 - repository runtime:** real local code, database, and applicable local service execution.
- **E3 - target runtime:** real target infrastructure, provider, payment rail, cluster, and recovery evidence.
- **E4 - commercial release:** same-commit, no-skip final verifier plus immutable external evidence pack.

Lower classes do not substitute for higher classes. A launch feature normally needs E2 before target execution and E3/E4 before final commercial-readiness claims.

## Feature Landscape

### Table Stakes (Commercial Launch Expectations)

Missing any row that remains in the public launch promise makes the product commercially incomplete. Fixed counts of providers, tools, channels, or workflow nodes are not requirements; lifecycle depth and evidence are.

| Feature / customer journey | Why expected | Current capability vs gap | Complexity | Key dependencies | Current -> required evidence | Confidence |
|---|---|---|---|---|---|---|
| Organization onboarding and secure sessions | Organization customers must register, join/select a tenant, recover access, and remain inside explicit roles | **Existing/Partial:** registration, SQL sessions, rotation, CSRF, organizations and memberships exist; permission matrix, cross-route enforcement, trusted identity propagation, and production recovery delivery remain incomplete | HIGH | Identity store, session security, tenant middleware, audit | E1/E2 -> E3/E4 | HIGH |
| Tenant isolation on every state path | Cross-tenant data exposure is a launch-blocking failure, including vector payloads, jobs, retries and admin queries | **Partial:** core scoping exists; uniform positive and deny evidence across HTTP/gRPC, service, SQL, vector, queue and admin layers is still required | HIGH | Organization identity, authorization policy, test data, shared service metadata | E1/E2 -> E3/E4 | HIGH |
| Real browser customer journeys | Customers buy a product journey, not a set of API handlers | **Gap:** current browser coverage is largely API-fixture based; signup, org setup, Chat, RAG, Agent, Workflow/Task, billing and Marketplace must reach real Web, Go, PostgreSQL and applicable external systems | HIGH | Stable local stack, seeded plans/providers, Playwright, trace capture | E1 -> E2, then E3/E4 | HIGH |
| Relay as the only billable AI authority | Routing, quota, pricing, usage, audit and monitoring must agree for every AI surface | **Existing/Partial:** Relay is the deepest module; lifecycle coverage and proof are incomplete for Realtime, Batch, Files and some cross-surface joins | HIGH | Tenant identity, provider registry, pricing, quota ledger, request log | E1/E2 -> E3/E4 | HIGH |
| Provider lifecycle with streaming, cancellation and compensation | A commercial model call needs true upstream behavior, cancellation, usage capture, retries and failure accounting | **Partial:** primary Relay paths exist; target provider SSE, abort settlement, fallback decisions and usage/billing/request-log reconciliation remain unproven | HIGH | Relay authority, provider credentials, route policy, ClickHouse, billing | E1/E2 -> E3/E4 | HIGH |
| Shared evidence identity spine | Support, finance and operations must join request, execution, usage, billing, payment, settlement, audit and trace records | **Partial:** identifiers and several joins exist, but the full cross-module spine is not yet proven | HIGH | ID propagation, immutable records, request-log sink, reconciliation APIs | E1/E2 -> E3/E4 | HIGH |
| Durable and recoverable Chat | Messages must persist; real SSE, retry, cancel, usage and provider failures must leave coherent state | **Existing/Partial:** CRUD, branching, export/share and Relay access exist; true browser-to-provider streaming, final usage and partial-message compensation are gaps | HIGH | Relay provider lifecycle, conversation store, browser E2E, evidence spine | E1/E2 -> E3/E4 | HIGH |
| Durable Knowledge/RAG lifecycle | Upload must progress through parse, chunk, embed, index, retrieve, cite, update and delete with visible failures | **Partial:** jobs, lease/retry/dead-letter, Qdrant and citations exist; production worker startup, shared object storage and target drain/recovery proof are missing | HIGH | PostgreSQL, pgvector/Qdrant, workers, object storage, Relay embeddings | E1/E2 -> E3/E4 | HIGH |
| Retrieval trust and diagnostics | Users need citations and operators need relevance/debug evidence, not fixture vectors or opaque text search | **Existing/Partial:** citation metadata and hybrid/vector paths exist; target retrieval, version filters and quality evaluation remain incomplete | MEDIUM | RAG lifecycle, tenant filters, test cases, evaluation data | E1/E2 -> E3 | HIGH |
| Durable Agent/SOLO execution | Agent runs need persisted plan/tool/memory state, approval, budget, retry, cancel and coherent billing | **Existing/Partial:** run/tool/plan/memory and approvals are substantial; structured live streaming, restart continuation, sandbox target proof and unified execution identity remain incomplete | HIGH | Relay, tool policy, sandbox, evidence spine, RAG/MCP | E1/E2 -> E3/E4 | HIGH |
| Durable Workflow execution and replay | Workflows must survive restarts and preserve node state, variables, failure policy and debug history | **Existing/Partial:** versions, events, trace/snapshot/replay and state machine exist; HTTP-node security, distributed lease/restart proof and complete transition durability remain gaps | HIGH | Durable store, outbound policy, scheduler, sandbox, observability | E1/E2 -> E3/E4 | HIGH |
| Task/Scheduler invokes real targets | A task that only advances status is not automation | **Gap:** scheduled claims exist, but SOLO Task still simulates steps/budget instead of invoking a real Agent or Workflow and preserving downstream identity | HIGH | Agent/Workflow APIs, idempotency, usage aggregation, cancellation | E1 -> E2/E3 | HIGH |
| Tenant-safe tools, MCP and outbound access | Custom URLs, webhooks, MCP, HTTP nodes and payout endpoints are a shared SSRF and credential boundary | **Partial:** registries and risk metadata exist; one fail-closed DNS/IP/redirect policy, OAuth/credential lifecycle and complete telemetry must cover every outbound consumer | HIGH | Shared outbound policy, secretbox, approval policy, audit | E1/E2 -> E3/E4 | HIGH |
| Production sandbox authority | Commercial code execution cannot fall back to the host process | **Partial:** Docker isolation path exists; all code paths, target capacity, cancellation, artifact/log retention and deployment proof are incomplete | HIGH | SandboxRunner, container runtime, object storage, queue/capacity controls | E1/E2 -> E3 | HIGH |
| Metering, quota and immutable billing lifecycle | Organizations expect usage limits, price transparency, invoices and reversible, auditable charges | **Existing/Partial:** quota, Stripe, subscriptions, top-ups, invoices, refunds and price snapshots exist; independent Billing parity and cross-system reconciliation remain incomplete | HIGH | Relay usage authority, ledger transactions, payment provider, admin | E1/E2 -> E3/E4 | HIGH |
| Payment, refund and reconciliation | A checkout success without signed webhooks, idempotency, refund and reconciliation is not a financial closure | **Partial:** Stripe lifecycle is substantial; scheduled reconciliation, exception handling and target rail proof remain gaps; domestic rails stay disabled until complete | HIGH | Immutable payment ledger, webhook security, accounting joins, alerts | E1/E2 -> E3/E4 | HIGH |
| Marketplace governance and paid order lifecycle | Publishers and buyers need review, entitlement, orders, refunds, settlements, payout and appeal history | **Existing/Partial:** publish/review/install/order/settlement/payout outbox and governance states exist; external payout, webhook, refund/chargeback impact and operator reconciliation are unproven | HIGH | Identity, billing, payment/payout provider, audit, policy review | E1/E2 -> E3/E4 | HIGH |
| At least one real publishing channel when channels are promised | Channel configuration without signed receive/send, receipt, retry and duplicate protection is not a product integration | **Partial:** adapters, logs, UI and retry structures exist; no real platform has complete target send/receive/receipt/rotation evidence | HIGH | Shared conversation identity, webhook security, credentials, retry/DLQ | E1/E2 -> E3 | HIGH |
| Admin and customer console backed by real operations | Operators need safe control, not pages that advertise unsupported providers or mutate no real backend | **Existing/Partial:** broad monolith API/UI exists; unified permissions, mutation audit, provider readiness state, reconciliation and independent-service parity remain incomplete | HIGH | Authorization matrix, audit envelope, all domain services | E1/E2 -> E3/E4 | HIGH |
| Persistent observability and audit | Commercial operations require request/usage/billing joins, SLOs, alert delivery and recovery history | **Partial:** metrics, request-log and alert components exist; independent service uses in-memory paths and target ClickHouse/alert/recovery proof is missing | HIGH | Evidence spine, ClickHouse, trace backend, alert sinks, retention policy | E1/E2 -> E3/E4 | HIGH |
| Deployment, rollback, backup and disaster recovery | A deployable SaaS must prove it can upgrade, roll back and restore state without relying on documentation alone | **Existing/Partial:** Compose/Kubernetes assets, migrations, PostgreSQL restore and runbooks are strong locally; target cluster, non-Postgres state recovery and drills remain gaps | HIGH | Immutable artifacts, secrets, data inventory, target cluster, runbooks | E2 -> E3/E4 | HIGH |
| Declared deployment-mode parity | Customers must receive the same behavior whether the supported runtime is monolith, dual-mode or independent services | **Gap/Partial:** Relay is comparatively deep; Gateway, Billing, RAG, Marketplace and Observability independent entrypoints are thin or health-only | HIGH | Local runtime closure, gRPC contracts, DB ownership, identity/trace propagation | E1/E2 -> E3/E4 | HIGH |
| Reproducible and attestable release artifacts | Commercial release evidence must bind tested code to deployed images and permit rollback | **Gap/Partial:** validation scripts exist; formal image build/push/sign, immutable digest, SBOM/provenance and full-state recovery evidence remain incomplete | HIGH | CI, registry, signing identity, SBOM, artifact manifest | E1/E2 -> E3/E4 | HIGH |
| Same-commit no-skip target release proof | Repository-local green tests do not prove real providers, payments, observability, cluster or recovery | **Gap:** target runner/tooling exists but current E4 proof across every commercial gate is absent | HIGH | All launch rows, external evidence workdir, artifact digests, strict verifier | E1/E2 -> E4 | HIGH |

### Conditional Table Stakes for Enterprise Plans

These are not required to prove the initial commercial launch baseline for smaller organizations, but become table stakes once Oblivious sells an enterprise tier or claims enterprise identity, governance, regional, or compliance readiness.

| Feature | Enterprise expectation | Current state | Complexity | Dependencies | Evidence target | Confidence |
|---|---|---|---|---|---|---|
| MFA and recovery codes | Strong authentication and recoverable account security | **Gap/Target** | MEDIUM | Session security, recovery policy, audit | E2/E3 | MEDIUM |
| OIDC/SAML SSO and verified domains | Centralized workforce identity and domain ownership | **Gap/Target** | HIGH | Tenant model, role mapping, IdP metadata, audit | E2/E3 | MEDIUM |
| SCIM provisioning/deprovisioning | Joiner/mover/leaver automation for larger organizations | **Gap/Target** | HIGH | SSO identity mapping, role policy, idempotency | E2/E3 | MEDIUM |
| Fine-grained roles, access review and dormant-account policy | Least privilege and periodic governance | **Partial/Target** | HIGH | Permission matrix, audit history, admin workflows | E2/E3 | HIGH |
| Configurable retention, legal hold and export | Organization data-governance control | **Gap/Target** | HIGH | Data inventory, object/vector deletion, audit | E2/E3 | MEDIUM |
| Multi-region routing, residency and recovery | Regional latency, availability and contractual data boundaries | **Gap/Target** | HIGH | Service parity, data replication, failover, DR | E3/E4 | MEDIUM |
| Policy packs and tenant allowlists | Central governance for models, tools, network access and data use | **Partial/Target** | HIGH | Provider catalog, tool policy, outbound policy, roles | E2/E3 | MEDIUM |
| Compliance evidence retention and penetration testing | Enterprise procurement and control assurance | **Gap/Target** | HIGH | Supply chain, audit, security testing, evidence store | E3/E4 | MEDIUM |

### Differentiators (Competitive Advantage)

These should deepen Oblivious's core value after table stakes are closed. They are valuable because they unify domains that are often separate products, not because they maximize feature counts.

| Feature | Value proposition | Current capability vs gap | Complexity | Dependencies | Evidence target | Confidence |
|---|---|---|---|---|---|---|
| One Relay governance plane across Chat, RAG, Agent, Workflow, MCP and `/v1/*` | Gives organizations one enforceable route, price, quota, policy and audit model | **Partial differentiator:** invariant exists; full lifecycle and target proof remain | HIGH | Relay table stakes, evidence spine, billing | E3/E4 | HIGH |
| Request-to-revenue evidence spine | Lets support and finance trace one request through run, usage, bill, payment, settlement and audit | **Partial differentiator:** identifiers exist but complete join is not proven | HIGH | Shared identities, immutable ledgers, ClickHouse | E3/E4 | HIGH |
| Durable AI execution with human takeover | Makes long-running AI automation controllable, recoverable and suitable for important work | **Partial differentiator:** approval/replay structures exist; restart continuation and takeover mature later | HIGH | Agent/Workflow/Task durability, authorization, observability | E3 | HIGH |
| Explainable, recoverable RAG | Combines citations and diagnostics with durable ingestion, reindex and stale-vector cleanup | **Partial differentiator:** deeper than a text-search facade; worker and target proof gaps remain | HIGH | RAG baseline, object storage, evaluation | E3 | HIGH |
| Integrated Marketplace financial governance | Unifies creator distribution with paid entitlement, refund impact, settlement, payout and appeals | **Partial differentiator:** core state exists; external rail/reconciliation incomplete | HIGH | Billing, payout, review, audit | E3/E4 | HIGH |
| Evidence-first commercial release workflow | Makes readiness claims reproducible and explicitly separates fixture, local, target and final proof | **Partial differentiator:** verifier/tooling exists; no current E4 closure | HIGH | All launch gates, supply-chain artifacts | E4 | HIGH |
| Shared fail-closed outbound and tool policy | Reduces a broad class of SSRF, secret and uncontrolled-tool risks across every automation surface | **Partial differentiator:** policy work exists but is not universally wired/proven | HIGH | Outbound validator, DNS/redirect checks, credentials, audit | E2/E3 | HIGH |
| Deployment parity without forcing premature decomposition | Lets customers start on a commercially usable runtime and adopt independent services only when parity is proven | **Gap/Target differentiator:** current independent services are uneven | HIGH | Contract ownership, service parity, target smoke | E3/E4 | HIGH |
| Quality and cost control center | Joins retrieval/Agent/Workflow quality, provider health, cost, margin and anomalies for operators | **Target differentiator** | HIGH | Evaluation datasets, provider scoring, observability, billing | E3 | MEDIUM |
| Governed multi-channel application identity | Preserves the same tenant, conversation, execution, usage and audit identity across supported channels | **Partial/Target differentiator** | HIGH | One real channel baseline, shared identity spine | E3 | MEDIUM |
| Creator analytics and policy-aware recommendations | Improves Marketplace discovery without weakening review, entitlement or financial controls | **Target differentiator** | HIGH | Mature Marketplace events, governance, privacy controls | E2/E3 | MEDIUM |
| Multi-agent teams and supervisor patterns | Expands complex automation after single-Agent durability and cost controls are trustworthy | **Target differentiator** | HIGH | Agent restart durability, budgets, tool policy, evaluation | E2/E3 | MEDIUM |
| Knowledge graphs and advanced retrieval evaluation | Adds relationship-aware retrieval where measured quality justifies complexity | **Target differentiator** | HIGH | Stable RAG lifecycle, evaluation corpus, tenant-safe graph store | E2/E3 | MEDIUM |

### Anti-Features (Commonly Requested, Commercially Harmful)

| Anti-feature | Why it is requested | Why problematic | Preferred alternative |
|---|---|---|---|
| Fixed targets such as 100+ providers, 150+ tools, 10+ channels or 20+ nodes | Creates an impressive feature count | Rewards shallow adapters and unproven lifecycle claims; multiplies security and support burden | Support a capability only after identity, policy, usage, billing, failure, operations and target evidence are complete |
| Wholesale copying from a reference project | Appears faster than product-specific design | Imports incompatible identity, billing, state and deployment assumptions; reference code is not Oblivious evidence | Borrow interaction and capability patterns, preserve Oblivious contracts |
| Big-bang rewrite to a fixed microservice count or frontend framework | Makes architecture diagrams look aligned | Creates migration risk before customer journeys work and can destroy current contracts | Close the monolith/dual-mode baseline first, then prove parity service by service |
| A second provider or billing runtime outside Relay | Lets a feature ship independently | Splits quota, pricing, usage, audit and incident evidence | Route every billable AI operation through Relay adapters and shared billing policy |
| Advertising an API before its complete commercial lifecycle exists | Expands apparent compatibility | Leaves missing identity, settlement, refund, retention or audit behavior | Keep unsupported lifecycle APIs disabled and fail closed |
| Counting a page, route, schema, proto or health endpoint as complete | Easy to demonstrate locally | Does not prove real state mutation, external integration or recovery | Require a real customer action plus evidence at the appropriate class |
| Fixture-only browser journeys | Makes E2E tests fast and stable | Hides Web/Go/DB/provider contract and deployment failures | Maintain fixtures for UI speed, but gate launch on a focused real full-stack suite |
| Production demo, fake provider, local payout or simulated telemetry fallback | Keeps demos running when dependencies are absent | Converts missing commercial dependencies into misleading success | Fail explicitly with operator-visible remediation |
| Host-process execution when sandbox capacity is unavailable | Avoids sandbox deployment work | Breaks tenant isolation and creates remote-code execution risk | Fail closed; queue or reject until isolated capacity is healthy |
| Cross-tenant semantic caching without full context governance | Promises cost reduction | Query-only keys can leak or return responses generated under different prompts, policies or tenant context | Default tenant isolation; use complete context fingerprints and explicit shareability policy |
| Enabling many channels before one real channel is complete | Maximizes integration logos | Leaves signing, receipt, retry, rotation and support semantics unproven everywhere | Prove one channel end to end, then reuse the adapter contract |
| Treating repository-local green tests as final readiness | Produces an easy completion claim | Cannot prove target secrets, providers, payment rails, clusters, observability or recovery | Preserve four evidence classes and require same-commit E4 proof |
| Committing target secrets or raw external evidence bodies | Makes evidence easy to find | Leaks credentials/customer data and couples proof to git history | Keep manifests and artifact bodies in a controlled external workdir; commit only safe contracts and digests |
| Using MAU, GMV or call volume as repository completion gates | Adds business-looking metrics | These depend on post-launch adoption, not implementation correctness | Track them after launch; use journey, SLO and evidence gates for engineering completion |
| Automatic remediation without review boundaries | Sounds operationally mature | Can amplify bad policy or destructive actions | Start with observable, bounded and auditable remediation; require approval for high-impact actions |

## Feature Dependencies

```text
[Identity + Tenant + Permission Matrix]
    -> [Shared Evidence Identity + Audit Envelope]
        -> [Relay Authority + Provider Lifecycle]
            -> [Chat]
            -> [RAG Workers + Retrieval]
            -> [Agent + Tools + Sandbox]
            -> [Workflow + Replay]
                -> [Task invokes real Agent/Workflow targets]

[Shared Outbound Security + Secret Lifecycle]
    -> [Agent API tools, Workflow HTTP nodes, MCP, Webhooks, Channels, Payout]

[Relay Usage + Immutable Pricing]
    -> [Quota + Billing Ledger]
        -> [Checkout + Refund + Reconciliation]
            -> [Paid Marketplace + Settlement + Payout]

[Persistent Request Log + Trace + Alert Delivery]
    -> [SLO and Recovery Proof]
        -> [Real Full-Stack Journeys]
            -> [Declared Deployment Parity]
                -> [Signed Artifacts + Target Evidence]
                    -> [Same-Commit No-Skip Commercial Release]

[Commercial Launch Baseline]
    -> [Enterprise Identity/Retention/Policy]
    -> [Advanced RAG Evaluation/Knowledge Graph]
    -> [Multi-Agent Teams]
    -> [Additional Channels]
    -> [Multi-Region/Compliance Expansion]
```

### Dependency Notes

- **Tenant identity precedes every product domain:** authorization cannot be bolted onto vector payloads, queues, admin queries or service calls after launch.
- **Evidence identity precedes reconciliation:** request, run, usage, billing, payment and settlement records need stable join keys before target evidence is collected.
- **Relay precedes AI customer journeys:** Chat, RAG embeddings, Agent, Workflow and MCP cannot each invent provider, usage or pricing behavior.
- **Outbound policy precedes external automation:** every URL-consuming feature shares DNS, IP, redirect, credential and audit risks.
- **Durable Agent/Workflow precedes Task:** Task completion must reference a real downstream execution and aggregate actual usage.
- **Billing precedes paid Marketplace:** payout and settlement correctness depends on immutable usage/order/payment/refund events.
- **One real channel precedes channel expansion:** adapter breadth should follow a proven signing, receipt, retry, idempotency and rotation contract.
- **Local runtime closure precedes microservice parity:** decomposing incomplete behavior multiplies failure modes and evidence work.
- **All launch gates precede E4:** the target runner assembles evidence; it cannot manufacture missing production behavior.

## Commercial Launch Baseline

This baseline is the minimum evidence-backed commercial closure of the existing product, not a reduced MVP. Existing capabilities are preserved and repaired; promised features that cannot meet the baseline must be removed from launch claims or remain explicitly disabled.

### Launch With

- [ ] **Secure organization journey:** signup/login, active organization, role enforcement, session security and cross-tenant denial across route, service, SQL, vector and job layers.
- [ ] **Authoritative Relay journey:** real provider streaming and cancellation with one request identity joining route decision, usage, quota, price snapshot, billing, request log and audit.
- [ ] **Real customer product journeys:** focused no-fixture browser flows for Chat, RAG, Agent, Workflow/Task, Billing and Marketplace against real Go and PostgreSQL.
- [ ] **Durable RAG:** ingestion and index workers are production-wired, recover after failure, drain to pgvector/Qdrant, preserve citations and clean stale vectors.
- [ ] **Durable automation:** Agent and Workflow persist state and failures; Task invokes real Agent/Workflow targets; tools and code run under approval, outbound and sandbox policy.
- [ ] **Financial closure:** subscription/top-up/usage/quota/invoice/refund and paid Marketplace order/settlement/payout states are idempotent, replayable and reconciled against real rails.
- [ ] **Operational closure:** persistent request logs, traces, SLOs, alert delivery, recovery audit, target backup/restore and rollback are demonstrated.
- [ ] **Honest deployment claim:** every advertised runtime mode has capability parity and target smoke; incomplete independent services are removed from the launch topology rather than marketed as ready.
- [ ] **Supply-chain and target closure:** reproducible immutable artifacts, signing, SBOM/provenance, external evidence pack and the same-commit no-skip commercial verifier succeed.

### Existing Capabilities to Preserve, Not Rebuild

- Organization, membership, session, tenant middleware and audit foundations.
- Relay provider/channel routing, fail-closed policy, price snapshots, quota/settlement, usage and request-log foundations.
- Chat, Agent/SOLO, Knowledge, Workflow, Scheduled Task, Admin and Marketplace production models and UI surfaces.
- PostgreSQL migrations, pgvector/Qdrant paths, durable execution records, retries and approvals.
- Stripe, subscription, top-up, invoice, refund, Marketplace order, settlement and payout state models.
- Docker/Kubernetes assets, Prometheus/Grafana assets, migration, backup/restore and release/recovery runbooks.
- Go, Vitest, Playwright, database evidence, OpenAPI, security, commercial verifier and target-evidence tooling.

### Expand After the Launch Baseline Is Proven

| Expansion | Trigger to start | Priority |
|---|---|---|
| Fine-grained enterprise access, MFA, OIDC/SAML, SCIM and access review | Baseline tenant-deny matrix and session lifecycle are stable | P1 |
| Workflow distributed workers, checkpoint replay and Saga compensation | Single-runtime restart/replay and SLO evidence pass | P1 |
| Agent restart continuation, human takeover and live structured tool streaming | Single-Agent durability, sandbox and billing joins pass | P1 |
| Provider price synchronization, margin analytics and reconciliation control center | Immutable usage/pricing/payment joins are stable | P1 |
| Marketplace ranking, risk scoring, creator analytics and configurable fee tiers | External payout/refund/reconciliation is proven | P1 |
| Full independent-service parity | RAG parity pilot and ownership/contract gates pass | P1 |
| Advanced lifecycle-compatible Relay APIs | Each lifecycle has identity, billing, compensation, audit and target proof | P2 |
| Knowledge graph, entity extraction and advanced RAG evaluation | Baseline ingestion/retrieval has measurable quality data | P2 |
| Multi-agent teams and supervisor patterns | Single-Agent cost, policy, recovery and evaluation are mature | P2 |
| Additional publishing channels | One real channel meets signing, receipt, retry, rotation and SLO gates | P2 |
| Multi-region, residency and compliance expansion | Single-region HA, DR and supply-chain evidence are mature | P2 |
| Desktop/PWA and additional clients | Web commercial journeys and cross-device contracts are stable | P2 |

## Feature Prioritization Matrix

| Capability group | User value | Implementation cost | Current state | Priority | Primary dependency |
|---|---|---|---|---|---|
| Tenant authorization and outbound security closure | HIGH | HIGH | Partial | P0 | Identity/secret foundations |
| Shared evidence identity spine | HIGH | HIGH | Partial | P0 | Tenant context and audit envelope |
| Relay/Chat real streaming lifecycle | HIGH | HIGH | Partial | P0 | Provider, pricing, ClickHouse |
| RAG worker production wiring and recovery | HIGH | HIGH | Partial | P0 | PostgreSQL, vector store, object lifecycle |
| Task real target execution | HIGH | HIGH | Gap | P0 | Agent/Workflow execution APIs |
| Real full-stack browser journeys | HIGH | HIGH | Gap | P0 | Stable local runtime closures |
| Payment and Marketplace payout reconciliation | HIGH | HIGH | Partial | P0 | Billing ledger and external rails |
| Persistent observability, alert and recovery proof | HIGH | HIGH | Partial | P0 | Evidence spine and target sinks |
| Honest deployment parity or narrowed launch claim | HIGH | HIGH | Gap/Partial | P0 | Local runtime closure |
| Signed artifacts and no-skip target release | HIGH | HIGH | Gap/Partial | P0 | Every other launch gate |
| Enterprise identity and policy controls | HIGH for enterprise | HIGH | Gap/Partial | P1 | Baseline tenant security |
| Workflow/Agent commercial maturity | HIGH | HIGH | Partial | P1 | Durable baseline and observability |
| Provider and financial analytics | MEDIUM/HIGH | HIGH | Partial/Gap | P1 | Reconciliation spine |
| Marketplace creator operations | MEDIUM/HIGH | HIGH | Partial | P1 | Paid lifecycle closure |
| Additional APIs, channels, clients and advanced AI | MEDIUM | HIGH | Target | P2 | Proven baseline contracts |

## Product-Family Pattern Analysis

This is not implementation evidence. It records the product families already synthesized in the approved capability design and what Oblivious should take from them.

| Product family | Expected pattern | Oblivious approach |
|---|---|---|
| Customer AI workspaces (`lobe-chat`, `LibreChat`, `open-webui`) | Polished Chat, model configuration, attachments, Knowledge and recoverable interaction state | Preserve customer UX but enforce Relay, tenant, usage and evidence contracts underneath it |
| AI gateways (`new-api`, `LiteLLM`, `Bifrost`, `one-api`) | Provider adapters, routing, retries, rate limits, model catalogs and usage visibility | Make Relay the sole billable authority and require lifecycle-level compensation and target proof |
| AI application builders (`dify`, `coze-studio`, `Flowise`, `FastGPT`) | Durable Agent/Workflow/Task state, tools, triggers, debugger and publishing | Prefer restart-safe execution, human control and evidence joins over node/tool count |
| RAG platforms (`ragflow`, `MaxKB`) | Ingestion pipeline, parsers, vector lifecycle, hybrid retrieval, citations and evaluation | Require production workers, tenant-safe storage, recovery and source-grounded diagnostics |
| AI operations analytics (`Helicone`, gateway operator tools) | Request logs, cost/latency analysis, provider health and alerting | Join request evidence to usage, billing, settlement and recovery instead of keeping telemetry islands |
| Commercial marketplace platforms | Review, entitlement, transaction, settlement, payout, dispute and creator operations | Treat financial and governance lifecycles as one auditable product, with no local payout simulation |

## Sources and Confidence

| Source | Use | Confidence |
|---|---|---|
| `.planning/PROJECT.md:3-142` | Product goal, users, existing baseline, active gaps, invariants, completion and evidence model | HIGH |
| `docs/superpowers/specs/2026-07-10-module-capability-reference-design.md:9-97` | Approved platform goal, invariants and evidence classes | HIGH |
| `docs/superpowers/specs/2026-07-10-module-capability-reference-design.md:98-668` | Approved launch and complete-target capability landscape for 17 modules | HIGH |
| `docs/superpowers/specs/2026-07-10-module-capability-reference-design.md:670-723` | Delivery priority, acceptance criteria and reference guardrails | HIGH |
| `docs/audit/2026-07-10-full-repository-scan.md:44-98` | Live module maturity, verified local baseline and P0 blockers | HIGH |
| `docs/audit/2026-07-10-full-repository-scan.md:100-136` | Recommended closure sequence and commercial completion conditions | HIGH |
| `docs/audit/2026-07-10-module-capability-gap-matrix.md:14-54` | Evidence-class definitions and module-level current scores | HIGH |
| `docs/audit/2026-07-10-module-capability-gap-matrix.md:56-738` | Existing production paths, launch gaps, commercial expansion and dependencies | HIGH |
| `docs/audit/2026-07-10-module-capability-gap-matrix.md:740-823` | Cross-cutting test/evidence gaps and execution ordering | HIGH |
| `docs/audit/oblivious-gap-matrix.md:5-76` | Conservative Observed/Partial/Gap cross-check and intentionally disabled surfaces | HIGH |
| `README.md:3-49` | Current mainline product and Relay boundary | HIGH |
| `README.md:81-127` | Current quality gates and no-final-readiness boundary | HIGH |
| Approved reference-project synthesis in the module design | Ecosystem pattern families and differentiator framing; reference code was not counted as implementation | MEDIUM |

**External lookup limitation:** the mandated research-plan seam first selected Brave, but `BRAVE_API_KEY` was not configured. After availability was corrected it selected built-in `websearch`, which is not exposed in this agent runtime. Per the no-loop and honest-reporting rules, external fetches were not retried or fabricated. Therefore, current repository state and approved capability boundaries are HIGH confidence, while broad ecosystem classification and post-launch differentiator ordering remain MEDIUM confidence and should be refreshed when an external provider is available.

---
*Feature research for: Oblivious commercial completion*
*Researched: 2026-07-14*
