# Oblivious

Oblivious is a multi-tenant AI SaaS platform with a Go backend, a React frontend, and PostgreSQL as the system of record. It integrates LobeHub-style C-end Chat and Agent experience with New-API-style B-end channel, billing, Relay, Admin, and Marketplace operations.

The core commercial invariant is Relay: all AI calls must go through Relay so billing, rate limiting, auditing, and monitoring stay unified across Chat, Agent workflows, Knowledge RAG, and supported `/v1/*` endpoints.

## Mainline Boundary

The current mainline covers:

- `src/server`
- `src/web`
- `config`
- `scripts`
- `.github/workflows`

`lobehub` and `new-api` remain in the repository as reference directories only. They are not part of the root workspace, root CI, or release scope.

## Product Surfaces

- Chat workspace with model configuration, Knowledge binding, SOLO handoff, quota-aware errors, and Relay-backed provider access.
- Agent and SOLO workflows with durable `agent_runs`, `agent_tool_runs`, approval/reject/retry state, memory evidence, tool boundaries, and budget context.
- Knowledge with Relay embeddings, pgvector retrieval, `embedding_rag` metadata, and source citations.
- MCP built-ins where `calculator` and `datetime` are real default commercial tools; `web_search` and `http_request` are disabled by default until real provider or tenant-safe outbound policy exists.
- Admin operations for channels, routes, plans, billing inspection, users, audit logs, and Marketplace review queues.
- Marketplace browse, publish, review, install, owner stats, governance, paid-install order handling, settlement, payout, and refund-impact evidence.
- Production operations evidence for compose validation, Kubernetes manifest validation path, backup/restore, observability, release/rollback, incident response, and disaster recovery.

## Prerequisites

- Go 1.22
- Node.js 20+
- pnpm 10.6.0
- PostgreSQL 14+

## Quick Start

1. Install workspace dependencies.

   ```bash
   pnpm install --frozen-lockfile
   ```

2. Export runtime environment variables from [`config/.env.example`](config/.env.example).

3. Apply database migrations.

   ```bash
   cd src/server
   go run ./cmd/migrate
   ```

4. Start the web app and API.

   ```bash
   bash scripts/dev.sh
   ```

## Quality Gates

Run the same top-level commands used by CI before pushing changes:

```bash
bash scripts/check.sh
bash scripts/test.sh
```

`bash scripts/check.sh` verifies release assets, docs and environment consistency, the web production build, and the server unit/contract packages.

`bash scripts/test.sh` runs the web Vitest suite, the server unit packages, and the HTTP integration package. Local runs skip DB-backed integration tests explicitly when `TEST_DATABASE_URL` is unset. CI sets `OBLIVIOUS_REQUIRE_TEST_DATABASE=true` with `TEST_DATABASE_URL`, so server integration coverage must run instead of silently skipping.

Commercial-readiness gates are defined in [`docs/release/commercial-gates.md`](docs/release/commercial-gates.md). The current routed HTTP surface is indexed in [`docs/API.md`](docs/API.md). Historical release-candidate evidence remains in [`docs/release/rc-checklist.md`](docs/release/rc-checklist.md).

## Commercial Documentation

- [`docs/product/public-overview.md`](docs/product/public-overview.md): public product overview for Chat, Agent, Knowledge RAG, Relay, Admin, Marketplace, billing, and operations
- [`docs/product/onboarding.md`](docs/product/onboarding.md): customer, admin, publisher, and operator onboarding paths
- [`docs/product/pricing.md`](docs/product/pricing.md): subscription, top-up, quota, invoice, refund, and Marketplace settlement model
- [`docs/product/operator-guide.md`](docs/product/operator-guide.md): deploy, backup, restore, observability, release, rollback, incident, and disaster recovery index
- [`docs/API.md`](docs/API.md): current routed HTTP API index
- [`docs/architecture/current-system-contracts.md`](docs/architecture/current-system-contracts.md): current API and runtime contract baseline
- [`docs/release/commercial-gates.md`](docs/release/commercial-gates.md): commercial-readiness gate contract
- [`docs/release/release-rollback-runbook.md`](docs/release/release-rollback-runbook.md): release and rollback procedure
- [`docs/release/backup-restore-runbook.md`](docs/release/backup-restore-runbook.md): PostgreSQL backup and restore procedure
- [`docs/release/observability-slos.md`](docs/release/observability-slos.md): observability, alert, dashboard, and SLO contract
- [`docs/release/incident-response-runbook.md`](docs/release/incident-response-runbook.md): incident response procedure
- [`docs/release/disaster-recovery-runbook.md`](docs/release/disaster-recovery-runbook.md): disaster recovery procedure

## Completion Boundary

Phase 29 closes only `PROD-05`: public docs, onboarding, pricing, and operator guide alignment. Phase 30 still must prove end-to-end commercial journeys and `AUDIT-01`.

`no-final-readiness`: do not claim final commercial readiness until Phase 30 maps every commercial gate to current repository evidence, automated verification, and runtime smoke where applicable.

## Repository Layout

- [`src/server`](src/server): Go API, migrations, and domain services
- [`src/web`](src/web): React workspace and console UI
- [`docs/API.md`](docs/API.md): current routed HTTP API index
- [`docs/architecture/current-system-contracts.md`](docs/architecture/current-system-contracts.md): current API and runtime contract baseline
- [`docs/release/commercial-gates.md`](docs/release/commercial-gates.md): commercial-readiness gate contract
- [`docs/product`](docs/product): public overview, onboarding, pricing, and operator guide
- [`docs/release/rc-checklist.md`](docs/release/rc-checklist.md): historical RC readiness checklist
- `lobehub/`: repository-local reference code, excluded from mainline workspace and CI
- `new-api/`: repository-local reference code, excluded from mainline workspace and CI
