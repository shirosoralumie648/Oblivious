# Phase 24 Verification: Release Rollback Incident DR and v07 Closeout

**Status:** Passed. Phase 24 runbook and evidence artifacts exist and pass docs gates, restricted-network/fallback release validation passes, cache-seeded default release validation now reaches migration and runtime smoke through the bare `scripts/deploy-validate.sh` command, backup/restore smoke passes, and Kubernetes prerequisite failure is explicit.

**Updated:** 2026-05-28

## Scope

Phase 24 owns `OPS-02`, `OPS-06`, and `DOC-06`.

This verification does not claim v08 Product Completeness or final commercial readiness.

## Evidence Summary

| Requirement | Result | Evidence |
| --- | --- | --- |
| `OPS-02` | Passed with explicit environment boundary | Restricted-network/fallback compose deployment passed with migration and runtime smoke. The bare default `bash scripts/deploy-validate.sh` also passed after default Docker image tags were available locally and `Dockerfile.server` was fixed so `go build` reuses the `/go/pkg/mod` cache populated by `go mod download`. Direct fresh Docker Hub layer pulls and default Go proxy downloads remain unreliable on this host, so the evidence records local-cache/default-command proof rather than claiming daemon-level registry reliability. Kubernetes proof is unavailable because `kubectl` is missing. |
| `OPS-06` | Passed | `docs/release/release-rollback-runbook.md`, `docs/release/incident-response-runbook.md`, and `docs/release/disaster-recovery-runbook.md` exist, link deployment, restore, alert, rollback, DR, and evidence commands, and are guarded by `scripts/verify-quality-gates.sh`. |
| `DOC-06` | Passed | `docs/release/v07-operations-evidence.md`, this verification file, `24-01-SUMMARY.md`, and `.planning/milestones/v07-*` map v07 evidence, skipped checks, residual v08 work, and the no-final-readiness boundary. |

## Commands Run

```bash
timeout 600 bash scripts/deploy-validate.sh
```

Result: failed with exit `124`.

Evidence: the normal-network path rendered compose config and built the web image, then stalled on Docker Hub layer downloads for `docker.io/library/golang:1.25-bookworm`. At timeout, BuildKit had only partial layer progress, for example `a4388a... 15.73MB / 92.48MB`, and the command was terminated before migration or smoke could run.

```bash
timeout 900 env OBLIVIOUS_SERVER_HOST_PORT=18080 OBLIVIOUS_WEB_HOST_PORT=14173 OBLIVIOUS_IMAGE_REGISTRY_PREFIX=docker.m.daocloud.io/library/ OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 OBLIVIOUS_GOPROXY=https://mirrors.aliyun.com/goproxy/,direct OBLIVIOUS_GOSUMDB=sum.golang.google.cn bash scripts/deploy-validate.sh
```

Result: passed.

Evidence:

- Server and web images built with mirror-prefixed base images and Go proxy overrides.
- `oblivious-migrate` ran with `migrations applied: 0, skipped: 30`.
- Server and web containers started healthy.
- Runtime smoke passed:
  - `/healthz`
  - `/metrics`
  - `/api/v1/auth/me` with status `401`
  - `/v1/chat/completions` with status `401`

```bash
env -u OBLIVIOUS_K8S_SECRET_FILE bash scripts/k8s-validate.sh
```

Result: expected fail, exit `127`.

Evidence: `[k8s-validate] kubectl is required`. This is explicit non-success evidence and is not Kubernetes proof.

```bash
timeout 900 env OBLIVIOUS_POSTGRES_IMAGE=oblivious-postgres-pgvector:pg16 bash scripts/backup-restore-smoke.sh
```

Result: passed.

Evidence:

- Disposable source and restore pgvector PostgreSQL databases started.
- Migrations applied to source: `migrations applied: 30, skipped: 0`.
- Backup dump and manifest were written under `.tmp/backups/...`.
- Restore verified `schema_migrations`: 30 checked-in migrations.
- Fixture verification passed for users, organizations, memberships, quotas, billing sessions, payment intents, Stripe webhook events, billing lifecycle events, invoices, refunds, published agents, Marketplace orders, settlements, payouts, governance events, abuse reports, and audit logs.
- Final line: `[backup-restore-smoke] backup/restore smoke ok`.

```bash
rg -n "deploy-validate|deploy-smoke|backup-postgres|restore-postgres|restricted-network|OBLIVIOUS_K8S_SECRET_FILE|rollback|v08 Product Completeness" docs/release/release-rollback-runbook.md
rg -n "RelayOutage|QuotaSettlementFailure|StripeWebhookFailure|MigrationFailure|HighProviderErrorRate|TenantIsolationIncident|rollback|disaster recovery|Relay-only|tenant isolation" docs/release/incident-response-runbook.md
rg -n "backup-restore-smoke|restore-postgres|schema_migrations|deploy-validate|deploy-smoke|tenant|quota|Marketplace|audit|no-final" docs/release/disaster-recovery-runbook.md
rg -n "OPS-01|OPS-02|OPS-03|OPS-04|OPS-05|OPS-06|DOC-06|v08 Product Completeness|no-final-readiness" docs/release/v07-operations-evidence.md
```

