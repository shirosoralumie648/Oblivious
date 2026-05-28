# Release And Rollback Runbook

This runbook closes the v07 `OPS-06` release and rollback procedure for Oblivious. It uses the Phase 21 deployment validation, Phase 22 backup/restore path, and Phase 23 observability artifacts. It is not final commercial readiness; v08 Product Completeness and the final commercial audit remain required.

## Preconditions

- Use placeholders only in committed files. Do not commit provider keys, Stripe secrets, database passwords, session secrets, kubeconfig material, or backup dumps.
- Confirm the intended release diff and accepted residual risks are documented in the Phase 24 verification record.
- Docker must be available for compose validation. Kubernetes proof additionally requires `kubectl`, a reachable cluster, and `OBLIVIOUS_K8S_SECRET_FILE` pointing at a filled secret manifest outside git.
- PostgreSQL must be pgvector-capable because migration `0016_pgvector.sql` requires the `vector` extension.
- Take a pre-release backup before running migrations or deployment validation.

## Pre-Release Backup

Create a backup manifest outside git:

```bash
BACKUP_DATABASE_URL=postgres://USER:REPLACE_ME@HOST:5432/oblivious?sslmode=require \
  BACKUP_DIR=.tmp/backups \
  bash scripts/backup-postgres.sh
```

Record the manifest path, environment class, timestamp, and SHA-256 checksum in release evidence. Do not attach dump contents.

## Normal-Network Release Validation

Run the migration-aware compose validation:

```bash
bash scripts/deploy-validate.sh
```

Go/no-go criteria:

- `docker compose config` and image build succeed.
- `/usr/local/bin/oblivious-migrate` runs before the stack is declared healthy.
- `scripts/deploy-smoke.sh` proves `/healthz`, `/metrics`, `/api/v1/auth/me`, and `/v1/chat/completions`.
- The Relay smoke returns local auth/policy handling, not `404` and not a provider-network failure.
- `bash scripts/check.sh docs` passes.

If Docker Hub or default Go module access fails, record the exact failure and use the restricted-network path below.

## Restricted-Network Release Validation

When base image pulls or Go module downloads are blocked, use the mirror and local pgvector fallback path:

```bash
docker build -f Dockerfile.postgres-pgvector -t oblivious-postgres-pgvector:pg16 .

OBLIVIOUS_SERVER_HOST_PORT=18080 \
  OBLIVIOUS_WEB_HOST_PORT=14173 \
  OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

This path is valid release evidence only when the command output shows migration execution and `scripts/deploy-smoke.sh` success.

## Kubernetes Release Validation

Copy and fill the secret manifest outside git:

```bash
cp deploy/kubernetes/secret.example.yaml /tmp/oblivious-secret.yaml
# edit /tmp/oblivious-secret.yaml outside git and replace every REPLACE_ME value
OBLIVIOUS_K8S_SECRET_FILE=/tmp/oblivious-secret.yaml bash scripts/k8s-validate.sh
```

Missing `kubectl`, missing cluster access, or using `deploy/kubernetes/secret.example.yaml` directly is non-success evidence. It must not be counted as Kubernetes proof.

## Release Evidence

For each release candidate, record:

- Exact command and environment class: normal network, restricted network, local compose, CI compose, staging Kubernetes, or production-like Kubernetes.
- Migration result and `schema_migrations` status.
- Backup manifest path, not dump contents.
- Smoke result for `/healthz`, `/metrics`, app route, and Relay route.
- Any skipped checks: Kubernetes cluster, external Prometheus, Grafana, OpenTelemetry collector, error-tracking vendor, live provider, Stripe, or payout credentials.
- Residual risk and the `no-final-readiness` boundary: v08 Product Completeness and final commercial audit remain required.

## Rollback Triggers

Start rollback when any of these occur:

- Migration failure or migration ledger checksum mismatch.
- Deployment or `scripts/deploy-smoke.sh` failure.
- `MigrationFailure`, `RelayOutage`, `HighProviderErrorRate`, `StripeWebhookFailure`, `QuotaSettlementFailure`, or `TenantIsolationIncident` fires during release validation.
- Operator identifies data corruption, tenant-isolation risk, quota/billing idempotency risk, or provider bypass risk.

## Rollback Sequence

1. Stop the rollout and freeze further migrations.
2. Preserve logs, command output, alert names, dashboard screenshots or exported panel data, and backup manifest paths.
3. Restore the previous application image and config. For compose, rerun `scripts/deploy-validate.sh` against the previous image/config set. For Kubernetes, apply the previous manifests and wait for rollouts.
4. Do not restore over production data automatically. Restore database dumps into a fresh target first with `scripts/restore-postgres.sh`, verify `schema_migrations`, then require an explicit maintenance approval before any production overwrite.
5. Run `BASE_URL=<rolled-back-url> bash scripts/deploy-smoke.sh`.
6. Run incident-specific checks from `docs/release/incident-response-runbook.md`.
7. Record rollback evidence, remaining risk, and whether disaster recovery was triggered.

## Boundary

Completing this runbook can close v07 release/rollback evidence only when Phase 24 verification records current command output. It does not complete v08 Product Completeness, customer-facing placeholder removal, final public pricing/onboarding docs, or final commercial readiness.
