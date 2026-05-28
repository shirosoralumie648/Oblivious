# Disaster Recovery Runbook

This runbook closes the v07 `OPS-06` disaster recovery procedure for Oblivious. It ties Phase 22 backup/restore evidence to Phase 21 deployment validation and Phase 23 observability checks. It must be executed with placeholders or operator-owned secrets only; do not commit backup dumps, passwords, kubeconfig material, or customer data.

## Declare DR

Declare disaster recovery when one of these conditions is true:

- Primary database loss or unrecoverable corruption.
- Failed migration leaves the target environment unsafe and rollback cannot restore service.
- Region, node, cluster, or storage loss requires fresh infrastructure.
- Tenant-commercial data corruption affects organizations, quota/billing, Marketplace settlement, or audit rows.

Record incident owner, declaration time, affected environment, data-risk summary, and the backup manifest selected for restore.

## Restore Data

Restore into fresh infrastructure first:

```bash
RESTORE_DATABASE_URL=postgres://USER:REPLACE_ME@RESTORE_HOST:5432/oblivious_restore?sslmode=require \
  BACKUP_FILE=.tmp/backups/oblivious-YYYYmmdd-HHMMSS.dump \
  bash scripts/restore-postgres.sh
```

Required evidence:

- Backup manifest path and dump checksum, not dump contents.
- `schema_migrations` count and checksum verification.
- Confirmation that restore target was fresh.
- Any skipped checks and why.

For local proof, run the full smoke:

```bash
OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 \
  bash scripts/backup-restore-smoke.sh
```

The smoke must prove tenant identity, quota/billing, Marketplace, and audit fixture categories. A fixture mismatch means DR is not proven.

## Redeploy Application

For compose recovery:

```bash
OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 \
  bash scripts/deploy-validate.sh
```

For restricted-network compose recovery:

```bash
OBLIVIOUS_SERVER_HOST_PORT=18080 \
  OBLIVIOUS_WEB_HOST_PORT=14173 \
  OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

For Kubernetes recovery:

```bash
OBLIVIOUS_K8S_SECRET_FILE=/path/to/filled-secret.yaml bash scripts/k8s-validate.sh
```

Missing `kubectl`, missing cluster access, or missing untracked secret file is unavailable Kubernetes proof and must be recorded as non-success evidence.

## Post-Restore Acceptance

Run or record equivalent checks:

- `BASE_URL=<target> bash scripts/deploy-smoke.sh` for `/healthz`, `/metrics`, app route, and Relay route.
- Migration ledger check from `scripts/restore-postgres.sh`.
- Tenant identity rows: users, organizations, memberships.
- Quota and billing rows: quotas, billing sessions, payment intents, webhook events, lifecycle events, invoices, refunds.
- Marketplace rows: published agents, installs, orders, settlements, payouts, governance events, abuse reports.
- Audit rows with organization identity.
- Observability artifacts: `deploy/observability/prometheus-alerts.yaml`, `deploy/observability/grafana-dashboard.json`, and `docs/release/observability-slos.md`.
- Incident response follow-up for `MigrationFailure`, `RelayOutage`, `QuotaSettlementFailure`, `StripeWebhookFailure`, `HighProviderErrorRate`, or `TenantIsolationIncident` if any fired during recovery.

## Evidence Capture

Record:

- DR declaration reason and owner.
- Backup manifest path, restore target class, and `schema_migrations` verification.
- Deployment command and runtime smoke output.
- Alert/dashboard/SLO checks and unavailable external observability integrations.
- Communication timeline and user-impact summary.
- Rollback decision if recovered service remains unsafe.
- `no-final-readiness`: v08 Product Completeness and final commercial audit remain required.

## Exit Criteria

DR is resolved only when restored data passes migration-ledger and tenant-commercial checks, deployment smoke passes, relevant incident alerts are cleared or explained, and operators approve return to service.

This runbook closes only v07 disaster recovery procedure evidence after Phase 24 verification. It does not prove customer-facing v08 product completeness or final commercial readiness.
