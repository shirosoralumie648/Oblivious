# Commercial Complete Program Design

## Context

Oblivious is already positioned as a multi-tenant AI platform that merges LobeHub-style user experience with New-API-style business operations. The core invariant remains unchanged: every AI call must go through Relay so billing, rate limiting, and monitoring are unified.

Current planning state does not yet describe a commercial complete product. The active v03.3 milestone is a mainline consolidation and release-candidate cleanup effort. Commercial billing details, production observability, Kubernetes runtime proof, and payment production rollout are explicitly listed as future or out of scope in the current requirements. This design promotes those items into the next program of work.

## Goal

Turn Oblivious from a release-candidate mainline into a directly deployable commercial SaaS product. Completion means the platform can be operated for real customers with tenant isolation, production security, complete Relay metering, commercial billing, marketplace governance, observable operations, documented deployment, and verified recovery.

This is not an MVP target. MVP wording is allowed only when describing historical state or a temporary implementation detail that must be removed before commercial completion.

## Non-Negotiable Commercial Gates

The product is not commercial complete until all gates below are proven with current repository evidence, automated verification, and runtime smoke where applicable.

### 1. Tenant And Identity Gate

- Organizations are first-class tenants, separate from individual user workspaces.
- Users can belong to multiple organizations with explicit roles.
- Tenant membership, role changes, invitations, and ownership transfers are audited.
- All application data that is tenant-scoped carries organization or tenant identity.
- Tests prove cross-tenant reads and writes are rejected for Chat, Agent, Knowledge, Memory, MCP, Marketplace publisher data, Quota, Console, and Admin surfaces.

### 2. Relay Authority Gate

- Production mode cannot call any upstream LLM provider except through Relay.
- Every registered `/v1/*` endpoint is classified as one of:
  - commercial-supported and billed,
  - internal/admin-only,
  - disabled in production.
- Realtime, Batch, Files, Responses streaming, Images, Audio, Embeddings, Completions, Chat, Assistants, Threads, Runs, Fine-tuning, and Moderations have explicit auth, rate-limit, billing, and audit behavior.
- Unsupported or partially implemented Relay endpoints fail closed in production.
- Tests prove no service imports or invokes a provider SDK or direct provider URL outside Relay.

### 3. Billing And Monetization Gate

- Plan, quota, subscription, top-up, refund, invoice, and payment-state transitions are implemented and auditable.
- Stripe checkout and webhook handlers are mounted on running routes, signature-verified, idempotent, and covered by integration tests.
- Relay usage settles quota exactly once per idempotency key and refunds failed or partial calls.
- Marketplace publisher revenue, platform fee, payout status, and refund impact are modeled before paid Marketplace operation is enabled.
- Admins can inspect billing sessions, failed settlements, quota balances, invoices, and webhook events.

### 4. Product Completeness Gate

- Chat is production-ready with streaming, history, model configuration, quota enforcement, and error states.
- Agent orchestration uses real tools and memory, not placeholder tool outputs.
- Built-in MCP `web_search` and `calculator` either use real providers/parsers or are disabled from default commercial use.
- Knowledge supports the promised commercial behavior. If it is marketed as RAG, retrieval must use embedding-backed ingestion, indexing, and source citation; otherwise the product copy must call it text retrieval.
- Admin supports channel, route, plan, user, audit, billing, and Marketplace review operations without placeholder pages.
- Marketplace supports browse, publish, review, install, owner stats, governance, and commercial settlement boundaries.

### 5. Security Gate

- Auth includes CSRF protection for cookie-authenticated mutating requests.
- Login, registration, password reset, and sensitive admin actions are rate limited.
- Password policy and session rotation are enforced.
- Session cookies are signed, secure in production, scoped correctly, and invalidated on logout and privilege changes.
- Secrets are never committed; production examples use placeholders and runbook instructions.
- Security tests cover tenant isolation, admin boundaries, CSRF failures, rate limits, webhook signatures, and Relay cost-abuse paths.

### 6. Operations Gate

- Database migrations use an append-only migration ledger and are safe to run repeatedly.
- Backups and restores are documented and smoke-tested against PostgreSQL data.
- Docker compose remains a local/runtime validation path, but commercial deployment also has Kubernetes manifests or an equivalent production orchestration path validated in a real or local cluster.
- Structured logs, Prometheus metrics, tracing, and error tracking cover HTTP, Relay, billing, jobs, and provider failures.
- Alerts and runbooks exist for Relay outage, quota settlement failure, webhook failure, database migration failure, high provider error rate, and tenant data isolation incidents.
- Release and rollback procedures are documented and verified.

### 7. Verification Gate

- CI runs unit, contract, frontend, E2E, and DB-backed integration tests without silently skipping critical persistence coverage.
- Commercial release evidence includes exact commands, environment class, runtime smoke, migration status, and known accepted debt.
- A final completion audit maps every commercial requirement to files, tests, runtime evidence, and documentation.

## Program Decomposition

The commercial target is too large for a single implementation phase. It should be executed as a program of linked milestones.

### v04 Commercial Foundation

Purpose: establish the SaaS foundation that all later commercial work depends on.

Scope:
- Organization/tenant model.
- Membership, roles, invitations, ownership transfer, and audit.
- Tenant-scoped data contract and cross-tenant regression tests.
- Production auth hardening: CSRF, rate limits, password policy, session rotation.
- Migration ledger.
- CI guarantee that DB-backed integration tests run in the server job.
- Commercial gate documentation replacing RC-only language for future milestones.

