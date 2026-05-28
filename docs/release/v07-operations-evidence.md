# v07 Operations Evidence

This document maps v07 Production Operations requirements to current repository evidence. It supports `DOC-06` and the Phase 24 closeout. v07 completion is still not final commercial readiness: v08 Product Completeness and the final commercial audit remain required.

## Evidence Map

| Requirement | Current status | Evidence |
| --- | --- | --- |
| `OPS-01` | Complete in Phase 21 | `scripts/deploy-validate.sh`, `scripts/deploy-smoke.sh`, `scripts/k8s-validate.sh`, `Dockerfile.postgres-pgvector`, `.planning/phases/21-production-orchestration-runtime-proof/21-VERIFICATION.md` |
| `OPS-02` | Complete with explicit environment boundary | Restricted-network/fallback deploy validation passed in Phase 24. The bare default `bash scripts/deploy-validate.sh` also passed after default image tags were available locally and `Dockerfile.server` was fixed to reuse the `/go/pkg/mod` cache during `go build`. Fresh Docker Hub daemon pulls remain unreliable on this host and are recorded as environment evidence rather than hidden. Kubernetes missing-tool evidence is explicit but not proof. |
| `OPS-03` | Complete in Phase 22 | `scripts/backup-postgres.sh`, `scripts/restore-postgres.sh`, `scripts/backup-restore-smoke.sh`, `docs/release/backup-restore-runbook.md`, `.planning/phases/22-backup-restore-and-migration-recovery/22-VERIFICATION.md` |
| `OPS-04` | Complete in Phase 23 | `src/server/internal/observability`, `src/server/internal/metrics/prometheus.go`, HTTP/Relay/billing/job/migration instrumentation, `.planning/phases/23-observability-alerts-dashboards-and-slos/23-VERIFICATION.md` |
| `OPS-05` | Complete in Phase 23 | `deploy/observability/prometheus-alerts.yaml`, `deploy/observability/grafana-dashboard.json`, `docs/release/observability-slos.md` |
| `OPS-06` | Complete | `docs/release/release-rollback-runbook.md`, `docs/release/incident-response-runbook.md`, `docs/release/disaster-recovery-runbook.md`, and `scripts/verify-quality-gates.sh` passed docs checks. |
| `DOC-06` | Complete | This file, `.planning/phases/24-release-rollback-incident-dr-and-v07-closeout/24-VERIFICATION.md`, `24-01-SUMMARY.md`, and `.planning/milestones/v07-*` map v07 evidence, skipped checks, and residual v08 work. |

## Phase 24 Verification Commands

The Phase 24 verification record must include:

```bash
bash scripts/deploy-validate.sh
```

```bash
OBLIVIOUS_SERVER_HOST_PORT=18080 \
  OBLIVIOUS_WEB_HOST_PORT=14173 \
  OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ \
  OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 \
  OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct \
  OBLIVIOUS_GOSUMDB=sum.golang.google.cn \
  bash scripts/deploy-validate.sh
```

```bash
env -u OBLIVIOUS_K8S_SECRET_FILE bash scripts/k8s-validate.sh
```

```bash
OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 \
  bash scripts/backup-restore-smoke.sh
```

```bash
bash scripts/check.sh docs
git diff --check
```

## Phase 24 Current Results

| Command | Result | Notes |
| --- | --- | --- |
| `timeout 600 bash scripts/deploy-validate.sh` | Failed, exit 124 | Docker Hub Go base image layers did not finish downloading before timeout; migration/smoke did not run. |
| `timeout 360 bash scripts/deploy-validate.sh` | Failed, exit 3 | Retry failed with BuildKit `short read` / `unexpected EOF` on the Docker Hub Go base image. |
| `timeout 120 docker pull golang:1.25-bookworm` and `timeout 600 docker pull golang:1.25-bookworm` | Failed, exit 124 | Docker daemon still could not complete a fresh large Docker Hub pull in bounded retries. |
| Default-image local tag setup | Passed | Existing mirror/local fallback images were tagged as `golang:1.25-bookworm`, `alpine:3.21`, `node:20-bookworm-slim`, `nginx:1.27-alpine`, and `pgvector/pgvector:pg16`; this is local-cache setup, not fresh Docker Hub pull proof. |
| `bash scripts/check.sh docs` before Dockerfile fix | Failed | Quality gate caught that `Dockerfile.server` had only one `/go/pkg/mod` cache mount. |
| `timeout 900 bash scripts/deploy-validate.sh` after Dockerfile fix | Passed | Bare default command used no `OBLIVIOUS_*` overrides, built server/web, ran migrations with 30 skipped, and passed `/healthz`, `/metrics`, app route, and Relay route smoke. |
| Restricted/fallback `scripts/deploy-validate.sh` with `OBLIVIOUS_IMAGE_REGISTRY_PREFIX`, `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16`, `OBLIVIOUS_GOPROXY`, and alternate ports | Passed | Migrations ran with 30 skipped, server/web started, `/healthz`, `/metrics`, app route, and Relay route smoke passed. |
| `env -u OBLIVIOUS_K8S_SECRET_FILE bash scripts/k8s-validate.sh` | Expected fail, exit 127 | `[k8s-validate] kubectl is required`; not Kubernetes proof. |
| `OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh` | Passed | 30 migration ledger checks and all commercial tenant fixture categories passed. |
| `bash scripts/check.sh docs && git diff --check` | Passed | Docs quality gates and whitespace checks passed. |

## Skipped Or Unavailable Runtime Integrations

The following must be recorded as skipped or unavailable when the environment lacks them:

- Kubernetes cluster proof when `kubectl`, a reachable cluster, or an untracked filled secret manifest is unavailable.
- External Prometheus server evaluation.
- Grafana import or live dashboard rendering.
- OpenTelemetry collector/exporter verification.
- External error-tracking vendor delivery.
- Live LLM provider, Stripe, or payout-provider credentials.

Skipped checks are not successful proof. They are allowed only when the related requirement is repository-local or when v07 explicitly records the residual risk.

## no-final-readiness Boundary

v07 Production Operations is complete, but the commercial-complete objective remains open until v08 Product Completeness and final commercial audit pass.

Remaining v08 work includes:

- Real or disabled built-in MCP tools.
- Durable Agent workflows with human approval points, memory injection, and observable execution state.
- Knowledge behavior matching product copy, including embedding-backed RAG if marketed as RAG.
- Commercial Admin and Marketplace UX polish.
- Public docs, onboarding, pricing, and operator guides beyond v07 operations.
- End-to-end commercial journeys across signup, organization creation, provider configuration, subscription, Chat, Agent, Knowledge, Marketplace publish/install, billing inspection, deploy, backup, and restore.