Result: passed. Required runbook and evidence strings are present.

```bash
bash scripts/check.sh docs && git diff --check
```

Result: passed.

Evidence: docs quality gate reports `[quality-gates] quality gate assets look complete`; diff whitespace check passes.

## Additional Normal-Network Retry

```bash
timeout 360 bash scripts/deploy-validate.sh
```

Result: failed with exit `3`.

Evidence: after prior partial downloads, BuildKit still failed on the Docker Hub Go base image with:

```text
failed to compute cache key: short read: expected 30669640 bytes but got 1379714: unexpected EOF
```

This confirms the normal-network Docker Hub path is not currently proven in this environment.

## Default-Command Fix And Runtime Smoke

```bash
timeout 120 docker pull golang:1.25-bookworm
timeout 600 docker pull golang:1.25-bookworm
```

Result: failed with exit `124`.

Evidence: Docker CLI could reach Docker Hub registry metadata, but daemon-level large layer pulls still did not complete within bounded retries. `sudo -n true` failed with `sudo: a password is required`, so this session could not configure Docker daemon proxy settings. `docker info` showed no daemon proxy entries.

```bash
timeout 240 env GOBIN="$PWD/.tmp/bin" GOPROXY=https://mirrors.aliyun.com/goproxy/,direct GOSUMDB=sum.golang.google.cn go install github.com/google/go-containerregistry/cmd/crane@latest
timeout 240 env GOBIN="$PWD/.tmp/bin" GOPROXY=https://proxy.golang.org,direct GOSUMDB=sum.golang.org go install github.com/google/go-containerregistry/cmd/crane@latest
```

Result: failed.

Evidence: the mirror attempt was rejected by Go checksum verification for `github.com/google/go-containerregistry@v0.21.6`; the default proxy attempt reached downloads but failed with checksum-server EOF/timeout. Checksum verification was not disabled.

```bash
docker tag docker.m.daocloud.io/library/golang:1.25-bookworm golang:1.25-bookworm
docker tag docker.m.daocloud.io/library/alpine:3.21 alpine:3.21
docker tag docker.m.daocloud.io/library/node:20-bookworm-slim node:20-bookworm-slim
docker tag docker.m.daocloud.io/library/nginx:1.27-alpine nginx:1.27-alpine
docker tag oblivious-postgres-pgvector:pg16 pgvector/pgvector:pg16
```

Result: passed.

Evidence: default compose image tags were available locally. This is recorded as local-cache setup, not as fresh Docker Hub pull proof.

```bash
bash scripts/check.sh docs
```

Result: failed before the Dockerfile fix with:

```text
[quality-gates] expected at least 3 occurrences of 'target=/go/pkg/mod' in .../Dockerfile.server; found 1
```

Root cause: `Dockerfile.server` mounted `/go/pkg/mod` for `go mod download`, but did not mount the same module cache for the two `go build` steps. The bare default build therefore redownloaded modules from `https://proxy.golang.org` during `go build` and failed with `unexpected EOF`.

Fix:

- Added a quality-gate assertion that `Dockerfile.server` keeps `/go/pkg/mod` mounted for the module download and both build steps.
- Added `--mount=type=cache,target=/go/pkg/mod` to both `go build` steps in `Dockerfile.server`.

```bash
bash scripts/check.sh docs
```

Result: passed after the Dockerfile fix.

```bash
timeout 900 bash scripts/deploy-validate.sh
```

Result: passed.

Evidence:

- Command used no `OBLIVIOUS_*` overrides.
- Compose resolved default image names such as `docker.io/library/golang:1.25-bookworm`, `docker.io/library/node:20-bookworm-slim`, `docker.io/library/nginx:1.27-alpine`, and `pgvector/pgvector:pg16` from local default tags.
- `go mod download` was cached, and both `go build` steps reused `/go/pkg/mod`; server build completed without redownloading modules from the default Go proxy.
- `oblivious-migrate` ran with `migrations applied: 0, skipped: 30`.
- Runtime smoke passed:
  - `/healthz`
  - `/metrics`
  - `/api/v1/auth/me` with status `401`
  - `/v1/chat/completions` with status `401`
  - final line: `[deploy-validate] deployment validation ok`

## Skipped Or Unavailable Checks

- Kubernetes proof: unavailable because `kubectl` is not installed.
- External Prometheus server rule evaluation: not run.
- Grafana import/live dashboard rendering: not run.
- OpenTelemetry collector/exporter delivery: not run.
- External error-tracking vendor delivery: not run.
- Live LLM provider, Stripe, and payout-provider credentials: not used.

## Current Gap

No v07 repository-local gap remains. Fresh Docker Hub daemon pulls and external Kubernetes/observability vendor checks remain environment capabilities, not repository proof, and are recorded above as skipped or unavailable.

## Next Action

Plan v08 Product Completeness: built-in MCP tool behavior, durable Agent workflows, Knowledge behavior/product-copy alignment, commercial UX, public docs, onboarding, pricing, operator guides, and final commercial journey evidence.

## Boundary

v07 Production Operations is complete. The full commercial-complete objective remains open until v08 Product Completeness and final commercial audit pass.
