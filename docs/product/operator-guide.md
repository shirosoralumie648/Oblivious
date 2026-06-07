# Oblivious Operator Guide

`PROD-05` operator guide surface.

This guide indexes the verified operational paths for running Oblivious as a commercial multi-tenant AI SaaS. It points to the current runbooks and evidence surfaces instead of replacing them.

## Runtime Prerequisites

- Go 1.22.
- Node.js 20+ and pnpm 10.6.0.
- PostgreSQL 14+ with pgvector support.
- Docker for compose validation.
- Kubernetes access only when proving the Kubernetes path.
- Secrets stored outside git.
- `RELAY_ENABLED=true` for commercial operation.

## Deploy

Use the migration-aware validation script:

```bash
bash scripts/deploy-validate.sh
```

For restricted networks, use the mirror and pgvector fallback path documented in `docs/release/release-rollback-runbook.md` and `docs/release/deployment-runtime-remediation.md`.

Kubernetes validation uses:

```bash
OBLIVIOUS_K8S_SECRET_FILE=/path/outside/git/secret.yaml bash scripts/k8s-validate.sh
```

Using `deploy/kubernetes/secret.example.yaml` directly is non-success evidence.

## Backup And Restore

Use the runbook in `docs/release/backup-restore-runbook.md`.

Automated smoke:

```bash
bash scripts/backup-restore-smoke.sh
```

Manual backup and restore:

```bash
BACKUP_DATABASE_URL=postgres://USER:REPLACE_ME@HOST:5432/oblivious?sslmode=require \
  bash scripts/backup-postgres.sh

RESTORE_DATABASE_URL=postgres://USER:REPLACE_ME@RESTORE_HOST:5432/oblivious_restore?sslmode=require \
  BACKUP_FILE=.tmp/backups/oblivious-YYYYmmdd-HHMMSS.dump \
  bash scripts/restore-postgres.sh
```

The restore path verifies `schema_migrations` filenames and SHA-256 checksums.

## Observability

Use:

- `docs/release/observability-slos.md`
- `docs/release/recovery-platform-contract.md`
- `deploy/observability/prometheus-alerts.yaml`
- `deploy/observability/grafana-dashboard.json`

The alert set covers Relay outage, quota settlement failure, Stripe webhook failure, migration failure, high provider error rate, and tenant isolation incidents.

The Grafana dashboard artifact also covers usage analytics dashboard panels for model usage, feature usage, user cost, time trend, and cross-dimension usage/cost analytics. This is repository-local dashboard coverage only; external Grafana deployment, datasource configuration, and live panel rendering remain deployment evidence.

Validate repository-owned Kubernetes recovery policy with:

```bash
bash scripts/verify-k8s-recovery-policy.sh
```

PostgreSQL Patroni or managed failover, Redis Sentinel or managed failover, Kafka leader election, load-balancer target removal/rejoin, and exact `<30%` custom autoscaler triggers are deployment-platform evidence recorded through `docs/release/recovery-platform-contract.md`.

## Release, Rollback, Incident, And Disaster Recovery

Use:

- `docs/release/release-rollback-runbook.md`
- `docs/release/incident-response-runbook.md`
- `docs/release/disaster-recovery-runbook.md`
- `docs/release/v07-operations-evidence.md`

Record exact commands, environment class, migration status, smoke output, skipped checks, and residual risk for each commercial release candidate.

## Product Operations

Operators should verify:

- Tenant and membership state before enabling customer access.
- Relay channels and routes before AI traffic.
- Subscription, top-up, quota, invoice, refund, and webhook ledger surfaces before paid access.
- Marketplace review, paid install, settlement, payout, refund-impact, takedown, appeal, and abuse workflows before public Marketplace operation.
- Knowledge RAG uses Relay embeddings and pgvector retrieval with source citations.
- Agent workflows persist `agent_runs` and `agent_tool_runs` with approval/retry evidence.

## Current Completion Boundary

Phase 29 closes only `PROD-05` operator guide documentation alignment. Phase 30 still must prove the full commercial journey across signup, organization, provider/channel setup, subscription, top-up, Chat, Agent, Knowledge, Marketplace, billing, deploy, backup, restore, and final audit mapping.

`no-final-readiness`: this operator guide is not final commercial readiness evidence by itself.