Exit evidence:
- New migrations with `schema_migrations` ledger and organization tables.
- Updated services and handlers use tenant context intentionally.
- Tests fail on cross-tenant access across core domains.
- `bash scripts/check.sh all` and `bash scripts/test.sh all` run with DB-backed integration enabled.

### v05 Relay Billing Completeness

Purpose: make the core invariant true for every commercial Relay surface.

Scope:
- Endpoint classification for all `/v1/*` routes.
- Production fail-closed behavior for unsupported endpoints.
- Auth, billing, rate limit, quota settlement, refund, and audit coverage for supported endpoints.
- Realtime and streaming settlement model.
- File mapping persistence or production disablement.
- Provider-abuse test suite.

Exit evidence:
- Route table documents supported, internal, and disabled endpoints.
- Automated tests prove billing and rate-limit behavior per endpoint class.
- Direct-provider bypass checks run in CI.

### v06 Billing And Marketplace Operations

Purpose: complete commercial money movement and Marketplace governance.

Scope:
- Stripe checkout and webhook routes mounted in the running server.
- Subscription lifecycle, invoices, refunds, failed-payment states, plan changes, and top-ups.
- Billing admin pages and audit trail.
- Marketplace publisher settlement model, platform fee, payout state, and refund handling.
- Marketplace moderation and abuse workflows.

Exit evidence:
- Stripe webhook fixture tests cover signature verification and idempotency.
- Admin can inspect billing and settlement state.
- Marketplace paid-flow tests cover publish, approve, install, charge, refund impact, and payout accounting.

### v07 Production Operations

Purpose: make the platform operable by a production team.

Scope:
- Kubernetes or equivalent production orchestration validation.
- Backup and restore runbooks plus smoke tests.
- Structured logs, OpenTelemetry tracing, error tracking, alert rules, dashboards, and SLOs.
- Release, rollback, incident, and disaster recovery runbooks.
- Runtime smoke for restricted-network and normal network deployment paths.

Exit evidence:
- Deployment validation starts the stack, applies migrations, proves `/healthz`, and proves Relay and app paths.
- Restore test proves a backup can recreate tenant data.
- Alert/runbook checks are included in release evidence.

### v08 Product Completeness

Purpose: remove MVP and placeholder product behavior from the customer-facing platform.

Scope:
- Real MCP built-in tools or explicit commercial disablement.
- Agent workflows with durable tool runs, human approval points, memory injection, and observable execution state.
- Knowledge retrieval upgraded to the marketed promise, including embedding-backed RAG if the UI or docs claim RAG.
- Commercial Admin and Marketplace UX polish.
- Final public documentation, onboarding, pricing, and operator guides.

Exit evidence:
- No customer-facing placeholder tool output remains enabled by default.
- Product copy matches actual behavior.
- End-to-end commercial journeys pass: signup, create organization, configure provider, subscribe, chat, create agent, use knowledge, publish agent, install agent, bill usage, inspect admin dashboards, deploy, backup, restore.

## First Subproject: v04 Commercial Foundation

The first implementation plan should focus only on v04. Later milestones depend on its data model and security boundary. v04 should not attempt to complete Relay billing or Stripe payouts; it should make those later changes safe.

### v04 Requirements Draft

- **TENANT-01:** Admin can create and manage organizations as first-class tenants.
- **TENANT-02:** User can belong to multiple organizations with member, admin, and owner roles.
- **TENANT-03:** User can invite, accept, remove, and transfer organization ownership with audit events.
- **TENANT-04:** Chat, Agent, Knowledge, Memory, MCP, Quota, Console, and Marketplace publisher data are scoped by tenant.
- **TENANT-05:** Tests prove cross-tenant access is denied for representative read and write paths.
- **SEC-01:** Cookie-authenticated mutating routes require CSRF protection.
- **SEC-02:** Login, registration, and sensitive admin actions are rate limited.
- **SEC-03:** Password policy and session rotation are enforced.
- **MIGR-01:** Migration runner records applied migrations in `schema_migrations`.
- **CI-01:** CI server job runs DB-backed HTTP integration tests instead of silently skipping them.
- **DOC-01:** Commercial gate documentation defines what must be true before any future milestone can claim commercial readiness.

### v04 Boundaries

Included:
- Tenant model, security hardening, migration ledger, CI persistence proof, and commercial gate docs.

Excluded:
- Full Stripe production rollout, Marketplace payouts, all Relay endpoint billing completion, Kubernetes proof, RAG upgrade, and Agent workflow expansion. These are later commercial milestones, not v04 scope.

## Evidence Model

Each commercial milestone must produce:

- Requirement IDs in `.planning/REQUIREMENTS.md`.
- Phase plans and summaries under `.planning/phases/`.
- Automated tests tied to each requirement.
- Runtime smoke where the requirement depends on deployment, database, provider, webhook, or queue behavior.
- A verification document listing passed checks, skipped checks, and accepted residual debt.
- A completion audit that refuses commercial-complete status if any gate remains unproven.

## Design Decision

Use a program of commercial milestones rather than a single giant phase. This keeps the final objective intact while creating executable slices:

1. v04 proves the SaaS tenant/security foundation.
2. v05 proves the Relay invariant across all AI surfaces.
3. v06 proves money movement and Marketplace operations.
4. v07 proves production operations.
5. v08 proves product completeness and removes MVP behavior.

This design deliberately does not mark the thread goal complete. It defines the path to completion and the evidence required to eventually prove it.
